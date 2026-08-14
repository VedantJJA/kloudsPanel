package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── klouds.yaml / Blueprint Parser ──────────────────────────────────────────

func generateSecureRandomSecret(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type ParsedRenderService struct {
	Name              string            `json:"name"`
	Slug              string            `json:"slug"`
	Kind              string            `json:"kind"`
	Env               string            `json:"env"`
	Preset            string            `json:"preset"`
	Image             string            `json:"image"`
	RootDir           string            `json:"root_dir,omitempty"`
	InternalPort      int               `json:"internal_port"`
	BuildCommand      string            `json:"build_command"`
	StartCommand      string            `json:"start_command"`
	StaticPublishPath string            `json:"static_publish_path,omitempty"`
	CronSchedule      string            `json:"cron_schedule,omitempty"`
	AutoDeploy        bool              `json:"auto_deploy"`
	EnvVars           map[string]string `json:"env_vars"`
	RequiredEnvVars   []string          `json:"required_env_vars,omitempty"`
}

type ParsedRenderResult struct {
	Services  []ParsedRenderService `json:"services"`
	Databases []fiber.Map           `json:"databases"`
}

func parseRenderYAMLString(yamlStr string) ParsedRenderResult {
	res := ParsedRenderResult{
		Services:  []ParsedRenderService{},
		Databases: []fiber.Map{},
	}

	lines := strings.Split(yamlStr, "\n")
	var currentSvc *ParsedRenderService
	var currentDb *fiber.Map
	var inEnvVars bool
	var inDatabases bool
	var inServices bool
	var currentEnvKey string

	flushCurrent := func() {
		if currentDb != nil {
			res.Databases = append(res.Databases, *currentDb)
			currentDb = nil
		}
		if currentSvc != nil {
			// Check if this service is actually a database
			lowerKind := strings.ToLower(currentSvc.Kind)
			lowerImg := strings.ToLower(currentSvc.Image)
			lowerName := strings.ToLower(currentSvc.Name)
			if lowerKind == "database" || lowerKind == "redis" || lowerKind == "postgres" || lowerKind == "mysql" || lowerKind == "mongodb" || lowerKind == "clickhouse" ||
				strings.Contains(lowerImg, "redis") || strings.Contains(lowerImg, "postgres") || strings.Contains(lowerImg, "mysql") || strings.Contains(lowerImg, "mongo") || strings.Contains(lowerImg, "clickhouse") {
				engine := "postgres"
				if strings.Contains(lowerKind, "redis") || strings.Contains(lowerImg, "redis") || strings.Contains(lowerName, "redis") {
					engine = "redis"
				} else if strings.Contains(lowerKind, "mysql") || strings.Contains(lowerImg, "mysql") || strings.Contains(lowerName, "mysql") {
					engine = "mysql"
				} else if strings.Contains(lowerKind, "mongo") || strings.Contains(lowerImg, "mongo") || strings.Contains(lowerName, "mongo") {
					engine = "mongodb"
				} else if strings.Contains(lowerKind, "clickhouse") || strings.Contains(lowerImg, "clickhouse") || strings.Contains(lowerName, "clickhouse") {
					engine = "clickhouse"
				}
				res.Databases = append(res.Databases, fiber.Map{
					"name":   currentSvc.Name,
					"engine": engine,
				})
			} else {
				res.Services = append(res.Services, *currentSvc)
			}
			currentSvc = nil
		}
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level sections
		if trimmed == "databases:" {
			flushCurrent()
			inDatabases = true
			inServices = false
			inEnvVars = false
			continue
		} else if trimmed == "services:" {
			flushCurrent()
			inServices = true
			inDatabases = false
			inEnvVars = false
			continue
		}

		// Database list items (e.g. "- name: devpanel-postgres" or "- name: devpanel-redis")
		if inDatabases {
			if strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "name:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					dbName := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					engine := "postgres"
					lower := strings.ToLower(dbName)
					if strings.Contains(lower, "redis") {
						engine = "redis"
					} else if strings.Contains(lower, "mysql") {
						engine = "mysql"
					} else if strings.Contains(lower, "mongo") {
						engine = "mongodb"
					} else if strings.Contains(lower, "clickhouse") {
						engine = "clickhouse"
					}
					newDb := fiber.Map{
						"name":   dbName,
						"engine": engine,
					}
					res.Databases = append(res.Databases, newDb)
				}
			} else if (strings.HasPrefix(trimmed, "engine:") || strings.HasPrefix(trimmed, "image:")) && len(res.Databases) > 0 {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), "\"'"))
					if strings.Contains(val, "redis") {
						res.Databases[len(res.Databases)-1]["engine"] = "redis"
					} else if strings.Contains(val, "mysql") {
						res.Databases[len(res.Databases)-1]["engine"] = "mysql"
					} else if strings.Contains(val, "mongo") {
						res.Databases[len(res.Databases)-1]["engine"] = "mongodb"
					} else if strings.Contains(val, "clickhouse") {
						res.Databases[len(res.Databases)-1]["engine"] = "clickhouse"
					} else if strings.Contains(val, "postgres") {
						res.Databases[len(res.Databases)-1]["engine"] = "postgres"
					}
				}
			}
			continue
		}

		// In services section: Detect new service start (either list item "- name:" / "- type:" or map key "  frontend:" / "  backend:" / "  redis:")
		isMapServiceHeader := inServices && !strings.HasPrefix(trimmed, "-") && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") &&
			(strings.HasPrefix(rawLine, "  ") || strings.HasPrefix(rawLine, "\t")) && !strings.HasPrefix(rawLine, "    ") && !strings.HasPrefix(rawLine, "\t\t") &&
			trimmed != "env:" && trimmed != "envVars:" && trimmed != "source:" && trimmed != "build:" && trimmed != "deploy:" && trimmed != "resources:" && trimmed != "volumes:"

		isListServiceHeader := strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- name:") || (strings.HasPrefix(trimmed, "type:") && currentSvc == nil)

		if isMapServiceHeader || isListServiceHeader {
			flushCurrent()

			svcType := "web"
			svcName := ""

			if isMapServiceHeader {
				svcName = strings.TrimSuffix(trimmed, ":")
				lowerKey := strings.ToLower(svcName)
				if strings.Contains(lowerKey, "redis") || lowerKey == "redis-cache" || lowerKey == "cache" {
					svcType = "database"
				} else if strings.Contains(lowerKey, "postgres") || strings.Contains(lowerKey, "database") || lowerKey == "db" {
					svcType = "database"
				} else if strings.Contains(lowerKey, "mysql") || strings.Contains(lowerKey, "mongo") || strings.Contains(lowerKey, "clickhouse") {
					svcType = "database"
				} else if strings.Contains(lowerKey, "front") || strings.Contains(lowerKey, "web") || strings.Contains(lowerKey, "client") || strings.Contains(lowerKey, "ui") {
					svcType = "static"
				}
			} else {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					if strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "type:") {
						svcType = strings.ToLower(val)
					} else if strings.HasPrefix(trimmed, "- name:") {
						svcName = val
					}
				}
			}

			currentSvc = &ParsedRenderService{
				Name:         svcName,
				Slug:         strings.ToLower(svcName),
				Kind:         svcType,
				InternalPort: 80,
				AutoDeploy:   true,
				EnvVars:      make(map[string]string),
			}
			inEnvVars = false
			currentEnvKey = ""
			continue
		}

		if currentSvc == nil {
			continue
		}

		// Property Parsing inside current service
		if !inEnvVars && strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Name = val
				currentSvc.Slug = strings.ToLower(val)
			}
		} else if !inEnvVars && strings.HasPrefix(trimmed, "type:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.Kind = strings.ToLower(strings.TrimSpace(parts[1]))
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "rootDir:") || strings.HasPrefix(trimmed, "directory:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.RootDir = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "staticPublishPath:") || strings.HasPrefix(trimmed, "output_dir:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.StaticPublishPath = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Kind = "static"
			}
		} else if !inEnvVars && strings.HasPrefix(trimmed, "image:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				img := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Image = img
				lowerImg := strings.ToLower(img)
				if strings.Contains(lowerImg, "redis") || strings.Contains(lowerImg, "postgres") || strings.Contains(lowerImg, "mysql") || strings.Contains(lowerImg, "mongo") || strings.Contains(lowerImg, "clickhouse") {
					currentSvc.Kind = "database"
				}
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "env:") || strings.HasPrefix(trimmed, "runtime:") || strings.HasPrefix(trimmed, "engine:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Env = val
				switch strings.ToLower(val) {
				case "python":
					currentSvc.Preset = "python"
					currentSvc.Image = "python:3.11-slim"
					currentSvc.InternalPort = 5000
				case "node", "nodejs":
					currentSvc.Preset = "node"
					currentSvc.Image = "node:20-alpine"
					currentSvc.InternalPort = 3000
				case "go", "golang":
					currentSvc.Preset = "go"
					currentSvc.Image = "golang:1.22-alpine"
					currentSvc.InternalPort = 8080
				case "rust":
					currentSvc.Preset = "rust"
					currentSvc.Image = "rust:1.77-alpine"
					currentSvc.InternalPort = 8080
				case "java":
					currentSvc.Preset = "java"
					currentSvc.Image = "eclipse-temurin:21-jdk-alpine"
					currentSvc.InternalPort = 8080
				case "php":
					currentSvc.Preset = "php"
					currentSvc.Image = "php:8.3-apache"
					currentSvc.InternalPort = 80
				case "ruby":
					currentSvc.Preset = "ruby"
					currentSvc.Image = "ruby:3.3-alpine"
					currentSvc.InternalPort = 3000
				case "docker", "dockerfile":
					currentSvc.Preset = "dockerfile"
					currentSvc.Image = "custom"
					currentSvc.InternalPort = 80
				case "static":
					currentSvc.Preset = "static-spa"
					currentSvc.Image = "nginx:alpine"
					currentSvc.InternalPort = 80
					currentSvc.Kind = "static"
				default:
					currentSvc.Preset = val
					currentSvc.Image = "alpine:latest"
				}
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "buildCommand:") || strings.HasPrefix(trimmed, "command:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				cmd := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if currentSvc.BuildCommand == "" {
					currentSvc.BuildCommand = cmd
				}
			}
		} else if !inEnvVars && strings.HasPrefix(trimmed, "startCommand:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				sCmd := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.StartCommand = sCmd
				if strings.Contains(sCmd, "--bind") || strings.Contains(sCmd, "--port") || strings.Contains(sCmd, ":") {
					for _, token := range strings.Fields(sCmd) {
						if strings.Contains(token, ":") && !strings.HasPrefix(token, "http") {
							subparts := strings.Split(token, ":")
							if len(subparts) == 2 {
								var p int
								if _, err := fmt.Sscanf(subparts[1], "%d", &p); err == nil && p > 0 {
									currentSvc.InternalPort = p
								}
							}
						}
					}
				}
			}
		} else if !inEnvVars && strings.HasPrefix(trimmed, "port:") {
			var p int
			if _, err := fmt.Sscanf(trimmed, "port: %d", &p); err == nil && p > 0 {
				currentSvc.InternalPort = p
			}
		} else if !inEnvVars && strings.HasPrefix(trimmed, "schedule:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.CronSchedule = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Kind = "cron"
			}
		} else if strings.HasPrefix(trimmed, "envVars:") || strings.HasPrefix(trimmed, "env:") {
			inEnvVars = true
			currentEnvKey = ""
		} else if inEnvVars && (strings.HasPrefix(trimmed, "- key:") || strings.HasPrefix(trimmed, "key:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentEnvKey = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[currentEnvKey] = ""
			}
		} else if inEnvVars && (strings.HasPrefix(trimmed, "value:") || strings.HasPrefix(trimmed, "- value:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if currentEnvKey != "" {
					currentSvc.EnvVars[currentEnvKey] = val
					if currentEnvKey == "PORT" {
						var p int
						if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
							currentSvc.InternalPort = p
						}
					}
				}
			}
		} else if inEnvVars && (strings.HasPrefix(trimmed, "generateValue:") || strings.HasPrefix(trimmed, "generate_value:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				val := strings.ToLower(strings.TrimSpace(parts[1]))
				if val == "true" || val == "yes" {
					if currentSvc.EnvVars[currentEnvKey] == "" {
						currentSvc.EnvVars[currentEnvKey] = generateSecureRandomSecret(16)
					}
				}
			}
		} else if inEnvVars && (strings.HasPrefix(trimmed, "sync:") || strings.HasPrefix(trimmed, "required:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				val := strings.ToLower(strings.TrimSpace(parts[1]))
				if val == "false" || val == "true" || val == "yes" {
					currentSvc.RequiredEnvVars = append(currentSvc.RequiredEnvVars, currentEnvKey)
				}
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "fromDatabase:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" && currentEnvKey != "" {
				dbRef := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("paas-db-%s", dbRef)
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "fromService:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" && currentEnvKey != "" {
				svcRef := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}/api", svcRef)
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				ref := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if currentSvc.EnvVars[currentEnvKey] == "" {
					currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}/api", ref)
				}
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "property:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				prop := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), "\"'"))
				if prop == "port" {
					currentSvc.EnvVars[currentEnvKey] = "5432"
				} else if prop == "user" {
					currentSvc.EnvVars[currentEnvKey] = "postgres"
				}
			}
		}
	}

	flushCurrent()

	// Find any backend service for auto-connecting frontend
	var primaryBackendSlug string
	var primaryBackendPort = 8080
	for _, s := range res.Services {
		if s.Kind == "web" || s.Kind == "api" {
			primaryBackendSlug = s.Slug
			if s.InternalPort > 0 {
				primaryBackendPort = s.InternalPort
			}
			break
		}
	}

	// Auto-wire backend URLs into frontend services if not already specified
	if primaryBackendSlug != "" {
		for i := range res.Services {
			s := &res.Services[i]
			if s.Kind == "static" || s.Preset == "static-spa" || s.StaticPublishPath != "" {
				if s.EnvVars == nil {
					s.EnvVars = make(map[string]string)
				}
				if _, ok := s.EnvVars["VITE_API_URL"]; !ok {
					s.EnvVars["VITE_API_URL"] = fmt.Sprintf("${services.%s.url}/api", primaryBackendSlug)
				}
				if _, ok := s.EnvVars["NEXT_PUBLIC_API_URL"]; !ok {
					s.EnvVars["NEXT_PUBLIC_API_URL"] = fmt.Sprintf("${services.%s.url}/api", primaryBackendSlug)
				}
				if _, ok := s.EnvVars["REACT_APP_API_URL"]; !ok {
					s.EnvVars["REACT_APP_API_URL"] = fmt.Sprintf("${services.%s.url}/api", primaryBackendSlug)
				}
				if _, ok := s.EnvVars["API_URL"]; !ok {
					s.EnvVars["API_URL"] = fmt.Sprintf("${services.%s.url}/api", primaryBackendSlug)
				}
				if _, ok := s.EnvVars["BACKEND_URL"]; !ok {
					s.EnvVars["BACKEND_URL"] = fmt.Sprintf("${services.%s.url}", primaryBackendSlug)
				}
				if _, ok := s.EnvVars["INTERNAL_API_URL"]; !ok {
					s.EnvVars["INTERNAL_API_URL"] = fmt.Sprintf("http://paas-svc-%s:%d", primaryBackendSlug, primaryBackendPort)
				}
			}
		}
	}

	// Deduplicate service names and slugs so frontend and backend never share the exact same name
	usedSlugs := make(map[string]int)
	for i := range res.Services {
		s := &res.Services[i]
		if s.Name == "" {
			s.Name = fmt.Sprintf("service-%d", i+1)
		}
		if s.Slug == "" {
			s.Slug = strings.ToLower(strings.ReplaceAll(s.Name, "_", "-"))
		}
	}

	// Disambiguate collisions based on kind / root directory
	for i := range res.Services {
		for j := range res.Services {
			if i != j && strings.EqualFold(res.Services[i].Name, res.Services[j].Name) {
				if res.Services[i].Kind == "static" || res.Services[i].Preset == "static-spa" || strings.Contains(strings.ToLower(res.Services[i].RootDir), "front") {
					base := strings.TrimSuffix(res.Services[i].Name, "-backend")
					if base == "" {
						base = "frontend"
					}
					res.Services[i].Name = base + "-frontend"
					res.Services[i].Slug = strings.ToLower(strings.ReplaceAll(res.Services[i].Name, "_", "-"))
				} else if res.Services[j].Kind == "static" || res.Services[j].Preset == "static-spa" || strings.Contains(strings.ToLower(res.Services[j].RootDir), "front") {
					base := strings.TrimSuffix(res.Services[j].Name, "-backend")
					if base == "" {
						base = "frontend"
					}
					res.Services[j].Name = base + "-frontend"
					res.Services[j].Slug = strings.ToLower(strings.ReplaceAll(res.Services[j].Name, "_", "-"))
				}
			}
		}
	}

	// Guarantee distinct unique slugs
	for i := range res.Services {
		s := &res.Services[i]
		baseSlug := s.Slug
		count := usedSlugs[baseSlug]
		if count > 0 {
			s.Slug = fmt.Sprintf("%s-%d", baseSlug, count+1)
			s.Name = fmt.Sprintf("%s (%d)", s.Name, count+1)
		}
		usedSlugs[baseSlug] = count + 1
	}

	return res
}

