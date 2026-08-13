// Package agentclient provides a typed RPC client for the node agent.
// In v1, it communicates over a Unix socket. Future versions add mTLS.
package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Client is a typed client for the agent's RPC interface.
type Client struct {
	socketPath string
	http       *http.Client
}

// NewClient creates a new agent client connected to socketPath.
func NewClient(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{
		socketPath: socketPath,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// baseURL is the fake HTTP base (dialing is done via Unix socket).
const baseURL = "http://agent"

// ─── Request/Response DTOs ────────────────────────────────────────────────────

type DeployRequest struct {
	ServiceID      string            `json:"service_id"`
	DeploymentID   string            `json:"deployment_id"`
	WorkspaceID    string            `json:"workspace_id"`
	ProjectID      string            `json:"project_id"`
	ImageRef       string            `json:"image_ref"`
	ImageDigest    string            `json:"image_digest"`
	InternalPort   int               `json:"internal_port"`
	Env            map[string]string `json:"env"`
	ResourceJSON   string            `json:"resource_json"`
	Labels         map[string]string `json:"labels"`
	SablierEnabled bool              `json:"sablier_enabled"`
}

type DeployResponse struct {
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
}

type StopRequest struct {
	ContainerID string `json:"container_id"`
}

type HostMetricsResponse struct {
	CPUPercent        float64 `json:"cpu_percent"`
	Load1             float64 `json:"load1"`
	MemoryTotalBytes  int64   `json:"memory_total_bytes"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	StorageTotalBytes int64   `json:"storage_total_bytes"`
	StorageUsedBytes  int64   `json:"storage_used_bytes"`
}

// ─── RPC Methods ──────────────────────────────────────────────────────────────

// Deploy instructs the agent to create and start a container.
func (c *Client) Deploy(ctx context.Context, req DeployRequest) (DeployResponse, error) {
	var resp DeployResponse
	err := c.post(ctx, "/deploy", req, &resp)
	return resp, err
}

// Stop instructs the agent to stop a container.
func (c *Client) Stop(ctx context.Context, req StopRequest) error {
	return c.post(ctx, "/stop", req, nil)
}

// GetHostMetrics retrieves the current host metrics snapshot.
func (c *Client) GetHostMetrics(ctx context.Context) (HostMetricsResponse, error) {
	var resp HostMetricsResponse
	err := c.get(ctx, "/metrics/host", &resp)
	return resp, err
}

// ─── HTTP transport helpers ───────────────────────────────────────────────────

func (c *Client) post(ctx context.Context, path string, body, resp any) error {
	// TODO Phase 4: implement full HTTP over Unix socket
	_ = ctx
	_ = body
	if resp != nil {
		b, _ := json.Marshal(map[string]string{"status": "stub"})
		_ = json.Unmarshal(b, resp)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, resp any) error {
	// TODO Phase 4: implement full HTTP over Unix socket
	return fmt.Errorf("agent client get %s: not yet implemented", path)
}
