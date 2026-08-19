// Package domain defines core domain entities, value objects, and errors.
package domain

import (
	"strings"
	"time"
)

// --- Users -------------------------------------------------------------------

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
	ID              string       `json:"id"`
	Email           string       `json:"email"`
	DisplayName     string       `json:"display_name"`
	PasswordHash    string       `json:"-"`
	Status          UserStatus   `json:"status"`
	PlatformRole    PlatformRole `json:"platform_role"`
	EmailVerifiedAt *time.Time   `json:"email_verified_at,omitempty"`
	ApprovedBy      *string      `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time   `json:"approved_at,omitempty"`
	LastLoginAt     *time.Time   `json:"last_login_at,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// --- Workspaces ---------------------------------------------------------------

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
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description *string         `json:"description,omitempty"`
	CreatedBy   string          `json:"created_by"`
	Status      WorkspaceStatus `json:"status"`
	QuotaJSON   string          `json:"quota_json,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type WorkspaceMember struct {
	WorkspaceID string              `json:"workspace_id"`
	UserID      string              `json:"user_id"`
	Role        WorkspaceMemberRole `json:"role"`
	Status      string              `json:"status"`
	JoinedAt    time.Time           `json:"joined_at"`
	InvitedBy   *string             `json:"invited_by,omitempty"`
	Version     int                 `json:"version"`
}

// --- Projects -----------------------------------------------------------------

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
	ID                   string        `json:"id"`
	WorkspaceID          string        `json:"workspace_id"`
	Name                 string        `json:"name"`
	Slug                 string        `json:"slug"`
	Description          *string       `json:"description,omitempty"`
	SourceKind           SourceKind    `json:"source_kind"`
	RepositoryURL        *string       `json:"repository_url,omitempty"`
	RepositoryCredential *string       `json:"repository_credential,omitempty"`
	DefaultBranch        *string       `json:"default_branch,omitempty"`
	RootDirectory        string        `json:"root_directory"`
	Status               ProjectStatus `json:"status"`
	CreatedBy            string        `json:"created_by"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

// --- Services -----------------------------------------------------------------

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
	ID              string               `json:"id"`
	ProjectID       string               `json:"project_id"`
	Name            string               `json:"name"`
	Slug            string               `json:"slug"`
	Kind            ServiceKind          `json:"kind"`
	DesiredState    ServiceDesiredState  `json:"desired_state"`
	RuntimeStatus   ServiceRuntimeStatus `json:"runtime_status"`
	InternalPort    *int                 `json:"internal_port,omitempty"`
	HealthcheckPath *string              `json:"healthcheck_path,omitempty"`
	AutoDeploy      bool                 `json:"auto_deploy"`
	ResourceJSON    string               `json:"resource_json,omitempty"`
	CreatedBy       string               `json:"created_by"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// --- Deployments --------------------------------------------------------------

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
	ID              string            `json:"id"`
	ServiceID       string            `json:"service_id"`
	RevisionID      *string           `json:"revision_id,omitempty"`
	Sequence        int64             `json:"sequence"`
	Trigger         DeploymentTrigger `json:"trigger"`
	TriggeredBy     *string           `json:"triggered_by,omitempty"`
	Status          DeploymentStatus  `json:"status"`
	BuildDriver     string            `json:"build_driver"`
	ConfigSnapshot  string            `json:"config_snapshot,omitempty"`
	ImageRef        *string           `json:"image_ref,omitempty"`
	ImageDigest     *string           `json:"image_digest,omitempty"`
	DockerContainer *string           `json:"docker_container,omitempty"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	FinishedAt      *time.Time        `json:"finished_at,omitempty"`
	ExitCode        *int              `json:"exit_code,omitempty"`
	ErrorCode       *string           `json:"error_code,omitempty"`
	ErrorSummary    *string           `json:"error_summary,omitempty"`
	RollbackOf      *string           `json:"rollback_of,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// --- Databases ----------------------------------------------------------------

type DatabaseEngine string

const (
	DBEnginePostgres   DatabaseEngine = "postgres"
	DBEngineMySQL      DatabaseEngine = "mysql"
	DBEngineRedis      DatabaseEngine = "redis"
	DBEngineMongoDB    DatabaseEngine = "mongodb"
	DBEngineClickHouse DatabaseEngine = "clickhouse"
)

// CanonicalizeEngine normalizes user/blueprint-supplied engine identifiers
// to the canonical lowercase form used throughout the system.
func CanonicalizeEngine(raw string) string {
	e := strings.ToLower(strings.TrimSpace(raw))
	switch e {
	case "postgresql", "pg":
		return "postgres"
	case "mongo":
		return "mongodb"
	case "":
		return "postgres" // default behavior
	default:
		return e
	}
}

type DatabaseStatus string

const (
	DBStatusProvisioning DatabaseStatus = "provisioning"
	DBStatusReady        DatabaseStatus = "ready"
	DBStatusStopped      DatabaseStatus = "stopped"
	DBStatusFailed       DatabaseStatus = "failed"
	DBStatusDeleting     DatabaseStatus = "deleting"
)

type Database struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	Name             string         `json:"name"`
	Engine           DatabaseEngine `json:"engine"`
	EngineVersion    string         `json:"engine_version"`
	ImageDigest      string         `json:"image_digest,omitempty"`
	RuntimeStatus    DatabaseStatus `json:"runtime_status"`
	InternalHostname string         `json:"internal_hostname"`
	InternalPort     int            `json:"internal_port"`
	DatabaseName     *string        `json:"database_name,omitempty"`
	CredentialSecret *string        `json:"credential_secret,omitempty"`
	ResourceJSON     string         `json:"resource_json,omitempty"`
	BackupPolicyJSON string         `json:"backup_policy_json,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// --- Jobs ---------------------------------------------------------------------

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

