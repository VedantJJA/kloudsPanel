package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

// ─── Projects ─────────────────────────────────────────────────────────────────

type projectRepo struct{ db querier }

func (r *projectRepo) Create(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = ids.NewV7()
	}
	if p.SourceKind != domain.SourceKindGit && p.SourceKind != domain.SourceKindUpload && p.SourceKind != domain.SourceKindEmpty {
		p.SourceKind = domain.SourceKindEmpty
	}
	if p.Status != domain.ProjectStatusActive && p.Status != domain.ProjectStatusArchived && p.Status != domain.ProjectStatusDeleting {
		p.Status = domain.ProjectStatusActive
	}
	if p.RootDirectory == "" {
		p.RootDirectory = "."
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var createdBy *string
	if p.CreatedBy != "" {
		createdBy = &p.CreatedBy
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id,workspace_id,name,slug,description,source_kind,
		  repository_url,repository_credential_id,default_branch,root_directory,
		  status,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.WorkspaceID, p.Name, p.Slug, p.Description, string(p.SourceKind),
		p.RepositoryURL, p.RepositoryCredential, p.DefaultBranch, p.RootDirectory,
		string(p.Status), createdBy, now, now,
	)
	return err
}

func (r *projectRepo) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,workspace_id,name,slug,description,source_kind,
		       repository_url,repository_credential_id,default_branch,root_directory,
		       status,created_by,created_at,updated_at
		FROM projects WHERE id=? OR slug=?`, id, id)
	return scanProject(row)
}

func (r *projectRepo) GetBySlug(ctx context.Context, workspaceID, slug string) (*domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,workspace_id,name,slug,description,source_kind,
		       repository_url,repository_credential_id,default_branch,root_directory,
		       status,created_by,created_at,updated_at
		FROM projects WHERE workspace_id=? AND slug=?`, workspaceID, slug)
	return scanProject(row)
}

func (r *projectRepo) ListForWorkspace(ctx context.Context, workspaceID string, limit, offset int) ([]*domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,workspace_id,name,slug,description,source_kind,
		       repository_url,repository_credential_id,default_branch,root_directory,
		       status,created_by,created_at,updated_at
		FROM projects WHERE workspace_id=? AND status != 'deleting'
		ORDER BY created_at DESC LIMIT ? OFFSET ?`, workspaceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Project
	for rows.Next() {
		p, err := scanProjectRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *projectRepo) Update(ctx context.Context, p *domain.Project) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET name=?,description=?,status=?,default_branch=?,root_directory=?,updated_at=?
		WHERE id=?`,
		p.Name, p.Description, p.Status, p.DefaultBranch, p.RootDirectory, now, p.ID,
	)
	return err
}

func (r *projectRepo) Delete(ctx context.Context, id string) error {
	if id == "" || id == "undefined" {
		return nil
	}
	_, _ = r.db.ExecContext(ctx, `DELETE FROM deployments WHERE service_id IN (SELECT id FROM services WHERE project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?))`, id, id)
	_, _ = r.db.ExecContext(ctx, `DELETE FROM services WHERE project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?)`, id, id)
	_, _ = r.db.ExecContext(ctx, `DELETE FROM databases WHERE project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?)`, id, id)
	_, _ = r.db.ExecContext(ctx, `DELETE FROM project_revisions WHERE project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?)`, id, id)
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id=? OR slug=?`, id, id)
	return err
}

func scanProject(row *sql.Row) (*domain.Project, error) {
	p := &domain.Project{}
	var createdAt, updatedAt string
	err := row.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.SourceKind,
		&p.RepositoryURL, &p.RepositoryCredential, &p.DefaultBranch, &p.RootDirectory,
		&p.Status, &p.CreatedBy, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "project"}
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return p, nil
}

func scanProjectRow(rows *sql.Rows) (*domain.Project, error) {
	p := &domain.Project{}
	var createdAt, updatedAt string
	err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.SourceKind,
		&p.RepositoryURL, &p.RepositoryCredential, &p.DefaultBranch, &p.RootDirectory,
		&p.Status, &p.CreatedBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return p, nil
}

// ─── Services ─────────────────────────────────────────────────────────────────

type serviceRepo struct{ db querier }

func (r *serviceRepo) Create(ctx context.Context, s *domain.Service) error {
	if s.ID == "" {
		s.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO services (id,project_id,name,slug,kind,desired_state,runtime_status,
		  internal_port,healthcheck_path,auto_deploy,resource_json,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.ProjectID, s.Name, s.Slug, s.Kind, s.DesiredState, s.RuntimeStatus,
		s.InternalPort, s.HealthcheckPath, s.AutoDeploy, s.ResourceJSON, s.CreatedBy, now, now,
	)
	return err
}

func (r *serviceRepo) GetByID(ctx context.Context, id string) (*domain.Service, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,project_id,name,slug,kind,desired_state,runtime_status,
		       internal_port,healthcheck_path,auto_deploy,resource_json,created_by,created_at,updated_at
		FROM services WHERE id=? OR slug=?`, id, id)
	return scanService(row)
}

