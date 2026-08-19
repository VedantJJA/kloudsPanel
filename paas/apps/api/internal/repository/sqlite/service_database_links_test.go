package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
)

func TestServiceDatabaseLinkRepo_CRUD(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_sdl.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Skipf("skipping SQLite test: %v", err)
		return
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Skipf("skipping SQLite migration test: %v", err)
		return
	}

	st := NewStore(db)

	// Create test user, workspace, project, service, database
	u := &domain.User{
		ID:           "u-test-1",
		Email:        "user@test.com",
		DisplayName:  "Test User",
		PasswordHash: "hash",
		Status:       domain.UserStatusActive,
		PlatformRole: domain.PlatformRoleUser,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := st.Users().Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	w := &domain.Workspace{
		ID:        "ws-test-1",
		Name:      "Test Workspace",
		Slug:      "test-ws",
		CreatedBy: u.ID,
		Status:    domain.WorkspaceStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := st.Workspaces().Create(ctx, w); err != nil {
		t.Fatalf("Create workspace: %v", err)
	}

	p := &domain.Project{
		ID:          "p-test-1",
		WorkspaceID: w.ID,
		Name:        "Test Project",
		Slug:        "test-proj",
		Status:      domain.ProjectStatusActive,
		SourceKind:  domain.SourceKindEmpty,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := st.Projects().Create(ctx, p); err != nil {
		t.Fatalf("Create project: %v", err)
	}

	svc := &domain.Service{
		ID:            "svc-test-1",
		ProjectID:     p.ID,
		Name:          "Web App",
		Slug:          "web-app",
		Kind:          domain.ServiceKindWeb,
		RuntimeStatus: domain.ServiceStatusRunning,
		DesiredState:  domain.ServiceDesiredRunning,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := st.Services().Create(ctx, svc); err != nil {
		t.Fatalf("Create service: %v", err)
	}

	d := &domain.Database{
		ID:               "db-test-1",
		ProjectID:        p.ID,
		Name:             "Main DB",
		Engine:           "postgres",
		RuntimeStatus:    domain.DBStatusReady,
		InternalHostname: "paas-db-main-db",
		InternalPort:     5432,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := st.Databases().Create(ctx, d); err != nil {
		t.Fatalf("Create database: %v", err)
	}

	// 1. Create ServiceDatabaseLink
	link := &domain.ServiceDatabaseLink{
		ID:             "link-1",
		ServiceID:      svc.ID,
		DatabaseID:     d.ID,
		EnvVarName:     "DATABASE_URL",
		ConnectionKind: domain.ConnectionInternal,
		Property:       "connectionString",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := st.ServiceDatabaseLinks().Create(ctx, link); err != nil {
		t.Fatalf("Create link: %v", err)
	}

	// 2. GetByID
	fetched, err := st.ServiceDatabaseLinks().GetByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetByID link: %v", err)
	}
	if fetched.EnvVarName != "DATABASE_URL" || fetched.ConnectionKind != domain.ConnectionInternal || fetched.Property != "connectionString" {
		t.Fatalf("unexpected fetched link: %+v", fetched)
	}

	// 3. ListForService
	svcLinks, err := st.ServiceDatabaseLinks().ListForService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("ListForService: %v", err)
	}
	if len(svcLinks) != 1 || svcLinks[0].ID != link.ID {
		t.Fatalf("expected 1 service link, got %d", len(svcLinks))
	}

	// 4. ListForDatabase
	dbLinks, err := st.ServiceDatabaseLinks().ListForDatabase(ctx, d.ID)
	if err != nil {
		t.Fatalf("ListForDatabase: %v", err)
	}
	if len(dbLinks) != 1 || dbLinks[0].ID != link.ID {
		t.Fatalf("expected 1 database link, got %d", len(dbLinks))
	}

	// 5. Unique constraint on (service_id, env_var_name)
	dupLink := &domain.ServiceDatabaseLink{
		ID:             "link-dup",
		ServiceID:      svc.ID,
		DatabaseID:     d.ID,
		EnvVarName:     "DATABASE_URL",
		ConnectionKind: domain.ConnectionExternal,
		Property:       "host",
	}
	if err := st.ServiceDatabaseLinks().Create(ctx, dupLink); err == nil {
		t.Fatalf("expected error on duplicate (service_id, env_var_name), got nil")
	}

	// 6. Delete
	if err := st.ServiceDatabaseLinks().Delete(ctx, link.ID); err != nil {
		t.Fatalf("Delete link: %v", err)
	}

	// 7. Verify deletion
	_, err = st.ServiceDatabaseLinks().GetByID(ctx, link.ID)
	if err == nil {
		t.Fatalf("expected not found error after delete, got nil")
	}
}
