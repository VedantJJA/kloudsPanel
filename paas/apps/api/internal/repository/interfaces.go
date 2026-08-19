// Package repository defines the storage interfaces for the platform.
// Both SQLite and PostgreSQL implementations satisfy these interfaces.
package repository

import (
	"context"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
)

// Store aggregates all repository interfaces.
type Store interface {
	Users() UserRepository
	AuthSessions() AuthSessionRepository
	Workspaces() WorkspaceRepository
	Projects() ProjectRepository
	Services() ServiceRepository
	Deployments() DeploymentRepository
	Databases() DatabaseRepository
	ServiceDatabaseLinks() ServiceDatabaseLinkRepository
	Jobs() JobRepository
	AuditEvents() AuditRepository
	GitIntegrations() GitIntegrationRepository
	// Transactional operation
	WithTx(ctx context.Context, fn func(Store) error) error
}

// AuthSessionRepository manages persistent user authentication sessions.
type AuthSessionRepository interface {
	Create(ctx context.Context, sessionID, userID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (userID string, expiresAt time.Time, err error)
	DeleteByToken(ctx context.Context, token string) error
}

// GitIntegrationRepository manages per-user git credentials.
type GitIntegrationRepository interface {
	Get(ctx context.Context, userID, provider string) (*domain.UserGitIntegration, error)
	ListForUser(ctx context.Context, userID string) ([]*domain.UserGitIntegration, error)
	Upsert(ctx context.Context, item *domain.UserGitIntegration) error
	Delete(ctx context.Context, userID, provider string) error
}

// UserRepository manages user persistence.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) error
	ListPending(ctx context.Context) ([]*domain.User, error)
	ListAll(ctx context.Context, limit, offset int) ([]*domain.User, error)
	CountByRole(ctx context.Context, role domain.PlatformRole) (int, error)
	Delete(ctx context.Context, id string) error
}

// WorkspaceRepository manages workspace and membership persistence.
type WorkspaceRepository interface {
	Create(ctx context.Context, w *domain.Workspace) error
	GetByID(ctx context.Context, id string) (*domain.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error)
	ListForUser(ctx context.Context, userID string) ([]*domain.Workspace, error)
	Update(ctx context.Context, w *domain.Workspace) error
	Delete(ctx context.Context, id string) error
	SlugExists(ctx context.Context, slug string) (bool, error)

	AddMember(ctx context.Context, m *domain.WorkspaceMember) error
	GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error)
	UpdateMember(ctx context.Context, m *domain.WorkspaceMember) error
	ListMembers(ctx context.Context, workspaceID string) ([]*domain.WorkspaceMember, error)
}

// ProjectRepository manages project persistence.
type ProjectRepository interface {
	Create(ctx context.Context, p *domain.Project) error
	GetByID(ctx context.Context, id string) (*domain.Project, error)
	GetBySlug(ctx context.Context, workspaceID, slug string) (*domain.Project, error)
	ListForWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*domain.Project, error)
	ListAll(ctx context.Context) ([]*domain.Project, error)
	Update(ctx context.Context, p *domain.Project) error
	Delete(ctx context.Context, id string) error
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// ServiceRepository manages service persistence.
type ServiceRepository interface {
	Create(ctx context.Context, s *domain.Service) error
	GetByID(ctx context.Context, id string) (*domain.Service, error)
	ListForProject(ctx context.Context, projectID string) ([]*domain.Service, error)
	ListAll(ctx context.Context) ([]*domain.Service, error)
	Update(ctx context.Context, s *domain.Service) error
	Delete(ctx context.Context, id string) error
	SlugExists(ctx context.Context, slug string) (bool, error)
}

// DeploymentRepository manages deployment persistence.
type DeploymentRepository interface {
	Create(ctx context.Context, d *domain.Deployment) error
	GetByID(ctx context.Context, id string) (*domain.Deployment, error)
	ListForService(ctx context.Context, serviceID string, limit int, cursor *string) ([]*domain.Deployment, error)
	Update(ctx context.Context, d *domain.Deployment) error
	GetLatestHealthy(ctx context.Context, serviceID string) (*domain.Deployment, error)
	GetNextSequence(ctx context.Context, serviceID string) (int64, error)
}

// DatabaseRepository manages database service persistence.
type DatabaseRepository interface {
	Create(ctx context.Context, db *domain.Database) error
	GetByID(ctx context.Context, id string) (*domain.Database, error)
	ListForProject(ctx context.Context, projectID string) ([]*domain.Database, error)
	ListAll(ctx context.Context) ([]*domain.Database, error)
	Update(ctx context.Context, db *domain.Database) error
	Delete(ctx context.Context, id string) error
}

// JobRepository manages the job queue.
type JobRepository interface {
	Enqueue(ctx context.Context, j *domain.Job) error
	ClaimNext(ctx context.Context, workerID string, kinds []string) (*domain.Job, error)
	MarkSucceeded(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, errMsg string) error
	GetByDedupeKey(ctx context.Context, key string) (*domain.Job, error)
}

// AuditRepository manages audit event persistence.
type AuditRepository interface {
	Append(ctx context.Context, e *domain.AuditEvent) error
	List(ctx context.Context, workspaceID *string, limit int, cursor *string) ([]*domain.AuditEvent, error)
}

// ServiceDatabaseLinkRepository manages explicit links between services and databases.
type ServiceDatabaseLinkRepository interface {
	Create(ctx context.Context, link *domain.ServiceDatabaseLink) error
	GetByID(ctx context.Context, id string) (*domain.ServiceDatabaseLink, error)
	ListForService(ctx context.Context, serviceID string) ([]*domain.ServiceDatabaseLink, error)
	ListForDatabase(ctx context.Context, databaseID string) ([]*domain.ServiceDatabaseLink, error)
	Delete(ctx context.Context, id string) error
}