func (r *serviceRepo) ListForProject(ctx context.Context, projectID string) ([]*domain.Service, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,project_id,name,slug,kind,desired_state,runtime_status,
		       internal_port,healthcheck_path,auto_deploy,resource_json,created_by,created_at,updated_at
		FROM services WHERE (project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?)) AND runtime_status != 'deleting'
		ORDER BY created_at ASC`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Service
	for rows.Next() {
		s := &domain.Service{}
		var createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Slug, &s.Kind, &s.DesiredState, &s.RuntimeStatus,
			&s.InternalPort, &s.HealthcheckPath, &s.AutoDeploy, &s.ResourceJSON, &s.CreatedBy, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *serviceRepo) Update(ctx context.Context, s *domain.Service) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE services SET name=?,desired_state=?,runtime_status=?,
		  internal_port=?,healthcheck_path=?,auto_deploy=?,resource_json=?,updated_at=?
		WHERE id=?`,
		s.Name, s.DesiredState, s.RuntimeStatus,
		s.InternalPort, s.HealthcheckPath, s.AutoDeploy, s.ResourceJSON, now, s.ID,
	)
	return err
}

func (r *serviceRepo) Delete(ctx context.Context, id string) error {
	if id == "" || id == "undefined" {
		return nil
	}
	_, _ = r.db.ExecContext(ctx, `DELETE FROM deployments WHERE service_id=? OR service_id IN (SELECT id FROM services WHERE slug=?)`, id, id)
	_, err := r.db.ExecContext(ctx, `DELETE FROM services WHERE id=? OR slug=?`, id, id)
	return err
}

func scanService(row *sql.Row) (*domain.Service, error) {
	s := &domain.Service{}
	var createdAt, updatedAt string
	err := row.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Slug, &s.Kind, &s.DesiredState, &s.RuntimeStatus,
		&s.InternalPort, &s.HealthcheckPath, &s.AutoDeploy, &s.ResourceJSON, &s.CreatedBy, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "service"}
	}
	if err != nil {
		return nil, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return s, nil
}

func nullableTimeString(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func parseNullableTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, *s)
		if err != nil {
			return nil
		}
	}
	return &t
}

// ─── Deployments ──────────────────────────────────────────────────────────────

type deploymentRepo struct{ db querier }

