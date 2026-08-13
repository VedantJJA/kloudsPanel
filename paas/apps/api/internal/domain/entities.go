// Package domain defines core domain entities, value objects, and errors.
package domain

import (
	"time"
)

// ─── Users ───────────────────────────────────────────────────────────────────

type UserStatus string

const (
	UserStatusPending   UserStatus = "pending"
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

type PlatformRole string

const (
	PlatformRoleMainAdmin PlatformRole = "main_admin"
	PlatformRoleAdmin     PlatformRole = "admin"
	PlatformRoleUser      PlatformRole = "user"
)

type User struct {
	ID              string
	Email           string
	DisplayName     string
	PasswordHash    string
	Status          UserStatus
	PlatformRole    PlatformRole
	EmailVerifiedAt *time.Time
	ApprovedBy      *string
	ApprovedAt      *time.Time
	LastLoginAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ─── Workspaces ───────────────────────────────────────────────────────────────

type WorkspaceStatus string

const (
	WorkspaceStatusActive   WorkspaceStatus = "active"
	WorkspaceStatusArchived WorkspaceStatus = "archived"
)

type WorkspaceMemberRole string

const (
	RoleOwner     WorkspaceMemberRole = "owner"
	RoleAdmin     WorkspaceMemberRole = "admin"
	RoleDeveloper WorkspaceMemberRole = "developer"
	RoleViewer    WorkspaceMemberRole = "viewer"
)

type Workspace struct {
	ID          string
	Name        string
	Slug        string
	Description *string
	CreatedBy   string
	Status      WorkspaceStatus
	QuotaJSON   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkspaceMember struct {
	WorkspaceID string
	UserID      string
	Role        WorkspaceMemberRole
	Status      string
	JoinedAt    time.Time
	InvitedBy   *string
	Version     int
}

// ─── Projects ─────────────────────────────────────────────────────────────────

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
	ProjectStatusDeleting ProjectStatus = "deleting"
)

type SourceKind string

const (
	SourceKindGit    SourceKind = "git"
	SourceKindUpload SourceKind = "upload"
	SourceKindEmpty  SourceKind = "empty"
)

type Project struct {
	ID                   string
	WorkspaceID          string
	Name                 string
	Slug                 string
	Description          *string
	SourceKind           SourceKind
	RepositoryURL        *string
	RepositoryCredential *string
	DefaultBranch        *string
	RootDirectory        string
	Status               ProjectStatus
	CreatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ─── Services ─────────────────────────────────────────────────────────────────

type ServiceKind string

const (
	ServiceKindWeb    ServiceKind = "web"
	ServiceKindWorker ServiceKind = "worker"
	ServiceKindCron   ServiceKind = "cron"
	ServiceKindStatic ServiceKind = "static"
)

type ServiceDesiredState string

const (
	ServiceDesiredRunning ServiceDesiredState = "running"
	ServiceDesiredStopped ServiceDesiredState = "stopped"
)

type ServiceRuntimeStatus string

const (
	ServiceStatusDraft     ServiceRuntimeStatus = "draft"
	ServiceStatusBuilding  ServiceRuntimeStatus = "building"
	ServiceStatusDeploying ServiceRuntimeStatus = "deploying"
	ServiceStatusRunning   ServiceRuntimeStatus = "running"
	ServiceStatusStopped   ServiceRuntimeStatus = "stopped"
	ServiceStatusFailed    ServiceRuntimeStatus = "failed"
	ServiceStatusDeleting  ServiceRuntimeStatus = "deleting"
)

type Service struct {
	ID             string
	ProjectID      string
	Name           string
	Slug           string
	Kind           ServiceKind
	DesiredState   ServiceDesiredState
	RuntimeStatus  ServiceRuntimeStatus
	InternalPort   *int
	HealthcheckPath *string
	AutoDeploy     bool
	ResourceJSON   string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ─── Deployments ──────────────────────────────────────────────────────────────

type DeploymentStatus string

const (
	DeploymentQueued      DeploymentStatus = "queued"
	DeploymentBuilding    DeploymentStatus = "building"
	DeploymentBuildFailed DeploymentStatus = "build_failed"
	DeploymentCreating    DeploymentStatus = "creating"
	DeploymentStarting    DeploymentStatus = "starting"
	DeploymentHealthy     DeploymentStatus = "healthy"
	DeploymentFailed      DeploymentStatus = "failed"
	DeploymentCancelled   DeploymentStatus = "cancelled"
	DeploymentRolledBack  DeploymentStatus = "rolled_back"
)

type DeploymentTrigger string

const (
	TriggerManual   DeploymentTrigger = "manual"
	TriggerAuto     DeploymentTrigger = "auto"
	TriggerRollback DeploymentTrigger = "rollback"
	TriggerMCP      DeploymentTrigger = "mcp"
	TriggerSystem   DeploymentTrigger = "system"
)

type Deployment struct {
	ID              string
	ServiceID       string
	RevisionID      *string
	Sequence        int64
	Trigger         DeploymentTrigger
	TriggeredBy     *string
	Status          DeploymentStatus
	BuildDriver     string
	ConfigSnapshot  string
	ImageRef        *string
	ImageDigest     *string
	DockerContainer *string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ExitCode        *int
	ErrorCode       *string
	ErrorSummary    *string
	RollbackOf      *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ─── Databases ────────────────────────────────────────────────────────────────

type DatabaseEngine string

const (
	DBEnginePostgres   DatabaseEngine = "postgres"
	DBEngineMySQL      DatabaseEngine = "mysql"
	DBEngineRedis      DatabaseEngine = "redis"
	DBEngineMongoDB    DatabaseEngine = "mongodb"
	DBEngineClickHouse DatabaseEngine = "clickhouse"
)

type DatabaseStatus string

const (
	DBStatusProvisioning DatabaseStatus = "provisioning"
	DBStatusReady        DatabaseStatus = "ready"
	DBStatusStopped      DatabaseStatus = "stopped"
	DBStatusFailed       DatabaseStatus = "failed"
	DBStatusDeleting     DatabaseStatus = "deleting"
)

type Database struct {
	ID               string
	ProjectID        string
	Name             string
	Engine           DatabaseEngine
	EngineVersion    string
	ImageDigest      string
	RuntimeStatus    DatabaseStatus
	InternalHostname string
	InternalPort     int
	DatabaseName     *string
	CredentialSecret *string
	ResourceJSON     string
	BackupPolicyJSON string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

type Job struct {
	ID          string
	Kind        string
	DedupeKey   string
	Payload     string
	Status      JobStatus
	Attempts    int
	MaxAttempts int
	RunAfter    time.Time
	LockedAt    *time.Time
	LockedBy    *string
	LastError   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ─── Audit ────────────────────────────────────────────────────────────────────

type AuditActorKind string

const (
	ActorUser   AuditActorKind = "user"
	ActorMCP    AuditActorKind = "mcp"
	ActorSystem AuditActorKind = "system"
)

type AuditEvent struct {
	ID          string
	ActorUserID *string
	ActorKind   AuditActorKind
	WorkspaceID *string
	Action      string
	TargetType  string
	TargetID    string
	RequestID   *string
	IPHash      *string
	Metadata    string
	OccurredAt  time.Time
}

// ─── Errors ───────────────────────────────────────────────────────────────────

type ErrNotFound struct{ Resource string }

func (e ErrNotFound) Error() string { return e.Resource + " not found" }

type ErrForbidden struct{ Reason string }

func (e ErrForbidden) Error() string { return "forbidden: " + e.Reason }

type ErrConflict struct{ Reason string }

func (e ErrConflict) Error() string { return "conflict: " + e.Reason }

type ErrValidation struct{ Field, Reason string }

func (e ErrValidation) Error() string { return "validation error on " + e.Field + ": " + e.Reason }
