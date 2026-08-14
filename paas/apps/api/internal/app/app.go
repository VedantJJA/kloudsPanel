// Package app wires together all application dependencies and starts the server.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/yourorg/klouds/api/internal/crypto"
	"github.com/yourorg/klouds/api/internal/http"
	"github.com/yourorg/klouds/api/internal/jobs"
	"github.com/yourorg/klouds/api/internal/repository"
	"github.com/yourorg/klouds/api/internal/repository/sqlite"
	"github.com/yourorg/klouds/api/internal/system"
)

// Config holds all configuration read from environment.
type Config struct {
	// Server
	ListenAddr string
	// Database
	DBDriver string // "sqlite" or "postgres"
	DBPath   string // sqlite path
	DBDSN    string // postgres DSN
	// Security
	MasterKeyHex string // 32 bytes hex-encoded AES-256 key
	// Agent
	AgentSocketPath string
	// Platform
	RootDomain string
	AdminEmail string
}

func configFromEnv() Config {
	return Config{
		ListenAddr:      envOr("LISTEN_ADDR", ":8080"),
		DBDriver:        envOr("DB_DRIVER", "sqlite"),
		DBPath:          envOr("DB_PATH", "/data/klouds.db"),
		DBDSN:           os.Getenv("DATABASE_URL"),
		MasterKeyHex:    os.Getenv("MASTER_KEY_HEX"),
		AgentSocketPath: envOr("AGENT_SOCKET", "/run/klouds/agent.sock"),
		RootDomain:      os.Getenv("ROOT_DOMAIN"),
		AdminEmail:      os.Getenv("ADMIN_EMAIL"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Run is the application entry point. It wires dependencies and blocks until ctx is done.
func Run(ctx context.Context, logger *slog.Logger) error {
	cfg := configFromEnv()

	// Validate master key
	masterKey, err := crypto.ParseMasterKeyHex(cfg.MasterKeyHex)
	if err != nil {
		return fmt.Errorf("invalid MASTER_KEY_HEX: %w", err)
	}

	// Open database
	var repo interface{} // will be typed repository.Store
	switch cfg.DBDriver {
	case "sqlite":
		db, err := sqlite.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("open sqlite: %w", err)
		}
		defer db.Close()
		if err := sqlite.RunMigrations(db); err != nil {
			return fmt.Errorf("sqlite migrations: %w", err)
		}
		repo = sqlite.NewStore(db)
	default:
		return fmt.Errorf("unsupported DB_DRIVER: %s", cfg.DBDriver)
	}

	_ = masterKey
	_ = repo

	// Start job worker
	worker := jobs.NewWorker(logger)
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	go worker.Run(workerCtx)

	// Start system self-healing supervisor (reconciles databases, networks, images)
	supervisor := system.NewSupervisor(logger, repo.(repository.Store))
	supervisor.Start(ctx)

	// Start HTTP server
	srv := http.NewServer(logger, repo.(repository.Store), cfg.ListenAddr)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting API server", "addr", cfg.ListenAddr)
		errCh <- srv.Listen(cfg.ListenAddr)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down API server")
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.ShutdownWithContext(shutCtx)
	case err := <-errCh:
		return err
	}
}
