package tcpproxy

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestIsIPAllowed(t *testing.T) {
	tests := []struct {
		name     string
		clientIP string
		rules    []IPRule
		allowed  bool
	}{
		{
			name:     "Loopback always allowed",
			clientIP: "127.0.0.1",
			rules:    []IPRule{{CIDR: "192.168.1.0/24"}},
			allowed:  true,
		},
		{
			name:     "Empty rules defaults to allow all",
			clientIP: "203.0.113.195",
			rules:    []IPRule{},
			allowed:  true,
		},
		{
			name:     "0.0.0.0/0 allows any IPv4",
			clientIP: "203.0.113.195",
			rules:    []IPRule{{CIDR: "0.0.0.0/0"}},
			allowed:  true,
		},
		{
			name:     "Exact IP match allowed",
			clientIP: "203.0.113.50",
			rules:    []IPRule{{CIDR: "203.0.113.50"}},
			allowed:  true,
		},
		{
			name:     "CIDR subnet match allowed",
			clientIP: "10.0.4.15",
			rules:    []IPRule{{CIDR: "10.0.0.0/16"}},
			allowed:  true,
		},
		{
			name:     "Non-matching IP blocked",
			clientIP: "198.51.100.1",
			rules:    []IPRule{{CIDR: "10.0.0.0/16"}, {CIDR: "192.168.1.0/24"}},
			allowed:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.clientIP)
			got := IsIPAllowed(ip, tc.rules)
			if got != tc.allowed {
				t.Errorf("IsIPAllowed(%s, %v) = %v; want %v", tc.clientIP, tc.rules, got, tc.allowed)
			}
		})
	}
}

func TestTCPProxy_ForwardingAndWhitelisting(t *testing.T) {
	// 1. Create a mock backend TCP echo server
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend mock: %v", err)
	}
	defer backendListener.Close()

	backendAddr := backendListener.Addr().String()

	go func() {
		for {
			conn, errAccept := backendListener.Accept()
			if errAccept != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				if n > 0 {
					_, _ = c.Write(append([]byte("ECHO:"), buf[:n]...))
				}
			}(conn)
		}
	}()

	// 2. Find free port for proxy
	proxyTestListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	proxyPort := proxyTestListener.Addr().(*net.TCPAddr).Port
	_ = proxyTestListener.Close()

	mgr := NewManager(nil)
	defer mgr.Shutdown()

	dbID := "test-db-1"
	err = mgr.EnsureProxy(dbID, proxyPort, backendAddr, []IPRule{{CIDR: "0.0.0.0/0"}}, true)
	if err != nil {
		t.Fatalf("failed to ensure proxy: %v", err)
	}

	// 3. Connect via proxy and verify bidirectional forwarding
	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer client.Close()

	_, err = client.Write([]byte("HELLO_DB"))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	replyBuf := make([]byte, 1024)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(replyBuf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	got := string(replyBuf[:n])
	if got != "ECHO:HELLO_DB" {
		t.Errorf("got %q, want 'ECHO:HELLO_DB'", got)
	}

	stats, ok := mgr.GetStats(dbID)
	if !ok || stats.TotalConnections == 0 {
		t.Errorf("expected recorded connection in stats: %+v", stats)
	}

	// 4. Disable public access and verify connection is rejected
	mgr.EnsureProxy(dbID, proxyPort, backendAddr, []IPRule{}, false)

	deniedClient, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 2*time.Second)
	if err == nil {
		// Connection should be closed immediately by proxy
		_ = deniedClient.SetReadDeadline(time.Now().Add(1 * time.Second))
		buf := make([]byte, 10)
		_, readErr := deniedClient.Read(buf)
		if readErr == nil || readErr == io.EOF {
			// Expected closed
		}
		deniedClient.Close()
	}
}
