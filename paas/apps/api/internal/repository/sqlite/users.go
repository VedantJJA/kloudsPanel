package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

type userRepo struct{ db querier }

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	if u.ID == "" {
		u.ID = ids.NewV7()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id,email,display_name,password_hash,status,platform_role,created_at,updated_at)
		VALUES (?,lower(?),?,?,?,?,?,?)`,
		u.ID, u.Email, u.DisplayName, u.PasswordHash, u.Status, u.PlatformRole, now, now,
	)
	if err != nil {
		return fmt.Errorf("user create: %w", err)
	}
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,email,display_name,password_hash,status,platform_role,
		        email_verified_at,approved_by,approved_at,last_login_at,created_at,updated_at
		 FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,email,display_name,password_hash,status,platform_role,
		        email_verified_at,approved_by,approved_at,last_login_at,created_at,updated_at
		 FROM users WHERE lower(email)=lower(?)`, email)
	return scanUser(row)
}

func (r *userRepo) Update(ctx context.Context, u *domain.User) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET display_name=?,status=?,platform_role=?,
		  email_verified_at=?,approved_by=?,approved_at=?,last_login_at=?,updated_at=?
		WHERE id=?`,
		u.DisplayName, u.Status, u.PlatformRole,
		nullableTime(u.EmailVerifiedAt), u.ApprovedBy, nullableTime(u.ApprovedAt), nullableTime(u.LastLoginAt),
		now, u.ID,
	)
	return err
}

func (r *userRepo) ListPending(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,email,display_name,password_hash,status,platform_role,
		        email_verified_at,approved_by,approved_at,last_login_at,created_at,updated_at
		 FROM users WHERE status='pending' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *userRepo) ListAll(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,email,display_name,password_hash,status,platform_role,
		        email_verified_at,approved_by,approved_at,last_login_at,created_at,updated_at
		 FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *userRepo) CountByRole(ctx context.Context, role domain.PlatformRole) (int, error) {
	var n int
	row := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE platform_role=?`, role)
	return n, row.Scan(&n)
}

func scanUser(row *sql.Row) (*domain.User, error) {
	u := &domain.User{}
	var evAt, appBy, appAt, llAt sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Status, &u.PlatformRole,
		&evAt, &appBy, &appAt, &llAt, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Resource: "user"}
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if evAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, evAt.String)
		u.EmailVerifiedAt = &t
	}
	if appBy.Valid {
		u.ApprovedBy = &appBy.String
	}
	if appAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, appAt.String)
		u.ApprovedAt = &t
	}
	if llAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, llAt.String)
		u.LastLoginAt = &t
	}
	return u, nil
}

func scanUsers(rows *sql.Rows) ([]*domain.User, error) {
	var out []*domain.User
	for rows.Next() {
		u := &domain.User{}
		var evAt, appBy, appAt, llAt sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Status, &u.PlatformRole,
			&evAt, &appBy, &appAt, &llAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, u)
	}
	return out, rows.Err()
}

func nullableTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{Valid: true, String: t.UTC().Format(time.RFC3339Nano)}
}