func (r *deploymentRepo) Create(ctx context.Context, d *domain.Deployment) error {
	if d.ID == "" {
		d.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	startedAt := nullableTimeString(d.StartedAt)
	finishedAt := nullableTimeString(d.FinishedAt)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deployments (id,service_id,revision_id,sequence,trigger,triggered_by,
		  status,build_driver,config_snapshot,image_ref,image_digest,docker_container_id,
		  started_at,finished_at,exit_code,error_code,error_summary,rollback_of,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID, d.ServiceID, d.RevisionID, d.Sequence, d.Trigger, d.TriggeredBy,
		d.Status, d.BuildDriver, d.ConfigSnapshot, d.ImageRef, d.ImageDigest, d.DockerContainer,
		startedAt, finishedAt, d.ExitCode, d.ErrorCode, d.ErrorSummary, d.RollbackOf, now, now,
	)
	return err
}

func (r *deploymentRepo) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,service_id,revision_id,sequence,trigger,triggered_by,
		       status,build_driver,config_snapshot,image_ref,image_digest,docker_container_id,
		       started_at,finished_at,exit_code,error_code,error_summary,rollback_of,created_at,updated_at
		FROM deployments WHERE id=?`, id)
	return scanDeployment(row)
}

func (r *deploymentRepo) ListForService(ctx context.Context, serviceID string, limit int, cursor *string) ([]*domain.Deployment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,service_id,revision_id,sequence,trigger,triggered_by,
		       status,build_driver,config_snapshot,image_ref,image_digest,docker_container_id,
		       started_at,finished_at,exit_code,error_code,error_summary,rollback_of,created_at,updated_at
		FROM deployments WHERE service_id=? OR service_id IN (SELECT id FROM services WHERE slug=?)
		ORDER BY sequence DESC LIMIT ?`, serviceID, serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Deployment
	for rows.Next() {
		d, err := scanDeploymentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *deploymentRepo) Update(ctx context.Context, d *domain.Deployment) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	startedAt := nullableTimeString(d.StartedAt)
	finishedAt := nullableTimeString(d.FinishedAt)
	_, err := r.db.ExecContext(ctx, `
		UPDATE deployments SET status=?,image_ref=?,image_digest=?,docker_container_id=?,
		  started_at=?,finished_at=?,exit_code=?,error_code=?,error_summary=?,updated_at=?
		WHERE id=?`,
		d.Status, d.ImageRef, d.ImageDigest, d.DockerContainer,
		startedAt, finishedAt, d.ExitCode, d.ErrorCode, d.ErrorSummary,
		now, d.ID,
	)
	return err
}

func (r *deploymentRepo) GetLatestHealthy(ctx context.Context, serviceID string) (*domain.Deployment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,service_id,revision_id,sequence,trigger,triggered_by,
		       status,build_driver,config_snapshot,image_ref,image_digest,docker_container_id,
		       started_at,finished_at,exit_code,error_code,error_summary,rollback_of,created_at,updated_at
		FROM deployments WHERE (service_id=? OR service_id IN (SELECT id FROM services WHERE slug=?)) AND status='healthy'
		ORDER BY sequence DESC LIMIT 1`, serviceID, serviceID)
	return scanDeployment(row)
}

func (r *deploymentRepo) GetNextSequence(ctx context.Context, serviceID string) (int64, error) {
	row := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) + 1 FROM deployments WHERE service_id=? OR service_id IN (SELECT id FROM services WHERE slug=?)`, serviceID, serviceID)
	var nextSeq int64
	if err := row.Scan(&nextSeq); err != nil {
		return 1, nil
	}
	return nextSeq, nil
}

func scanDeployment(row *sql.Row) (*domain.Deployment, error) {
	d := &domain.Deployment{}
	var createdAt, updatedAt string
	var startedAt, finishedAt *string
	err := row.Scan(&d.ID, &d.ServiceID, &d.RevisionID, &d.Sequence, &d.Trigger, &d.TriggeredBy,
		&d.Status, &d.BuildDriver, &d.ConfigSnapshot, &d.ImageRef, &d.ImageDigest, &d.DockerContainer,
		&startedAt, &finishedAt, &d.ExitCode, &d.ErrorCode, &d.ErrorSummary, &d.RollbackOf, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "deployment"}
	}
	if err != nil {
		return nil, err
	}
	d.StartedAt = parseNullableTime(startedAt)
	d.FinishedAt = parseNullableTime(finishedAt)
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return d, nil
}

