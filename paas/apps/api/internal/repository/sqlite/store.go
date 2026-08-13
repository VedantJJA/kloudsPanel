package sqlite

import (
	"context"
	"database/sql"

	"github.com/yourorg/klouds/api/internal/repository"
)

// store implements repository.Store for SQLite.
type store struct {
	db *sql.DB
}

// NewStore creates a new SQLite-backed Store.
func NewStore(db *sql.DB) repository.Store {
	return &store{db: db}
}

func (s *store) Users() repository.UserRepository {
	return &userRepo{db: s.db}
}

func (s *store) Workspaces() repository.WorkspaceRepository {
	return &workspaceRepo{db: s.db}
}

func (s *store) Projects() repository.ProjectRepository {
	return &projectRepo{db: s.db}
}

func (s *store) Services() repository.ServiceRepository {
	return &serviceRepo{db: s.db}
}

func (s *store) Deployments() repository.DeploymentRepository {
	return &deploymentRepo{db: s.db}
}

func (s *store) Databases() repository.DatabaseRepository {
	return &databaseRepo{db: s.db}
}

func (s *store) Jobs() repository.JobRepository {
	return &jobRepo{db: s.db}
}

func (s *store) AuditEvents() repository.AuditRepository {
	return &auditRepo{db: s.db}
}

// WithTx runs fn inside a database transaction.
// On error, the transaction is rolled back.
func (s *store) WithTx(ctx context.Context, fn func(repository.Store) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txStore := &txStore{tx: tx}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// txStore wraps a *sql.Tx to satisfy repository.Store inside a transaction.
type txStore struct {
	tx *sql.Tx
}

func (t *txStore) Users() repository.UserRepository       { return &userRepo{db: t.tx} }
func (t *txStore) Workspaces() repository.WorkspaceRepository { return &workspaceRepo{db: t.tx} }
func (t *txStore) Projects() repository.ProjectRepository     { return &projectRepo{db: t.tx} }
func (t *txStore) Services() repository.ServiceRepository     { return &serviceRepo{db: t.tx} }
func (t *txStore) Deployments() repository.DeploymentRepository { return &deploymentRepo{db: t.tx} }
func (t *txStore) Databases() repository.DatabaseRepository   { return &databaseRepo{db: t.tx} }
func (t *txStore) Jobs() repository.JobRepository             { return &jobRepo{db: t.tx} }
func (t *txStore) AuditEvents() repository.AuditRepository   { return &auditRepo{db: t.tx} }
func (t *txStore) WithTx(ctx context.Context, fn func(repository.Store) error) error {
	return fn(t) // already in tx
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}
