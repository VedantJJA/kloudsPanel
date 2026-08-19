package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/repository"
)

// --- Mock In-Memory Store for API Tests ---

type mockStore struct {
	services    map[string]*domain.Service
	databases   map[string]*domain.Database
	links       map[string]*domain.ServiceDatabaseLink
	deployments map[string]*domain.Deployment
	mu          sync.RWMutex
}

func newMockStore() *mockStore {
	return &mockStore{
		services:    make(map[string]*domain.Service),
		databases:   make(map[string]*domain.Database),
		links:       make(map[string]*domain.ServiceDatabaseLink),
		deployments: make(map[string]*domain.Deployment),
	}
}

func (m *mockStore) Users() repository.UserRepository               { return nil }
func (m *mockStore) AuthSessions() repository.AuthSessionRepository   { return nil }
func (m *mockStore) Workspaces() repository.WorkspaceRepository       { return nil }
func (m *mockStore) Projects() repository.ProjectRepository           { return nil }
func (m *mockStore) Jobs() repository.JobRepository                   { return nil }
func (m *mockStore) AuditEvents() repository.AuditRepository         { return nil }
func (m *mockStore) GitIntegrations() repository.GitIntegrationRepository { return nil }
func (m *mockStore) WithTx(ctx context.Context, fn func(repository.Store) error) error {
	return fn(m)
}

func (m *mockStore) Services() repository.ServiceRepository {
	return &mockServiceRepo{m: m}
}

func (m *mockStore) Databases() repository.DatabaseRepository {
	return &mockDatabaseRepo{m: m}
}

func (m *mockStore) Deployments() repository.DeploymentRepository {
	return &mockDeploymentRepo{m: m}
}

func (m *mockStore) ServiceDatabaseLinks() repository.ServiceDatabaseLinkRepository {
	return &mockLinkRepo{m: m}
}

// Mock Service Repo
type mockServiceRepo struct{ m *mockStore }

func (r *mockServiceRepo) Create(ctx context.Context, s *domain.Service) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.services[s.ID] = s
	return nil
}
func (r *mockServiceRepo) GetByID(ctx context.Context, id string) (*domain.Service, error) {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	s, ok := r.m.services[id]
	if !ok {
		return nil, domain.ErrNotFound{Resource: "service"}
	}
	return s, nil
}
func (r *mockServiceRepo) GetByProjectAndSlug(ctx context.Context, projectID, slug string) (*domain.Service, error) {
	return nil, nil
}
func (r *mockServiceRepo) ListForProject(ctx context.Context, projectID string) ([]*domain.Service, error) {
	return nil, nil
}
func (r *mockServiceRepo) ListAll(ctx context.Context) ([]*domain.Service, error) {
	return nil, nil
}
func (r *mockServiceRepo) Update(ctx context.Context, s *domain.Service) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.services[s.ID] = s
	return nil
}
func (r *mockServiceRepo) Delete(ctx context.Context, id string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	delete(r.m.services, id)
	return nil
}
func (r *mockServiceRepo) SlugExists(ctx context.Context, slug string) (bool, error) {
	return false, nil
}

// Mock Database Repo
type mockDatabaseRepo struct{ m *mockStore }

func (r *mockDatabaseRepo) Create(ctx context.Context, db *domain.Database) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.databases[db.ID] = db
	return nil
}
func (r *mockDatabaseRepo) GetByID(ctx context.Context, id string) (*domain.Database, error) {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	db, ok := r.m.databases[id]
	if !ok {
		return nil, domain.ErrNotFound{Resource: "database"}
	}
	return db, nil
}
func (r *mockDatabaseRepo) ListForProject(ctx context.Context, projectID string) ([]*domain.Database, error) {
	return nil, nil
}
func (r *mockDatabaseRepo) ListAll(ctx context.Context) ([]*domain.Database, error) {
	return nil, nil
}
func (r *mockDatabaseRepo) Update(ctx context.Context, db *domain.Database) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.databases[db.ID] = db
	return nil
}
func (r *mockDatabaseRepo) Delete(ctx context.Context, id string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	delete(r.m.databases, id)
	return nil
}

// Mock Deployment Repo
type mockDeploymentRepo struct{ m *mockStore }

func (r *mockDeploymentRepo) Create(ctx context.Context, d *domain.Deployment) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.deployments[d.ID] = d
	return nil
}
func (r *mockDeploymentRepo) GetByID(ctx context.Context, id string) (*domain.Deployment, error) {
	return nil, nil
}
func (r *mockDeploymentRepo) ListForService(ctx context.Context, serviceID string, limit int, cursor *string) ([]*domain.Deployment, error) {
	return nil, nil
}
func (r *mockDeploymentRepo) Update(ctx context.Context, d *domain.Deployment) error {
	return nil
}
func (r *mockDeploymentRepo) GetLatestHealthy(ctx context.Context, serviceID string) (*domain.Deployment, error) {
	return nil, nil
}
func (r *mockDeploymentRepo) GetNextSequence(ctx context.Context, serviceID string) (int64, error) {
	return 1, nil
}

