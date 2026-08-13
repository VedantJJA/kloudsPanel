// Package app wires the agent dependencies and starts the RPC server.
package app

import (
	"context"
	"log/slog"
	"os"
)

// Config holds agent configuration from environment.
type Config struct {
	DockerHost      string
	SocketPath      string // Unix socket for API communication
	MinDockerAPI    string // minimum Docker API version
	Architecture    string // amd64 or arm64
}

func configFromEnv() Config {
	return Config{
		DockerHost:   envOr("DOCKER_HOST", "unix:///var/run/docker.sock"),
		SocketPath:   envOr("AGENT_SOCKET", "/run/klouds/agent.sock"),
		MinDockerAPI: envOr("MIN_DOCKER_API", "1.40"),
		Architecture: detectArch(),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func detectArch() string {
	// runtime.GOARCH gives the compiled architecture
	return os.Getenv("GOARCH") // overridable for testing
}

// Run starts the agent. It blocks until ctx is done.
func Run(ctx context.Context, logger *slog.Logger) error {
	cfg := configFromEnv()
	logger.Info("agent starting",
		"docker_host", cfg.DockerHost,
		"socket", cfg.SocketPath,
		"arch", cfg.Architecture,
	)

	// TODO Phase 4: connect Docker client with API version negotiation
	// TODO Phase 4: start reconciliation loop
	// TODO Phase 4: start metrics collector
	// TODO Phase 4: start RPC server on cfg.SocketPath

	<-ctx.Done()
	logger.Info("agent shutting down")
	return nil
}
