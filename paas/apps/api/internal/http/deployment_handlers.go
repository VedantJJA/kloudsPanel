package http

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// --- Deployment Helpers -----------------------------------------------------

// failDeployment marks both the deployment and service as failed and logs the reason.
func (h *Handler) failDeployment(service *domain.Service, dep *domain.Deployment, reason string) {
	appendLog(service.ID, dep.ID, "stderr", reason)
	dep.Status = domain.DeploymentFailed
	_ = h.store.Deployments().Update(context.Background(), dep)
	service.RuntimeStatus = domain.ServiceStatusFailed
	_ = h.store.Services().Update(context.Background(), service)
}

func (h *Handler) findGitAuthToken(repoUrl, projectID string) string {
	lowerUrl := strings.ToLower(repoUrl)
	var provider string
	if strings.Contains(lowerUrl, "github.com") {
		provider = "github"
	} else if strings.Contains(lowerUrl, "gitlab.com") {
		provider = "gitlab"
	} else if strings.Contains(lowerUrl, "bitbucket.org") {
		provider = "bitbucket"
	}

	// 1. Check environment variables
	if provider == "github" {
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			return t
		}
		if t := os.Getenv("GH_TOKEN"); t != "" {
			return t
		}
	} else if provider == "gitlab" {
		if t := os.Getenv("GITLAB_TOKEN"); t != "" {
			return t
		}
	} else if provider == "bitbucket" {
		if t := os.Getenv("BITBUCKET_TOKEN"); t != "" {
			return t
		}
	}

	if provider == "" || projectID == "" {
		return ""
	}

	// 2. Check project creator / workspace creator git integration tokens
	ctx := context.Background()
	if proj, err := h.store.Projects().GetByID(ctx, projectID); err == nil && proj != nil {
		if ws, err := h.store.Workspaces().GetByID(ctx, proj.WorkspaceID); err == nil && ws != nil {
			if it, err := h.store.GitIntegrations().Get(ctx, ws.CreatedBy, provider); err == nil && it != nil && it.Token != "" {
				return it.Token
			}
			// Check workspace members
			if members, err := h.store.Workspaces().ListMembers(ctx, ws.ID); err == nil {
				for _, m := range members {
					if it, err := h.store.GitIntegrations().Get(ctx, m.UserID, provider); err == nil && it != nil && it.Token != "" {
						return it.Token
					}
				}
			}
		}
	}
	return ""
}

func injectGitToken(rawUrl, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return rawUrl
	}
	u, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}
	lowerHost := strings.ToLower(u.Host)
	if strings.Contains(lowerHost, "github.com") {
		u.User = url.UserPassword("x-access-token", token)
	} else if strings.Contains(lowerHost, "gitlab.com") {
		u.User = url.UserPassword("oauth2", token)
	} else if strings.Contains(lowerHost, "bitbucket.org") {
		u.User = url.UserPassword("x-token-auth", token)
	} else {
		u.User = url.User(token)
	}
	return u.String()
}

func sanitizeGitUrl(raw string) string {
	if !strings.Contains(raw, "@") || !strings.Contains(raw, "://") {
		return raw
	}
	parts := strings.SplitN(raw, "://", 2)
	if len(parts) < 2 {
		return raw
	}
	scheme := parts[0]
	rest := parts[1]
	atIdx := strings.Index(rest, "@")
	if atIdx != -1 {
		return fmt.Sprintf("%s://***@%s", scheme, rest[atIdx+1:])
	}
	return raw
}

// --- Real Deployment Execution Engine ---------------------------------------

