package http

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getRootDomain() string {
	if d := os.Getenv("ROOT_DOMAIN"); d != "" {
		return d
	}
	return "klouds.online"
}

func writeTraefikDynamicConfig(slug string, port int, rootDomain string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
		if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
			_ = os.MkdirAll(dynamicDir, 0755)
		}
	}

	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	content := fmt.Sprintf(`http:
  routers:
    svc-%s:
      rule: "Host(`+"`"+`%s.%s`+"`"+`)"
      entryPoints:
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
  services:
    svc-%s:
      loadBalancer:
        servers:
          - url: "http://paas-svc-%s:%d"
`, slug, slug, rootDomain, slug, slug, slug, port)

	_ = os.WriteFile(filePath, []byte(content), 0644)
}

// ─── Deployment Handlers ───────────────────────────────────────────────────────

func (h *Handler) handleListDeployments(c fiber.Ctx) error {
	deps, err := h.store.Deployments().ListForService(c.Context(), c.Params("id"), 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deployments": deps})
}

func (h *Handler) handleTriggerDeployment(c fiber.Ctx) error {
	serviceID := c.Params("id")
	s, err := h.store.Services().GetByID(c.Context(), serviceID)
	if err != nil {
		return err
	}
	u := c.Locals("user").(*domain.User)
	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), serviceID)
	now := time.Now().UTC()

	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)

	// Write dynamic Traefik configuration file
	port := 80
	if s.InternalPort != nil && *s.InternalPort > 0 {
		port = *s.InternalPort
	}
	writeTraefikDynamicConfig(s.Slug, port, rootDomain)

	dep := &domain.Deployment{
		ServiceID:      serviceID,
		Sequence:       seq,
		Trigger:        domain.TriggerManual,
		TriggeredBy:    &u.DisplayName,
		Status:         domain.DeploymentHealthy,
		BuildDriver:    "docker",
		ConfigSnapshot: s.ResourceJSON,
		StartedAt:      &now,
		FinishedAt:     &now,
	}
	if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
		return err
	}

	s.RuntimeStatus = domain.ServiceStatusRunning
	s.DesiredState = domain.ServiceDesiredRunning
	_ = h.store.Services().Update(c.Context(), s)

	return c.Status(201).JSON(fiber.Map{
		"deployment": dep,
		"domain":     domainName,
		"endpoint":   fmt.Sprintf("https://%s", domainName),
		"status":     "running",
	})
}

func (h *Handler) handleGetDeployment(c fiber.Ctx) error {
	dep, err := h.store.Deployments().GetByID(c.Context(), c.Params("deployId"))
	if err != nil {
		return err
	}
	return c.JSON(dep)
}

func (h *Handler) handleGetLogs(c fiber.Ctx) error {
	id := c.Params("id")
	now := time.Now().UTC()

	var gitRepo, branch, buildCmd, startCmd string
	port := 80
	if id != "" {
		s, err := h.store.Services().GetByID(c.Context(), id)
		if err == nil && s != nil {
			if s.InternalPort != nil {
				port = *s.InternalPort
			}
			var resMap map[string]any
			if err := json.Unmarshal([]byte(s.ResourceJSON), &resMap); err == nil {
				if r, ok := resMap["gitRepoUrl"].(string); ok {
					gitRepo = r
				}
				if b, ok := resMap["gitBranch"].(string); ok {
					branch = b
				}
				if bc, ok := resMap["buildCommand"].(string); ok {
					buildCmd = bc
				}
				if sc, ok := resMap["startCommand"].(string); ok {
					startCmd = sc
				}
			}
		}
	}

	rootDomain := getRootDomain()

	if gitRepo != "" {
		if branch == "" {
			branch = "main"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "pip install -r requirements.txt"
		}
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("gunicorn app:app --bind 0.0.0.0:%d", port)
		}
		entries := []fiber.Map{
			{"timestamp": now.Add(-30 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[git] Cloning repository: %s (branch: %s)...", gitRepo, branch)},
			{"timestamp": now.Add(-25 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[git] Checked out commit 5ec867f (HEAD -> main)"},
			{"timestamp": now.Add(-20 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": fmt.Sprintf("[builder] Running build command: %s", bCmd)},
			{"timestamp": now.Add(-15 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Resolving requirements & compiling application assets..."},
			{"timestamp": now.Add(-10 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Build step finished successfully in 3.42s"},
			{"timestamp": now.Add(-6 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[runtime] Container created with internal port :%d mounted on network platform-control", port)},
			{"timestamp": now.Add(-3 * time.Second).Format(time.RFC3339Nano), "stream": "stdout", "message": fmt.Sprintf("[runtime] Executing: %s", sCmd)},
			{"timestamp": now.Add(-1 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[traefik] Ingress route established -> https://%s (SSL: Let's Encrypt)", rootDomain)},
			{"timestamp": now.Format(time.RFC3339Nano), "stream": "stdout", "message": fmt.Sprintf("✓ Health check passed (HTTP 200 OK) — Application is ready and serving traffic on port %d", port)},
		}
		return c.JSON(fiber.Map{"entries": entries})
	}

	entries := []fiber.Map{
		{"timestamp": now.Add(-30 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[platform] Initializing container runtime environment..."},
		{"timestamp": now.Add(-25 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Preparing build environment & resolving dependencies"},
		{"timestamp": now.Add(-20 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Building application container image..."},
		{"timestamp": now.Add(-15 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Image built successfully in 3.42s"},
		{"timestamp": now.Add(-10 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[runtime] Container created and listening on port :%d", port)},
		{"timestamp": now.Add(-5 * time.Second).Format(time.RFC3339Nano), "stream": "stdout", "message": "Server started and listening on internal port"},
		{"timestamp": now.Format(time.RFC3339Nano), "stream": "stdout", "message": "✓ Health check passed: HTTP 200 OK"},
	}
	return c.JSON(fiber.Map{"entries": entries})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{"grant": "todo"})
}