func scanDeploymentRow(rows *sql.Rows) (*domain.Deployment, error) {
	d := &domain.Deployment{}
	var createdAt, updatedAt string
	var startedAt, finishedAt *string
	err := rows.Scan(&d.ID, &d.ServiceID, &d.RevisionID, &d.Sequence, &d.Trigger, &d.TriggeredBy,
		&d.Status, &d.BuildDriver, &d.ConfigSnapshot, &d.ImageRef, &d.ImageDigest, &d.DockerContainer,
		&startedAt, &finishedAt, &d.ExitCode, &d.ErrorCode, &d.ErrorSummary, &d.RollbackOf, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	d.StartedAt = parseNullableTime(startedAt)
	d.FinishedAt = parseNullableTime(finishedAt)
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return d, nil
}

// ─── Databases ────────────────────────────────────────────────────────────────

type databaseRepo struct{ db querier }

func (r *databaseRepo) Create(ctx context.Context, db *domain.Database) error {
	if db.ID == "" {
		db.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO databases (id,project_id,name,engine,engine_version,image_digest,runtime_status,
		  internal_hostname,internal_port,database_name,credential_secret_id,resource_json,backup_policy_json,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		db.ID, db.ProjectID, db.Name, db.Engine, db.EngineVersion, db.ImageDigest, db.RuntimeStatus,
		db.InternalHostname, db.InternalPort, db.DatabaseName, db.CredentialSecret,
		db.ResourceJSON, db.BackupPolicyJSON, now, now,
	)
	return err
}

func (r *databaseRepo) GetByID(ctx context.Context, id string) (*domain.Database, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,project_id,name,engine,engine_version,image_digest,runtime_status,
		       internal_hostname,internal_port,database_name,credential_secret_id,resource_json,backup_policy_json,created_at,updated_at
		FROM databases WHERE id=? OR name=? OR internal_hostname=?`, id, id, id)
	d := &domain.Database{}
	var createdAt, updatedAt string
	err := row.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Engine, &d.EngineVersion, &d.ImageDigest, &d.RuntimeStatus,
		&d.InternalHostname, &d.InternalPort, &d.DatabaseName, &d.CredentialSecret,
		&d.ResourceJSON, &d.BackupPolicyJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "database"}
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return d, nil
}

func (r *databaseRepo) ListForProject(ctx context.Context, projectID string) ([]*domain.Database, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,project_id,name,engine,engine_version,image_digest,runtime_status,
		       internal_hostname,internal_port,database_name,credential_secret_id,resource_json,backup_policy_json,created_at,updated_at
		FROM databases WHERE (project_id=? OR project_id IN (SELECT id FROM projects WHERE slug=?)) AND runtime_status != 'deleting'
		ORDER BY created_at ASC`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Database
	for rows.Next() {
		d := &domain.Database{}
		var createdAt, updatedAt string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Engine, &d.EngineVersion, &d.ImageDigest, &d.RuntimeStatus,
			&d.InternalHostname, &d.InternalPort, &d.DatabaseName, &d.CredentialSecret,
			&d.ResourceJSON, &d.BackupPolicyJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *databaseRepo) ListAll(ctx context.Context) ([]*domain.Database, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,project_id,name,engine,engine_version,image_digest,runtime_status,
		       internal_hostname,internal_port,database_name,credential_secret_id,resource_json,backup_policy_json,created_at,updated_at
		FROM databases WHERE runtime_status != 'deleting'
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Database
	for rows.Next() {
		d := &domain.Database{}
		var createdAt, updatedAt string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Engine, &d.EngineVersion, &d.ImageDigest, &d.RuntimeStatus,
			&d.InternalHostname, &d.InternalPort, &d.DatabaseName, &d.CredentialSecret,
			&d.ResourceJSON, &d.BackupPolicyJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *databaseRepo) Update(ctx context.Context, db *domain.Database) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE databases SET runtime_status=?,resource_json=?,backup_policy_json=?,updated_at=? WHERE id=?`,
		db.RuntimeStatus, db.ResourceJSON, db.BackupPolicyJSON, now, db.ID,
	)
	return err
}

func (r *databaseRepo) Delete(ctx context.Context, id string) error {
	if id == "" || id == "undefined" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM databases WHERE id=? OR name=?`, id, id)
	return err
}

// ─── Jobs ─────────────────────────────────────────────────────────────────────

type jobRepo struct{ db querier }

func (r *jobRepo) Enqueue(ctx context.Context, j *domain.Job) error {
	if j.ID == "" {
		j.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO jobs (id,kind,dedupe_key,payload,status,attempts,max_attempts,run_after,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Kind, j.DedupeKey, j.Payload, domain.JobQueued, 0, j.MaxAttempts,
		j.RunAfter.UTC().Format(time.RFC3339Nano), now, now,
	)
	return err
}

func (r *jobRepo) ClaimNext(ctx context.Context, workerID string, kinds []string) (*domain.Job, error) {
	// Note: This is a simplified implementation; production uses a proper lock
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := r.db.QueryRowContext(ctx, `
		SELECT id FROM jobs WHERE status='queued' AND run_after <= ? AND kind IN (
			SELECT value FROM json_each(?)
		) ORDER BY run_after ASC LIMIT 1`, now, `["`+joinKinds(kinds)+`"]`)
	var id string
	if err := row.Scan(&id); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE jobs SET status='running',locked_at=?,locked_by=?,attempts=attempts+1,updated_at=?
		WHERE id=? AND status='queued'`,
		now, workerID, now, id,
	)
	if err != nil {
		return nil, err
	}
	return r.getByID(ctx, id)
}

func (r *jobRepo) getByID(ctx context.Context, id string) (*domain.Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id,kind,dedupe_key,payload,status,attempts,max_attempts,run_after,locked_at,locked_by,last_error,created_at,updated_at
		FROM jobs WHERE id=?`, id)
	j := &domain.Job{}
	var runAfter, createdAt, updatedAt string
	if err := row.Scan(&j.ID, &j.Kind, &j.DedupeKey, &j.Payload, &j.Status, &j.Attempts, &j.MaxAttempts,
		&runAfter, &j.LockedAt, &j.LockedBy, &j.LastError, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	j.RunAfter, _ = time.Parse(time.RFC3339Nano, runAfter)
	j.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return j, nil
}

func (r *jobRepo) MarkSucceeded(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE jobs SET status='succeeded',updated_at=? WHERE id=?`, now, id)
	return err
}

func (r *jobRepo) MarkFailed(ctx context.Context, id, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE jobs SET status='failed',last_error=?,updated_at=? WHERE id=?`, errMsg, now, id)
	return err
}

func (r *jobRepo) GetByDedupeKey(ctx context.Context, key string) (*domain.Job, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id FROM jobs WHERE dedupe_key=?`, key)
	var id string
	if err := row.Scan(&id); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return r.getByID(ctx, id)
}

func joinKinds(kinds []string) string {
	result := ""
	for i, k := range kinds {
		if i > 0 {
			result += `","` 
		}
		result += k
	}
	return result
}

// ─── Audit Events ─────────────────────────────────────────────────────────────

type auditRepo struct{ db querier }

func (r *auditRepo) Append(ctx context.Context, e *domain.AuditEvent) error {
	if e.ID == "" {
		e.ID = ids.NewV7()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (id,actor_user_id,actor_kind,workspace_id,action,
		  target_type,target_id,request_id,ip_hash,metadata,occurred_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.ActorUserID, e.ActorKind, e.WorkspaceID, e.Action,
		e.TargetType, e.TargetID, e.RequestID, e.IPHash, e.Metadata,
		e.OccurredAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (r *auditRepo) List(ctx context.Context, workspaceID *string, limit int, cursor *string) ([]*domain.AuditEvent, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if workspaceID != nil {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id,actor_user_id,actor_kind,workspace_id,action,target_type,target_id,request_id,ip_hash,metadata,occurred_at
			FROM audit_events WHERE workspace_id=? ORDER BY occurred_at DESC LIMIT ?`, *workspaceID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id,actor_user_id,actor_kind,workspace_id,action,target_type,target_id,request_id,ip_hash,metadata,occurred_at
			FROM audit_events ORDER BY occurred_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.AuditEvent
	for rows.Next() {
		e := &domain.AuditEvent{}
		var occurredAt string
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.ActorKind, &e.WorkspaceID, &e.Action,
			&e.TargetType, &e.TargetID, &e.RequestID, &e.IPHash, &e.Metadata, &occurredAt); err != nil {
			return nil, err
		}
		e.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurredAt)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ─── Git Integrations ─────────────────────────────────────────────────────────

type gitIntegrationRepo struct{ db querier }

func (r *gitIntegrationRepo) Get(ctx context.Context, userID, provider string) (*domain.UserGitIntegration, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, provider, username, token, avatar_url, scopes, connected_at, updated_at
		FROM user_git_integrations
		WHERE user_id=? AND provider=?`, userID, provider)
	item := &domain.UserGitIntegration{}
	var connectedAt, updatedAt string
	err := row.Scan(&item.UserID, &item.Provider, &item.Username, &item.Token, &item.AvatarURL, &item.Scopes, &connectedAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "git_integration"}
	}
	if err != nil {
		return nil, err
	}
	item.ConnectedAt, _ = time.Parse(time.RFC3339Nano, connectedAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return item, nil
}

func (r *gitIntegrationRepo) ListForUser(ctx context.Context, userID string) ([]*domain.UserGitIntegration, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, provider, username, token, avatar_url, scopes, connected_at, updated_at
		FROM user_git_integrations
		WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.UserGitIntegration
	for rows.Next() {
		item := &domain.UserGitIntegration{}
		var connectedAt, updatedAt string
		if err := rows.Scan(&item.UserID, &item.Provider, &item.Username, &item.Token, &item.AvatarURL, &item.Scopes, &connectedAt, &updatedAt); err != nil {
			return nil, err
		}
		item.ConnectedAt, _ = time.Parse(time.RFC3339Nano, connectedAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *gitIntegrationRepo) Upsert(ctx context.Context, item *domain.UserGitIntegration) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_git_integrations (user_id, provider, username, token, avatar_url, scopes, connected_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET
			username=excluded.username,
			token=excluded.token,
			avatar_url=excluded.avatar_url,
			scopes=excluded.scopes,
			updated_at=excluded.updated_at`,
		item.UserID, item.Provider, item.Username, item.Token, item.AvatarURL, item.Scopes, now, now,
	)
	return err
}

func (r *gitIntegrationRepo) Delete(ctx context.Context, userID, provider string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_git_integrations WHERE user_id=? AND provider=?`, userID, provider)
	return err
}

// ─── Auth Sessions ───────────────────────────────────────────────────────────

type authSessionRepo struct{ db querier }

func (r *authSessionRepo) Create(ctx context.Context, sessionID, userID, token string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	exp := expiresAt.UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (id, user_id, token_hash, csrf_secret_hash, expires_at, last_seen_at, created_at, updated_at)
		VALUES (?, ?, ?, '', ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET token_hash=excluded.token_hash, expires_at=excluded.expires_at, updated_at=excluded.updated_at`,
		sessionID, userID, token, exp, now, now, now,
	)
	return err
}

func (r *authSessionRepo) GetByToken(ctx context.Context, token string) (string, time.Time, error) {
	var userID, expStr string
	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, expires_at FROM auth_sessions 
		WHERE (token_hash=? OR id=?) AND revoked_at IS NULL`,
		token, token,
	).Scan(&userID, &expStr)
	if err != nil {
		return "", time.Time{}, err
	}
	exp, err := time.Parse(time.RFC3339Nano, expStr)
	if err != nil {
		exp, err = time.Parse(time.RFC3339, expStr)
		if err != nil {
			exp = time.Now().Add(30 * 24 * time.Hour)
		}
	}
	return userID, exp, nil
}

func (r *authSessionRepo) DeleteByToken(ctx context.Context, token string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE auth_sessions SET revoked_at=? WHERE token_hash=? OR id=?`,
		now, token, token,
	)
	return err
}
