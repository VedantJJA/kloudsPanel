package tcpproxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// IPRule defines an allowed IP address or CIDR block for public database access.
type IPRule struct {
	CIDR        string    `json:"cidr"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ProxyStats holds runtime metrics for a TCP database proxy instance.
type ProxyStats struct {
	DatabaseID        string    `json:"databaseId"`
	PublicPort        int       `json:"publicPort"`
	TargetAddr        string    `json:"targetAddr"`
	PublicAccess      bool      `json:"publicAccess"`
	ActiveConnections int64     `json:"activeConnections"`
	TotalConnections  int64     `json:"totalConnections"`
	BlockedAttempts   int64     `json:"blockedAttempts"`
	LastActivity      time.Time `json:"lastActivity,omitempty"`
}

// DatabaseTCPProxy represents an active TCP forwarding listener for a database instance.
type DatabaseTCPProxy struct {
	dbID         string
	publicPort   int
	targetAddr   string
	listener     net.Listener
	publicAccess bool
	ipRules      []IPRule
	logger       *slog.Logger

	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	activeConnections int64
	totalConnections  int64
	blockedAttempts   int64
	lastActivity      time.Time
}

// IsIPAllowed evaluates whether a client IP is authorized under the given IP whitelist rules.
func IsIPAllowed(clientIP net.IP, rules []IPRule) bool {
	if clientIP == nil {
		return false
	}

	// Always allow local loopback traffic
	if clientIP.IsLoopback() {
		return true
	}

	// If no whitelist rules are configured, default to allow all (standard initial behavior)
	if len(rules) == 0 {
		return true
	}

	for _, rule := range rules {
		trimmed := strings.TrimSpace(rule.CIDR)
		if trimmed == "" || trimmed == "*" || trimmed == "0.0.0.0/0" || trimmed == "::/0" {
			return true
		}

		// Handle CIDR blocks
		if strings.Contains(trimmed, "/") {
			_, ipNet, err := net.ParseCIDR(trimmed)
			if err == nil && ipNet.Contains(clientIP) {
				return true
			}
			continue
		}

		// Handle exact single IP
		parsed := net.ParseIP(trimmed)
		if parsed != nil && parsed.Equal(clientIP) {
			return true
		}
	}

	return false
}

// ExtractIP extracts the IP address from a remote network address string.
func ExtractIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	// Strip IPv6 brackets if present
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return net.ParseIP(host)
}

// Manager coordinates all active database TCP forwarders on the platform.
type Manager struct {
	logger  *slog.Logger
	mu      sync.RWMutex
	proxies map[string]*DatabaseTCPProxy
}

var (
	defaultManagerInstance *Manager
	defaultManagerOnce     sync.Once
)

// DefaultManager returns the global TCP proxy manager instance.
func DefaultManager() *Manager {
	defaultManagerOnce.Do(func() {
		defaultManagerInstance = NewManager(slog.Default())
	})
	return defaultManagerInstance
}

// NewManager creates a new TCP Proxy Manager instance.
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		logger:  logger,
		proxies: make(map[string]*DatabaseTCPProxy),
	}
}

// EnsureProxy creates or updates a TCP proxy for the specified database instance.
func (m *Manager) EnsureProxy(dbID string, publicPort int, targetAddr string, ipWhitelist []IPRule, publicAccess bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.proxies[dbID]
	if exists {
		// If port or target changed, restart proxy listener
		if existing.publicPort != publicPort || existing.targetAddr != targetAddr {
			existing.Stop()
			delete(m.proxies, dbID)
		} else {
			// Update rules live without dropping listener
			existing.UpdateRules(ipWhitelist, publicAccess)
			return nil
		}
	}

	if publicPort <= 0 {
		return fmt.Errorf("invalid public port: %d", publicPort)
	}

	ctx, cancel := context.WithCancel(context.Background())
	proxy := &DatabaseTCPProxy{
		dbID:         dbID,
		publicPort:   publicPort,
		targetAddr:   targetAddr,
		publicAccess: publicAccess,
		ipRules:      ipWhitelist,
		logger:       m.logger,
		ctx:          ctx,
		cancel:       cancel,
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%d", publicPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to bind TCP proxy on %s: %w", listenAddr, err)
	}
	proxy.listener = listener

	m.proxies[dbID] = proxy
	m.logger.Info("[tcpproxy] Started TCP forwarding proxy",
		"databaseId", dbID,
		"publicPort", publicPort,
		"targetAddr", targetAddr,
		"publicAccess", publicAccess,
		"whitelistRules", len(ipWhitelist),
	)

	go proxy.serve()
	return nil
}

// StopProxy stops the TCP forwarding proxy for a database instance.
func (m *Manager) StopProxy(dbID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if proxy, exists := m.proxies[dbID]; exists {
		proxy.Stop()
		delete(m.proxies, dbID)
		m.logger.Info("[tcpproxy] Stopped TCP forwarding proxy", "databaseId", dbID)
	}
}

// UpdateRules updates the IP whitelist and public access setting in real-time.
func (p *DatabaseTCPProxy) UpdateRules(rules []IPRule, publicAccess bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ipRules = rules
	p.publicAccess = publicAccess
	p.logger.Info("[tcpproxy] Updated live IP whitelist rules",
		"databaseId", p.dbID,
		"publicAccess", publicAccess,
		"rulesCount", len(rules),
	)
}

// Stop terminates the TCP listener and active proxy connections.
func (p *DatabaseTCPProxy) Stop() {
	p.cancel()
	if p.listener != nil {
		_ = p.listener.Close()
	}
}

// serve runs the TCP connection accept loop.
func (p *DatabaseTCPProxy) serve() {
	for {
		clientConn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return
			default:
				// If temporary error, brief backoff
				time.Sleep(20 * time.Millisecond)
				continue
			}
		}

		go p.handleConnection(clientConn)
	}
}

// handleConnection filters client IP against whitelist rules and forwards TCP traffic.
func (p *DatabaseTCPProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	clientAddr := clientConn.RemoteAddr().String()
	clientIP := ExtractIP(clientAddr)

	p.mu.RLock()
	publicAccess := p.publicAccess
	rules := make([]IPRule, len(p.ipRules))
	copy(rules, p.ipRules)
	targetAddr := p.targetAddr
	p.mu.RUnlock()

	// 1. Access validation
	if !publicAccess {
		atomic.AddInt64(&p.blockedAttempts, 1)
		p.logger.Warn("[tcpproxy] Blocked connection: public access disabled",
			"databaseId", p.dbID,
			"clientAddr", clientAddr,
		)
		return
	}

	if !IsIPAllowed(clientIP, rules) {
		atomic.AddInt64(&p.blockedAttempts, 1)
		p.logger.Warn("[tcpproxy] Blocked connection: client IP not in whitelist",
			"databaseId", p.dbID,
			"clientIP", clientIP.String(),
			"clientAddr", clientAddr,
		)
		return
	}

	// 2. Dial target backend database container
	targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		p.logger.Error("[tcpproxy] Failed to dial database backend",
			"databaseId", p.dbID,
			"targetAddr", targetAddr,
			"error", err.Error(),
		)
		return
	}
	defer targetConn.Close()

	atomic.AddInt64(&p.totalConnections, 1)
	atomic.AddInt64(&p.activeConnections, 1)
	defer atomic.AddInt64(&p.activeConnections, -1)

	p.mu.Lock()
	p.lastActivity = time.Now().UTC()
	p.mu.Unlock()

	// 3. Bidirectional TCP stream piping
	errCh := make(chan error, 2)
	go func() {
		_, errCopy := io.Copy(targetConn, clientConn)
		errCh <- errCopy
	}()
	go func() {
		_, errCopy := io.Copy(clientConn, targetConn)
		errCh <- errCopy
	}()

	// Wait for first stream to terminate
	<-errCh
}

// GetStats returns current metrics for a specific database proxy.
func (m *Manager) GetStats(dbID string) (ProxyStats, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.proxies[dbID]
	if !exists {
		return ProxyStats{}, false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProxyStats{
		DatabaseID:        p.dbID,
		PublicPort:        p.publicPort,
		TargetAddr:        p.targetAddr,
		PublicAccess:      p.publicAccess,
		ActiveConnections: atomic.LoadInt64(&p.activeConnections),
		TotalConnections:  atomic.LoadInt64(&p.totalConnections),
		BlockedAttempts:   atomic.LoadInt64(&p.blockedAttempts),
		LastActivity:      p.lastActivity,
	}, true
}

// Shutdown closes all active TCP proxies.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for dbID, proxy := range m.proxies {
		proxy.Stop()
		delete(m.proxies, dbID)
	}
}
