package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Log Storage ──────────────────────────────────────────────────────────────

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"` // "stdout" | "stderr" | "build" | "system"
	Message   string `json:"message"`
}

var (
	logMu             sync.RWMutex
	serviceLatestLogs = make(map[string][]LogEntry)
)

func appendLog(serviceID, stream, message string) {
	logMu.Lock()
	defer logMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format("15:04:05"),
		Stream:    stream,
		Message:   message,
	}

	logs := serviceLatestLogs[serviceID]
	if len(logs) > 500 {
		logs = logs[1:]
	}
	serviceLatestLogs[serviceID] = append(logs, entry)
}

func clearLogs(serviceID string) {
	logMu.Lock()
	defer logMu.Unlock()
	serviceLatestLogs[serviceID] = []LogEntry{}
}

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

func removeTraefikDynamicConfig(slug string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
	}
	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	_ = os.Remove(filePath)
}

func generateDockerfileForPreset(preset, buildCmd, startCmd string, port int) string {
	switch strings.ToLower(preset) {
	case "python":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("python app.py || python main.py || gunicorn app:app --bind 0.0.0.0:%d --workers 2", port)
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "pip install --no-cache-dir -r requirements.txt"
		}
		return fmt.Sprintf(`FROM python:3.11-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends gcc libpq-dev curl && rm -rf /var/lib/apt/lists/*
COPY . /app
RUN if [ -f requirements.txt ]; then %s; fi
ENV PORT=%d PYTHONUNBUFFERED=1
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, bCmd, port, port, sCmd)

	case "node", "nodejs":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "npm start || node index.js || node server.js || node app.js"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f package.json ]; then npm install; fi && if grep -q '\"build\":' package.json 2>/dev/null; then npm run build; fi"
		}
		return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY . /app
RUN %s
ENV PORT=%d
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, bCmd, port, port, sCmd)

	case "go", "golang":
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "CGO_ENABLED=0 go build -o server ."
		}
		return fmt.Sprintf(`FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN %s
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/server /app/server
EXPOSE %d
CMD ["/app/server"]
`, bCmd, port)

	case "java":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar target/*.jar || java -jar build/libs/*.jar"
		}
		return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-21-alpine AS builder
WORKDIR /app
COPY . .
RUN if [ -f pom.xml ]; then mvn clean package -DskipTests; fi
FROM eclipse-temurin:21-jdk-alpine
WORKDIR /app
COPY --from=builder /app /app
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)

	case "php":
		return fmt.Sprintf(`FROM php:8.3-apache
COPY . /var/www/html/
RUN a2enmod rewrite
EXPOSE 80
CMD ["apache2-foreground"]
`)

	case "ruby":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "bundle exec rackup -p 3000 -o 0.0.0.0 || ruby app.rb"
		}
		return fmt.Sprintf(`FROM ruby:3.3-alpine
WORKDIR /app
RUN apk add --no-cache build-base
COPY . /app
RUN if [ -f Gemfile ]; then bundle install; fi
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)

	case "rust":
		return fmt.Sprintf(`FROM rust:1.77-alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY . .
RUN cargo build --release
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/target/release/* /app/
EXPOSE %d
CMD ["sh", "-c", "./$(ls -p | grep -v / | head -n 1)"]
`, port)

	case "static", "static-spa", "nginx":
		bCmd := buildCmd
		if bCmd != "" {
			return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
COPY . ./
RUN %s
FROM nginx:alpine
RUN printf 'server {\n    listen 80;\n    server_name _;\n    root /usr/share/nginx/html;\n    index index.html;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html/ || COPY --from=builder /app/build /usr/share/nginx/html/ || COPY --from=builder /app/public /usr/share/nginx/html/ || COPY --from=builder /app /usr/share/nginx/html/
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`, bCmd)
		}
		return fmt.Sprintf(`FROM nginx:alpine
RUN printf 'server {\n    listen 80;\n    server_name _;\n    root /usr/share/nginx/html;\n    index index.html;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`)

	default:
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("python app.py || python main.py || python server.py || node server.js || node index.js || ./server || nginx -g 'daemon off;'")
		}
		return fmt.Sprintf(`FROM python:3.11-slim
WORKDIR /app
COPY . /app
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; fi
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)
	}
}

// ─── Real Deployment Execution Engine ─────────────────────────────────────────

func (h *Handler) executeDeployment(service *domain.Service, dep *domain.Deployment, rootDomain string) {
	serviceID := service.ID
	clearLogs(serviceID)
	appendLog(serviceID, "system", fmt.Sprintf("[platform] Deployment #%d triggered for service '%s' (%s)", dep.Sequence, service.Name, service.Slug))

	var resMap map[string]any
	var gitRepoUrl, gitBranch, buildCommand, startCommand, presetId string
	var envMap = make(map[string]string)

	if service.ResourceJSON != "" {
		if err := json.Unmarshal([]byte(service.ResourceJSON), &resMap); err == nil {
			if r, ok := resMap["gitRepoUrl"].(string); ok {
				gitRepoUrl = r
			}
			if b, ok := resMap["gitBranch"].(string); ok {
				gitBranch = b
			}
			if bc, ok := resMap["buildCommand"].(string); ok {
				buildCommand = bc
			}
			if sc, ok := resMap["startCommand"].(string); ok {
				startCommand = sc
			}
			if p, ok := resMap["presetId"].(string); ok {
				presetId = p
			}
			if envs, ok := resMap["env"].(map[string]any); ok {
				for k, v := range envs {
					envMap[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	if presetId == "" {
		presetId = string(service.Kind)
	}
	if gitBranch == "" {
		gitBranch = "main"
	}

	port := 80
	if service.InternalPort != nil && *service.InternalPort > 0 {
		port = *service.InternalPort
	}

	workspaceDir := filepath.Join("/tmp/builds", service.Slug)
	_ = os.RemoveAll(workspaceDir)
	_ = os.MkdirAll(workspaceDir, 0755)

	imageTag := fmt.Sprintf("paas-svc-%s:latest", service.Slug)

	// Step 1: Clone Git Repository if URL is provided
	if gitRepoUrl != "" {
		appendLog(serviceID, "system", fmt.Sprintf("[git] Cloning %s (branch: %s)...", gitRepoUrl, gitBranch))
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", gitBranch, gitRepoUrl, workspaceDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Fallback clone without branch
			cmd = exec.Command("git", "clone", "--depth", "1", gitRepoUrl, workspaceDir)
			out, err = cmd.CombinedOutput()
		}

		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) != "" {
				appendLog(serviceID, "stdout", line)
			}
		}

		if err != nil {
			appendLog(serviceID, "stderr", fmt.Sprintf("[git] Error cloning repository: %v", err))
			dep.Status = domain.DeploymentFailed
			_ = h.store.Deployments().Update(context.Background(), dep)
			return
		}
		appendLog(serviceID, "system", "[git] Repository checkout complete.")

		var rootDirectory string
		if rd, ok := resMap["rootDirectory"].(string); ok && rd != "" && rd != "." {
			rootDirectory = rd
		} else if rd, ok := resMap["rootDir"].(string); ok && rd != "" && rd != "." {
			rootDirectory = rd
		}

		contextDir := workspaceDir
		if rootDirectory != "" {
			subDir := filepath.Join(workspaceDir, rootDirectory)
			if info, err := os.Stat(subDir); err == nil && info.IsDir() {
				contextDir = subDir
				appendLog(serviceID, "system", fmt.Sprintf("[builder] Building from subdirectory: %s", rootDirectory))
			}
		}

		// Step 2: Auto-detect render.yaml or devpanel.yaml inside repository
		renderFile := filepath.Join(workspaceDir, "render.yaml")
		if _, err := os.Stat(renderFile); os.IsNotExist(err) {
			renderFile = filepath.Join(workspaceDir, "render.yml")
		}
		if _, err := os.Stat(renderFile); os.IsNotExist(err) {
			renderFile = filepath.Join(workspaceDir, "devpanel.yaml")
		}
		if _, err := os.Stat(renderFile); err == nil {
			if yamlBytes, err := os.ReadFile(renderFile); err == nil {
				parsed := parseRenderYAMLString(string(yamlBytes))
				if len(parsed.Services) > 0 {
					var matchingSvc *ParsedRenderService
					for _, s := range parsed.Services {
						if strings.EqualFold(s.Name, service.Name) || strings.EqualFold(s.Slug, service.Slug) {
							matchingSvc = &s
							break
						}
					}
					if matchingSvc == nil {
						matchingSvc = &parsed.Services[0]
					}
					svc := *matchingSvc
					appendLog(serviceID, "system", fmt.Sprintf("[render] Applied config for '%s' (preset: %s, rootDir: %s, port: %d)", svc.Name, svc.Preset, svc.RootDir, svc.InternalPort))
					if svc.RootDir != "" && rootDirectory == "" {
						rootDirectory = svc.RootDir
						subDir := filepath.Join(workspaceDir, rootDirectory)
						if info, err := os.Stat(subDir); err == nil && info.IsDir() {
							contextDir = subDir
							appendLog(serviceID, "system", fmt.Sprintf("[builder] Switched build context to: %s", rootDirectory))
						}
					}
					if buildCommand == "" && svc.BuildCommand != "" {
						buildCommand = svc.BuildCommand
					}
					if startCommand == "" && svc.StartCommand != "" {
						startCommand = svc.StartCommand
					}
					if svc.InternalPort > 0 && (service.InternalPort == nil || *service.InternalPort == 80) {
						port = svc.InternalPort
					}
					if svc.Preset != "" && (presetId == "" || presetId == "web" || presetId == "custom") {
						presetId = svc.Preset
					}
					for k, v := range svc.EnvVars {
						if _, exists := envMap[k]; !exists {
							envMap[k] = v
						}
					}
				}
			}
		}

		// Step 3: Universal Fallback Engine & Manifests Check
		// 3a. Check Procfile
		procfilePath := filepath.Join(contextDir, "Procfile")
		if pBytes, err := os.ReadFile(procfilePath); err == nil {
			for _, pLine := range strings.Split(string(pBytes), "\n") {
				pLine = strings.TrimSpace(pLine)
				if strings.HasPrefix(pLine, "web:") {
					extractedCmd := strings.TrimSpace(strings.TrimPrefix(pLine, "web:"))
					if startCommand == "" {
						startCommand = extractedCmd
						appendLog(serviceID, "system", fmt.Sprintf("[procfile] Found Procfile start command: %s", startCommand))
					}
					break
				}
			}
		}

		// 3b. Monorepo Subfolder Auto-Discovery if root has no project manifest
		if contextDir == workspaceDir && rootDirectory == "" {
			subFolders := []string{"web", "frontend", "client", "ui", "api", "backend", "server", "app", "src/backend", "src/frontend"}
			for _, sub := range subFolders {
				subPath := filepath.Join(workspaceDir, sub)
				if info, err := os.Stat(subPath); err == nil && info.IsDir() {
					if _, err := os.Stat(filepath.Join(subPath, "package.json")); err == nil {
						contextDir = subPath
						appendLog(serviceID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
						break
					} else if _, err := os.Stat(filepath.Join(subPath, "requirements.txt")); err == nil {
						contextDir = subPath
						appendLog(serviceID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
						break
					} else if _, err := os.Stat(filepath.Join(subPath, "go.mod")); err == nil {
						contextDir = subPath
						appendLog(serviceID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
						break
					}
				}
			}
		}

		// 3c. Auto-detect runtime from repository files
		if presetId == "" || presetId == "web" || presetId == "custom" {
			if _, err := os.Stat(filepath.Join(contextDir, "requirements.txt")); err == nil {
				presetId = "python"
				if port == 80 {
					port = 5000
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "pyproject.toml")); err == nil {
				presetId = "python"
				if port == 80 {
					port = 5000
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "package.json")); err == nil {
				presetId = "node"
				if port == 80 {
					port = 3000
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "go.mod")); err == nil {
				presetId = "go"
				if port == 80 {
					port = 8080
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "Cargo.toml")); err == nil {
				presetId = "rust"
				if port == 80 {
					port = 8080
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "pom.xml")); err == nil {
				presetId = "java"
				if port == 80 {
					port = 8080
				}
			} else if _, err := os.Stat(filepath.Join(contextDir, "index.html")); err == nil {
				presetId = "static"
			}
		}

		// Step 4: Check if Dockerfile exists or generate one
		dockerfilePath := filepath.Join(contextDir, "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			appendLog(serviceID, "build", fmt.Sprintf("[builder] Generating runtime Dockerfile (preset: %s, port: %d)", presetId, port))
			dfContent := generateDockerfileForPreset(presetId, buildCommand, startCommand, port)
			_ = os.WriteFile(dockerfilePath, []byte(dfContent), 0644)
		}

		// Step 5: Build Container Image with Docker
		appendLog(serviceID, "build", fmt.Sprintf("[builder] Running 'docker build -t %s %s'...", imageTag, contextDir))
		buildCmd := exec.Command("docker", "build", "-t", imageTag, contextDir)
		buildOut, err := buildCmd.CombinedOutput()
		for _, line := range strings.Split(string(buildOut), "\n") {
			if strings.TrimSpace(line) != "" {
				appendLog(serviceID, "build", line)
			}
		}
		if err != nil {
			appendLog(serviceID, "stderr", fmt.Sprintf("[builder] Build failed: %v", err))
			dep.Status = domain.DeploymentFailed
			_ = h.store.Deployments().Update(context.Background(), dep)
			return
		}
		appendLog(serviceID, "build", "✓ Container image built successfully.")
	}

	// Step 5: Stop previous container and run the new container
	containerName := fmt.Sprintf("paas-svc-%s", service.Slug)
	appendLog(serviceID, "runtime", fmt.Sprintf("[runtime] Deploying container '%s' on network platform-control...", containerName))

	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"--network", "platform-control",
		"--restart", "unless-stopped",
		"-e", fmt.Sprintf("PORT=%d", port),
		"-e", "PYTHONUNBUFFERED=1",
		"--label", "io.paas.managed=true",
		"--label", fmt.Sprintf("io.paas.service=%s", service.ID),
		"--label", "traefik.enable=true",
		"--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s.%s`)", service.Slug, service.Slug, rootDomain),
		"--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", service.Slug),
		"--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", service.Slug),
		"--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", service.Slug, port),
	}

	for k, v := range envMap {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	runArgs = append(runArgs, imageTag)

	runCmd := exec.Command("docker", runArgs...)
	runOut, err := runCmd.CombinedOutput()
	for _, line := range strings.Split(string(runOut), "\n") {
		if strings.TrimSpace(line) != "" {
			appendLog(serviceID, "stdout", line)
		}
	}
	if err != nil {
		appendLog(serviceID, "stderr", fmt.Sprintf("[runtime] Failed to launch container: %v", err))
		dep.Status = domain.DeploymentFailed
		_ = h.store.Deployments().Update(context.Background(), dep)
		return
	}

	// Step 6: Write Dynamic Traefik Configuration
	writeTraefikDynamicConfig(service.Slug, port, rootDomain)
	appendLog(serviceID, "system", fmt.Sprintf("[traefik] Ingress route active -> https://%s.%s (port :%d)", service.Slug, rootDomain, port))

	// Step 7: Stream container logs
	time.Sleep(2 * time.Second)
	containerLogsCmd := exec.Command("docker", "logs", "--tail", "50", containerName)
	cLogs, _ := containerLogsCmd.CombinedOutput()
	for _, line := range strings.Split(string(cLogs), "\n") {
		if strings.TrimSpace(line) != "" {
			appendLog(serviceID, "stdout", line)
		}
	}

	appendLog(serviceID, "stdout", fmt.Sprintf("✓ Application '%s' is live and accessible at https://%s.%s", service.Name, service.Slug, rootDomain))

	finishTime := time.Now().UTC()
	dep.Status = domain.DeploymentHealthy
	dep.FinishedAt = &finishTime
	_ = h.store.Deployments().Update(context.Background(), dep)

	service.RuntimeStatus = domain.ServiceStatusRunning
	service.DesiredState = domain.ServiceDesiredRunning
	_ = h.store.Services().Update(context.Background(), service)
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
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Service '%s' not found", serviceID)})
	}

	var userID *string
	if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
		if dbUser, err := h.store.Users().GetByID(c.Context(), u.ID); err == nil && dbUser != nil {
			userID = &dbUser.ID
		}
	}

	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), s.ID)
	now := time.Now().UTC()

	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)

	dep := &domain.Deployment{
		ServiceID:      s.ID,
		Sequence:       seq,
		Trigger:        domain.TriggerManual,
		TriggeredBy:    userID,
		Status:         domain.DeploymentBuilding,
		BuildDriver:    "docker",
		ConfigSnapshot: s.ResourceJSON,
		StartedAt:      &now,
	}
	if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to initialize deployment: %v", err)})
	}

	// Trigger real deployment execution in background goroutine
	go h.executeDeployment(s, dep, rootDomain)

	return c.Status(201).JSON(fiber.Map{
		"deployment": dep,
		"domain":     domainName,
		"endpoint":   fmt.Sprintf("https://%s", domainName),
		"status":     "deploying",
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
	logMu.RLock()
	entries, exists := serviceLatestLogs[id]
	logMu.RUnlock()

	if exists && len(entries) > 0 {
		return c.JSON(fiber.Map{"entries": entries})
	}

	// Try resolving service to check by other identifier (slug or ID)
	s, err := h.store.Services().GetByID(c.Context(), id)
	if err == nil && s != nil {
		logMu.RLock()
		entries, exists = serviceLatestLogs[s.ID]
		if !exists || len(entries) == 0 {
			entries, exists = serviceLatestLogs[s.Slug]
		}
		logMu.RUnlock()
		if exists && len(entries) > 0 {
			return c.JSON(fiber.Map{"entries": entries})
		}

		// Fallback to query live docker logs from container
		containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
		cmd := exec.Command("docker", "logs", "--tail", "100", containerName)
		out, err := cmd.CombinedOutput()
		if err == nil && len(out) > 0 {
			var liveEntries []LogEntry
			now := time.Now().UTC()
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) != "" {
					liveEntries = append(liveEntries, LogEntry{
						Timestamp: now.Format("15:04:05"),
						Stream:    "stdout",
						Message:   line,
					})
				}
			}
			if len(liveEntries) > 0 {
				return c.JSON(fiber.Map{"entries": liveEntries})
			}
		}
	}

	return c.JSON(fiber.Map{"entries": []LogEntry{
		{
			Timestamp: time.Now().UTC().Format("15:04:05"),
			Stream:    "system",
			Message:   "Ready for deployment. Click 'Deploy Now' to clone, build, and start container.",
		},
	}})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{"grant": "todo"})
}
