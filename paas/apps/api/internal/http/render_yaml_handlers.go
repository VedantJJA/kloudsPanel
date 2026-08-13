package http

import (
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// ─── Render YAML Parser ───────────────────────────────────────────────────────

type ParsedRenderService struct {
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Kind         string            `json:"kind"`
	Env          string            `json:"env"`
	Preset       string            `json:"preset"`
	Image        string            `json:"image"`
	InternalPort int               `json:"internal_port"`
	BuildCommand string            `json:"build_command"`
	StartCommand string            `json:"start_command"`
	CronSchedule string            `json:"cron_schedule,omitempty"`
	AutoDeploy   bool              `json:"auto_deploy"`
	EnvVars      map[string]string `json:"env_vars"`
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

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- type:") || (strings.HasPrefix(trimmed, "type:") && currentSvc == nil) {
			if currentSvc != nil {
				res.Services = append(res.Services, *currentSvc)
			}
			parts := strings.SplitN(trimmed, ":", 2)
			svcType := "web"
			if len(parts) > 1 {
				svcType = strings.ToLower(strings.TrimSpace(parts[1]))
			}
			currentSvc = &ParsedRenderService{
				Kind:         svcType,
				InternalPort: 80,
				AutoDeploy:   true,
				EnvVars:      make(map[string]string),
			}
			inEnvVars = false
			continue
		}

		if strings.HasPrefix(trimmed, "- name:") && currentSvc == nil {
			// Database item
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				dbName := strings.TrimSpace(parts[1])
				res.Databases = append(res.Databases, fiber.Map{
					"name":   dbName,
					"engine": "postgres",
				})
			}
			continue
		}

		if currentSvc == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Name = val
				currentSvc.Slug = strings.ToLower(val)
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "env:") {
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
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "buildCommand:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.BuildCommand = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "startCommand:") {
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
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "schedule:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.CronSchedule = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Kind = "cron"
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "envVars:") {
			inEnvVars = true
		} else if inEnvVars && strings.HasPrefix(trimmed, "- key:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				key := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[key] = ""
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "value:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				for k, v := range currentSvc.EnvVars {
					if v == "" {
						currentSvc.EnvVars[k] = val
						if k == "PORT" {
							var p int
							if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
								currentSvc.InternalPort = p
							}
						}
						break
					}
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
				rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/render.yaml", parts[1])
				client := &nethttp.Client{Timeout: 6 * time.Second}
				resp, err := client.Get(rawURL)
				if err == nil && resp.StatusCode == 200 {
					var b strings.Builder
					_, _ = io.Copy(&b, resp.Body)
					content = b.String()
					resp.Body.Close()
				}
			}
		}
	}

	if strings.TrimSpace(content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "No render.yaml content provided or found in repository"})
	}

	result := parseRenderYAMLString(content)
	return c.JSON(fiber.Map{
		"success":   true,
		"services":  result.Services,
		"databases": result.Databases,
	})
}
