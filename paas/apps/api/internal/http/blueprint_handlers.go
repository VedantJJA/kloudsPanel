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

type ParsedRoute struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
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
	Routes            []ParsedRoute     `json:"routes,omitempty"`
	RuntimeVersion    string            `json:"runtime_version,omitempty"`
	MemoryLimit       string            `json:"memory_limit,omitempty"`
	CPULimit          string            `json:"cpu_limit,omitempty"`
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
	var inRoutes bool
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
					"name":    currentSvc.Name,
					"engine":  engine,
					"version": currentSvc.RuntimeVersion,
				})
			} else {
				res.Services = append(res.Services, *currentSvc)
			}
			currentSvc = nil
		}
		inEnvVars = false
		inRoutes = false
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
		isMapServiceHeader := inServices && !inRoutes && !inEnvVars && !strings.HasPrefix(trimmed, "-") && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") &&
			(strings.HasPrefix(rawLine, "  ") || strings.HasPrefix(rawLine, "\t")) && !strings.HasPrefix(rawLine, "    ") && !strings.HasPrefix(rawLine, "\t\t") &&
			trimmed != "env:" && trimmed != "envVars:" && trimmed != "source:" && trimmed != "build:" && trimmed != "deploy:" && trimmed != "resources:" && trimmed != "volumes:" && trimmed != "routes:"

		isListServiceHeader := inServices && !inRoutes && !inEnvVars && !strings.HasPrefix(rawLine, "    ") && !strings.HasPrefix(rawLine, "\t\t") && (strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- name:") || (strings.HasPrefix(trimmed, "type:") && currentSvc == nil))

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
			inRoutes = false
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
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "version:") || strings.HasPrefix(trimmed, "runtime_version:")) {
			// Parse version: "20" or version: "3.12" for runtime version selection
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if val != "" {
					currentSvc.RuntimeVersion = sanitizeVersionString(val)
				}
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "mem_limit:") || strings.HasPrefix(trimmed, "memory:") || strings.HasPrefix(trimmed, "memory_limit:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if val != "" {
					currentSvc.MemoryLimit = sanitizeResourceLimit(val)
				}
			}
		} else if !inEnvVars && (strings.HasPrefix(trimmed, "cpu_limit:") || strings.HasPrefix(trimmed, "cpus:") || strings.HasPrefix(trimmed, "cpu:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if val != "" {
					currentSvc.CPULimit = sanitizeResourceLimit(val)
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
					currentSvc.Image = "python:3.12-slim"
					currentSvc.InternalPort = 5000
				case "node", "nodejs":
					currentSvc.Preset = "node"
					currentSvc.Image = "node:20-alpine"
					currentSvc.InternalPort = 3000
				case "go", "golang":
					currentSvc.Preset = "go"
					currentSvc.Image = "golang:1.23-alpine"
					currentSvc.InternalPort = 8080
				case "rust":
					currentSvc.Preset = "rust"
					currentSvc.Image = "rust:1.82-alpine"
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
				case "elixir", "phoenix":
					currentSvc.Preset = "elixir"
					currentSvc.Image = "elixir:1.17-alpine"
					currentSvc.InternalPort = 4000
				case "deno":
					currentSvc.Preset = "deno"
					currentSvc.Image = "denoland/deno:alpine"
					currentSvc.InternalPort = 8000
				case "bun":
					currentSvc.Preset = "bun"
					currentSvc.Image = "oven/bun:alpine"
					currentSvc.InternalPort = 3000
				case "dotnet", "csharp", "aspnet", ".net":
					currentSvc.Preset = "dotnet"
					currentSvc.Image = "mcr.microsoft.com/dotnet/sdk:8.0-alpine"
					currentSvc.InternalPort = 5000
				case "scala", "sbt":
					currentSvc.Preset = "scala"
					currentSvc.Image = "eclipse-temurin:21-jdk-alpine"
					currentSvc.InternalPort = 9000
				case "kotlin", "ktor":
					currentSvc.Preset = "kotlin"
					currentSvc.Image = "eclipse-temurin:21-jdk-alpine"
					currentSvc.InternalPort = 8080
				case "swift", "vapor":
					currentSvc.Preset = "swift"
					currentSvc.Image = "swift:5.10-jammy"
					currentSvc.InternalPort = 8080
				case "haskell":
					currentSvc.Preset = "haskell"
					currentSvc.Image = "haskell:9.8-slim"
					currentSvc.InternalPort = 3000
				case "clojure":
					currentSvc.Preset = "clojure"
					currentSvc.Image = "clojure:temurin-21-lein-alpine"
					currentSvc.InternalPort = 3000
				case "crystal":
					currentSvc.Preset = "crystal"
					currentSvc.Image = "crystallang/crystal:latest"
					currentSvc.InternalPort = 3000
				case "zig":
					currentSvc.Preset = "zig"
					currentSvc.Image = "alpine:latest"
					currentSvc.InternalPort = 8080
				case "dart":
					currentSvc.Preset = "dart"
					currentSvc.Image = "dart:stable"
					currentSvc.InternalPort = 8080
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
				currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}", svcRef)
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				ref := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if currentSvc.EnvVars[currentEnvKey] == "" {
					currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}", ref)
				}
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "envVarKey:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && currentEnvKey != "" {
				// RENDER_EXTERNAL_URL or similar fromService key
				_ = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
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
		} else if strings.HasPrefix(trimmed, "routes:") {
			inRoutes = true
			inEnvVars = false
		} else if inRoutes && (strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "type:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				rType := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if strings.HasPrefix(trimmed, "- type:") || len(currentSvc.Routes) == 0 {
					currentSvc.Routes = append(currentSvc.Routes, ParsedRoute{Type: rType})
				} else {
					currentSvc.Routes[len(currentSvc.Routes)-1].Type = rType
				}
			}
		} else if inRoutes && (strings.HasPrefix(trimmed, "- source:") || strings.HasPrefix(trimmed, "source:")) {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				src := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if strings.HasPrefix(trimmed, "- source:") || len(currentSvc.Routes) == 0 {
					currentSvc.Routes = append(currentSvc.Routes, ParsedRoute{Type: "rewrite", Source: src})
				} else {
					currentSvc.Routes[len(currentSvc.Routes)-1].Source = src
				}
			}
		} else if inRoutes && strings.HasPrefix(trimmed, "destination:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 && len(currentSvc.Routes) > 0 {
				dest := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Routes[len(currentSvc.Routes)-1].Destination = dest

				// If destination points to a backend URL (like https://backend.example.com/api/* or ${services.backend.url}/*)
				// automatically extract the target service name and wire VITE_API_URL
				lowerDest := strings.ToLower(dest)
				if strings.Contains(lowerDest, "render.com") || strings.Contains(lowerDest, "${services.") {
					var targetName string
					if strings.Contains(dest, "${services.") {
						start := strings.Index(dest, "${services.") + len("${services.")
						end := strings.Index(dest[start:], ".")
						if end > 0 {
							targetName = dest[start : start+end]
						}
					} else if strings.Contains(lowerDest, ".onrender.com") {
						sub := strings.TrimPrefix(lowerDest, "https://")
						sub = strings.TrimPrefix(sub, "http://")
						targetName = strings.Split(sub, ".onrender.com")[0]
					}
					if targetName != "" {
						if currentSvc.EnvVars == nil {
							currentSvc.EnvVars = make(map[string]string)
						}
						currentSvc.EnvVars["VITE_API_URL"] = fmt.Sprintf("${services.%s.url}", targetName)
					}
				}
			}
		}
	}

	flushCurrent()

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

	// Strip -backend, -api, -server, -frontend, -client, -ui, -web suffixes from static sites so they have clean primary root domains
	for i := range res.Services {
		s := &res.Services[i]
		if s.Kind == "static" || s.Preset == "static-spa" || strings.Contains(strings.ToLower(s.RootDir), "front") || strings.Contains(strings.ToLower(s.RootDir), "web") || strings.Contains(strings.ToLower(s.RootDir), "ui") || strings.Contains(strings.ToLower(s.RootDir), "client") {
			cleanName := s.Name
			for _, suffix := range []string{"-backend", "-api", "-server", "-frontend", "-client", "-ui", "-web", "_backend", "_api", "_server", "_frontend", "_client", "_ui", "_web"} {
				cleanName = strings.TrimSuffix(cleanName, suffix)
			}
			if cleanName != "" {
				s.Name = cleanName
			}
			cleanSlug := s.Slug
			for _, suffix := range []string{"-backend", "-api", "-server", "-frontend", "-client", "-ui", "-web", "_backend", "_api", "_server", "_frontend", "_client", "_ui", "_web"} {
				cleanSlug = strings.TrimSuffix(cleanSlug, suffix)
			}
			if cleanSlug != "" {
				s.Slug = cleanSlug
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

	// Find primary backend service
	var primaryBackendSlug string
	for _, s := range res.Services {
		if s.Kind == "web" || s.Kind == "api" {
			primaryBackendSlug = s.Slug
			break
		}
	}

	// Auto-wire VITE_API_URL into frontend static services (single clean variable like Render)
	if primaryBackendSlug != "" {
		for i := range res.Services {
			s := &res.Services[i]
			if s.Kind == "static" || s.Preset == "static-spa" || s.StaticPublishPath != "" || strings.Contains(strings.ToLower(s.RootDir), "front") || strings.Contains(strings.ToLower(s.RootDir), "web") || strings.Contains(strings.ToLower(s.RootDir), "ui") || strings.Contains(strings.ToLower(s.RootDir), "client") {
				if s.EnvVars == nil {
					s.EnvVars = make(map[string]string)
				}
				if _, ok := s.EnvVars["VITE_API_URL"]; !ok {
					s.EnvVars["VITE_API_URL"] = fmt.Sprintf("${services.%s.url}", primaryBackendSlug)
				}
			}
		}
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
		Token   string `json:"token"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	content := req.Content
	repoBase := "app"
	detectedSource := "klouds.yaml"
	var detectedResult ParsedRenderResult
	hasResult := false

	if strings.TrimSpace(content) != "" {
		detectedResult = parseRenderYAMLString(content)
		hasResult = len(detectedResult.Services) > 0 || len(detectedResult.Databases) > 0
		detectedSource = "custom-content"
	} else if req.RepoURL != "" {
		rawURL := req.RepoURL
		if strings.Contains(rawURL, "github.com") {
			clean := strings.TrimSuffix(strings.TrimSuffix(rawURL, "/"), ".git")
			parts := strings.Split(clean, "github.com/")
			if len(parts) == 2 {
				repoPath := parts[1]
				subparts := strings.Split(repoPath, "/")
				if len(subparts) > 1 {
					repoBase = subparts[1]
				}

				var userToken string
				if req.Token != "" {
					userToken = req.Token
				}
				if userToken == "" {
					if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
						if it, err := h.store.GitIntegrations().Get(c.Context(), u.ID, "github"); err == nil && it != nil && it.Token != "" {
							userToken = it.Token
						}
					}
				}

				client := &nethttp.Client{Timeout: 10 * time.Second}
				fetchRaw := func(filename string) string {
					// 1. Try raw.githubusercontent.com
					for _, br := range []string{"main", "master", "HEAD"} {
						testURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repoPath, br, filename)
						r, err := nethttp.NewRequest("GET", testURL, nil)
						if err == nil {
							r.Header.Set("User-Agent", "kloudsPanel-App/1.0")
							if userToken != "" {
								r.Header.Set("Authorization", "token "+userToken)
							}
							resp, err := client.Do(r)
							if err == nil && resp.StatusCode == 200 {
								var b strings.Builder
								_, _ = io.Copy(&b, resp.Body)
								resp.Body.Close()
								return b.String()
							}
							if resp != nil {
								resp.Body.Close()
							}
						}
					}

					// 2. Try api.github.com
					apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repoPath, filename)
					apiReq, err := nethttp.NewRequest("GET", apiURL, nil)
					if err == nil {
						apiReq.Header.Set("User-Agent", "kloudsPanel-App/1.0")
						apiReq.Header.Set("Accept", "application/vnd.github.raw+json")
						if userToken != "" {
							apiReq.Header.Set("Authorization", "Bearer "+userToken)
						}
						resp, err := client.Do(apiReq)
						if err == nil && resp.StatusCode == 200 {
							var b strings.Builder
							_, _ = io.Copy(&b, resp.Body)
							resp.Body.Close()
							return b.String()
						}
						if resp != nil {
							resp.Body.Close()
						}
					}
					return ""
				}

				// Efficient Single-Request Repository Tree Scan
				treeFiles := make(map[string]bool)
				for _, br := range []string{"HEAD", "main", "master"} {
					treeURL := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", repoPath, br)
					tReq, err := nethttp.NewRequest("GET", treeURL, nil)
					if err == nil {
						tReq.Header.Set("User-Agent", "kloudsPanel-App/1.0")
						if userToken != "" {
							tReq.Header.Set("Authorization", "Bearer "+userToken)
						}
						tResp, err := client.Do(tReq)
						if err == nil && tResp.StatusCode == 200 {
							var tData struct {
								Tree []struct {
									Path string `json:"path"`
									Type string `json:"type"`
								} `json:"tree"`
							}
							if json.NewDecoder(tResp.Body).Decode(&tData) == nil && len(tData.Tree) > 0 {
								for _, item := range tData.Tree {
									treeFiles[item.Path] = true
								}
								tResp.Body.Close()
								break
							}
							tResp.Body.Close()
						} else if tResp != nil {
							tResp.Body.Close()
						}
					}
				}

				// Check for Blueprint files in tree
				blueprintCandidates := []string{"klouds.yaml", "klouds.yml", ".klouds.yaml", ".klouds.yml", "render.yaml", "render.yml", ".render.yaml", ".render.yml"}
				for _, bf := range blueprintCandidates {
					if len(treeFiles) == 0 || treeFiles[bf] {
						if cStr := fetchRaw(bf); strings.TrimSpace(cStr) != "" {
							parsed := parseRenderYAMLString(cStr)
							if len(parsed.Services) > 0 || len(parsed.Databases) > 0 {
								detectedResult = parsed
								hasResult = true
								detectedSource = bf
								break
							}
						}
					}
				}

				// If no blueprint found, run the Intelligent Framework & Runtime Analyzer
				if !hasResult {
					detectedResult = detectFrameworkFromTree(repoPath, treeFiles, fetchRaw, repoBase)
					if len(detectedResult.Services) > 0 {
						hasResult = true
						detectedSource = "auto-detected"
					}
				}
			}
		}
	}

	if !hasResult || len(detectedResult.Services) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No blueprint or recognizable framework configuration found in repository"})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"blueprintType": detectedSource,
		"services":      detectedResult.Services,
		"databases":     detectedResult.Databases,
	})
}

// detectFrameworkFromTree analyzes repository files and structure to automatically suggest runtime, build/start commands, and ports
func detectFrameworkFromTree(repoPath string, treeFiles map[string]bool, fetchRaw func(filename string) string, repoBase string) ParsedRenderResult {
	var result ParsedRenderResult

	// Helper to check file existence
	hasFile := func(path string) bool {
		if len(treeFiles) > 0 {
			return treeFiles[path]
		}
		return fetchRaw(path) != ""
	}

	// Helper to extract env vars from .env.example
	extractEnv := func(envPath string) (map[string]string, []string) {
		envMap := make(map[string]string)
		var reqKeys []string
		if content := fetchRaw(envPath); content != "" {
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) >= 1 {
					k := strings.TrimSpace(parts[0])
					if k != "" {
						v := ""
						if len(parts) == 2 {
							v = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
						}
						envMap[k] = v
						reqKeys = append(reqKeys, k)
					}
				}
			}
		}
		return envMap, reqKeys
	}

	// Component analyzer for a given root directory
	analyzeComponent := func(rootDir string, defaultName string) *ParsedRenderService {
		prefix := ""
		if rootDir != "" && rootDir != "." {
			prefix = rootDir + "/"
		}

		envMap, reqKeys := extractEnv(prefix + ".env.example")
		if len(envMap) == 0 {
			envMap, reqKeys = extractEnv(".env.example")
		}

		// 1. Node.js / JavaScript / TypeScript Ecosystem
		if hasFile(prefix + "package.json") {
			pkgContent := fetchRaw(prefix + "package.json")
			var pkg struct {
				Name         string            `json:"name"`
				Dependencies map[string]string `json:"dependencies"`
				DevDeps      map[string]string `json:"devDependencies"`
				Scripts      map[string]string `json:"scripts"`
			}
			_ = json.Unmarshal([]byte(pkgContent), &pkg)
			svcName := defaultName
			if pkg.Name != "" {
				svcName = slugify(pkg.Name)
			}

			deps := make(map[string]bool)
			for k := range pkg.Dependencies {
				deps[k] = true
			}
			for k := range pkg.DevDeps {
				deps[k] = true
			}

			buildCommand := ""
			startCommand := ""
			internalPort := 3000
			preset := "nodejs"
			kind := "web"

			if pkg.Scripts["build"] != "" {
				buildCommand = "npm run build"
			}
			if pkg.Scripts["start"] != "" {
				startCommand = "npm start"
			}

			// Framework specific rules
			if deps["next"] {
				preset = "nodejs"
				kind = "web"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				if startCommand == "" {
					startCommand = "npm start"
				}
				internalPort = 3000
			} else if deps["nuxt"] || deps["@nuxt/kit"] || deps["nuxt3"] {
				preset = "nodejs"
				kind = "web"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				startCommand = "node .output/server/index.mjs"
				internalPort = 3000
			} else if deps["@sveltejs/kit"] {
				preset = "nodejs"
				kind = "web"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				startCommand = "node build/index.js"
				internalPort = 3000
			} else if deps["@remix-run/node"] || deps["@remix-run/react"] {
				preset = "nodejs"
				kind = "web"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				if startCommand == "" {
					startCommand = "npm start"
				}
				internalPort = 3000
			} else if deps["gatsby"] {
				preset = "static-spa"
				kind = "static"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				internalPort = 80
			} else if deps["@angular/core"] {
				preset = "static-spa"
				kind = "static"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				internalPort = 80
			} else if deps["solid-js"] || deps["solid-start"] {
				preset = "static-spa"
				kind = "static"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				internalPort = 80
			} else if deps["astro"] {
				// Astro can be SSR or static; default to static but check for SSR adapter
				preset = "static-spa"
				kind = "static"
				if deps["@astrojs/node"] || deps["@astrojs/deno"] {
					preset = "nodejs"
					kind = "web"
					internalPort = 3000
				}
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				if kind == "static" {
					internalPort = 80
				}
			} else if deps["vite"] {
				preset = "static-spa"
				kind = "static"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				internalPort = 80
			} else if deps["react-scripts"] {
				preset = "static-spa"
				kind = "static"
				if buildCommand == "" {
					buildCommand = "npm run build"
				}
				internalPort = 80
			} else if deps["hono"] {
				preset = "nodejs"
				kind = "web"
				internalPort = 3000
				if startCommand == "" {
					startCommand = "node dist/index.js || node index.js"
				}
			} else if deps["elysia"] || deps["@elysiajs/eden"] {
				preset = "bun"
				kind = "web"
				internalPort = 3000
				if startCommand == "" {
					startCommand = "bun run src/index.ts || bun run index.ts"
				}
			} else if deps["express"] || deps["fastify"] || deps["@nestjs/core"] || deps["koa"] || deps["hapi"] || deps["@hapi/hapi"] || deps["restify"] || deps["polka"] {
				preset = "nodejs"
				kind = "web"
				internalPort = 3000
				if startCommand == "" {
					startCommand = "node index.js || node server.js || node dist/main.js"
				}
			} else {
				if kind == "web" && startCommand == "" {
					startCommand = "node index.js || node server.js"
				}
			}

			return &ParsedRenderService{
				Name:            svcName,
				Kind:            kind,
				Preset:          preset,
				RootDir:         rootDir,
				BuildCommand:    buildCommand,
				StartCommand:    startCommand,
				InternalPort:    internalPort,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 2. Python Ecosystem
		if hasFile(prefix+"requirements.txt") || hasFile(prefix+"pyproject.toml") || hasFile(prefix+"Pipfile") || hasFile(prefix+"poetry.lock") || hasFile(prefix+"pdm.lock") {
			reqContent := fetchRaw(prefix + "requirements.txt")
			pyprojectContent := fetchRaw(prefix + "pyproject.toml")
			combined := strings.ToLower(reqContent + "\n" + pyprojectContent)

			internalPort := 8000
			startCommand := "python app.py || python main.py || python server.py"
			buildCommand := ""

			if strings.Contains(combined, "fastapi") || strings.Contains(combined, "uvicorn") {
				startCommand = "uvicorn main:app --host 0.0.0.0 --port $PORT || python main.py"
				internalPort = 8000
			} else if strings.Contains(combined, "flask") {
				startCommand = "flask run --host=0.0.0.0 --port=$PORT || python app.py"
				internalPort = 5000
			} else if strings.Contains(combined, "django") {
				buildCommand = "python manage.py collectstatic --noinput || true"
				startCommand = "gunicorn config.wsgi:application --bind 0.0.0.0:$PORT || python manage.py runserver 0.0.0.0:$PORT"
				internalPort = 8000
			} else if strings.Contains(combined, "starlette") {
				startCommand = "uvicorn main:app --host 0.0.0.0 --port $PORT || python main.py"
				internalPort = 8000
			} else if strings.Contains(combined, "sanic") {
				startCommand = "sanic server.app --host 0.0.0.0 --port $PORT || python server.py"
				internalPort = 8000
			} else if strings.Contains(combined, "tornado") {
				startCommand = "python app.py || python server.py || python main.py"
				internalPort = 8888
			} else if strings.Contains(combined, "aiohttp") {
				startCommand = "python app.py || python server.py || python main.py"
				internalPort = 8080
			} else if strings.Contains(combined, "litestar") {
				startCommand = "litestar run --host 0.0.0.0 --port $PORT || uvicorn app:app --host 0.0.0.0 --port $PORT"
				internalPort = 8000
			}

			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "python",
				RootDir:         rootDir,
				BuildCommand:    buildCommand,
				StartCommand:    startCommand,
				InternalPort:    internalPort,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 3. Go Ecosystem
		if hasFile(prefix + "go.mod") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "go",
				RootDir:         rootDir,
				BuildCommand:    "go build -o server .",
				StartCommand:    "./server",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 4. Rust Ecosystem
		if hasFile(prefix + "Cargo.toml") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "rust",
				RootDir:         rootDir,
				BuildCommand:    "cargo build --release",
				StartCommand:    "./target/release/app",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 5. Java Ecosystem (Maven / Gradle)
		if hasFile(prefix+"pom.xml") || hasFile(prefix+"build.gradle") {
			buildCmd := "mvn clean package -DskipTests || ./gradlew build"
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "java",
				RootDir:         rootDir,
				BuildCommand:    buildCmd,
				StartCommand:    "java -jar target/*.jar || java -jar build/libs/*.jar",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 6. PHP Ecosystem
		if hasFile(prefix + "composer.json") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "php",
				RootDir:         rootDir,
				InternalPort:    80,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 7. Ruby Ecosystem
		if hasFile(prefix + "Gemfile") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "ruby",
				RootDir:         rootDir,
				StartCommand:    "bundle exec rackup -p $PORT -o 0.0.0.0 || ruby app.rb",
				InternalPort:    3000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 8. Dockerfile
		if hasFile(prefix + "Dockerfile") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "dockerfile",
				RootDir:         rootDir,
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 9. Elixir / Phoenix
		if hasFile(prefix + "mix.exs") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "elixir",
				RootDir:         rootDir,
				StartCommand:    "mix phx.server || mix run --no-halt",
				InternalPort:    4000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 10. Deno
		if hasFile(prefix+"deno.json") || hasFile(prefix+"deno.jsonc") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "deno",
				RootDir:         rootDir,
				StartCommand:    "deno run --allow-net --allow-env --allow-read main.ts",
				InternalPort:    8000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 11. .NET / C#
		{
			hasDotnet := false
			for path := range treeFiles {
				if strings.HasPrefix(path, prefix) && (strings.HasSuffix(path, ".csproj") || strings.HasSuffix(path, ".fsproj") || strings.HasSuffix(path, ".sln")) {
					hasDotnet = true
					break
				}
			}
			if hasDotnet {
				return &ParsedRenderService{
					Name:            defaultName,
					Kind:            "web",
					Preset:          "dotnet",
					RootDir:         rootDir,
					BuildCommand:    "dotnet publish -c Release -o /app/publish",
					InternalPort:    5000,
					AutoDeploy:      true,
					EnvVars:         envMap,
					RequiredEnvVars: reqKeys,
				}
			}
		}

		// 12. Scala / sbt
		if hasFile(prefix + "build.sbt") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "scala",
				RootDir:         rootDir,
				BuildCommand:    "sbt stage || sbt assembly",
				InternalPort:    9000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 13. Crystal
		if hasFile(prefix + "shard.yml") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "crystal",
				RootDir:         rootDir,
				BuildCommand:    "shards install && crystal build src/*.cr --release -o app",
				StartCommand:    "./app",
				InternalPort:    3000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 14. Swift / Vapor
		if hasFile(prefix + "Package.swift") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "swift",
				RootDir:         rootDir,
				BuildCommand:    "swift build -c release",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 15. Dart
		if hasFile(prefix + "pubspec.yaml") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "dart",
				RootDir:         rootDir,
				BuildCommand:    "dart pub get && dart compile exe bin/server.dart -o server",
				StartCommand:    "./server",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 16. Clojure
		if hasFile(prefix+"project.clj") || hasFile(prefix+"deps.edn") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "clojure",
				RootDir:         rootDir,
				BuildCommand:    "lein uberjar",
				StartCommand:    "java -jar target/*-standalone.jar",
				InternalPort:    3000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 17. Haskell
		if hasFile(prefix+"stack.yaml") || hasFile(prefix+"cabal.project") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "haskell",
				RootDir:         rootDir,
				BuildCommand:    "stack build || cabal build",
				InternalPort:    3000,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 18. Zig
		if hasFile(prefix + "build.zig") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "web",
				Preset:          "zig",
				RootDir:         rootDir,
				BuildCommand:    "zig build -Doptimize=ReleaseSafe",
				InternalPort:    8080,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		// 19. Plain Static HTML (last resort)
		if hasFile(prefix + "index.html") {
			return &ParsedRenderService{
				Name:            defaultName,
				Kind:            "static",
				Preset:          "static",
				RootDir:         rootDir,
				InternalPort:    80,
				AutoDeploy:      true,
				EnvVars:         envMap,
				RequiredEnvVars: reqKeys,
			}
		}

		return nil
	}

	// 1. Check Root Directory
	if rootSvc := analyzeComponent(".", repoBase); rootSvc != nil {
		result.Services = append(result.Services, *rootSvc)
	}

	// 2. Check Monorepo Subdirectories if root is empty or has standard monorepo folders
	commonDirs := []string{"frontend", "backend", "client", "server", "web", "api", "app", "apps/web", "apps/api", "apps/frontend", "apps/backend", "packages/client", "packages/server", "services/api", "services/web"}
	monorepoManifests := []string{"package.json", "requirements.txt", "pyproject.toml", "go.mod", "Cargo.toml", "pom.xml", "build.gradle", "build.gradle.kts", "Gemfile", "composer.json", "mix.exs", "deno.json", "pubspec.yaml", "shard.yml", "Dockerfile"}
	for _, dir := range commonDirs {
		hasManifest := false
		for _, mf := range monorepoManifests {
			if hasFile(dir + "/" + mf) {
				hasManifest = true
				break
			}
		}
		if hasManifest {
			// Avoid adding duplicate if root already captured this directory
			alreadyAdded := false
			for _, s := range result.Services {
				if s.RootDir == dir {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				subName := strings.ReplaceAll(dir, "/", "-")
				if subSvc := analyzeComponent(dir, subName); subSvc != nil {
					// If root was just a generic wrapper, replace with specific monorepo services
					if len(result.Services) == 1 && result.Services[0].RootDir == "." && len(commonDirs) > 1 {
						// Keep root if it wasn't just monorepo root
					}
					result.Services = append(result.Services, *subSvc)
				}
			}
		}
	}

	// If monorepo components were found and root was generic node/empty, favor the components
	if len(result.Services) > 1 && result.Services[0].RootDir == "." {
		hasSubServices := false
		for _, s := range result.Services[1:] {
			if s.RootDir != "." {
				hasSubServices = true
				break
			}
		}
		if hasSubServices {
			result.Services = result.Services[1:]
		}
	}

	return result
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
	dbUriMap := make(map[string]string)
	dbHostMap := make(map[string]string)
	dbPortMap := make(map[string]string)

	for _, dbInfo := range req.Databases {
		dbName := fmt.Sprintf("%v", dbInfo["name"])
		engine := fmt.Sprintf("%v", dbInfo["engine"])
		if dbName == "" {
			continue
		}

		dbVersion := fmt.Sprintf("%v", dbInfo["version"])
		if dbVersion == "<nil>" {
			dbVersion = ""
		}

		dbRec, err := h.provisionDatabaseInternal(c.Context(), project.ID, dbName, engine, dbVersion, "", "")
		if err == nil && dbRec != nil {
			createdDatabases = append(createdDatabases, dbRec)
			var meta struct {
				ConnectionURI         string `json:"connectionUri"`
				InternalConnectionURI string `json:"internalConnectionUri"`
			}
			if dbRec.ResourceJSON != "" {
				_ = json.Unmarshal([]byte(dbRec.ResourceJSON), &meta)
			}
			uri := meta.InternalConnectionURI
			if uri == "" {
				uri = meta.ConnectionURI
			}
			dbUriMap[dbName] = uri
			dbUriMap[strings.ToLower(dbName)] = uri
			dbHostMap[dbName] = dbRec.InternalHostname
			dbHostMap[strings.ToLower(dbName)] = dbRec.InternalHostname
			dbPortMap[dbName] = fmt.Sprintf("%d", dbRec.InternalPort)
			dbPortMap[strings.ToLower(dbName)] = fmt.Sprintf("%d", dbRec.InternalPort)
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
		baseSlug := slugify(svcInfo.Slug)
		if baseSlug == "" || baseSlug == "app" {
			baseSlug = slugify(svcName)
		}
		if baseSlug == "" {
			baseSlug = "app"
		}
		
		actualSlug := baseSlug
		counter := 1
		for {
			exists, err := h.store.Services().SlugExists(c.Context(), actualSlug)
			_, inBatch := slugMap[actualSlug]
			if (err != nil || !exists) && !inBatch {
				break
			}
			counter++
			actualSlug = fmt.Sprintf("%s-%d", baseSlug, counter)
		}
		
		port := svcInfo.InternalPort
		if port <= 0 {
			port = 8080
			if svcInfo.Kind == "static" {
				port = 80
			} else if svcInfo.Preset == "python" {
				port = 5000
			} else if svcInfo.Preset == "node" {
				port = 3000
			}
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
		slugMap[actualSlug] = actualSlug
		urlMap[svcName] = fmt.Sprintf("https://%s.%s", actualSlug, rootDomain)
		urlMap[baseSlug] = fmt.Sprintf("https://%s.%s", actualSlug, rootDomain)
		urlMap[actualSlug] = fmt.Sprintf("https://%s.%s", actualSlug, rootDomain)
		internalUrlMap[svcName] = fmt.Sprintf("http://paas-svc-%s:%d", actualSlug, port)
		internalUrlMap[baseSlug] = fmt.Sprintf("http://paas-svc-%s:%d", actualSlug, port)
		internalUrlMap[actualSlug] = fmt.Sprintf("http://paas-svc-%s:%d", actualSlug, port)
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
			for dbName, dbUri := range dbUriMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("paas-db-%s", dbName), dbUri)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.connectionString}", dbName), dbUri)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.url}", dbName), dbUri)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.connectionString}", dbName), dbUri)
				if strings.EqualFold(k, "DATABASE_URL") && (val == "" || val == dbName || strings.HasPrefix(val, "paas-db-")) {
					val = dbUri
				}
			}
			for dbName, dbHost := range dbHostMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.host}", dbName), dbHost)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.host}", dbName), dbHost)
			}
			for dbName, dbPort := range dbPortMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.port}", dbName), dbPort)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.port}", dbName), dbPort)
			}
			resolvedEnv[k] = val
		}

		resMap := map[string]any{
			"gitRepoUrl":     req.RepoURL,
			"gitBranch":      branch,
			"rootDir":        svcInfo.RootDir,
			"rootDirectory":  svcInfo.RootDir,
			"buildCommand":   svcInfo.BuildCommand,
			"startCommand":   svcInfo.StartCommand,
			"presetId":       svcInfo.Preset,
			"runtimeVersion": svcInfo.RuntimeVersion,
			"mem_limit":      svcInfo.MemoryLimit,
			"cpu_limit":      svcInfo.CPULimit,
			"routes":         svcInfo.Routes,
			"env":            resolvedEnv,
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

func (h *Handler) handleGetServiceBlueprint(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}

	var resMap map[string]any
	if s.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}

	gitRepoUrl, _ := resMap["gitRepoUrl"].(string)
	gitBranch, _ := resMap["gitBranch"].(string)
	if gitBranch == "" {
		gitBranch = "main"
	}
	if gitRepoUrl == "" {
		return c.Status(400).JSON(fiber.Map{"error": "service does not have a git repository linked"})
	}

	cleanRepo := strings.TrimSuffix(gitRepoUrl, ".git")
	cleanRepo = strings.TrimPrefix(cleanRepo, "https://github.com/")
	cleanRepo = strings.TrimPrefix(cleanRepo, "http://github.com/")
	cleanRepo = strings.TrimPrefix(cleanRepo, "github.com/")

	candidates := []string{"klouds.yaml", "klouds.yml", ".klouds.yaml", "render.yaml", "render.yml"}
	var yamlContent string
	var detectedSource string

	for _, filename := range candidates {
		rawUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", cleanRepo, gitBranch, filename)
		req, err := nethttp.NewRequestWithContext(c.Context(), "GET", rawUrl, nil)
		if err != nil {
			continue
		}
		client := &nethttp.Client{Timeout: 6 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if len(body) > 0 {
				yamlContent = string(body)
				detectedSource = filename
				break
			}
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}

	if yamlContent == "" {
		return c.Status(404).JSON(fiber.Map{"error": "No klouds.yaml or render.yaml detected in repository"})
	}

	parsed := parseRenderYAMLString(yamlContent)
	var matchingSvc *ParsedRenderService
	for _, ps := range parsed.Services {
		if strings.EqualFold(ps.Name, s.Name) || strings.EqualFold(ps.Slug, s.Slug) {
			matchingSvc = &ps
			break
		}
	}
	if matchingSvc == nil && len(parsed.Services) > 0 {
		matchingSvc = &parsed.Services[0]
	}

	if matchingSvc == nil {
		return c.JSON(fiber.Map{
			"blueprintSource": detectedSource,
			"rawYaml":         yamlContent,
			"envVars":         map[string]string{},
			"routes":          []ParsedRoute{},
		})
	}

	return c.JSON(fiber.Map{
		"blueprintSource": detectedSource,
		"service":         matchingSvc,
		"envVars":         matchingSvc.EnvVars,
		"routes":          matchingSvc.Routes,
		"rawYaml":         yamlContent,
	})
}
