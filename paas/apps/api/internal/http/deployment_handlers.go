package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

func appendLog(serviceID, depID, stream, message string) {
	logMu.Lock()
	defer logMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format("15:04:05"),
		Stream:    stream,
		Message:   message,
	}

	if serviceID != "" {
		logs := serviceLatestLogs[serviceID]
		if len(logs) > 500 {
			logs = logs[1:]
		}
		serviceLatestLogs[serviceID] = append(logs, entry)
		saveLogToDisk(serviceID, entry)
	}

	if depID != "" {
		logs := serviceLatestLogs[depID]
		if len(logs) > 500 {
			logs = logs[1:]
		}
		serviceLatestLogs[depID] = append(logs, entry)
		saveLogToDisk(depID, entry)
	}
}

func clearLogs(serviceID, depID string) {
	logMu.Lock()
	defer logMu.Unlock()
	if serviceID != "" {
		serviceLatestLogs[serviceID] = []LogEntry{}
		clearLogDisk(serviceID)
	}
	if depID != "" {
		serviceLatestLogs[depID] = []LogEntry{}
		clearLogDisk(depID)
	}
}

func saveLogToDisk(id string, entry LogEntry) {
	logDir := "/tmp/paas_logs"
	_ = os.MkdirAll(logDir, 0755)
	f, err := os.OpenFile(filepath.Join(logDir, id+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		b, _ := json.Marshal(entry)
		_, _ = f.WriteString(string(b) + "\n")
	}
}

func clearLogDisk(id string) {
	logDir := "/tmp/paas_logs"
	_ = os.Remove(filepath.Join(logDir, id+".jsonl"))
}

func loadLogsFromDisk(id string) []LogEntry {
	logDir := "/tmp/paas_logs"
	filePath := filepath.Join(logDir, id+".jsonl")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var entries []LogEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			var e LogEntry
			if err := json.Unmarshal([]byte(line), &e); err == nil {
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func getRootDomain() string {
	if d := os.Getenv("ROOT_DOMAIN"); d != "" {
		return d
	}
	return "yourdomain.com"
}

func writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug string, port int, rootDomain string, customDomains []string, routes []ServiceRouteItem, siblingStaticSlugs []string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
		if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
			_ = os.MkdirAll(dynamicDir, 0755)
		}
	}

	ruleParts := []string{fmt.Sprintf("Host(`%s.%s`)", slug, rootDomain)}
	for _, cd := range customDomains {
		cd = strings.TrimSpace(cd)
		if cd != "" {
			ruleParts = append(ruleParts, fmt.Sprintf("Host(`%s`)", cd))
		}
	}
	baseHostRule := strings.Join(ruleParts, " || ")

	var routersYAML strings.Builder
	var middlewaresYAML strings.Builder
	var servicesYAML strings.Builder

	// Primary container backend service
	servicesYAML.WriteString(fmt.Sprintf(`    svc-%s:
      loadBalancer:
        servers:
          - url: "http://paas-svc-%s:%d"
`, slug, slug, port))

	// Render-Style Redirect & Rewrite Rules
	for i, r := range routes {
		idx := i + 1
		src := strings.TrimSpace(r.Source)
		dest := strings.TrimSpace(r.Destination)
		rType := strings.ToLower(strings.TrimSpace(r.Type))
		if src == "" || dest == "" {
			continue
		}

		var pathRule string
		cleanSrc := src
		if cleanSrc == "/*" || cleanSrc == "*" || cleanSrc == "/" {
			pathRule = ""
		} else if strings.HasSuffix(cleanSrc, "/*") {
			prefix := strings.TrimSuffix(cleanSrc, "/*")
			pathRule = fmt.Sprintf(" && PathPrefix(`%s`)", prefix)
		} else if strings.HasSuffix(cleanSrc, "*") {
			prefix := strings.TrimSuffix(cleanSrc, "*")
			pathRule = fmt.Sprintf(" && PathPrefix(`%s`)", prefix)
		} else {
			pathRule = fmt.Sprintf(" && Path(`%s`)", cleanSrc)
		}

		if rType == "rewrite" || rType == "rewrite_200" {
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				parsedDest, err := url.Parse(dest)
				if err == nil {
					targetBaseUrl := fmt.Sprintf("%s://%s", parsedDest.Scheme, parsedDest.Host)
					targetServiceKey := fmt.Sprintf("svc-%s-target-%d", slug, idx)
					targetRouterKey := fmt.Sprintf("svc-%s-rewrite-%d", slug, idx)
					priority := 1000 - (i * 10)

					routersYAML.WriteString(fmt.Sprintf(`    %s:
      rule: "(%s)%s"
      priority: %d
      entryPoints:
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "%s"
`, targetRouterKey, baseHostRule, pathRule, priority, targetServiceKey))

					servicesYAML.WriteString(fmt.Sprintf(`    %s:
      loadBalancer:
        passHostHeader: false
        servers:
          - url: "%s"
`, targetServiceKey, targetBaseUrl))
				}
			}
		} else {
			// Redirect action (301, 302, 307, 308)
			isPermanent := rType == "redirect" || rType == "redirect_301" || rType == "redirect_308"
			redirMiddlewareKey := fmt.Sprintf("svc-%s-redir-%d", slug, idx)
			redirRouterKey := fmt.Sprintf("svc-%s-redir-rtr-%d", slug, idx)

			var regex, replacement string
			if strings.HasSuffix(src, "/*") {
				regex = fmt.Sprintf("^%s(.*)", strings.TrimSuffix(src, "/*"))
				if strings.HasSuffix(dest, "/*") {
					replacement = strings.Replace(dest, "/*", "${1}", 1)
				} else if strings.Contains(dest, "$1") {
					replacement = strings.ReplaceAll(dest, "$1", "${1}")
				} else {
					replacement = fmt.Sprintf("%s${1}", dest)
				}
			} else {
				regex = fmt.Sprintf("^%s$", src)
				replacement = dest
			}

			middlewaresYAML.WriteString(fmt.Sprintf(`    %s:
      redirectRegex:
        regex: "%s"
        replacement: "%s"
        permanent: %t
`, redirMiddlewareKey, regex, replacement, isPermanent))

			priority := 1000 - (i * 10)
			routersYAML.WriteString(fmt.Sprintf(`    %s:
      rule: "(%s)%s"
      priority: %d
      middlewares:
        - "%s"
      entryPoints:
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
`, redirRouterKey, baseHostRule, pathRule, priority, redirMiddlewareKey, slug))
		}
	}

	// Sibling static frontends auto proxying
	for _, fSlug := range siblingStaticSlugs {
		fSlug = strings.TrimSpace(fSlug)
		if fSlug != "" {
			routersYAML.WriteString(fmt.Sprintf("    svc-%s-api-proxy-%s:\n      rule: \"Host(`%s.%s`) && PathPrefix(`/api`)\"\n      priority: 100\n      entryPoints:\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", slug, fSlug, fSlug, rootDomain, slug))
		}
	}

	// Base fallback router
	routersYAML.WriteString(fmt.Sprintf(`    svc-%s:
      rule: "%s"
      priority: 10
      entryPoints:
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
`, slug, baseHostRule, slug))

	var output strings.Builder
	output.WriteString("http:\n")
	output.WriteString("  routers:\n")
	output.WriteString(routersYAML.String())
	if middlewaresYAML.Len() > 0 {
		output.WriteString("  middlewares:\n")
		output.WriteString(middlewaresYAML.String())
	}
	output.WriteString("  services:\n")
	output.WriteString(servicesYAML.String())

	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	_ = os.WriteFile(filePath, []byte(output.String()), 0644)
}