// Mock Link Repo
type mockLinkRepo struct{ m *mockStore }

func (r *mockLinkRepo) Create(ctx context.Context, link *domain.ServiceDatabaseLink) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	// Unique constraint check: (service_id, env_var_name)
	for _, existing := range r.m.links {
		if existing.ServiceID == link.ServiceID && existing.EnvVarName == link.EnvVarName {
			return fmt.Errorf("UNIQUE constraint failed: service_database_links.service_id, service_database_links.env_var_name")
		}
	}
	r.m.links[link.ID] = link
	return nil
}
func (r *mockLinkRepo) GetByID(ctx context.Context, id string) (*domain.ServiceDatabaseLink, error) {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	l, ok := r.m.links[id]
	if !ok {
		return nil, domain.ErrNotFound{Resource: "service_database_link"}
	}
	return l, nil
}
func (r *mockLinkRepo) ListForService(ctx context.Context, serviceID string) ([]*domain.ServiceDatabaseLink, error) {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	var out []*domain.ServiceDatabaseLink
	for _, l := range r.m.links {
		if l.ServiceID == serviceID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *mockLinkRepo) ListForDatabase(ctx context.Context, databaseID string) ([]*domain.ServiceDatabaseLink, error) {
	r.m.mu.RLock()
	defer r.m.mu.RUnlock()
	var out []*domain.ServiceDatabaseLink
	for _, l := range r.m.links {
		if l.DatabaseID == databaseID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *mockLinkRepo) Delete(ctx context.Context, id string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	if _, ok := r.m.links[id]; !ok {
		return domain.ErrNotFound{Resource: "service_database_link"}
	}
	delete(r.m.links, id)
	return nil
}

// --- API Test Cases ---

func setupTestApp(st repository.Store) *fiber.App {
	app := fiber.New()
	h := &Handler{store: st}

	v1 := app.Group("/api/v1")
	v1.Post("/services/:id/database-links", h.handleCreateServiceDatabaseLink)
	v1.Get("/services/:id/database-links", h.handleListServiceDatabaseLinks)
	v1.Delete("/services/:id/database-links/:linkId", h.handleDeleteServiceDatabaseLink)
	v1.Get("/databases/:id/linked-services", h.handleGetDatabaseLinkedServices)

	return app
}

func TestAPICreateDatabaseLink_HappyPath(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	svc := &domain.Service{
		ID:        "svc-app-1",
		ProjectID: "proj-1",
		Name:      "Web App",
		Slug:      "web-app",
	}
	_ = st.Services().Create(context.Background(), svc)

	db := &domain.Database{
		ID:               "db-pg-1",
		ProjectID:        "proj-1",
		Name:             "Main DB",
		Engine:           "postgres",
		InternalHostname: "paas-db-main-db",
		InternalPort:     5432,
		ResourceJSON:     `{"username":"postgres","password":"mypassword","databaseName":"mydb"}`,
	}
	_ = st.Databases().Create(context.Background(), db)

	body := map[string]string{
		"databaseId":     db.ID,
		"envVarName":     "DATABASE_URL",
		"connectionKind": "internal",
		"property":       "connectionString",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/svc-app-1/database-links", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
	}

	var resData map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&resData)
	if resData["link"] == nil {
		t.Fatalf("expected link in response, got nil")
	}
}

func TestAPICreateDatabaseLink_CrossProjectForbidden(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	svc := &domain.Service{
		ID:        "svc-app-1",
		ProjectID: "proj-alpha",
		Name:      "Web App",
	}
	_ = st.Services().Create(context.Background(), svc)

	db := &domain.Database{
		ID:        "db-pg-2",
		ProjectID: "proj-beta", // Different project!
		Name:      "Other DB",
	}
	_ = st.Databases().Create(context.Background(), db)

	body := map[string]string{
		"databaseId": db.ID,
		"envVarName": "DATABASE_URL",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/svc-app-1/database-links", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request for cross-project link, got %d", resp.StatusCode)
	}
}

