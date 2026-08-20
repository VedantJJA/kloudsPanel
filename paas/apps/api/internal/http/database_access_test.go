package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/tcpproxy"
)

func TestAPIGetDatabaseAccess_Defaults(t *testing.T) {
	app := fiber.New()
	store := newMockStore()
	h := &Handler{store: store}

	db := &domain.Database{
		ID:               "db-access-test-1",
		ProjectID:        "proj-1",
		Name:             "testdb",
		Engine:           "postgres",
		EngineVersion:    "16",
		InternalHostname: "paas-db-testdb",
		InternalPort:     5432,
		ResourceJSON:     `{"externalPort":15432,"publicAccess":true,"ipWhitelist":[{"cidr":"0.0.0.0/0","description":"Anywhere"}]}`,
		CreatedAt:        time.Now().UTC(),
	}
	_ = store.Databases().Create(context.Background(), db)

	app.Get("/api/v1/databases/:id/access", h.handleGetDatabaseAccess)

	req := httptest.NewRequest("GET", "/api/v1/databases/db-access-test-1/access", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data struct {
		DatabaseID   string            `json:"databaseId"`
		PublicAccess bool              `json:"publicAccess"`
		IPWhitelist  []tcpproxy.IPRule `json:"ipWhitelist"`
		ExternalPort int               `json:"externalPort"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&data)

	if data.DatabaseID != "db-access-test-1" {
		t.Errorf("expected databaseId 'db-access-test-1', got %s", data.DatabaseID)
	}
	if !data.PublicAccess {
		t.Errorf("expected publicAccess true, got %v", data.PublicAccess)
	}
	if len(data.IPWhitelist) == 0 || data.IPWhitelist[0].CIDR != "0.0.0.0/0" {
		t.Errorf("expected default 0.0.0.0/0 rule, got %+v", data.IPWhitelist)
	}
}

func TestAPIUpdateDatabaseAccess_Whitelisting(t *testing.T) {
	app := fiber.New()
	store := newMockStore()
	h := &Handler{store: store}

	db := &domain.Database{
		ID:               "db-access-test-2",
		ProjectID:        "proj-1",
		Name:             "testdb2",
		Engine:           "postgres",
		EngineVersion:    "16",
		InternalHostname: "paas-db-testdb2",
		InternalPort:     5432,
		ResourceJSON:     `{"externalPort":15433}`,
		CreatedAt:        time.Now().UTC(),
	}
	_ = store.Databases().Create(context.Background(), db)

	app.Put("/api/v1/databases/:id/access", h.handleUpdateDatabaseAccess)

	body := map[string]any{
		"publicAccess": true,
		"ipWhitelist": []map[string]any{
			{"cidr": "192.168.1.50/32", "description": "Office VPN"},
			{"cidr": "10.0.0.0/16", "description": "Private Subnet"},
		},
	}
	payloadBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PUT", "/api/v1/databases/db-access-test-2/access", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	updatedDb, _ := store.Databases().GetByID(context.Background(), "db-access-test-2")
	var meta map[string]any
	_ = json.Unmarshal([]byte(updatedDb.ResourceJSON), &meta)

	if meta["publicAccess"] != true {
		t.Errorf("expected publicAccess true in DB, got %v", meta["publicAccess"])
	}

	rawRules, ok := meta["ipWhitelist"].([]any)
	if !ok || len(rawRules) != 2 {
		t.Fatalf("expected 2 rules in DB, got %+v", meta["ipWhitelist"])
	}
}

func TestAPIAddAndDeleteDatabaseIPWhitelist(t *testing.T) {
	app := fiber.New()
	store := newMockStore()
	h := &Handler{store: store}

	db := &domain.Database{
		ID:               "db-access-test-3",
		ProjectID:        "proj-1",
		Name:             "testdb3",
		Engine:           "postgres",
		EngineVersion:    "16",
		InternalHostname: "paas-db-testdb3",
		InternalPort:     5432,
		ResourceJSON:     `{"externalPort":15434,"ipWhitelist":[{"cidr":"0.0.0.0/0","description":"Anywhere"}]}`,
		CreatedAt:        time.Now().UTC(),
	}
	_ = store.Databases().Create(context.Background(), db)

	app.Post("/api/v1/databases/:id/access/whitelist", h.handleAddDatabaseIPWhitelist)
	app.Delete("/api/v1/databases/:id/access/whitelist", h.handleDeleteDatabaseIPWhitelist)

	// 1. Add new rule
	addBody := map[string]any{
		"cidr":        "203.0.113.10",
		"description": "Production Server",
	}
	b, _ := json.Marshal(addBody)
	addReq := httptest.NewRequest("POST", "/api/v1/databases/db-access-test-3/access/whitelist", bytes.NewReader(b))
	addReq.Header.Set("Content-Type", "application/json")

	addResp, err := app.Test(addReq)
	if err != nil || addResp.StatusCode != 201 {
		t.Fatalf("expected 201 from add rule, got %d, err: %v", addResp.StatusCode, err)
	}

	// 2. Delete rule
	delReq := httptest.NewRequest("DELETE", "/api/v1/databases/db-access-test-3/access/whitelist?cidr=0.0.0.0/0", nil)
	delResp, err := app.Test(delReq)
	if err != nil || delResp.StatusCode != 200 {
		t.Fatalf("expected 200 from delete rule, got %d, err: %v", delResp.StatusCode, err)
	}

	updatedDb, _ := store.Databases().GetByID(context.Background(), "db-access-test-3")
	var meta map[string]any
	_ = json.Unmarshal([]byte(updatedDb.ResourceJSON), &meta)

	rules := meta["ipWhitelist"].([]any)
	if len(rules) != 1 {
		t.Fatalf("expected 1 remaining rule, got %d", len(rules))
	}
}

func TestAPIGetClientIP(t *testing.T) {
	app := fiber.New()
	store := newMockStore()
	h := &Handler{store: store}

	app.Get("/api/v1/client-ip", h.handleGetClientIP)

	req := httptest.NewRequest("GET", "/api/v1/client-ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.88, 10.0.0.1")

	resp, err := app.Test(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var res map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&res)

	if res["clientIp"] != "203.0.113.88" {
		t.Errorf("expected clientIp '203.0.113.88', got %q", res["clientIp"])
	}
}
