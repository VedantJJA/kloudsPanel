package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPortFromDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name: "EXPOSE standard",
			content: `FROM node:20-alpine
WORKDIR /app
COPY . .
EXPOSE 8080
CMD ["npm", "start"]`,
			expected: 8080,
		},
		{
			name: "EXPOSE lowercase with spacing",
			content: `FROM python:3.12-slim
WORKDIR /app
expose   5000
CMD ["python", "app.py"]`,
			expected: 5000,
		},
		{
			name: "ENV PORT with equals",
			content: `FROM golang:1.23-alpine
WORKDIR /app
ENV PORT=3000
CMD ["./server"]`,
			expected: 3000,
		},
		{
			name: "ENV PORT with space",
			content: `FROM ruby:3.2
ENV PORT 4567
CMD ["bundle", "exec", "rackup"]`,
			expected: 4567,
		},
		{
			name: "No port directive",
			content: `FROM alpine:latest
RUN echo hello`,
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectPortFromDockerfile(tc.content)
			if got != tc.expected {
				t.Errorf("detectPortFromDockerfile() = %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestTraefikDynamicConfigWithRetryMiddleware(t *testing.T) {
	tmpDir := t.TempDir()
	origDynamicDir := os.Getenv("TRAEFIK_DYNAMIC_DIR")
	t.Setenv("TRAEFIK_DYNAMIC_DIR", tmpDir)
	defer func() {
		if origDynamicDir != "" {
			_ = os.Setenv("TRAEFIK_DYNAMIC_DIR", origDynamicDir)
		}
	}()

	slug := "my-api-app"
	port := 8080
	rootDomain := "klouds.test"

	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, nil, nil, nil)

	cfgFile := filepath.Join(tmpDir, "svc-my-api-app.yaml")
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read generated dynamic config: %v", err)
	}

	content := string(data)

	// Check that backend service points to detected port 8080
	expectedServerUrl := "url: \"http://paas-svc-my-api-app:8080\""
	if !strings.Contains(content, expectedServerUrl) {
		t.Errorf("expected %s in dynamic config, got:\n%s", expectedServerUrl, content)
	}

	// Check that retry middleware is defined
	expectedRetryMw := "svc-my-api-app-retry:"
	if !strings.Contains(content, expectedRetryMw) {
		t.Errorf("expected %s retry middleware in dynamic config, got:\n%s", expectedRetryMw, content)
	}

	if !strings.Contains(content, "attempts: 4") || !strings.Contains(content, "initialInterval: \"250ms\"") {
		t.Errorf("expected retry middleware configuration in dynamic config, got:\n%s", content)
	}

	// Check that router attaches retry middleware
	if !strings.Contains(content, "- \"svc-my-api-app-retry\"") {
		t.Errorf("expected router to attach retry middleware, got:\n%s", content)
	}
}
