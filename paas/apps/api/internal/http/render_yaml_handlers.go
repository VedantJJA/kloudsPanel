package http

import (
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Render / DevPanel YAML Parser ────────────────────────────────────────────

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
	var inEnvVars bool
	var inDatabases bool
	var currentEnvKey string

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if trimmed == "databases:" {
			inDatabases = true
			inEnvVars = false
			if currentSvc != nil {
				res.Services = append(res.Services, *currentSvc)
				currentSvc = nil
			}
			continue
		} else if trimmed == "services:" {
			inDatabases = false
			inEnvVars = false
			if currentSvc != nil {
				res.Services = append(res.Services, *currentSvc)
				currentSvc = nil
			}
			continue
		}

		if inDatabases {
			if strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "name:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					dbName := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					engine := "postgres"
					if strings.Contains(strings.ToLower(dbName), "redis") {
						engine = "redis"
					} else if strings.Contains(strings.ToLower(dbName), "mysql") {
						engine = "mysql"
					} else if strings.Contains(strings.ToLower(dbName), "mongo") {
						engine = "mongodb"
					}
					res.Databases = append(res.Databases, fiber.Map{
						"name":   dbName,
						"engine": engine,
					})
				}
			} else if strings.HasPrefix(trimmed, "image:") && len(res.Databases) > 0 {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					img := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					if strings.Contains(img, "redis") {
						res.Databases[len(res.Databases)-1]["engine"] = "redis"
					} else if strings.Contains(img, "mysql") {
						res.Databases[len(res.Databases)-1]["engine"] = "mysql"
					} else if strings.Contains(img, "mongo") {
						res.Databases[len(res.Databases)-1]["engine"] = "mongodb"
					}
				}
			}
			continue
		}

		if strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- name:") || (strings.HasPrefix(trimmed, "type:") && currentSvc == nil) {
			if currentSvc != nil {
				res.Services = append(res.Services, *currentSvc)
			}
			svcType := "web"
			svcName := ""
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.ToLower(strings.TrimSpace(parts[1]))
				if strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "type:") {
					svcType = val
				} else if strings.HasPrefix(trimmed, "- name:") {
					svcName = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
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

		if !inEnvVars && strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Name = val
				currentSvc.Slug = strings.ToLower(val)
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "type:")) {
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
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "env:") || strings.HasPrefix(trimmed, "runtime:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
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
		} else if inEnvVars && strings.HasPrefix(trimmed, "fromDatabase:") {
			// Database reference block started
		} else if inEnvVars && strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				dbRef := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("paas-db-%s", dbRef)
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

	if currentSvc != nil {
		res.Services = append(res.Services, *currentSvc)
	}

	return res
}

func (h *Handler) handleParseRenderYaml(c fiber.Ctx) error {
	var req struct {
		Content string `json:"content"`
		RepoURL string `json:"repoUrl"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	content := req.Content

	if strings.TrimSpace(content) == "" && req.RepoURL != "" {
		rawURL := req.RepoURL
		if strings.Contains(rawURL, "github.com") {
			clean := strings.TrimSuffix(strings.TrimSuffix(rawURL, "/"), ".git")
			parts := strings.Split(clean, "github.com/")
			if len(parts) == 2 {
				client := &nethttp.Client{Timeout: 6 * time.Second}
				// Try render.yaml, render.yml, devpanel.yaml on main and master branches
				paths := []string{
					"main/render.yaml",
					"main/render.yml",
					"main/devpanel.yaml",
					"master/render.yaml",
					"master/devpanel.yaml",
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
			}
		}
	}

	if strings.TrimSpace(content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "No render.yaml or devpanel.yaml found in repository"})
	}

	result := parseRenderYAMLString(content)
	return c.JSON(fiber.Map{
		"success":   true,
		"services":  result.Services,
		"databases": result.Databases,
	})
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

	// 2. Provision all Services declared in blueprint
	for _, svcInfo := range req.Services {
		svcName := svcInfo.Name
		if svcName == "" {
			continue
		}
		svcSlug := svcInfo.Slug
		if svcSlug == "" {
			svcSlug = strings.ToLower(strings.ReplaceAll(svcName, "_", "-"))
		}

		kind := domain.ServiceKind(svcInfo.Kind)
		if kind == "" {
			kind = domain.ServiceKindWeb
		}

		port := svcInfo.InternalPort
		if port <= 0 {
			port = 8080
		}

		resMap := map[string]any{
			"gitRepoUrl":    req.RepoURL,
			"gitBranch":     branch,
			"rootDir":       svcInfo.RootDir,
			"rootDirectory": svcInfo.RootDir,
			"buildCommand":  svcInfo.BuildCommand,
			"startCommand":  svcInfo.StartCommand,
			"presetId":      svcInfo.Preset,
			"env":           svcInfo.EnvVars,
		}
		resJSON, _ := json.Marshal(resMap)

		s := &domain.Service{
			ProjectID:     project.ID,
			Name:          svcName,
			Slug:          svcSlug,
			Kind:          kind,
			CreatedBy:     u.ID,
			InternalPort:  &port,
			ResourceJSON:  string(resJSON),
			DesiredState:  domain.ServiceDesiredRunning,
			RuntimeStatus: domain.ServiceStatusDeploying,
		}
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

		go h.executeDeployment(s, dep, getRootDomain())
		createdServices = append(createdServices, s)
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"services":  createdServices,
		"databases": createdDatabases,
	})
}
