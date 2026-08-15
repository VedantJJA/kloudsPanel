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
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

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
					if matchingSvc == nil && rootDirectory != "" && rootDirectory != "." {
						for _, s := range parsed.Services {
							if s.RootDir != "" && (strings.EqualFold(s.RootDir, rootDirectory) || strings.EqualFold(filepath.Clean(s.RootDir), filepath.Clean(rootDirectory))) {
								matchingSvc = &s
								break
							}
						}
					}
					if matchingSvc == nil {
						cleanSvcSlug := slugify(service.Slug)
						cleanSvcName := slugify(service.Name)
						for _, s := range parsed.Services {
							cleanBlueSlug := slugify(s.Slug)
							cleanBlueName := slugify(s.Name)
							if (cleanBlueSlug != "" && cleanBlueSlug != "app" && (strings.Contains(cleanSvcSlug, cleanBlueSlug) || strings.Contains(cleanBlueSlug, cleanSvcSlug))) ||
								(cleanBlueName != "" && cleanBlueName != "app" && (strings.Contains(cleanSvcName, cleanBlueName) || strings.Contains(cleanBlueName, cleanSvcName))) {
								matchingSvc = &s
								break
							}
						}
					}
					if matchingSvc == nil && len(parsed.Services) == 1 {
						matchingSvc = &parsed.Services[0]
					}
					if matchingSvc != nil {
						svc := *matchingSvc
						appendLog(serviceID, depID, "system", fmt.Sprintf("[blueprint] Applied config for '%s' (preset: %s, rootDir: %s, port: %d)", svc.Name, svc.Preset, svc.RootDir, svc.InternalPort))
						if svc.RootDir != "" && (rootDirectory == "" || rootDirectory == ".") {
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
							sslNameDirective := ""
							if u != nil && u.Host != "" {
								hostHeader = u.Host
								sslNameDirective = fmt.Sprintf("        proxy_ssl_name %s;\n        proxy_ssl_protocols TLSv1.2 TLSv1.3;\n", u.Host)
							}
							proxyDirectives.WriteString(fmt.Sprintf("    location %s {\n        resolver 127.0.0.11 8.8.8.8 1.1.1.1 valid=30s ipv6=off;\n        proxy_pass %s;\n        proxy_ssl_server_name on;\n%s        proxy_http_version 1.1;\n        proxy_set_header Upgrade $http_upgrade;\n        proxy_set_header Connection 'upgrade';\n        proxy_set_header Host %s;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n        proxy_set_header X-Forwarded-Proto $scheme;\n    }\n", locPath, targetUrl, sslNameDirective, hostHeader))
							appendLog(serviceID, depID, "build", fmt.Sprintf("[router] Configured rewrite rule %s -> %s", src, targetUrl))
						} else {
							proxyDirectives.WriteString(fmt.Sprintf("    location %s {\n        try_files $uri $uri/ %s;\n    }\n", locPath, dest))
							appendLog(serviceID, depID, "build", fmt.Sprintf("[router] Configured internal rewrite rule %s -> %s", src, dest))
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
					proxyDirectives.WriteString(fmt.Sprintf("    location /api/ {\n        resolver 127.0.0.11 8.8.8.8 valid=30s ipv6=off;\n        proxy_pass http://paas-svc-%s:%d;\n        proxy_http_version 1.1;\n        proxy_set_header Upgrade $http_upgrade;\n        proxy_set_header Connection 'upgrade';\n        proxy_set_header Host $host;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n        proxy_set_header X-Forwarded-Proto $scheme;\n    }\n", primaryBackend.Slug, otherPort))
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

		// Step 4: Ensure Nginx configuration exists for static sites and generate Dockerfile
		if service.Kind == domain.ServiceKindStatic || presetId == "static" || presetId == "static-spa" || presetId == "nginx" {
			nginxConfPath := filepath.Join(contextDir, "nginx.default.conf")
			if _, err := os.Stat(nginxConfPath); os.IsNotExist(err) {
				nginxConf := `server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html index.htm;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript image/svg+xml;

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "SAMEORIGIN" always;

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, max-age=31536000, immutable";
        try_files $uri =404;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }

    error_page 404 /index.html;
}
`
				_ = os.WriteFile(nginxConfPath, []byte(nginxConf), 0644)
			}
		}

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
		"-e", "HOST=0.0.0.0",
		"-e", "FLASK_RUN_HOST=0.0.0.0",
		"-e", fmt.Sprintf("FLASK_RUN_PORT=%d", port),
		"-e", "UVICORN_HOST=0.0.0.0",
		"-e", fmt.Sprintf("UVICORN_PORT=%d", port),
		"-e", "FASTAPI_HOST=0.0.0.0",
		"-e", fmt.Sprintf("FASTAPI_PORT=%d", port),
		"-e", fmt.Sprintf("GUNICORN_CMD_ARGS=--bind=0.0.0.0:%d", port),
		"-e", fmt.Sprintf("BIND=0.0.0.0:%d", port),
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

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{"grant": "todo"})
}