type Job struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	DedupeKey   string     `json:"dedupe_key"`
	Payload     string     `json:"payload"`
	Status      JobStatus  `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	RunAfter    time.Time  `json:"run_after"`
	LockedAt    *time.Time `json:"locked_at,omitempty"`
	LockedBy    *string    `json:"locked_by,omitempty"`
	LastError   *string    `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// --- Audit --------------------------------------------------------------------

type AuditActorKind string

const (
	ActorUser   AuditActorKind = "user"
	ActorMCP    AuditActorKind = "mcp"
	ActorSystem AuditActorKind = "system"
)

type AuditEvent struct {
	ID          string         `json:"id"`
	ActorUserID *string        `json:"actor_user_id,omitempty"`
	ActorKind   AuditActorKind `json:"actor_kind"`
	WorkspaceID *string        `json:"workspace_id,omitempty"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	RequestID   *string        `json:"request_id,omitempty"`
	IPHash      *string        `json:"ip_hash,omitempty"`
	Metadata    string         `json:"metadata,omitempty"`
	OccurredAt  time.Time      `json:"occurred_at"`
}

type UserGitIntegration struct {
	UserID      string    `json:"user_id"`
	Provider    string    `json:"provider"`
	Username    string    `json:"username"`
	Token       string    `json:"-"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Scopes      string    `json:"scopes"`
	ConnectedAt time.Time `json:"connected_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// --- Service Database Links --------------------------------------------------

type ConnectionKind string

const (
	// ConnectionInternal is the default for production services: communicates over
	// the isolated private Docker network 'platform-control' using internal hostnames/ports.
	ConnectionInternal ConnectionKind = "internal"
	// ConnectionExternal is strictly opt-in: connects via public host/port. Not recommended
	// for production service traffic.
	ConnectionExternal ConnectionKind = "external"
)

type ServiceDatabaseLink struct {
	ID             string         `json:"id"`
	ServiceID      string         `json:"service_id"`
	DatabaseID     string         `json:"database_id"`
	EnvVarName     string         `json:"env_var_name"`
	ConnectionKind ConnectionKind `json:"connection_kind"`
	Property       string         `json:"property"` // connectionString | host | port | username | password | database
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// --- Errors -------------------------------------------------------------------

type ErrNotFound struct{ Resource string }

func (e ErrNotFound) Error() string { return e.Resource + " not found" }

type ErrForbidden struct{ Reason string }

func (e ErrForbidden) Error() string { return "forbidden: " + e.Reason }

type ErrConflict struct{ Reason string }

func (e ErrConflict) Error() string { return "conflict: " + e.Reason }

type ErrValidation struct{ Field, Reason string }

func (e ErrValidation) Error() string { return "validation error on " + e.Field + ": " + e.Reason }