func parseDotEnvExample(content string, repoName string) ParsedRenderResult {
	res := ParsedRenderResult{
		Services:  []ParsedRenderService{},
		Databases: []fiber.Map{},
	}
	envMap := make(map[string]string)
	var reqKeys []string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			envMap[k] = v
			vLower := strings.ToLower(v)
			if v == "" || strings.HasPrefix(vLower, "your_") || strings.HasPrefix(vLower, "replace_") || vLower == "changeme" || vLower == "todo" {
				reqKeys = append(reqKeys, k)
			}
		}
	}

	svc := ParsedRenderService{
		Name:            repoName,
		Slug:            strings.ToLower(repoName),
		Kind:            "web",
		InternalPort:    8080,
		AutoDeploy:      true,
		EnvVars:         envMap,
		RequiredEnvVars: reqKeys,
	}
	res.Services = append(res.Services, svc)
	return res
}

func (h *Handler) handleParseBlueprint(c fiber.Ctx) error {
	var req struct {
		Content string `json:"content"`
		RepoURL string `json:"repoUrl"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	content := req.Content
	isDotEnv := false
	repoBase := "app"

	if strings.TrimSpace(content) == "" && req.RepoURL != "" {
		rawURL := req.RepoURL
		if strings.Contains(rawURL, "github.com") {
			clean := strings.TrimSuffix(strings.TrimSuffix(rawURL, "/"), ".git")
			parts := strings.Split(clean, "github.com/")
			if len(parts) == 2 {
				subparts := strings.Split(parts[1], "/")
				if len(subparts) > 1 {
					repoBase = subparts[1]
				}
				client := &nethttp.Client{Timeout: 6 * time.Second}
				// Try klouds.yaml (primary), render.yaml on main and master branches
				paths := []string{
					"main/klouds.yaml",
					"main/klouds.yml",
					"main/.klouds.yaml",
					"master/klouds.yaml",
					"master/klouds.yml",
					"master/.klouds.yaml",
					"main/render.yaml",
					"main/render.yml",
					"master/render.yaml",
					"master/render.yml",
				}
				for _, p := range paths {
					testURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s", parts[1], p)
					resp, err := client.Get(testURL)
					if err == nil && resp.StatusCode == 200 {
						var b strings.Builder
						_, _ = io.Copy(&b, resp.Body)
						content = b.String()
						resp.Body.Close()
						break
					}
					if resp != nil {
						resp.Body.Close()
					}
				}

				// If no blueprint found, check for .env.example
				if strings.TrimSpace(content) == "" {
					envPaths := []string{
						"main/.env.example",
						"main/backend/.env.example",
						"main/api/.env.example",
						"main/server/.env.example",
						"master/.env.example",
						"master/backend/.env.example",
						"master/api/.env.example",
					}
					for _, p := range envPaths {
						testURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s", parts[1], p)
						resp, err := client.Get(testURL)
						if err == nil && resp.StatusCode == 200 {
							var b strings.Builder
							_, _ = io.Copy(&b, resp.Body)
							content = b.String()
							isDotEnv = true
							resp.Body.Close()
							break
						}
						if resp != nil {
							resp.Body.Close()
						}
					}
				}
			}
		}
	}

	if strings.TrimSpace(content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "No klouds.yaml, render.yaml, or .env.example found in repository"})
	}

	var result ParsedRenderResult
	if isDotEnv {
		result = parseDotEnvExample(content, repoBase)
	} else {
		result = parseRenderYAMLString(content)
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"services":  result.Services,
		"databases": result.Databases,
	})
}

// Alias for backward compatibility
func (h *Handler) handleParseRenderYaml(c fiber.Ctx) error {
	return h.handleParseBlueprint(c)
}

func (h *Handler) handleDeployBlueprint(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	projectSlugOrID := c.Params("id")

	project, err := h.store.Projects().GetByID(c.Context(), projectSlugOrID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Project not found"})
	}

	var req struct {
		RepoURL   string                `json:"repoUrl"`
		Branch    string                `json:"branch"`
		Services  []ParsedRenderService `json:"services"`
		Databases []fiber.Map           `json:"databases"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	createdDatabases := []any{}
	createdServices := []any{}

	// 1. Provision Databases declared in blueprint
	for _, dbInfo := range req.Databases {
		dbName := fmt.Sprintf("%v", dbInfo["name"])
		engine := fmt.Sprintf("%v", dbInfo["engine"])
		if dbName == "" {
			continue
		}

		dbRec, err := h.provisionDatabaseInternal(c.Context(), project.ID, dbName, engine)
		if err == nil && dbRec != nil {
			createdDatabases = append(createdDatabases, dbRec)
		}
	}

	// 2. Pre-allocate unique slugs and mapping for all services in blueprint
	rootDomain := getRootDomain()
	if rootDomain == "" {
		rootDomain = "yourdomain.com"
	}
	type serviceEntry struct {
		info ParsedRenderService
		svc  *domain.Service
		port int
	}
	var entries []serviceEntry
	slugMap := make(map[string]string)
	urlMap := make(map[string]string)
	internalUrlMap := make(map[string]string)

	for _, svcInfo := range req.Services {
		svcName := svcInfo.Name
		if svcName == "" {
			continue
		}
		baseSlug := svcInfo.Slug
		if baseSlug == "" {
			baseSlug = strings.ToLower(strings.ReplaceAll(svcName, "_", "-"))
		}
		
		actualSlug := baseSlug
		port := svcInfo.InternalPort
		if port <= 0 {
			port = 8080
		}

		s := &domain.Service{
			ProjectID:     project.ID,
			Name:          svcName,
			Slug:          actualSlug,
			Kind:          domain.ServiceKind(svcInfo.Kind),
			CreatedBy:     u.ID,
			InternalPort:  &port,
			DesiredState:  domain.ServiceDesiredRunning,
			RuntimeStatus: domain.ServiceStatusDeploying,
		}
		if s.Kind == "" {
			s.Kind = domain.ServiceKindWeb
		}
		entries = append(entries, serviceEntry{info: svcInfo, svc: s, port: port})
		slugMap[svcName] = actualSlug
		slugMap[baseSlug] = actualSlug
		urlMap[svcName] = fmt.Sprintf("https://%s.%s", actualSlug, rootDomain)
		urlMap[baseSlug] = fmt.Sprintf("https://%s.%s", actualSlug, rootDomain)
		internalUrlMap[svcName] = fmt.Sprintf("http://paas-svc-%s:%d", actualSlug, port)
		internalUrlMap[baseSlug] = fmt.Sprintf("http://paas-svc-%s:%d", actualSlug, port)
	}

	// 3. Resolve dynamic template tags & create services in database
	for _, entry := range entries {
		s := entry.svc
		svcInfo := entry.info

		resolvedEnv := make(map[string]string)
		for k, v := range svcInfo.EnvVars {
			val := v
			for refName, refUrl := range urlMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.url}", refName), refUrl)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.url}", refName), refUrl)
			}
			for refName, refSlug := range slugMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.host}", refName), fmt.Sprintf("%s.%s", refSlug, rootDomain))
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.host}", refName), fmt.Sprintf("%s.%s", refSlug, rootDomain))
			}
			for refName, refIntUrl := range internalUrlMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${services.%s.internalUrl}", refName), refIntUrl)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.internalUrl}", refName), refIntUrl)
			}
			resolvedEnv[k] = val
		}

		resMap := map[string]any{
			"gitRepoUrl":    req.RepoURL,
			"gitBranch":     branch,
			"rootDir":       svcInfo.RootDir,
			"rootDirectory": svcInfo.RootDir,
			"buildCommand":  svcInfo.BuildCommand,
			"startCommand":  svcInfo.StartCommand,
			"presetId":      svcInfo.Preset,
			"env":           resolvedEnv,
		}
		resJSON, _ := json.Marshal(resMap)
		s.ResourceJSON = string(resJSON)

		if err := h.store.Services().Create(c.Context(), s); err != nil {
			continue
		}

		// Trigger deployment
		seq, _ := h.store.Deployments().GetNextSequence(c.Context(), s.ID)
		now := time.Now().UTC()
		dep := &domain.Deployment{
			ServiceID:   s.ID,
			Sequence:    seq,
			Trigger:     domain.TriggerManual,
			TriggeredBy: &u.ID,
			Status:      domain.DeploymentQueued,
			BuildDriver: "docker",
			StartedAt:   &now,
		}
		_ = h.store.Deployments().Create(c.Context(), dep)

		go h.executeDeployment(s, dep, rootDomain)
		createdServices = append(createdServices, s)
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"services":  createdServices,
		"databases": createdDatabases,
	})
}
