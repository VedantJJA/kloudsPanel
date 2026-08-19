package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/repository"
)

// Supervisor monitors and reconciles system dependencies, database containers, and runtime images.
type Supervisor struct {
	logger *slog.Logger
	store  repository.Store
}

func NewSupervisor(logger *slog.Logger, store repository.Store) *Supervisor {
	return &Supervisor{
		logger: logger,
		store:  store,
	}
}

func (s *Supervisor) Start(ctx context.Context) {
	s.logger.Info("[supervisor] Starting system self-healing supervisor...")

	// 1. Ensure Docker platform network exists
	s.EnsurePlatformNetwork()

	// 2. Run initial database reconciliation and image pre-pull
	go func() {
		s.ReconcileDatabases(ctx)
		s.PrePullCoreImages()
	}()

	// 3. Periodic health and self-healing loop
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.ReconcileDatabases(ctx)
			}
		}
	}()
}

func (s *Supervisor) EnsurePlatformNetwork() {
	out, err := exec.Command("docker", "network", "inspect", "platform-control").CombinedOutput()
	if err != nil || strings.Contains(string(out), "Error: No such network") {
		s.logger.Info("[supervisor] Creating Docker network: platform-control")
		_ = exec.Command("docker", "network", "create", "platform-control").Run()
	}
}

func (s *Supervisor) PrePullCoreImages() {
	images := []string{
		"postgres:16-alpine",
		"mysql:8.0",
		"redis:7.2-alpine",
		"mongo:7.0",
		"clickhouse/clickhouse-server:24.3-alpine",
		"nginx:alpine",
		"node:20-alpine",
		"python:3.11-slim",
		"golang:1.22-alpine",
		"rust:1.77-alpine",
	}

	for _, img := range images {
		inspectCmd := exec.Command("docker", "image", "inspect", img)
		if err := inspectCmd.Run(); err != nil {
			s.logger.Info("[supervisor] Pre-pulling image in background", "image", img)
			_ = exec.Command("docker", "pull", "-q", img).Run()
		}
	}
}

func (s *Supervisor) ReconcileDatabases(ctx context.Context) {
	dbs, err := s.store.Databases().ListAll(ctx)
	if err != nil {
		return
	}

	for _, db := range dbs {
		s.EnsureDatabaseContainerRunning(ctx, db)
	}
}

func (s *Supervisor) EnsureDatabaseContainerRunning(ctx context.Context, db *domain.Database) error {
	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}

	// Check if container is running
	inspectCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	out, err := inspectCmd.CombinedOutput()
	isRunning := err == nil && strings.TrimSpace(string(out)) == "true"

	if isRunning {
		return nil
	}

	s.logger.Info("[supervisor] Healing database container", "name", db.Name, "engine", db.Engine, "container", containerName)

	var meta struct {
		Username     string  `json:"username"`
		Password     string  `json:"password"`
		DatabaseName string  `json:"databaseName"`
		ExternalPort float64 `json:"externalPort"`
	}
	if db.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
	}

	defaultUser := meta.Username
	if defaultUser == "" {
		defaultUser = "postgres"
		if db.Engine == "mysql" {
			defaultUser = "root"
		} else if db.Engine == "mongodb" {
			defaultUser = "admin"
		} else if db.Engine == "redis" || db.Engine == "clickhouse" {
			defaultUser = "default"
		}
	}

	password := meta.Password
	if password == "" {
		password = fmt.Sprintf("kp_sec_%d", time.Now().UnixNano()%1000000)
	}

	dbName := meta.DatabaseName
	if dbName == "" {
		dbName = strings.ToLower(strings.ReplaceAll(db.Name, "_", "-"))
	}

	engine := domain.CanonicalizeEngine(string(db.Engine))

	internalPort := db.InternalPort
	if internalPort <= 0 {
		switch engine {
		case "mysql":
			internalPort = 3306
		case "redis":
			internalPort = 6379
		case "mongodb":
			internalPort = 27017
		case "clickhouse":
			internalPort = 8123
		default:
			internalPort = 5432
		}
	}

	externalPort := int(meta.ExternalPort)
	if externalPort <= 0 {
		switch engine {
		case "mysql":
			externalPort = 13306
		case "redis":
			externalPort = 16379
		case "mongodb":
			externalPort = 17017
		case "clickhouse":
			externalPort = 18123
		default:
			externalPort = 15432
		}
	}

	// Remove stale stopped container if any
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	dbSlug := strings.ToLower(strings.ReplaceAll(db.Name, "_", "-"))

	volumeName := fmt.Sprintf("paas-db-data-%s", dbSlug)
	if volCheck := exec.Command("docker", "volume", "inspect", volumeName).Run(); volCheck != nil {
		s.logger.Warn("[supervisor] Database volume does not exist yet; Docker will create a new volume on startup", "volume", volumeName, "db", db.Name)
	}

	runArgs := BuildDatabaseRunArgs(engine, containerName, dbSlug, defaultUser, password, dbName, externalPort, internalPort)

	startCmd := exec.Command("docker", runArgs...)
	if runOut, err := startCmd.CombinedOutput(); err != nil {
		s.logger.Error("[supervisor] Failed to launch database container", "name", db.Name, "err", err, "out", string(runOut))
		return fmt.Errorf("launch container: %s", string(runOut))
	}

	s.logger.Info("[supervisor] Successfully started database container", "name", db.Name, "container", containerName)
	return nil
}

// BuildDatabaseRunArgs constructs the docker run arguments for a database container.
func BuildDatabaseRunArgs(engine, containerName, dbSlug, defaultUser, password, dbName string, externalPort, internalPort int) []string {
	engine = domain.CanonicalizeEngine(engine)
	switch engine {
	case "postgres", "postgresql":
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("POSTGRES_USER=%s", defaultUser),
			"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			"-e", fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/postgresql/data", dbSlug),
			"postgres:16-alpine",
		}
	case "mysql":
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", password),
			"-e", fmt.Sprintf("MYSQL_DATABASE=%s", dbName),
			"-e", "MYSQL_ROOT_HOST=%",
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/mysql", dbSlug),
			"mysql:8.0",
		}
	case "redis":
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-v", fmt.Sprintf("paas-db-data-%s:/data", dbSlug),
			"redis:7.2-alpine",
			"redis-server", "--requirepass", password,
		}
	case "mongodb", "mongo":
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", defaultUser),
			"-e", fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", password),
			"-v", fmt.Sprintf("paas-db-data-%s:/data/db", dbSlug),
			"mongo:7.0",
		}
	case "clickhouse":
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/clickhouse", dbSlug),
			"clickhouse/clickhouse-server:24.3-alpine",
		}
	default:
		return []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("POSTGRES_USER=%s", defaultUser),
			"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			"-e", fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/postgresql/data", dbSlug),
			"postgres:16-alpine",
		}
	}
}