func TestAPICreateDatabaseLink_DuplicateEnvVarConflict(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	svc := &domain.Service{
		ID:        "svc-app-1",
		ProjectID: "proj-1",
	}
	_ = st.Services().Create(context.Background(), svc)

	db := &domain.Database{
		ID:        "db-1",
		ProjectID: "proj-1",
		Name:      "DB 1",
	}
	_ = st.Databases().Create(context.Background(), db)

	// Pre-create link with DATABASE_URL
	_ = st.ServiceDatabaseLinks().Create(context.Background(), &domain.ServiceDatabaseLink{
		ID:         "link-existing",
		ServiceID:  svc.ID,
		DatabaseID: db.ID,
		EnvVarName: "DATABASE_URL",
	})

	body := map[string]string{
		"databaseId": db.ID,
		"envVarName": "DATABASE_URL", // Duplicate key for same service
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/svc-app-1/database-links", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict on duplicate env var, got %d", resp.StatusCode)
	}
}

func TestAPIListDatabaseLinks_PasswordRedaction(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	svc := &domain.Service{
		ID:        "svc-app-1",
		ProjectID: "proj-1",
	}
	_ = st.Services().Create(context.Background(), svc)

	db := &domain.Database{
		ID:               "db-1",
		ProjectID:        "proj-1",
		Name:             "Secure DB",
		Engine:           "postgres",
		InternalHostname: "paas-db-secure-db",
		InternalPort:     5432,
		ResourceJSON:     `{"username":"postgres","password":"SecretPassword123","databaseName":"secure_db"}`,
	}
	_ = st.Databases().Create(context.Background(), db)

	_ = st.ServiceDatabaseLinks().Create(context.Background(), &domain.ServiceDatabaseLink{
		ID:             "link-1",
		ServiceID:      svc.ID,
		DatabaseID:     db.ID,
		EnvVarName:     "DATABASE_URL",
		ConnectionKind: domain.ConnectionInternal,
		Property:       "connectionString",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})

	// 1. Default request without reveal: password should be redacted with ********
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/svc-app-1/database-links", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var data struct {
		Links []struct {
			EnvVarName   string `json:"env_var_name"`
			PreviewValue string `json:"preview_value"`
		} `json:"links"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if len(data.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(data.Links))
	}
	if !bytes.Contains([]byte(data.Links[0].PreviewValue), []byte("********")) {
		t.Fatalf("expected password to be masked with ********, got %q", data.Links[0].PreviewValue)
	}

	// 2. Reveal request: password should be revealed
	reqReveal := httptest.NewRequest(http.MethodGet, "/api/v1/services/svc-app-1/database-links?reveal=true", nil)
	respReveal, _ := app.Test(reqReveal)
	var dataReveal struct {
		Links []struct {
			PreviewValue string `json:"preview_value"`
		} `json:"links"`
	}
	_ = json.NewDecoder(respReveal.Body).Decode(&dataReveal)
	if !bytes.Contains([]byte(dataReveal.Links[0].PreviewValue), []byte("SecretPassword123")) {
		t.Fatalf("expected revealed password SecretPassword123, got %q", dataReveal.Links[0].PreviewValue)
	}
}

func TestAPIDeleteDatabaseLink(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	_ = st.ServiceDatabaseLinks().Create(context.Background(), &domain.ServiceDatabaseLink{
		ID:         "link-to-delete",
		ServiceID:  "svc-1",
		DatabaseID: "db-1",
		EnvVarName: "DATABASE_URL",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/services/svc-1/database-links/link-to-delete", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var resData map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&resData)
	if resData["success"] != true {
		t.Fatalf("expected success true, got %v", resData["success"])
	}
}

func TestAPIGetDatabaseLinkedServices(t *testing.T) {
	st := newMockStore()
	app := setupTestApp(st)

	svc := &domain.Service{
		ID:        "svc-backend",
		Name:      "Backend Service",
		Slug:      "backend-svc",
		ProjectID: "proj-1",
	}
	_ = st.Services().Create(context.Background(), svc)

	_ = st.ServiceDatabaseLinks().Create(context.Background(), &domain.ServiceDatabaseLink{
		ID:             "link-back",
		ServiceID:      svc.ID,
		DatabaseID:     "db-shared",
		EnvVarName:     "DATABASE_URL",
		ConnectionKind: domain.ConnectionInternal,
		Property:       "connectionString",
		CreatedAt:      time.Now().UTC(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/databases/db-shared/linked-services", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	var data struct {
		LinkedServices []struct {
			ServiceID   string `json:"service_id"`
			ServiceName string `json:"service_name"`
			EnvVarName  string `json:"env_var_name"`
		} `json:"linkedServices"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if len(data.LinkedServices) != 1 {
		t.Fatalf("expected 1 linked service, got %d", len(data.LinkedServices))
	}
	if data.LinkedServices[0].ServiceID != "svc-backend" || data.LinkedServices[0].ServiceName != "Backend Service" {
		t.Fatalf("unexpected linked service: %+v", data.LinkedServices[0])
	}
}