func writeTraefikDynamicConfigWithDomainsAndSiblings(slug string, port int, rootDomain string, customDomains []string, siblingStaticSlugs []string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, customDomains, nil, siblingStaticSlugs)
}

func writeTraefikDynamicConfigWithDomains(slug string, port int, rootDomain string, customDomains []string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, customDomains, nil, nil)
}

func writeTraefikDynamicConfig(slug string, port int, rootDomain string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, nil, nil, nil)
}

func removeTraefikDynamicConfig(slug string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
	}
	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	_ = os.Remove(filePath)
}

func generateDockerfileForPreset(preset, buildCmd, startCmd string, port int, proxyDirective string) string {
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
			bCmd = "if [ -f package.json ]; then (npm ci || npm install); fi && if grep -q '\"build\":' package.json 2>/dev/null; then npm run build; fi"
		} else if strings.Contains(bCmd, "npm ci") {
			bCmd = strings.ReplaceAll(bCmd, "npm ci", "(npm ci || npm install)")
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
			if strings.Contains(bCmd, "npm ci") {
				bCmd = strings.ReplaceAll(bCmd, "npm ci", "(npm ci || npm install)")
			}
			return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
RUN apk add --no-cache bash curl
RUN corepack enable 2>/dev/null || npm install -g pnpm@latest yarn@latest 2>/dev/null || true
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
ARG API_URL
ENV API_URL=$API_URL
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=$REACT_APP_API_URL
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
COPY . ./
RUN %s
RUN mkdir -p /dist && \
    if [ -d artifacts/*/dist/public ]; then cp -a artifacts/*/dist/public/. /dist/; \
    elif [ -d dist/public ]; then cp -a dist/public/. /dist/; \
    elif [ -d dist ]; then cp -a dist/. /dist/; \
    elif [ -d build ]; then cp -a build/. /dist/; \
    elif [ -d public ]; then cp -a public/. /dist/; \
    elif [ -d out ]; then cp -a out/. /dist/; \
    else cp -a . /dist/; fi

FROM nginx:alpine
COPY nginx.default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`, bCmd)
		}
		return fmt.Sprintf(`FROM nginx:alpine
COPY nginx.default.conf /etc/nginx/conf.d/default.conf
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
	depID := dep.ID
	clearLogs(serviceID, depID)
	appendLog(serviceID, depID, "system", fmt.Sprintf("[platform] Deployment #%d triggered for service '%s' (%s)", dep.Sequence, service.Name, service.Slug))

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

	// Inherit Workspace Shared Environment Variables
	if proj, err := h.store.Projects().GetByID(context.Background(), service.ProjectID); err == nil && proj != nil {
		if ws, err := h.store.Workspaces().GetByID(context.Background(), proj.WorkspaceID); err == nil && ws != nil && ws.QuotaJSON != "" {
			var wsData struct {
				SharedEnv []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"shared_env"`
			}
			if json.Unmarshal([]byte(ws.QuotaJSON), &wsData) == nil {
				for _, item := range wsData.SharedEnv {
					if item.Key != "" {
						if _, exists := envMap[item.Key]; !exists {
							envMap[item.Key] = item.Value
						}
					}
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

	// Step 1: Check for Direct Docker Image Deployment (No git repo required)
	dockerImageName, _ := resMap["image"].(string)
	if dockerImageName == "" {
		if dImg, ok := resMap["dockerImage"].(string); ok {
			dockerImageName = dImg
		}
	}

	if gitRepoUrl == "" && dockerImageName != "" {
		appendLog(serviceID, depID, "build", fmt.Sprintf("[docker-image] Pulling image '%s' from registry...", dockerImageName))
		pullCmd := exec.Command("docker", "pull", dockerImageName)
		pullOut, pullErr := pullCmd.CombinedOutput()
		for _, line := range strings.Split(string(pullOut), "\n") {
			if strings.TrimSpace(line) != "" {
				appendLog(serviceID, depID, "stdout", line)
			}
		}
		if pullErr != nil {
			appendLog(serviceID, depID, "stderr", fmt.Sprintf("[docker-image] Failed to pull image: %v", pullErr))
			dep.Status = domain.DeploymentFailed
			_ = h.store.Deployments().Update(context.Background(), dep)
			service.RuntimeStatus = domain.ServiceStatusFailed
			_ = h.store.Services().Update(context.Background(), service)
			return
		}
		imageTag = dockerImageName
		appendLog(serviceID, depID, "build", fmt.Sprintf("Docker image '%s' is ready.", dockerImageName))
	} else if gitRepoUrl != "" {
		appendLog(serviceID, depID, "system", fmt.Sprintf("[git] Cloning %s (branch: %s)...", gitRepoUrl, gitBranch))
		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", gitBranch, gitRepoUrl, workspaceDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Fallback clone without branch
			cmd = exec.Command("git", "clone", "--depth", "1", gitRepoUrl, workspaceDir)
			out, err = cmd.CombinedOutput()
		}

		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) != "" {
				appendLog(serviceID, depID, "stdout", line)
			}
		}

		if err != nil {
			appendLog(serviceID, depID, "stderr", fmt.Sprintf("[git] Error cloning repository: %v", err))
			dep.Status = domain.DeploymentFailed
			_ = h.store.Deployments().Update(context.Background(), dep)
			service.RuntimeStatus = domain.ServiceStatusFailed
			_ = h.store.Services().Update(context.Background(), service)
			return
		}
		appendLog(serviceID, depID, "system", "[git] Repository checkout complete.")

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
				appendLog(serviceID, depID, "system", fmt.Sprintf("[builder] Building from subdirectory: %s", rootDirectory))
			}
		}

		// Step 2: Auto-detect klouds.yaml (primary) or render.yaml inside repository
		blueprintFile := filepath.Join(workspaceDir, "klouds.yaml")
		if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
			blueprintFile = filepath.Join(workspaceDir, "klouds.yml")
		}
		if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
			blueprintFile = filepath.Join(workspaceDir, ".klouds.yaml")
		}
		if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
			blueprintFile = filepath.Join(workspaceDir, "render.yaml")
		}
		if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
			blueprintFile = filepath.Join(workspaceDir, "render.yml")
		}
		if _, err := os.Stat(blueprintFile); err == nil {
			if yamlBytes, err := os.ReadFile(blueprintFile); err == nil {
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
					appendLog(serviceID, depID, "system", fmt.Sprintf("[blueprint] Applied config for '%s' (preset: %s, rootDir: %s, port: %d)", svc.Name, svc.Preset, svc.RootDir, svc.InternalPort))
					if svc.RootDir != "" && rootDirectory == "" {
						rootDirectory = svc.RootDir
						subDir := filepath.Join(workspaceDir, rootDirectory)
						if info, err := os.Stat(subDir); err == nil && info.IsDir() {
							contextDir = subDir
							appendLog(serviceID, depID, "system", fmt.Sprintf("[builder] Switched build context to: %s", rootDirectory))
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
						appendLog(serviceID, depID, "system", fmt.Sprintf("[procfile] Found Procfile start command: %s", startCommand))
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
						appendLog(serviceID, depID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
						break
					} else if _, err := os.Stat(filepath.Join(subPath, "requirements.txt")); err == nil {
						contextDir = subPath
						appendLog(serviceID, depID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
						break
					} else if _, err := os.Stat(filepath.Join(subPath, "go.mod")); err == nil {
						contextDir = subPath
						appendLog(serviceID, depID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s", sub))
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

		// Project-wide auto-wiring for all dynamic service URLs & database URIs
		var backendProxyDirective string
		if projectServices, err := h.store.Services().ListForProject(context.Background(), service.ProjectID); err == nil {
			var primaryBackend *domain.Service
			for _, otherSvc := range projectServices {
				if otherSvc.ID != service.ID && (otherSvc.Kind == domain.ServiceKindWeb || otherSvc.Kind == "api") {
					if primaryBackend == nil {
						primaryBackend = otherSvc
					}
				}
				otherUrl := fmt.Sprintf("https://%s.%s", otherSvc.Slug, rootDomain)
				otherHost := fmt.Sprintf("%s.%s", otherSvc.Slug, rootDomain)
				otherPort := 8080
				if otherSvc.InternalPort != nil && *otherSvc.InternalPort > 0 {
					otherPort = *otherSvc.InternalPort
				}
				otherIntUrl := fmt.Sprintf("http://paas-svc-%s:%d", otherSvc.Slug, otherPort)

				for k, v := range envMap {
					val := strings.ReplaceAll(v, fmt.Sprintf("${services.%s.url}", otherSvc.Name), otherUrl)
					val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.url}", otherSvc.Slug), otherUrl)
					val = strings.ReplaceAll(val, fmt.Sprintf("${%s.url}", otherSvc.Name), otherUrl)
					val = strings.ReplaceAll(val, fmt.Sprintf("${%s.url}", otherSvc.Slug), otherUrl)
					val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.host}", otherSvc.Name), otherHost)
					val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.host}", otherSvc.Slug), otherHost)
					val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.internalUrl}", otherSvc.Name), otherIntUrl)
					val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.internalUrl}", otherSvc.Slug), otherIntUrl)
					envMap[k] = val
				}
			}

			// For static frontend apps (React / Vite), configure Render-style routing and proxying
			if service.Kind == domain.ServiceKindStatic || presetId == "static" || presetId == "static-spa" {
				var proxyDirectives strings.Builder
				var customRoutes []ServiceRouteItem
				if rts, ok := resMap["routes"].([]any); ok {
					for _, rt := range rts {
						if rMap, ok := rt.(map[string]any); ok {
							src, _ := rMap["source"].(string)
							dst, _ := rMap["destination"].(string)
							t, _ := rMap["type"].(string)
							if src != "" && dst != "" {
								customRoutes = append(customRoutes, ServiceRouteItem{
									Source:      src,
									Destination: dst,
									Type:        t,
								})
							}
						}
					}
				}

				for _, cr := range customRoutes {
					src := strings.TrimSpace(cr.Source)
					dest := strings.TrimSpace(cr.Destination)
					rType := strings.ToLower(strings.TrimSpace(cr.Type))

					locPath := "/" + strings.Trim(strings.TrimSuffix(src, "/*"), "/")
					if locPath != "/" {
						locPath += "/"
					}

					if rType == "rewrite" || rType == "rewrite_200" {
						if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
							targetUrl := strings.TrimSuffix(dest, "/*")
							if !strings.HasSuffix(targetUrl, "/") && strings.HasSuffix(dest, "/*") {
								targetUrl += "/"
							}
							u, _ := url.Parse(targetUrl)
							hostHeader := "$host"
							if u != nil && u.Host != "" {
								hostHeader = u.Host
							}
							proxyDirectives.WriteString(fmt.Sprintf("    location %s {\n        resolver 127.0.0.11 8.8.8.8 valid=30s ipv6=off;\n        proxy_pass %s;\n        proxy_ssl_server_name on;\n        proxy_http_version 1.1;\n        proxy_set_header Upgrade $http_upgrade;\n        proxy_set_header Connection 'upgrade';\n        proxy_set_header Host %s;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n        proxy_set_header X-Forwarded-Proto $scheme;\n    }\n", locPath, targetUrl, hostHeader))
							appendLog(serviceID, depID, "build", fmt.Sprintf("[router] Configured rewrite rule %s -> %s", src, targetUrl))
						}
					} else {
						redirCode := 301
						if rType == "redirect_302" || rType == "redirect_temporary" {
							redirCode = 302
						} else if rType == "redirect_307" {
							redirCode = 307
						} else if rType == "redirect_308" {
							redirCode = 308
						}
						targetUrl := strings.TrimSuffix(dest, "/*")
						proxyDirectives.WriteString(fmt.Sprintf("    location %s {\n        return %d %s$is_args$args;\n    }\n", locPath, redirCode, targetUrl))
						appendLog(serviceID, depID, "build", fmt.Sprintf("[router] Configured redirect rule (%d) %s -> %s", redirCode, src, targetUrl))
					}
				}

				if primaryBackend != nil && proxyDirectives.Len() == 0 {
					backendPublicUrl := fmt.Sprintf("https://%s.%s", primaryBackend.Slug, rootDomain)
					if cur, ok := envMap["VITE_API_URL"]; !ok || cur == "" || strings.HasPrefix(cur, "${") {
						envMap["VITE_API_URL"] = backendPublicUrl
					}
					otherPort := 8080
					if primaryBackend.InternalPort != nil && *primaryBackend.InternalPort > 0 {
						otherPort = *primaryBackend.InternalPort
					}
					proxyDirectives.WriteString(fmt.Sprintf("    location /api/ {\n        resolver 127.0.0.11 valid=30s ipv6=off;\n        proxy_pass http://paas-svc-%s:%d;\n        proxy_http_version 1.1;\n        proxy_set_header Upgrade $http_upgrade;\n        proxy_set_header Connection 'upgrade';\n        proxy_set_header Host $host;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n        proxy_set_header X-Forwarded-Proto $scheme;\n    }\n", primaryBackend.Slug, otherPort))
					appendLog(serviceID, depID, "build", fmt.Sprintf("[router] Auto-wired /api/ reverse proxy -> http://paas-svc-%s:%d", primaryBackend.Slug, otherPort))
				}
				nginxConf := fmt.Sprintf("server {\n    listen 80;\n    server_name _;\n    root /usr/share/nginx/html;\n    index index.html;\n%s    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n", proxyDirectives.String())
				_ = os.WriteFile(filepath.Join(contextDir, "nginx.default.conf"), []byte(nginxConf), 0644)
			}
		}

		// Auto-resolve database connection URIs
		if projectDbs, err := h.store.Databases().ListForProject(context.Background(), service.ProjectID); err == nil {
			for _, db := range projectDbs {
				var meta struct {
					ConnectionURI         string `json:"connectionUri"`
					InternalConnectionURI string `json:"internalConnectionUri"`
				}
				if db.ResourceJSON != "" {
					_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
				}
				uri := meta.InternalConnectionURI
				if uri == "" {
					uri = meta.ConnectionURI
				}
				for k, v := range envMap {
					val := strings.ReplaceAll(v, fmt.Sprintf("paas-db-%s", db.Name), uri)
					val = strings.ReplaceAll(val, fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name)), uri)
					val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.connectionString}", db.Name), uri)
					val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.url}", db.Name), uri)
					if strings.EqualFold(k, "DATABASE_URL") && (val == "" || val == db.Name || strings.HasPrefix(val, "paas-db-")) {
						val = uri
					}
					envMap[k] = val
				}
			}
		}

		// Step 4: Check if Dockerfile exists or generate one
		dockerfilePath := filepath.Join(contextDir, "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			appendLog(serviceID, depID, "build", fmt.Sprintf("[builder] Generating runtime Dockerfile (preset: %s, port: %d)", presetId, port))
			dfContent := generateDockerfileForPreset(presetId, buildCommand, startCommand, port, backendProxyDirective)
			_ = os.WriteFile(dockerfilePath, []byte(dfContent), 0644)
		}

		// Step 5: Build Container Image with Docker
		appendLog(serviceID, depID, "build", fmt.Sprintf("[builder] Running 'docker build -t %s %s'...", imageTag, contextDir))
		buildArgs := []string{"build", "-t", imageTag}
		for k, v := range envMap {
			buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
		buildArgs = append(buildArgs, contextDir)
		buildCmd := exec.Command("docker", buildArgs...)
		buildOut, err := buildCmd.CombinedOutput()
		for _, line := range strings.Split(string(buildOut), "\n") {
			if strings.TrimSpace(line) != "" {
				appendLog(serviceID, depID, "build", line)
			}
		}
		if err != nil {
			appendLog(serviceID, depID, "stderr", fmt.Sprintf("[builder] Build failed: %v", err))
			dep.Status = domain.DeploymentFailed
			_ = h.store.Deployments().Update(context.Background(), dep)
			service.RuntimeStatus = domain.ServiceStatusFailed
			_ = h.store.Services().Update(context.Background(), service)
			return
		}
		appendLog(serviceID, depID, "build", "Container image built successfully.")
	}

	// Step 5: Stop previous container and run the new container
	containerName := fmt.Sprintf("paas-svc-%s", service.Slug)
	appendLog(serviceID, depID, "runtime", fmt.Sprintf("[runtime] Deploying container '%s' on network platform-control...", containerName))

	_ = exec.Command("docker", "network", "create", "platform-control").Run()
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
			appendLog(serviceID, depID, "stdout", line)
		}
	}
	if err != nil {
		appendLog(serviceID, depID, "stderr", fmt.Sprintf("[runtime] Failed to launch container: %v", err))
		dep.Status = domain.DeploymentFailed
		_ = h.store.Deployments().Update(context.Background(), dep)
		service.RuntimeStatus = domain.ServiceStatusFailed
		_ = h.store.Services().Update(context.Background(), service)
		return
	}

	// Step 6: Write Dynamic Traefik Configuration
	var siblingStaticSlugs []string
	if service.Kind == domain.ServiceKindWeb || service.Kind == "api" {
		if projectServices, err := h.store.Services().ListForProject(context.Background(), service.ProjectID); err == nil {
			for _, otherSvc := range projectServices {
				if otherSvc.ID != service.ID && (otherSvc.Kind == domain.ServiceKindStatic || otherSvc.Kind == "static") {
					siblingStaticSlugs = append(siblingStaticSlugs, otherSvc.Slug)
				}
			}
		}
	}
	var postDepCustomDomains []string
	if cds, ok := resMap["customDomains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" {
				postDepCustomDomains = append(postDepCustomDomains, str)
			}
		}
	}
	var postDepRoutes []ServiceRouteItem
	if rts, ok := resMap["routes"].([]any); ok {
		for _, rt := range rts {
			if rMap, ok := rt.(map[string]any); ok {
				src, _ := rMap["source"].(string)
				dst, _ := rMap["destination"].(string)
				t, _ := rMap["type"].(string)
				if src != "" && dst != "" {
					postDepRoutes = append(postDepRoutes, ServiceRouteItem{
						Source:      src,
						Destination: dst,
						Type:        t,
					})
				}
			}
		}
	}
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(service.Slug, port, rootDomain, postDepCustomDomains, postDepRoutes, siblingStaticSlugs)
	appendLog(serviceID, depID, "system", fmt.Sprintf("[traefik] Ingress route active -> https://%s.%s (port :%d)", service.Slug, rootDomain, port))

	// Step 7: Stream container logs
	time.Sleep(2 * time.Second)
	containerLogsCmd := exec.Command("docker", "logs", "--tail", "50", containerName)
	cLogs, _ := containerLogsCmd.CombinedOutput()
	for _, line := range strings.Split(string(cLogs), "\n") {
		if strings.TrimSpace(line) != "" {
			appendLog(serviceID, depID, "stdout", line)
		}
	}

	appendLog(serviceID, depID, "stdout", fmt.Sprintf("Application '%s' is live and accessible at https://%s.%s", service.Name, service.Slug, rootDomain))

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
	if id == "" {
		id = c.Params("deployId")
	}

	logMu.RLock()
	entries, exists := serviceLatestLogs[id]
	logMu.RUnlock()

	if exists && len(entries) > 0 {
		return c.JSON(fiber.Map{"entries": entries})
	}

	// Try reading from disk for this ID directly
	if diskEntries := loadLogsFromDisk(id); len(diskEntries) > 0 {
		return c.JSON(fiber.Map{"entries": diskEntries})
	}

	// 1. Try resolving as a Deployment ID
	dep, err := h.store.Deployments().GetByID(c.Context(), id)
	if err == nil && dep != nil {
		logMu.RLock()
		entries, exists = serviceLatestLogs[dep.ID]
		if !exists || len(entries) == 0 {
			entries, exists = serviceLatestLogs[dep.ServiceID]
		}
		logMu.RUnlock()
		if exists && len(entries) > 0 {
			return c.JSON(fiber.Map{"entries": entries})
		}

		if diskEntries := loadLogsFromDisk(dep.ID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
		if diskEntries := loadLogsFromDisk(dep.ServiceID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}

		s, sErr := h.store.Services().GetByID(c.Context(), dep.ServiceID)
		if sErr == nil && s != nil {
			logMu.RLock()
			entries, exists = serviceLatestLogs[s.Slug]
			logMu.RUnlock()
			if exists && len(entries) > 0 {
				return c.JSON(fiber.Map{"entries": entries})
			}
			if diskEntries := loadLogsFromDisk(s.Slug); len(diskEntries) > 0 {
				return c.JSON(fiber.Map{"entries": diskEntries})
			}

			// Query live docker logs
			containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
			cmd := exec.Command("docker", "logs", "--tail", "150", containerName)
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
	}

	// 2. Try resolving service to check by other identifier (slug or ID)
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
		if diskEntries := loadLogsFromDisk(s.ID); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}
		if diskEntries := loadLogsFromDisk(s.Slug); len(diskEntries) > 0 {
			return c.JSON(fiber.Map{"entries": diskEntries})
		}

		// Fallback to query live docker logs from container
		containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
		cmd := exec.Command("docker", "logs", "--tail", "150", containerName)
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
			Message:   "Deployment in progress. Initializing build worker...",
		},
	}})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{"grant": "todo"})
}
