package sqlite

// sqliteMigrations is the ordered list of DDL migrations for SQLite.
// Each entry is idempotent using CREATE TABLE IF NOT EXISTS.
var sqliteMigrations = []string{
	// ── Migration 1: Schema version tracking ─────────────────────────────────
	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version     INTEGER PRIMARY KEY,
		applied_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 2: Users ────────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS users (
		id                  TEXT PRIMARY KEY,
		email               TEXT NOT NULL,
		display_name        TEXT NOT NULL,
		password_hash       TEXT NOT NULL,
		status              TEXT NOT NULL CHECK(status IN ('pending','active','suspended')),
		platform_role       TEXT NOT NULL CHECK(platform_role IN ('main_admin','admin','user')),
		email_verified_at   TEXT NULL,
		approved_by         TEXT NULL REFERENCES users(id),
		approved_at         TEXT NULL,
		last_login_at       TEXT NULL,
		created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(lower(email))`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_main_admin ON users(platform_role) WHERE platform_role = 'main_admin'`,

	// ── Migration 3: Auth sessions ────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS auth_sessions (
		id               TEXT PRIMARY KEY,
		user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash       TEXT NOT NULL UNIQUE,
		csrf_secret_hash TEXT NOT NULL,
		ip_hash          TEXT NULL,
		user_agent       TEXT NULL,
		expires_at       TEXT NOT NULL,
		last_seen_at     TEXT NOT NULL,
		revoked_at       TEXT NULL,
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions(user_id)`,

	// ── Migration 4: Workspaces ───────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS workspaces (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		slug        TEXT NOT NULL UNIQUE,
		description TEXT NULL,
		created_by  TEXT NOT NULL REFERENCES users(id),
		status      TEXT NOT NULL CHECK(status IN ('active','archived')),
		quota_json  TEXT NOT NULL DEFAULT '{}',
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 5: Workspace members ───────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS workspace_members (
		workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		role         TEXT NOT NULL CHECK(role IN ('owner','admin','developer','viewer')),
		status       TEXT NOT NULL CHECK(status IN ('active','removed')),
		joined_at    TEXT NOT NULL,
		invited_by   TEXT NULL REFERENCES users(id),
		version      INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (workspace_id, user_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id)`,

	// ── Migration 6: Workspace invitations ───────────────────────────────────
	`CREATE TABLE IF NOT EXISTS workspace_invitations (
		id           TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		email        TEXT NOT NULL,
		role         TEXT NOT NULL CHECK(role IN ('owner','admin','developer','viewer')),
		token_hash   TEXT NOT NULL UNIQUE,
		invited_by   TEXT REFERENCES users(id),
		expires_at   TEXT NOT NULL,
		accepted_at  TEXT NULL,
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_invitations_open
		ON workspace_invitations(workspace_id, lower(email)) WHERE accepted_at IS NULL`,

	// ── Migration 7: Encrypted secrets ───────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS encrypted_secrets (
		id           TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		purpose      TEXT NOT NULL,
		ciphertext   BLOB NOT NULL,
		nonce        BLOB NOT NULL,
		key_version  INTEGER NOT NULL,
		aad          TEXT NOT NULL,
		fingerprint  TEXT NOT NULL,
		rotated_at   TEXT NULL,
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 8: Projects ─────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS projects (
		id                       TEXT PRIMARY KEY,
		workspace_id             TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
		name                     TEXT NOT NULL,
		slug                     TEXT NOT NULL,
		description              TEXT NULL,
		source_kind              TEXT NOT NULL CHECK(source_kind IN ('git','upload','empty')),
		repository_url           TEXT NULL,
		repository_credential_id TEXT NULL REFERENCES encrypted_secrets(id),
		default_branch           TEXT NULL,
		root_directory           TEXT NOT NULL DEFAULT '.',
		status                   TEXT NOT NULL CHECK(status IN ('active','archived','deleting')),
		created_by               TEXT REFERENCES users(id),
		created_at               TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at               TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(workspace_id, slug)
	)`,

	// ── Migration 9: Project revisions ───────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS project_revisions (
		id                  TEXT PRIMARY KEY,
		project_id          TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		commit_sha          TEXT NULL,
		ref_name            TEXT NULL,
		source_archive_ref  TEXT NULL,
		tree_manifest       TEXT NOT NULL DEFAULT '{}',
		created_by          TEXT REFERENCES users(id),
		created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 10: Services ────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS services (
		id               TEXT PRIMARY KEY,
		project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		name             TEXT NOT NULL,
		slug             TEXT NOT NULL,
		kind             TEXT NOT NULL CHECK(kind IN ('web','worker','cron','static')),
		desired_state    TEXT NOT NULL CHECK(desired_state IN ('running','stopped')),
		runtime_status   TEXT NOT NULL CHECK(runtime_status IN ('draft','building','deploying','running','stopped','failed','deleting')),
		internal_port    INTEGER NULL CHECK(internal_port IS NULL OR (internal_port >= 1 AND internal_port <= 65535)),
		healthcheck_path TEXT NULL,
		auto_deploy      INTEGER NOT NULL DEFAULT 0,
		resource_json    TEXT NOT NULL DEFAULT '{}',
		created_by       TEXT REFERENCES users(id),
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(project_id, slug)
	)`,

	// ── Migration 11: Service build configs ──────────────────────────────────
	`CREATE TABLE IF NOT EXISTS service_build_configs (
		id               TEXT PRIMARY KEY,
		service_id       TEXT NOT NULL UNIQUE REFERENCES services(id) ON DELETE CASCADE,
		strategy         TEXT NOT NULL CHECK(strategy IN ('cnb','nixpacks','dockerfile','manual','image')),
		manual_stack     TEXT NULL CHECK(manual_stack IS NULL OR manual_stack IN ('python','node','go','rust','static')),
		dockerfile_path  TEXT NULL,
		build_context    TEXT NOT NULL DEFAULT '.',
		builder_image    TEXT NULL,
		buildpacks       TEXT NULL,
		build_command    TEXT NULL,
		start_command    TEXT NULL,
		image_reference  TEXT NULL,
		detected_plan    TEXT NULL,
		build_env_json   TEXT NOT NULL DEFAULT '{}',
		version          INTEGER NOT NULL DEFAULT 1,
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 12: Service scale-to-zero ──────────────────────────────────
	`CREATE TABLE IF NOT EXISTS service_scale_to_zero (
		service_id                TEXT PRIMARY KEY REFERENCES services(id) ON DELETE CASCADE,
		enabled                   INTEGER NOT NULL DEFAULT 0,
		group_name                TEXT NOT NULL UNIQUE,
		idle_timeout_seconds      INTEGER NOT NULL DEFAULT 900 CHECK(idle_timeout_seconds >= 60 AND idle_timeout_seconds <= 86400),
		strategy                  TEXT NOT NULL DEFAULT 'dynamic' CHECK(strategy IN ('dynamic','blocking')),
		waiting_page_name         TEXT NOT NULL DEFAULT 'default',
		refresh_seconds           INTEGER NOT NULL DEFAULT 5 CHECK(refresh_seconds >= 1 AND refresh_seconds <= 60),
		keep_alive_seconds        INTEGER NULL,
		ignored_user_agents       TEXT NOT NULL DEFAULT '[]',
		cold_start_timeout_seconds INTEGER NOT NULL DEFAULT 90,
		created_at               TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at               TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 13: Environment variables ──────────────────────────────────
	`CREATE TABLE IF NOT EXISTS environment_variables (
		id          TEXT PRIMARY KEY,
		service_id  TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		key         TEXT NOT NULL,
		is_secret   INTEGER NOT NULL DEFAULT 0,
		plain_value TEXT NULL,
		secret_id   TEXT NULL REFERENCES encrypted_secrets(id),
		scope       TEXT NOT NULL CHECK(scope IN ('runtime','build')),
		created_by  TEXT REFERENCES users(id),
		created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(service_id, key, scope),
		CHECK((is_secret = 0 AND plain_value IS NOT NULL AND secret_id IS NULL)
		   OR (is_secret = 1 AND plain_value IS NULL AND secret_id IS NOT NULL))
	)`,

	// ── Migration 14: Databases ───────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS databases (
		id                 TEXT PRIMARY KEY,
		project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		name               TEXT NOT NULL,
		engine             TEXT NOT NULL CHECK(engine IN ('postgres','mysql','redis','mongodb','clickhouse')),
		engine_version     TEXT NOT NULL,
		image_digest       TEXT NOT NULL,
		runtime_status     TEXT NOT NULL CHECK(runtime_status IN ('provisioning','ready','stopped','failed','deleting')),
		internal_hostname  TEXT NOT NULL UNIQUE,
		internal_port      INTEGER NOT NULL,
		database_name      TEXT NULL,
		credential_secret_id TEXT NULL REFERENCES encrypted_secrets(id),
		resource_json      TEXT NOT NULL DEFAULT '{}',
		backup_policy_json TEXT NOT NULL DEFAULT '{}',
		created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(project_id, name)
	)`,

	// ── Migration 15: Service-database links ─────────────────────────────────
	`CREATE TABLE IF NOT EXISTS service_database_links (
		id               TEXT PRIMARY KEY,
		service_id       TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		database_id      TEXT NOT NULL REFERENCES databases(id) ON DELETE RESTRICT,
		alias            TEXT NOT NULL,
		injection_mode   TEXT NOT NULL CHECK(injection_mode IN ('url','fields','custom')),
		env_mapping_json TEXT NOT NULL DEFAULT '{}',
		created_by       TEXT REFERENCES users(id),
		created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(service_id, database_id),
		UNIQUE(service_id, alias)
	)`,

	// ── Migration 16: Persistent volumes ─────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS persistent_volumes (
		id                  TEXT PRIMARY KEY,
		workspace_id        TEXT NOT NULL REFERENCES workspaces(id),
		project_id          TEXT NULL REFERENCES projects(id) ON DELETE CASCADE,
		database_id         TEXT NULL REFERENCES databases(id) ON DELETE CASCADE,
		docker_volume_name  TEXT NOT NULL UNIQUE,
		purpose             TEXT NOT NULL CHECK(purpose IN ('database','service','build-cache','platform')),
		mount_path          TEXT NULL,
		driver              TEXT NOT NULL DEFAULT 'local',
		status              TEXT NOT NULL CHECK(status IN ('active','deleting','orphaned')),
		size_bytes_observed INTEGER NULL,
		retention_json      TEXT NOT NULL DEFAULT '{}',
		created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 17: Domains ─────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS domains (
		id                   TEXT PRIMARY KEY,
		service_id           TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		hostname             TEXT NOT NULL UNIQUE,
		kind                 TEXT NOT NULL CHECK(kind IN ('generated','custom')),
		path_prefix          TEXT NOT NULL DEFAULT '/',
		is_primary           INTEGER NOT NULL DEFAULT 0,
		verification_status  TEXT NOT NULL CHECK(verification_status IN ('pending','verified','failed')),
		certificate_status   TEXT NOT NULL CHECK(certificate_status IN ('pending','issued','renewing','failed')),
		verification_token_hash TEXT NULL,
		last_verified_at     TEXT NULL,
		last_error           TEXT NULL,
		created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_primary_service
		ON domains(service_id) WHERE is_primary = 1`,

	// ── Migration 18: Deployments ─────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS deployments (
		id                  TEXT PRIMARY KEY,
		service_id          TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
		revision_id         TEXT NULL REFERENCES project_revisions(id),
		sequence            INTEGER NOT NULL,
		trigger             TEXT NOT NULL CHECK(trigger IN ('manual','auto','rollback','mcp','system')),
		triggered_by        TEXT NULL REFERENCES users(id),
		status              TEXT NOT NULL CHECK(status IN ('queued','building','build_failed','creating','starting','healthy','failed','cancelled','rolled_back')),
		build_driver        TEXT NOT NULL,
		config_snapshot     TEXT NOT NULL,
		image_ref           TEXT NULL,
		image_digest        TEXT NULL,
		docker_container_id TEXT NULL UNIQUE,
		started_at          TEXT NULL,
		finished_at         TEXT NULL,
		exit_code           INTEGER NULL,
		error_code          TEXT NULL,
		error_summary       TEXT NULL,
		rollback_of         TEXT NULL REFERENCES deployments(id),
		created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		UNIQUE(service_id, sequence)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_deployments_service_seq ON deployments(service_id, sequence DESC)`,

	// ── Migration 19: Deployment log entries ─────────────────────────────────
	`CREATE TABLE IF NOT EXISTS deployment_log_entries (
		deployment_id TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
		sequence      INTEGER NOT NULL,
		emitted_at    TEXT NOT NULL,
		stream        TEXT NOT NULL CHECK(stream IN ('build','stdout','stderr','system')),
		message       TEXT NOT NULL,
		is_redacted   INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (deployment_id, sequence)
	)`,

	// ── Migration 20: Terminal sessions ──────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS terminal_sessions (
		id            TEXT PRIMARY KEY,
		service_id    TEXT NOT NULL REFERENCES services(id),
		deployment_id TEXT NOT NULL REFERENCES deployments(id),
		user_id       TEXT NOT NULL REFERENCES users(id),
		docker_exec_id TEXT NULL,
		status        TEXT NOT NULL CHECK(status IN ('issued','open','closed','expired','denied')),
		grant_hash    TEXT NOT NULL UNIQUE,
		opened_at     TEXT NULL,
		closed_at     TEXT NULL,
		exit_code     INTEGER NULL,
		client_ip_hash TEXT NULL,
		created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 21: Jobs ────────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS jobs (
		id           TEXT PRIMARY KEY,
		kind         TEXT NOT NULL,
		dedupe_key   TEXT NOT NULL UNIQUE,
		payload      TEXT NOT NULL,
		status       TEXT NOT NULL CHECK(status IN ('queued','running','succeeded','failed','cancelled')),
		attempts     INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		run_after    TEXT NOT NULL,
		locked_at    TEXT NULL,
		locked_by    TEXT NULL,
		last_error   TEXT NULL,
		created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_jobs_status_run_after ON jobs(status, run_after) WHERE status = 'queued'`,

	// ── Migration 22: Outbox events ───────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS outbox_events (
		id             TEXT PRIMARY KEY,
		topic          TEXT NOT NULL,
		aggregate_type TEXT NOT NULL,
		aggregate_id   TEXT NOT NULL,
		payload        TEXT NOT NULL,
		occurred_at    TEXT NOT NULL,
		published_at   TEXT NULL,
		attempts       INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events(occurred_at) WHERE published_at IS NULL`,

	// ── Migration 23: System hosts and metrics ───────────────────────────────
	`CREATE TABLE IF NOT EXISTS system_hosts (
		id                   TEXT PRIMARY KEY,
		name                 TEXT NOT NULL UNIQUE,
		architecture         TEXT NOT NULL CHECK(architecture IN ('amd64','arm64')),
		agent_version        TEXT NOT NULL,
		docker_engine_version TEXT NOT NULL,
		cpu_model            TEXT NULL,
		cpu_logical_count    INTEGER NULL,
		memory_total_bytes   INTEGER NULL,
		storage_total_bytes  INTEGER NULL,
		last_seen_at         TEXT NOT NULL,
		created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	`CREATE TABLE IF NOT EXISTS system_metrics (
		host_id              TEXT NOT NULL REFERENCES system_hosts(id),
		observed_at          TEXT NOT NULL,
		cpu_percent          REAL NOT NULL,
		load1                REAL NULL,
		memory_total_bytes   INTEGER NOT NULL,
		memory_used_bytes    INTEGER NOT NULL,
		storage_total_bytes  INTEGER NOT NULL,
		storage_used_bytes   INTEGER NOT NULL,
		network_rx_bytes     INTEGER NULL,
		network_tx_bytes     INTEGER NULL,
		PRIMARY KEY (host_id, observed_at)
	)`,

	`CREATE TABLE IF NOT EXISTS container_metrics (
		deployment_id       TEXT NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
		observed_at         TEXT NOT NULL,
		cpu_percent         REAL NOT NULL,
		memory_used_bytes   INTEGER NOT NULL,
		memory_limit_bytes  INTEGER NULL,
		network_rx_bytes    INTEGER NULL,
		network_tx_bytes    INTEGER NULL,
		block_read_bytes    INTEGER NULL,
		block_write_bytes   INTEGER NULL,
		PRIMARY KEY (deployment_id, observed_at)
	)`,

	// ── Migration 24: Audit events ────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS audit_events (
		id           TEXT PRIMARY KEY,
		actor_user_id TEXT NULL REFERENCES users(id),
		actor_kind   TEXT NOT NULL CHECK(actor_kind IN ('user','mcp','system')),
		workspace_id TEXT NULL REFERENCES workspaces(id),
		action       TEXT NOT NULL,
		target_type  TEXT NOT NULL,
		target_id    TEXT NOT NULL,
		request_id   TEXT NULL,
		ip_hash      TEXT NULL,
		metadata     TEXT NOT NULL DEFAULT '{}',
		occurred_at  TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_workspace_time ON audit_events(workspace_id, occurred_at DESC)`,

	// ── Migration 25: MCP OAuth ───────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS mcp_oauth_clients (
		id             TEXT PRIMARY KEY,
		client_id      TEXT NOT NULL UNIQUE,
		client_name    TEXT NOT NULL,
		redirect_uris  TEXT NOT NULL DEFAULT '[]',
		allowed_scopes TEXT NOT NULL DEFAULT '[]',
		status         TEXT NOT NULL CHECK(status IN ('active','revoked')),
		created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	`CREATE TABLE IF NOT EXISTS mcp_authorizations (
		id                 TEXT PRIMARY KEY,
		user_id            TEXT NOT NULL REFERENCES users(id),
		client_id          TEXT NOT NULL REFERENCES mcp_oauth_clients(id),
		workspace_id       TEXT NOT NULL REFERENCES workspaces(id),
		scopes             TEXT NOT NULL DEFAULT '[]',
		refresh_token_hash TEXT NULL UNIQUE,
		expires_at         TEXT NOT NULL,
		revoked_at         TEXT NULL,
		created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		updated_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 26: Platform settings ──────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS platform_settings (
		key        TEXT PRIMARY KEY,
		value_json TEXT NOT NULL,
		is_secret  INTEGER NOT NULL DEFAULT 0,
		updated_by TEXT NULL REFERENCES users(id),
		updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`,

	// ── Migration 27: Default platform settings ───────────────────────────────
	`INSERT OR IGNORE INTO platform_settings(key, value_json, updated_at) VALUES
		('setup_complete', 'false', strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		('root_domain', 'null', strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		('acme_email', 'null', strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		('dns_mode', '"http-01"', strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		('disk_warn_pct', '80', strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		('disk_critical_pct', '90', strftime('%Y-%m-%dT%H:%M:%fZ','now'))`,
}
