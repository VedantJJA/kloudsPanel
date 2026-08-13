// Package jobs implements a database-backed job queue and worker.
// Workers poll the jobs table and execute registered handlers.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

// Handler is a function that processes a job payload.
type Handler func(ctx context.Context, payload string) error

// Worker polls the job queue and dispatches registered handlers.
type Worker struct {
	logger   *slog.Logger
	handlers map[string]Handler
	interval time.Duration
}

// NewWorker creates a new Worker with a default poll interval.
func NewWorker(logger *slog.Logger) *Worker {
	return &Worker{
		logger:   logger,
		handlers: make(map[string]Handler),
		interval: 2 * time.Second,
	}
}

// Register adds a handler for a job kind.
func (w *Worker) Register(kind string, fn Handler) {
	w.handlers[kind] = fn
}

// Run starts the worker loop. It blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("job worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("job worker stopping")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	// TODO: in Phase 2 wire to repository.JobRepository.ClaimNext
	// and dispatch to registered handlers
}

// JobKinds are the well-known job types.
const (
	KindDeploy         = "deploy"
	KindStop           = "stop"
	KindRestart        = "restart"
	KindDelete         = "delete"
	KindBackup         = "backup"
	KindRestore        = "restore"
	KindTLSValidate    = "tls_validate"
	KindMetricsCollect = "metrics_collect"
)