func (h *Handler) executeDeployment(service *domain.Service, dep *domain.Deployment, rootDomain string) {
	serviceID := service.ID
	depID := dep.ID
	clearLogs(serviceID, depID)
	appendLog(serviceID, depID, "system", fmt.Sprintf("[platform] Deployment #%d triggered for service '%s' (%s)", dep.Sequence, service.Name, service.Slug))

	// Security: Validate slug before using in container names/paths
	if err := ValidateSlug(service.Slug); err != nil {
		h.failDeployment(service, dep, fmt.Sprintf("[security] Invalid service slug: %v", err))
		return
	}

	// Build security profile from service resource config
	var resMap map[string]any
	if service.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(service.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}
	secProfile := BuildSecurityProfile(resMap)

	// Track runtime version for dynamic resolution
	var runtimeVersion string
	if v, ok := resMap["runtimeVersion"].(string); ok && v != "" {
		runtimeVersion = v
		appendLog(serviceID, depID, "system", fmt.Sprintf("[version] User-specified runtime version: %s", runtimeVersion))
	}

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
			if bc, ok := resMap["buildCommand"].(string); ok && strings.TrimSpace(bc) != "" {
				buildCommand = strings.TrimSpace(bc)
			} else if bc, ok := resMap["build_command"].(string); ok && strings.TrimSpace(bc) != "" {
				buildCommand = strings.TrimSpace(bc)
			} else if bc, ok := resMap["buildCmd"].(string); ok && strings.TrimSpace(bc) != "" {
				buildCommand = strings.TrimSpace(bc)
			}
			if sc, ok := resMap["startCommand"].(string); ok && strings.TrimSpace(sc) != "" {
				startCommand = strings.TrimSpace(sc)
			} else if sc, ok := resMap["start_command"].(string); ok && strings.TrimSpace(sc) != "" {
				startCommand = strings.TrimSpace(sc)
			} else if sc, ok := resMap["startCmd"].(string); ok && strings.TrimSpace(sc) != "" {
				startCommand = strings.TrimSpace(sc)
			}
			if p, ok := resMap["presetId"].(string); ok && strings.TrimSpace(p) != "" {
				presetId = strings.TrimSpace(p)
			} else if p, ok := resMap["preset"].(string); ok && strings.TrimSpace(p) != "" {
				presetId = strings.TrimSpace(p)
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
	if service.Kind == domain.ServiceKindStatic || presetId == "static" || presetId == "static-spa" || presetId == "nginx" {
		port = 80
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
			h.failDeployment(service, dep, fmt.Sprintf("[docker-image] Failed to pull image: %v", pullErr))
			return
		}
		imageTag = dockerImageName
		appendLog(serviceID, depID, "build", fmt.Sprintf("Docker image '%s' is ready.", dockerImageName))
	} else if gitRepoUrl != "" {
		authGitUrl := gitRepoUrl
		if token := h.findGitAuthToken(gitRepoUrl, service.ProjectID); token != "" {
			authGitUrl = injectGitToken(gitRepoUrl, token)
		}

		safeLogUrl := sanitizeGitUrl(gitRepoUrl)
		appendLog(serviceID, depID, "system", fmt.Sprintf("[git] Cloning %s (branch: %s)...", safeLogUrl, gitBranch))

		cmd := exec.Command("git", "clone", "--depth", "1", "--branch", gitBranch, authGitUrl, workspaceDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=echo")
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Fallback clone without branch
			cmd = exec.Command("git", "clone", "--depth", "1", authGitUrl, workspaceDir)
			cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=echo")
			out, err = cmd.CombinedOutput()
		}
		if err != nil && authGitUrl == gitRepoUrl {
			// Fallback clone with .git appended if missing
			if !strings.HasSuffix(authGitUrl, ".git") {
				cmd = exec.Command("git", "clone", "--depth", "1", authGitUrl+".git", workspaceDir)
				cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=echo")
				out, err = cmd.CombinedOutput()
			}
		}

		for _, line := range strings.Split(string(out), "\n") {
			cleanLine := sanitizeGitUrl(strings.TrimSpace(line))
			if cleanLine != "" {
				appendLog(serviceID, depID, "stdout", cleanLine)
			}
		}

		if err != nil {
			h.failDeployment(service, dep, fmt.Sprintf("[git] Error cloning repository: %v", err))
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
						if runtimeVersion == "" && svc.RuntimeVersion != "" {
							runtimeVersion = svc.RuntimeVersion
							appendLog(serviceID, depID, "system", fmt.Sprintf("[blueprint] Found runtime version in blueprint: %s", runtimeVersion))
						}
						if svc.MemoryLimit != "" {
							secProfile.MemoryLimit = svc.MemoryLimit
						}
						if svc.CPULimit != "" {
							secProfile.CPULimit = svc.CPULimit
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
		// 3a. Check Procfile (web, worker)
		procfilePath := filepath.Join(contextDir, "Procfile")
		if pBytes, err := os.ReadFile(procfilePath); err == nil {
			for _, pLine := range strings.Split(string(pBytes), "\n") {
				pLine = strings.TrimSpace(pLine)
				if service.Kind == domain.ServiceKindWorker && strings.HasPrefix(pLine, "worker:") {
					extractedCmd := strings.TrimSpace(strings.TrimPrefix(pLine, "worker:"))
					if startCommand == "" {
						startCommand = extractedCmd
						appendLog(serviceID, depID, "system", fmt.Sprintf("[procfile] Found Procfile worker start command: %s", startCommand))
					}
					break
				} else if strings.HasPrefix(pLine, "web:") {
					extractedCmd := strings.TrimSpace(strings.TrimPrefix(pLine, "web:"))
					if startCommand == "" {
						startCommand = extractedCmd
						appendLog(serviceID, depID, "system", fmt.Sprintf("[procfile] Found Procfile web start command: %s", startCommand))
					}
					break
				}
			}
		}

		// 3b. Monorepo Subfolder Auto-Discovery if root has no project manifest
		if contextDir == workspaceDir && rootDirectory == "" {
			subFolders := []string{"web", "frontend", "client", "ui", "api", "backend", "server", "app", "src/backend", "src/frontend"}
			manifests := []string{"package.json", "requirements.txt", "pyproject.toml", "go.mod", "Cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts", "Gemfile", "composer.json", "mix.exs", "deno.json", "pubspec.yaml", "shard.yml", "Dockerfile"}
			for _, sub := range subFolders {
				subPath := filepath.Join(workspaceDir, sub)
				if info, err := os.Stat(subPath); err == nil && info.IsDir() {
					for _, mf := range manifests {
						if _, err := os.Stat(filepath.Join(subPath, mf)); err == nil {
							contextDir = subPath
							appendLog(serviceID, depID, "system", fmt.Sprintf("[universal-builder] Auto-discovered project in subfolder: /%s (found %s)", sub, mf))
							break
						}
					}
					if contextDir != workspaceDir {
						break
					}
				}
			}
		}

		// 3c. Auto-detect runtime from repository files
		if presetId == "" || presetId == "web" || presetId == "custom" {
			type runtimeDetection struct {
				file   string
				preset string
				port   int
			}
			detections := []runtimeDetection{
				{"deno.json", "deno", 8000},
				{"deno.jsonc", "deno", 8000},
				{"bun.lockb", "bun", 3000},
				{"bunfig.toml", "bun", 3000},
				{"mix.exs", "elixir", 4000},
				{"requirements.txt", "python", 5000},
				{"pyproject.toml", "python", 5000},
				{"Pipfile", "python", 5000},
				{"package.json", "node", 3000},
				{"go.mod", "go", 8080},
				{"Cargo.toml", "rust", 8080},
				{"pom.xml", "java", 8080},
				{"build.gradle", "java", 8080},
				{"build.gradle.kts", "kotlin", 8080},
				{"build.sbt", "scala", 9000},
				{"Gemfile", "ruby", 3000},
				{"composer.json", "php", 80},
				{"Package.swift", "swift", 8080},
				{"shard.yml", "crystal", 3000},
				{"pubspec.yaml", "dart", 8080},
				{"build.zig", "zig", 8080},
				{"index.html", "static", 80},
			}
			for _, d := range detections {
				if _, err := os.Stat(filepath.Join(contextDir, d.file)); err == nil {
					presetId = d.preset
					if port == 80 && d.port != 80 {
						port = d.port
					}
					break
				}
			}
			// Check for .NET projects (*.csproj, *.fsproj)
			if presetId == "" || presetId == "web" || presetId == "custom" {
				if entries, err := os.ReadDir(contextDir); err == nil {
					for _, e := range entries {
						if strings.HasSuffix(e.Name(), ".csproj") || strings.HasSuffix(e.Name(), ".fsproj") || strings.HasSuffix(e.Name(), ".sln") {
							presetId = "dotnet"
							if port == 80 {
								port = 5000
							}
							break
						}
					}
				}
			}
		}

		// Force port 80 for static sites
		if service.Kind == domain.ServiceKindStatic || presetId == "static" || presetId == "static-spa" || presetId == "nginx" {
			port = 80
		}

		// Project-wide auto-wiring for all dynamic service URLs and database URIs
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
					internalHost := fmt.Sprintf("paas-svc-%s", otherSvc.Slug)
					if val == otherSvc.Name || val == otherSvc.Slug {
						val = internalHost
					}
					envMap[k] = val
				}
			}

			// For static frontend apps, configure routing and proxying
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

					// Interpolate references like ${services.api-server.url} in route destinations
					for _, otherSvc := range projectServices {
						otherUrl := fmt.Sprintf("https://%s.%s", otherSvc.Slug, rootDomain)
						otherPort := 8080
						if otherSvc.InternalPort != nil && *otherSvc.InternalPort > 0 {
							otherPort = *otherSvc.InternalPort
						}
						otherIntUrl := fmt.Sprintf("http://paas-svc-%s:%d", otherSvc.Slug, otherPort)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${services.%s.url}", otherSvc.Name), otherUrl)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${services.%s.url}", otherSvc.Slug), otherUrl)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${%s.url}", otherSvc.Name), otherUrl)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${%s.url}", otherSvc.Slug), otherUrl)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${services.%s.internalUrl}", otherSvc.Name), otherIntUrl)
						dest = strings.ReplaceAll(dest, fmt.Sprintf("${services.%s.internalUrl}", otherSvc.Slug), otherIntUrl)
					}

					// Skip internal SPA fallback rules (e.g. /* -> /index.html) as it is already the base root handler
					if (src == "/*" || src == "*" || src == "/") && (dest == "/index.html" || dest == "index.html" || strings.HasSuffix(dest, "/index.html")) && !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
						continue
					}

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
			port = 80
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
			// Auto-detect version from project files or live registry if not specified or set to auto
			if runtimeVersion == "" || strings.EqualFold(runtimeVersion, "auto") {
				resolved := resolveRuntimeVersion(presetId, contextDir, "")
				runtimeVersion = resolved.Version
				if resolved.Source == "project-file" {
					appendLog(serviceID, depID, "system", fmt.Sprintf("[version] Auto-detected %s version %s from %s", presetId, resolved.Version, resolved.DetectedFrom))
				} else {
					appendLog(serviceID, depID, "system", fmt.Sprintf("[version] Using latest %s version %s (%s)", presetId, resolved.Version, resolved.FullImage))
				}
			} else {
				appendLog(serviceID, depID, "system", fmt.Sprintf("[version] Using configured %s version %s", presetId, runtimeVersion))
			}
			appendLog(serviceID, depID, "build", fmt.Sprintf("[builder] Generating runtime Dockerfile (preset: %s, port: %d, version: %s)", presetId, port, runtimeVersion))
			dfContent := generateDockerfileForPreset(presetId, buildCommand, startCommand, port, runtimeVersion)
			_ = os.WriteFile(dockerfilePath, []byte(dfContent), 0644)
		} else {
			// User-provided Dockerfile: scan for dangerous patterns
			if dfBytes, err := os.ReadFile(dockerfilePath); err == nil {
				warnings, dangers := ScanDockerfileForDangers(string(dfBytes))
				for _, w := range warnings {
					appendLog(serviceID, depID, "system", fmt.Sprintf("[security] %s", w))
				}
				if len(dangers) > 0 {
					for _, d := range dangers {
						appendLog(serviceID, depID, "stderr", fmt.Sprintf("[security] %s", d))
					}
					h.failDeployment(service, dep, "[security] Dockerfile contains blocked patterns. Remove privileged/host-network directives.")
					return
				}
			}
		}

		// Step 5: Build Container Image with Docker
		appendLog(serviceID, depID, "build", fmt.Sprintf("[builder] Running 'docker build -t %s %s'...", imageTag, contextDir))
		buildArgs := []string{"build", "-t", imageTag}
		for k, v := range envMap {
			buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
		buildArgs = append(buildArgs, contextDir)

		// streamDockerBuild runs a docker build command and streams its output
		// line-by-line to appendLog so the UI gets real-time feedback.
		streamDockerBuild := func(env []string) error {
			cmd := exec.Command("docker", buildArgs...)
			cmd.Env = env
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return err
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return err
			}
			if err := cmd.Start(); err != nil {
				return err
			}
			var wg sync.WaitGroup
			scanStream := func(r io.Reader) {
				defer wg.Done()
				scanner := bufio.NewScanner(r)
				scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.TrimSpace(line) != "" {
						appendLog(serviceID, depID, "build", line)
					}
				}
			}
			wg.Add(2)
			go scanStream(stdout)
			go scanStream(stderr)
			wg.Wait()
			return cmd.Wait()
		}

		// Quick probe: check if BuildKit/buildx is available (non-streaming, fast)
		probeArgs := []string{"buildx", "version"}
		probeOut, probeErr := exec.Command("docker", probeArgs...).CombinedOutput()
		usesBuildKit := probeErr == nil && !strings.Contains(string(probeOut), "not found")

		var buildErr error
		if usesBuildKit {
			buildErr = streamDockerBuild(append(os.Environ(), "DOCKER_BUILDKIT=1", "BUILDKIT_PROGRESS=plain"))
		}
		if !usesBuildKit || buildErr != nil {
			if !usesBuildKit {
				appendLog(serviceID, depID, "build", "[builder] Docker buildx plugin not found on host. Falling back to standard container builder...")
			}
			buildErr = streamDockerBuild(append(os.Environ(), "DOCKER_BUILDKIT=0"))
		}

		if buildErr != nil {
			h.failDeployment(service, dep, fmt.Sprintf("[builder] Build failed: %v", buildErr))
			return
		}
		appendLog(serviceID, depID, "build", "Container image built successfully.")
	}

	// Force port 80 for static sites on container launch
	if service.Kind == domain.ServiceKindStatic || presetId == "static" || presetId == "static-spa" || presetId == "nginx" {
		port = 80
	}

	// Step 5: Stop previous container and run the new container
	containerName := fmt.Sprintf("paas-svc-%s", service.Slug)
	appendLog(serviceID, depID, "runtime", fmt.Sprintf("[runtime] Deploying container '%s' on network platform-control (mem: %s, cpu: %s)...", containerName, secProfile.MemoryLimit, secProfile.CPULimit))

	_ = exec.Command("docker", "network", "create", "platform-control").Run()
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"--network", "platform-control",
		"--restart", "unless-stopped",
	}

	// Append security hardening flags
	runArgs = append(runArgs, ContainerSecurityArgs(secProfile)...)

	runArgs = append(runArgs,
		"-e", fmt.Sprintf("PORT=%d", port),
		"-e", fmt.Sprintf("APP_PORT=%d", port),
		"-e", fmt.Sprintf("SERVER_PORT=%d", port),
		"-e", fmt.Sprintf("HTTP_PORT=%d", port),
		"-e", fmt.Sprintf("INTERNAL_PORT=%d", port),
		"-e", "HOST=0.0.0.0",
		"-e", "HOSTNAME=0.0.0.0",
		"-e", "SERVER_HOST=0.0.0.0",
		"-e", "ADDRESS=0.0.0.0",
		"-e", "NITRO_HOST=0.0.0.0",
		"-e", "NUXT_HOST=0.0.0.0",
		"-e", "ASTRO_HOST=0.0.0.0",
		"-e", "FASTIFY_ADDRESS=0.0.0.0",
		"-e", "NESTJS_HOST=0.0.0.0",
		"-e", "HOST_NAME=0.0.0.0",
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
		"--label", "traefik.docker.network=platform-control",
		"--label", fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s.%s`)", service.Slug, service.Slug, rootDomain),
		"--label", fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", service.Slug),
		"--label", fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", service.Slug),
		"--label", fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", service.Slug, port),
	)

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
		h.failDeployment(service, dep, fmt.Sprintf("[runtime] Failed to launch container: %v", err))
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
					if projectServices, err := h.store.Services().ListForProject(context.Background(), service.ProjectID); err == nil {
						for _, otherSvc := range projectServices {
							otherUrl := fmt.Sprintf("https://%s.%s", otherSvc.Slug, rootDomain)
							dst = strings.ReplaceAll(dst, fmt.Sprintf("${services.%s.url}", otherSvc.Name), otherUrl)
							dst = strings.ReplaceAll(dst, fmt.Sprintf("${services.%s.url}", otherSvc.Slug), otherUrl)
							dst = strings.ReplaceAll(dst, fmt.Sprintf("${%s.url}", otherSvc.Name), otherUrl)
							dst = strings.ReplaceAll(dst, fmt.Sprintf("${%s.url}", otherSvc.Slug), otherUrl)
						}
					}
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

// --- Deployment Handlers ----------------------------------------------------

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

	// Parse optional deployment overrides from body
	var req struct {
		BuildCommand   *string `json:"buildCommand"`
		StartCommand   *string `json:"startCommand"`
		GitBranch      *string `json:"gitBranch"`
		RuntimeVersion *string `json:"runtimeVersion"`
		PresetID       *string `json:"presetId"`
		ResourceJSON   *string `json:"resourceJson"`
	}
	if err := c.Bind().JSON(&req); err == nil {
		var resMap map[string]any
		if req.ResourceJSON != nil && *req.ResourceJSON != "" {
			_ = json.Unmarshal([]byte(*req.ResourceJSON), &resMap)
		} else if s.ResourceJSON != "" {
			_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
		}
		if resMap == nil {
			resMap = make(map[string]any)
		}
		if req.BuildCommand != nil {
			resMap["buildCommand"] = *req.BuildCommand
			resMap["build_command"] = *req.BuildCommand
		}
		if req.StartCommand != nil {
			resMap["startCommand"] = *req.StartCommand
			resMap["start_command"] = *req.StartCommand
		}
		if req.GitBranch != nil && *req.GitBranch != "" {
			resMap["gitBranch"] = *req.GitBranch
		}
		if req.RuntimeVersion != nil {
			resMap["runtimeVersion"] = *req.RuntimeVersion
		}
		if req.PresetID != nil && *req.PresetID != "" {
			resMap["presetId"] = *req.PresetID
		}
		if b, err := json.Marshal(resMap); err == nil {
			s.ResourceJSON = string(b)
			_ = h.store.Services().Update(c.Context(), s)
		}
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
