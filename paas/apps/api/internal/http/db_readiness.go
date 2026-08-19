package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
)

// CommandRunner defines the abstraction for executing container commands.
// Allows unit testing without requiring a live Docker daemon.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type defaultCommandRunner struct{}

func (d *defaultCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

var globalCommandRunner CommandRunner = &defaultCommandRunner{}

// buildReadinessProbeArgs builds the protocol-specific Docker CLI command to check database readiness.
func buildReadinessProbeArgs(db *domain.Database) []string {
	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}

	var meta struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if db.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
	}

	engine := domain.CanonicalizeEngine(string(db.Engine))
	switch engine {
	case "postgres":
		user := meta.Username
		if user == "" {
			user = "postgres"
		}
		return []string{"exec", containerName, "pg_isready", "-U", user}
	case "mysql":
		user := meta.Username
		if user == "" {
			user = "root"
		}
		args := []string{"exec", containerName, "mysqladmin", "ping", fmt.Sprintf("-u%s", user)}
		if meta.Password != "" {
			args = append(args, fmt.Sprintf("-p%s", meta.Password))
		}
		return args
	case "redis":
		if meta.Password != "" {
			return []string{"exec", containerName, "redis-cli", "-a", meta.Password, "ping"}
		}
		return []string{"exec", containerName, "redis-cli", "ping"}
	case "mongodb":
		return []string{"exec", containerName, "mongosh", "--eval", "db.adminCommand('ping')"}
	case "clickhouse":
		return []string{"exec", containerName, "clickhouse-client", "--query", "SELECT 1"}
	default:
		user := meta.Username
		if user == "" {
			user = "postgres"
		}
		return []string{"exec", containerName, "pg_isready", "-U", user}
	}
}

// waitForDatabaseReady polls the database using its protocol-level readiness probe.
// Returns true on success, or false if the timeout elapses.
func waitForDatabaseReady(ctx context.Context, db *domain.Database, timeout time.Duration) bool {
	return waitForDatabaseReadyWithRunner(ctx, globalCommandRunner, db, timeout, 1*time.Second)
}

// waitForDatabaseReadyWithRunner allows injecting custom runners and poll intervals for unit tests.
func waitForDatabaseReadyWithRunner(ctx context.Context, runner CommandRunner, db *domain.Database, timeout time.Duration, pollInterval time.Duration) bool {
	if db == nil {
		return false
	}
	if pollInterval <= 0 {
		pollInterval = 1 * time.Second
	}

	args := buildReadinessProbeArgs(db)
	deadline := time.Now().Add(timeout)

	for {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := runner.Run(probeCtx, "docker", args...)
		cancel()

		if err == nil {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(pollInterval):
		}
	}
}
