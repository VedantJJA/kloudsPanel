// Package docker provides a version-negotiated Docker Engine API client.
// It wraps the Docker SDK with platform-specific safety and validation.
package docker

import (
	"context"
	"fmt"
	"log/slog"
)

const MinAPIVersion = "1.40"

// Client is the agent's Docker Engine client with version negotiation.
type Client struct {
	logger *slog.Logger
	host   string
	// TODO Phase 4: embed github.com/docker/docker/client.Client
}

// NewClient creates a new Docker client and verifies API version compatibility.
func NewClient(ctx context.Context, host string, logger *slog.Logger) (*Client, error) {
	c := &Client{
		logger: logger,
		host:   host,
	}
	// TODO Phase 4: initialize docker/docker client with WithAPIVersionNegotiation
	// and verify minimum API version MinAPIVersion
	logger.Info("docker client initialized (stub)", "host", host)
	return c, nil
}

// PlatformLabels returns the required labels for a platform-managed resource.
func PlatformLabels(workspaceID, projectID, serviceID, deploymentID string) map[string]string {
	return map[string]string{
		"io.paas.managed":    "true",
		"io.paas.workspace":  workspaceID,
		"io.paas.project":    projectID,
		"io.paas.service":    serviceID,
		"io.paas.deployment": deploymentID,
	}
}

// IsPlatformManaged checks whether a container/network/volume has the required
// platform label, preventing the agent from managing unknown resources.
func IsPlatformManaged(labels map[string]string) bool {
	return labels["io.paas.managed"] == "true"
}

// NetworkName returns the deterministic per-project Docker network name.
func NetworkName(projectID string) string {
	return fmt.Sprintf("paas-prj-%s", projectID)
}

// VolumeName returns the deterministic volume name for a database.
func VolumeName(databaseID string) string {
	return fmt.Sprintf("paas-db-%s-data", databaseID)
}

// ContainerName returns the deterministic container name for a deployment.
func ContainerName(serviceID, deploymentID string) string {
	return fmt.Sprintf("paas-svc-%s-%s", serviceID[:8], deploymentID[:8])
}
