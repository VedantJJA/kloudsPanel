package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

type workspaceRepo struct{ db querier }

func (r *workspaceRepo) Create(ctx context.Context, w *domain.Workspace) error {
	if w.ID == "" {
		w.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workspaces (id,name,slug,description,created_by,status,quota_json,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		w.ID, w.Name, w.Slug, w.Description, w.CreatedBy, w.Status, w.QuotaJSON, now, now,
	)
	return err
}

func (r *workspaceRepo) GetByID(ctx context.Context, id string) (*domain.Workspace, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,slug,description,created_by,status,quota_json,created_at,updated_at
		 FROM workspaces WHERE id=?`, id)
	return scanWorkspace(row)
}

func (r *workspaceRepo) GetBySlug(ctx context.Context, slug string) (*domain.Workspace, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,slug,description,created_by,status,quota_json,created_at,updated_at
		 FROM workspaces WHERE slug=?`, slug)
	return scanWorkspace(row)
}

func (r *workspaceRepo) ListForUser(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT w.id,w.name,w.slug,w.description,w.created_by,w.status,w.quota_json,w.created_at,w.updated_at
		 FROM workspaces w
		 JOIN workspace_members m ON m.workspace_id=w.id
		 WHERE m.user_id=? AND m.status='active' AND w.status='active'
		 ORDER BY w.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWorkspaces(rows)
}

func (r *workspaceRepo) Update(ctx context.Context, w *domain.Workspace) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`UPDATE workspaces SET name=?,description=?,status=?,quota_json=?,updated_at=? WHERE id=?`,
		w.Name, w.Description, w.Status, w.QuotaJSON, now, w.ID,
	)
	return err
}

func (r *workspaceRepo) AddMember(ctx context.Context, m *domain.WorkspaceMember) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO workspace_members (workspace_id,user_id,role,status,joined_at,invited_by,version)
		 VALUES (?,?,?,?,?,?,?)`,
		m.WorkspaceID, m.UserID, m.Role, m.Status, m.JoinedAt.UTC().Format(time.RFC3339Nano),
		m.InvitedBy, m.Version,
	)
	return err
}

func (r *workspaceRepo) GetMember(ctx context.Context, workspaceID, userID string) (*domain.WorkspaceMember, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT workspace_id,user_id,role,status,joined_at,invited_by,version
		 FROM workspace_members WHERE workspace_id=? AND user_id=?`, workspaceID, userID)
	m := &domain.WorkspaceMember{}
	var joinedAt string
	err := row.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.Status, &joinedAt, &m.InvitedBy, &m.Version)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "workspace_member"}
	}
	if err != nil {
		return nil, err
	}
	m.JoinedAt, _ = time.Parse(time.RFC3339Nano, joinedAt)
	return m, nil
}

func (r *workspaceRepo) UpdateMember(ctx context.Context, m *domain.WorkspaceMember) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workspace_members SET role=?,status=?,version=version+1 WHERE workspace_id=? AND user_id=?`,
		m.Role, m.Status, m.WorkspaceID, m.UserID,
	)
	return err
}

func (r *workspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]*domain.WorkspaceMember, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT workspace_id,user_id,role,status,joined_at,invited_by,version
		 FROM workspace_members WHERE workspace_id=? AND status='active'`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.WorkspaceMember
	for rows.Next() {
		m := &domain.WorkspaceMember{}
		var joinedAt string
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.Status, &joinedAt, &m.InvitedBy, &m.Version); err != nil {
			return nil, err
		}
		m.JoinedAt, _ = time.Parse(time.RFC3339Nano, joinedAt)
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanWorkspace(row *sql.Row) (*domain.Workspace, error) {
	w := &domain.Workspace{}
	var createdAt, updatedAt string
	err := row.Scan(&w.ID, &w.Name, &w.Slug, &w.Description, &w.CreatedBy, &w.Status, &w.QuotaJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "workspace"}
	}
	if err != nil {
		return nil, err
	}
	w.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	w.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return w, nil
}

func scanWorkspaces(rows *sql.Rows) ([]*domain.Workspace, error) {
	var out []*domain.Workspace
	for rows.Next() {
		w := &domain.Workspace{}
		var createdAt, updatedAt string
		if err := rows.Scan(&w.ID, &w.Name, &w.Slug, &w.Description, &w.CreatedBy, &w.Status, &w.QuotaJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		w.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		w.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, w)
	}
	return out, rows.Err()
}
