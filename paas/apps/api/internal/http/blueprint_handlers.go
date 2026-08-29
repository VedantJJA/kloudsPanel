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
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

// --- klouds.yaml / Blueprint Parser ------------------------------------------

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
	Privileged        bool              `json:"privileged,omitempty"`
}

type ParsedRenderResult struct {
	ProjectName string                `json:"project_name,omitempty"`
	Services    []ParsedRenderService `json:"services"`
	Databases   []fiber.Map           `json:"databases"`
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
	var inVolumes bool
	var inBuild bool
	var inDeploy bool
	var inSource bool
	var inResources bool
	var currentEnvKey string
	var currentRefType string // "service" or "database"
	var currentRefName string

	flushCurrent := func() {
		if currentDb != nil {
			res.Databases = append(res.Databases, *currentDb)
			currentDb = nil
		}
		if currentSvc != nil {
			lowerKind := strings.ToLower(currentSvc.Kind)
			lowerImg := strings.ToLower(currentSvc.Image)
			if lowerKind == "database" || lowerKind == "redis" || lowerKind == "postgres" || lowerKind == "postgresql" || lowerKind == "mysql" || lowerKind == "mongodb" || lowerKind == "clickhouse" ||
				strings.Contains(lowerImg, "redis") || strings.Contains(lowerImg, "postgres") || strings.Contains(lowerImg, "mysql") || strings.Contains(lowerImg, "mongo") || strings.Contains(lowerImg, "clickhouse") {
				engine := "postgres"
				if strings.Contains(lowerKind, "redis") || strings.Contains(lowerImg, "redis") {
					engine = "redis"
				} else if strings.Contains(lowerKind, "mysql") || strings.Contains(lowerImg, "mysql") {
					engine = "mysql"
				} else if strings.Contains(lowerKind, "mongo") || strings.Contains(lowerImg, "mongo") {
					engine = "mongodb"
				} else if strings.Contains(lowerKind, "clickhouse") || strings.Contains(lowerImg, "clickhouse") {
					engine = "clickhouse"
				}
				res.Databases = append(res.Databases, fiber.Map{
					"name":    currentSvc.Name,
					"engine":  engine,
					"version": currentSvc.RuntimeVersion,
				})
			} else if lowerKind != "rewrite" && lowerKind != "redirect" && lowerKind != "header" && lowerKind != "proxy" {
				if currentSvc.Kind == "" {
					currentSvc.Kind = "web"
				}
				if currentSvc.Slug == "" {
					currentSvc.Slug = strings.ToLower(currentSvc.Name)
				}
				res.Services = append(res.Services, *currentSvc)
			}
			currentSvc = nil
		}
		inEnvVars = false
		inRoutes = false
		inVolumes = false
		inBuild = false
		inDeploy = false
		inSource = false
		inResources = false
		currentEnvKey = ""
		currentRefType = ""
		currentRefName = ""
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Top-level project name
		if strings.HasPrefix(trimmed, "project:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				res.ProjectName = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
			continue
		}

		// Top-level sections
		if trimmed == "databases:" {
			flushCurrent()
			inDatabases = true
			inServices = false
			continue
		} else if trimmed == "services:" {
			flushCurrent()
			inServices = true
			inDatabases = false
			continue
		}

		// In top-level databases section
		if inDatabases {
			isNewDbItem := strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "- databaseName:") || strings.HasPrefix(trimmed, "- engine:") || strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- ") || trimmed == "-"
			if isNewDbItem {
				parts := strings.SplitN(trimmed, ":", 2)
				key := strings.Trim(strings.TrimPrefix(parts[0], "-"), " ")
				val := ""
				if len(parts) > 1 {
					val = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				}
				dbName := "database"
				engine := "postgres"
				if key == "name" || key == "databaseName" {
					dbName = val
				} else if key == "engine" || key == "type" {
					engine = domain.CanonicalizeEngine(val)
				}
				lower := strings.ToLower(dbName)
				if engine == "postgres" && (key != "engine" && key != "type") {
					if strings.Contains(lower, "redis") {
						engine = "redis"
					} else if strings.Contains(lower, "mysql") {
						engine = "mysql"
					} else if strings.Contains(lower, "mongo") {
						engine = "mongodb"
					} else if strings.Contains(lower, "clickhouse") {
						engine = "clickhouse"
					}
				}
				newDb := fiber.Map{
					"name":   dbName,
					"engine": engine,
				}
				res.Databases = append(res.Databases, newDb)
			} else if len(res.Databases) > 0 {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					k := strings.ToLower(strings.TrimSpace(parts[0]))
					val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					idx := len(res.Databases) - 1
					switch k {
					case "name":
						res.Databases[idx]["name"] = val
						lower := strings.ToLower(val)
						if strings.Contains(lower, "redis") {
							res.Databases[idx]["engine"] = "redis"
						} else if strings.Contains(lower, "mysql") {
							res.Databases[idx]["engine"] = "mysql"
						} else if strings.Contains(lower, "mongo") {
							res.Databases[idx]["engine"] = "mongodb"
						} else if strings.Contains(lower, "clickhouse") {
							res.Databases[idx]["engine"] = "clickhouse"
						}
					case "databasename", "database_name", "database":
						res.Databases[idx]["databaseName"] = val
						res.Databases[idx]["database_name"] = val
					case "user", "username":
						res.Databases[idx]["user"] = val
						res.Databases[idx]["username"] = val
					case "engine", "image", "type":
						res.Databases[idx]["engine"] = domain.CanonicalizeEngine(val)
					case "version", "runtime_version":
						res.Databases[idx]["version"] = sanitizeVersionString(val)
					}
				}
			}
			continue
		}

		// Service sub-section header triggers (4-space indent)
		if inServices && currentSvc != nil {
			if trimmed == "source:" {
				inSource = true
				inBuild = false
				inDeploy = false
				inResources = false
				inVolumes = false
				inEnvVars = false
				inRoutes = false
				continue
			} else if trimmed == "build:" {
				inBuild = true
				inSource = false
				inDeploy = false
				inResources = false
				inVolumes = false
				inEnvVars = false
				inRoutes = false
				continue
			} else if trimmed == "deploy:" {
				inDeploy = true
				inBuild = false
				inSource = false
				inResources = false
				inVolumes = false
				inEnvVars = false
				inRoutes = false
				continue
			} else if trimmed == "resources:" {
				inResources = true
				inBuild = false
				inDeploy = false
				inSource = false
				inVolumes = false
				inEnvVars = false
				inRoutes = false
				continue
			} else if trimmed == "volumes:" {
				inVolumes = true
				inResources = false
				inBuild = false
				inDeploy = false
				inSource = false
				inEnvVars = false
				inRoutes = false
				continue
			} else if trimmed == "env:" || trimmed == "envVars:" {
				inEnvVars = true
				inVolumes = false
				inResources = false
				inBuild = false
				inDeploy = false
				inSource = false
				inRoutes = false
				currentEnvKey = ""
				continue
			} else if trimmed == "routes:" {
				inRoutes = true
				inEnvVars = false
				inVolumes = false
				inResources = false
				inBuild = false
				inDeploy = false
				inSource = false
				continue
			}
		}

		// In services section: Detect new service start (2-space indent or top list item)
		isListServiceHeader := inServices && (strings.HasPrefix(rawLine, "  - ") || strings.HasPrefix(rawLine, "- ")) && (strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "- service:") || strings.HasPrefix(trimmed, "- kind:"))
		isMapServiceHeader := inServices && (strings.HasPrefix(rawLine, "  ") || strings.HasPrefix(rawLine, "\t")) && !strings.HasPrefix(rawLine, "    ") && !strings.HasPrefix(rawLine, "\t\t") && !strings.HasPrefix(trimmed, "-") && strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") &&
			trimmed != "env:" && trimmed != "envVars:" && trimmed != "source:" && trimmed != "build:" && trimmed != "deploy:" && trimmed != "resources:" && trimmed != "volumes:" && trimmed != "routes:" && trimmed != "healthCheckPath:" && trimmed != "headers:"

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
				} else if strings.Contains(lowerKey, "front") || strings.Contains(lowerKey, "client") || strings.Contains(lowerKey, "ui") {
					svcType = "static"
				}
			} else {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
					if strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "- kind:") {
						svcType = strings.ToLower(val)
					} else if strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "- service:") {
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
			continue
		}

		if currentSvc == nil {
			continue
		}

		// Property Parsing inside current service based on sub-section
		if inBuild {
			if strings.HasPrefix(trimmed, "command:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.BuildCommand = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			} else if strings.HasPrefix(trimmed, "engine:") || strings.HasPrefix(trimmed, "runtime:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.Env = val
					currentSvc.Preset = val
				}
			} else if strings.HasPrefix(trimmed, "output_dir:") || strings.HasPrefix(trimmed, "publish_dir:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.StaticPublishPath = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.Kind = "static"
				}
			}
		} else if inDeploy {
			if strings.HasPrefix(trimmed, "command:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.StartCommand = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			} else if strings.HasPrefix(trimmed, "port:") {
				var p int
				if _, err := fmt.Sscanf(trimmed, "port: %d", &p); err == nil && p > 0 {
					currentSvc.InternalPort = p
				}
			}
		} else if inSource {
			if strings.HasPrefix(trimmed, "directory:") || strings.HasPrefix(trimmed, "rootDir:") || strings.HasPrefix(trimmed, "path:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.RootDir = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			}
		} else if inResources {
			if strings.HasPrefix(trimmed, "cpu_limit:") || strings.HasPrefix(trimmed, "cpu:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.CPULimit = sanitizeResourceLimit(strings.Trim(strings.TrimSpace(parts[1]), `"'`))
				}
			} else if strings.HasPrefix(trimmed, "mem_limit:") || strings.HasPrefix(trimmed, "memory:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.MemoryLimit = sanitizeResourceLimit(strings.Trim(strings.TrimSpace(parts[1]), `"'`))
				}
			}
		} else if inVolumes {
			// Volumes metadata is preserved without spawning new services
			continue
		} else if inEnvVars {
			if strings.HasPrefix(trimmed, "- key:") || strings.HasPrefix(trimmed, "key:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentEnvKey = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.EnvVars[currentEnvKey] = ""
					currentRefType = ""
					currentRefName = ""
				}
			} else if strings.HasPrefix(trimmed, "value:") || strings.HasPrefix(trimmed, "- value:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" {
					val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.EnvVars[currentEnvKey] = val
					if currentEnvKey == "PORT" {
						var p int
						if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
							currentSvc.InternalPort = p
						}
					}
				}
			} else if strings.HasPrefix(trimmed, "fromService:") {
				currentRefType = "service"
			} else if strings.HasPrefix(trimmed, "fromDatabase:") {
				currentRefType = "database"
			} else if strings.HasPrefix(trimmed, "name:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" {
					currentRefName = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if currentRefType == "service" {
						currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}", currentRefName)
					} else if currentRefType == "database" {
						currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.connectionString}", currentRefName)
					}
				}
			} else if strings.HasPrefix(trimmed, "property:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" && currentRefName != "" {
					prop := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), `"'`))
					if currentRefType == "database" {
						switch prop {
						case "connectionstring", "url":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.connectionString}", currentRefName)
						case "host", "hostname":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.host}", currentRefName)
						case "port":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.port}", currentRefName)
						case "username", "user":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.username}", currentRefName)
						case "password", "pass":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.password}", currentRefName)
						case "database", "dbname", "name":
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${databases.%s.database}", currentRefName)
						}
					} else if currentRefType == "service" {
						if prop == "url" || prop == "endpoint" {
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.url}", currentRefName)
						} else if prop == "host" || prop == "hostname" {
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.host}", currentRefName)
						} else if prop == "internalurl" || prop == "internal_url" {
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.internalUrl}", currentRefName)
						} else if prop == "port" {
							currentSvc.EnvVars[currentEnvKey] = fmt.Sprintf("${services.%s.port}", currentRefName)
						}
					}
				}
			} else if strings.HasPrefix(trimmed, "format:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" && currentRefName != "" {
					fmtTemplate := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if strings.Contains(fmtTemplate, "{host}") {
						if strings.Contains(fmtTemplate, ".onrender.com") {
							fmtTemplate = strings.ReplaceAll(fmtTemplate, "{host}.onrender.com", fmt.Sprintf("${services.%s.host}", currentRefName))
						} else {
							fmtTemplate = strings.ReplaceAll(fmtTemplate, "{host}", fmt.Sprintf("${services.%s.host}", currentRefName))
						}
					}
					currentSvc.EnvVars[currentEnvKey] = fmtTemplate
				}
			} else if strings.HasPrefix(trimmed, "generateValue:") || strings.HasPrefix(trimmed, "generate_value:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" {
					val := strings.ToLower(strings.TrimSpace(parts[1]))
					if val == "true" || val == "yes" {
						if currentSvc.EnvVars[currentEnvKey] == "" {
							currentSvc.EnvVars[currentEnvKey] = generateSecureRandomSecret(16)
						}
					}
				}
			} else if strings.HasPrefix(trimmed, "sync:") || strings.HasPrefix(trimmed, "required:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && currentEnvKey != "" {
					val := strings.ToLower(strings.TrimSpace(parts[1]))
					if val == "false" || val == "true" || val == "yes" {
						currentSvc.RequiredEnvVars = append(currentSvc.RequiredEnvVars, currentEnvKey)
					}
				}
			}
		} else if inRoutes {
			if strings.HasPrefix(trimmed, "- type:") || strings.HasPrefix(trimmed, "type:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					rType := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if strings.HasPrefix(trimmed, "- type:") || len(currentSvc.Routes) == 0 {
						currentSvc.Routes = append(currentSvc.Routes, ParsedRoute{Type: rType})
					} else {
						currentSvc.Routes[len(currentSvc.Routes)-1].Type = rType
					}
				}
			} else if strings.HasPrefix(trimmed, "- source:") || strings.HasPrefix(trimmed, "source:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					src := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if strings.HasPrefix(trimmed, "- source:") || len(currentSvc.Routes) == 0 {
						currentSvc.Routes = append(currentSvc.Routes, ParsedRoute{Type: "rewrite", Source: src})
					} else {
						currentSvc.Routes[len(currentSvc.Routes)-1].Source = src
					}
				}
			} else if strings.HasPrefix(trimmed, "destination:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 && len(currentSvc.Routes) > 0 {
					dest := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.Routes[len(currentSvc.Routes)-1].Destination = dest
				}
			}
		} else {
			// Direct root-level properties of service
			if strings.HasPrefix(trimmed, "name:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.Name = val
					currentSvc.Slug = strings.ToLower(val)
				}
			} else if strings.HasPrefix(trimmed, "type:") || strings.HasPrefix(trimmed, "kind:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.Kind = strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), `"'`))
				}
			} else if strings.HasPrefix(trimmed, "image:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					img := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					currentSvc.Image = img
					lowerImg := strings.ToLower(img)
					if strings.Contains(lowerImg, "redis") || strings.Contains(lowerImg, "postgres") || strings.Contains(lowerImg, "mysql") || strings.Contains(lowerImg, "mongo") || strings.Contains(lowerImg, "clickhouse") {
						currentSvc.Kind = "database"
					}
				}
			} else if strings.HasPrefix(trimmed, "port:") {
				var p int
				if _, err := fmt.Sscanf(trimmed, "port: %d", &p); err == nil && p > 0 {
					currentSvc.InternalPort = p
				}
			} else if strings.HasPrefix(trimmed, "buildCommand:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.BuildCommand = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			} else if strings.HasPrefix(trimmed, "startCommand:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.StartCommand = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			} else if strings.HasPrefix(trimmed, "rootDir:") || strings.HasPrefix(trimmed, "directory:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					currentSvc.RootDir = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
			} else if strings.HasPrefix(trimmed, "runtime:") || strings.HasPrefix(trimmed, "preset:") || strings.HasPrefix(trimmed, "env:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), `"'`))
					if val != "" {
						currentSvc.Env = val
						currentSvc.Preset = val
					}
				}
			} else if strings.HasPrefix(trimmed, "privileged:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.ToLower(strings.TrimSpace(parts[1]))
					currentSvc.Privileged = (val == "true" || val == "yes")
				}
			} else if strings.HasPrefix(trimmed, "mode:") {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) > 1 {
					val := strings.ToLower(strings.TrimSpace(parts[1]))
					if val == "privileged" {
						currentSvc.Privileged = true
					}
				}
			}
		}
	}

	flushCurrent()

	// Ensure distinct unique slugs
	usedSlugs := make(map[string]int)
	for i := range res.Services {
		s := &res.Services[i]
		if s.Name == "" {
			s.Name = fmt.Sprintf("service-%d", i+1)
		}
		if s.Slug == "" {
			s.Slug = strings.ToLower(strings.ReplaceAll(s.Name, "_", "-"))
		}
		baseSlug := s.Slug
		count := usedSlugs[baseSlug]
		if count > 0 {
			s.Slug = fmt.Sprintf("%s-%d", baseSlug, count+1)
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
			clean := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(rawURL), "/"), ".git")
			parts := strings.Split(clean, "github.com/")
			if len(parts) == 2 {
				pathParts := strings.Split(parts[1], "/")
				if len(pathParts) >= 2 {
					repoPath := fmt.Sprintf("%s/%s", pathParts[0], pathParts[1])
					repoBase = pathParts[1]

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
					branchesToTry := []string{"main", "master", "HEAD", "develop", "dev", "trunk"}
					fetchRaw := func(filename string) string {
						// 1. Try raw.githubusercontent.com across branches
						for _, br := range branchesToTry {
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
									content := b.String()
									if strings.TrimSpace(content) != "" && !strings.Contains(content, "404: Not Found") {
										return content
									}
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
								content := b.String()
								if strings.TrimSpace(content) != "" && !strings.Contains(content, "404: Not Found") {
									return content
								}
							}
							if resp != nil {
								resp.Body.Close()
							}
						}
						return ""
					}

					// Efficient Single-Request Repository Tree Scan
					treeFiles := make(map[string]bool)
					for _, br := range branchesToTry {
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

					// Check for Blueprint files (prioritize render.yaml and klouds.yaml)
					blueprintCandidates := []string{"render.yaml", "render.yml", ".render.yaml", ".render.yml", "klouds.yaml", "klouds.yml", ".klouds.yaml", ".klouds.yml"}
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

		// Helper to register detected databases and auto-inject connection env vars
		registerDetectedDB := func(engine string) {
			dbName := fmt.Sprintf("%s-%s", repoBase, engine)
			exists := false
			for _, db := range result.Databases {
				if fmt.Sprintf("%v", db["engine"]) == engine {
					exists = true
					dbName = fmt.Sprintf("%v", db["name"])
					break
				}
			}
			if !exists {
				result.Databases = append(result.Databases, fiber.Map{
					"name":   dbName,
					"engine": engine,
				})
			}
			if envMap == nil {
				envMap = make(map[string]string)
			}
			switch engine {
			case "postgres":
				if envMap["DATABASE_URL"] == "" {
					envMap["DATABASE_URL"] = fmt.Sprintf("${databases.%s.connectionString}", dbName)
				}
				if envMap["DB_HOST"] == "" {
					envMap["DB_HOST"] = fmt.Sprintf("${databases.%s.host}", dbName)
				}
				if envMap["DB_PORT"] == "" {
					envMap["DB_PORT"] = "5432"
				}
				if envMap["DB_USER"] == "" {
					envMap["DB_USER"] = "postgres"
				}
			case "redis":
				if envMap["REDIS_URL"] == "" {
					envMap["REDIS_URL"] = fmt.Sprintf("${databases.%s.connectionString}", dbName)
				}
				if envMap["REDIS_HOST"] == "" {
					envMap["REDIS_HOST"] = fmt.Sprintf("${databases.%s.host}", dbName)
				}
				if envMap["REDIS_PORT"] == "" {
					envMap["REDIS_PORT"] = "6379"
				}
			case "mysql":
				if envMap["DATABASE_URL"] == "" {
					envMap["DATABASE_URL"] = fmt.Sprintf("${databases.%s.connectionString}", dbName)
				}
				if envMap["MYSQL_HOST"] == "" {
					envMap["MYSQL_HOST"] = fmt.Sprintf("${databases.%s.host}", dbName)
				}
				if envMap["MYSQL_PORT"] == "" {
					envMap["MYSQL_PORT"] = "3306"
				}
			case "mongodb":
				if envMap["MONGODB_URI"] == "" && envMap["MONGO_URL"] == "" {
					envMap["MONGODB_URI"] = fmt.Sprintf("${databases.%s.connectionString}", dbName)
				}
				if envMap["MONGO_HOST"] == "" {
					envMap["MONGO_HOST"] = fmt.Sprintf("${databases.%s.host}", dbName)
				}
			case "clickhouse":
				if envMap["CLICKHOUSE_URL"] == "" {
					envMap["CLICKHOUSE_URL"] = fmt.Sprintf("${databases.%s.connectionString}", dbName)
				}
			}
		}

		detectDatabaseDeps := func(deps map[string]bool, rawText string) {
			combined := strings.ToLower(rawText)
			// PostgreSQL
			if deps["pg"] || deps["pg-promise"] || deps["postgres"] || deps["@prisma/client"] || deps["typeorm"] || deps["knex"] || deps["sequelize"] || deps["drizzle-orm"] || deps["slonik"] ||
				deps["psycopg2"] || deps["psycopg2-binary"] || deps["psycopg"] || deps["asyncpg"] || deps["databases"] || deps["tortoise-orm"] ||
				deps["github.com/lib/pq"] || deps["github.com/jackc/pgx"] || deps["gorm.io/driver/postgres"] ||
				deps["tokio-postgres"] || deps["sqlx"] || deps["diesel"] ||
				deps["ext-pdo_pgsql"] || deps["org.postgresql"] || strings.Contains(combined, "psycopg") || strings.Contains(combined, "asyncpg") || strings.Contains(combined, "lib/pq") || strings.Contains(combined, "pgx") || strings.Contains(combined, "org.postgresql") {
				registerDetectedDB("postgres")
			}
			// Redis
			if deps["redis"] || deps["ioredis"] || deps["@ioredis/commands"] || deps["bull"] || deps["bullmq"] || deps["connect-redis"] || deps["redis-om"] ||
				deps["aioredis"] || deps["celery"] ||
				deps["github.com/redis/go-redis"] || deps["github.com/go-redis/redis"] ||
				deps["fred"] || deps["predis/predis"] || deps["ext-redis"] ||
				deps["jedis"] || deps["lettuce"] || strings.Contains(combined, "ioredis") || strings.Contains(combined, "go-redis") {
				registerDetectedDB("redis")
			}
			// MySQL / MariaDB
			if deps["mysql"] || deps["mysql2"] || deps["mariadb"] ||
				deps["pymysql"] || deps["mysqlclient"] || deps["aiomysql"] || deps["mysql-connector-python"] ||
				deps["github.com/go-sql-driver/mysql"] || deps["gorm.io/driver/mysql"] ||
				deps["mysql_async"] || deps["ext-pdo_mysql"] || deps["ext-mysqli"] ||
				deps["mysql-connector-java"] || strings.Contains(combined, "pymysql") || strings.Contains(combined, "mysqlclient") {
				registerDetectedDB("mysql")
			}
			// MongoDB
			if deps["mongodb"] || deps["mongoose"] ||
				deps["pymongo"] || deps["motor"] || deps["mongoengine"] ||
				deps["go.mongodb.org/mongo-driver"] ||
				deps["mongodb/mongodb"] || deps["spring-boot-starter-data-mongodb"] || strings.Contains(combined, "mongoose") || strings.Contains(combined, "pymongo") {
				registerDetectedDB("mongodb")
			}
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

			// Run database dependency detector
			detectDatabaseDeps(deps, pkgContent)

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

			detectDatabaseDeps(map[string]bool{}, combined)

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
			goModContent := fetchRaw(prefix + "go.mod")
			detectDatabaseDeps(map[string]bool{}, goModContent)
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
			cargoContent := fetchRaw(prefix + "Cargo.toml")
			detectDatabaseDeps(map[string]bool{}, cargoContent)
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
			pomContent := fetchRaw(prefix + "pom.xml")
			gradleContent := fetchRaw(prefix + "build.gradle")
			detectDatabaseDeps(map[string]bool{}, pomContent+"\n"+gradleContent)
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
			composerContent := fetchRaw(prefix + "composer.json")
			detectDatabaseDeps(map[string]bool{}, composerContent)
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
			gemContent := fetchRaw(prefix + "Gemfile")
			detectDatabaseDeps(map[string]bool{}, gemContent)
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
			mixContent := fetchRaw(prefix + "mix.exs")
			detectDatabaseDeps(map[string]bool{}, mixContent)
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

	// Auto-detect databases referenced in environment variables (.env.example or service envVars)
	dbAdded := make(map[string]bool)
	for _, db := range result.Databases {
		dbAdded[fmt.Sprintf("%v", db["engine"])] = true
	}
	for _, s := range result.Services {
		for k, v := range s.EnvVars {
			kUpper := strings.ToUpper(k)
			vLower := strings.ToLower(v)
			if strings.Contains(kUpper, "DATABASE_URL") || strings.Contains(kUpper, "POSTGRES") || strings.Contains(kUpper, "PGDATABASE") || strings.Contains(vLower, "postgres") {
				if !dbAdded["postgres"] {
					dbName := fmt.Sprintf("%s-postgres", repoBase)
					result.Databases = append(result.Databases, fiber.Map{
						"name":   dbName,
						"engine": "postgres",
					})
					dbAdded["postgres"] = true
				}
			} else if strings.Contains(kUpper, "REDIS_URL") || strings.Contains(kUpper, "REDIS_HOST") || strings.Contains(vLower, "redis") {
				if !dbAdded["redis"] {
					dbName := fmt.Sprintf("%s-redis", repoBase)
					result.Databases = append(result.Databases, fiber.Map{
						"name":   dbName,
						"engine": "redis",
					})
					dbAdded["redis"] = true
				}
			} else if strings.Contains(kUpper, "MYSQL_URL") || strings.Contains(kUpper, "MYSQL_HOST") || strings.Contains(kUpper, "MYSQL_DATABASE") || strings.Contains(vLower, "mysql") {
				if !dbAdded["mysql"] {
					dbName := fmt.Sprintf("%s-mysql", repoBase)
					result.Databases = append(result.Databases, fiber.Map{
						"name":   dbName,
						"engine": "mysql",
					})
					dbAdded["mysql"] = true
				}
			} else if strings.Contains(kUpper, "MONGO_URI") || strings.Contains(kUpper, "MONGODB_URI") || strings.Contains(kUpper, "MONGO_URL") || strings.Contains(vLower, "mongo") {
				if !dbAdded["mongodb"] {
					dbName := fmt.Sprintf("%s-mongodb", repoBase)
					result.Databases = append(result.Databases, fiber.Map{
						"name":   dbName,
						"engine": "mongodb",
					})
					dbAdded["mongodb"] = true
				}
			}
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
	dbUserMap := make(map[string]string)
	dbPassMap := make(map[string]string)
	dbNameMap := make(map[string]string)

	for _, dbInfo := range req.Databases {
		dbName := fmt.Sprintf("%v", dbInfo["name"])
		if dbName == "" || dbName == "<nil>" {
			continue
		}
		rawEngine := fmt.Sprintf("%v", dbInfo["engine"])
		if rawEngine == "<nil>" || rawEngine == "null" {
			rawEngine = ""
		}
		engine := domain.CanonicalizeEngine(rawEngine)
		if rawEngine == "" {
			lowerName := strings.ToLower(dbName)
			if strings.Contains(lowerName, "redis") {
				engine = "redis"
			} else if strings.Contains(lowerName, "mysql") {
				engine = "mysql"
			} else if strings.Contains(lowerName, "mongo") {
				engine = "mongodb"
			} else if strings.Contains(lowerName, "clickhouse") {
				engine = "clickhouse"
			}
		}

		dbVersion := fmt.Sprintf("%v", dbInfo["version"])
		if dbVersion == "<nil>" || dbVersion == "auto" || dbVersion == "null" {
			dbVersion = ""
		}

		var customPass, customDbName string
		if dbInfo["databaseName"] != nil && fmt.Sprintf("%v", dbInfo["databaseName"]) != "<nil>" && fmt.Sprintf("%v", dbInfo["databaseName"]) != "" {
			customDbName = fmt.Sprintf("%v", dbInfo["databaseName"])
		} else if dbInfo["database_name"] != nil && fmt.Sprintf("%v", dbInfo["database_name"]) != "<nil>" && fmt.Sprintf("%v", dbInfo["database_name"]) != "" {
			customDbName = fmt.Sprintf("%v", dbInfo["database_name"])
		} else if dbInfo["database"] != nil && fmt.Sprintf("%v", dbInfo["database"]) != "<nil>" && fmt.Sprintf("%v", dbInfo["database"]) != "" {
			customDbName = fmt.Sprintf("%v", dbInfo["database"])
		}
		if rawEnv, ok := dbInfo["env"].([]any); ok {
			for _, eItem := range rawEnv {
				if eMap, ok := eItem.(map[string]any); ok {
					k := fmt.Sprintf("%v", eMap["key"])
					v := fmt.Sprintf("%v", eMap["value"])
					if strings.EqualFold(k, "POSTGRES_DB") || strings.EqualFold(k, "MYSQL_DATABASE") {
						customDbName = v
					} else if strings.EqualFold(k, "POSTGRES_PASSWORD") || strings.EqualFold(k, "MYSQL_ROOT_PASSWORD") {
						customPass = v
					}
				}
			}
		}

		dbRec, err := h.provisionDatabaseInternal(c.Context(), project.ID, dbName, engine, dbVersion, customPass, customDbName)
		if err == nil && dbRec != nil {
			createdDatabases = append(createdDatabases, dbRec)
			var meta struct {
				ConnectionURI         string `json:"connectionUri"`
				InternalConnectionURI string `json:"internalConnectionUri"`
				Username              string `json:"username"`
				Password              string `json:"password"`
				DatabaseName          string `json:"databaseName"`
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
			dbUserMap[dbName] = meta.Username
			dbUserMap[strings.ToLower(dbName)] = meta.Username
			dbPassMap[dbName] = meta.Password
			dbPassMap[strings.ToLower(dbName)] = meta.Password
			dbNameMap[dbName] = meta.DatabaseName
			dbNameMap[strings.ToLower(dbName)] = meta.DatabaseName
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
			AutoDeploy:    true,
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
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.connectionString}", dbName), dbUri)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.url}", dbName), dbUri)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.connectionString}", dbName), dbUri)
				if strings.EqualFold(k, "DATABASE_URL") && (val == "" || val == dbName || val == fmt.Sprintf("paas-db-%s", strings.ToLower(dbName))) {
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
			for dbName, dbUser := range dbUserMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.username}", dbName), dbUser)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.user}", dbName), dbUser)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.username}", dbName), dbUser)
			}
			for dbName, dbPass := range dbPassMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.password}", dbName), dbPass)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.pass}", dbName), dbPass)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.password}", dbName), dbPass)
			}
			for dbName, dbDatabase := range dbNameMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.database}", dbName), dbDatabase)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.databaseName}", dbName), dbDatabase)
				val = strings.ReplaceAll(val, fmt.Sprintf("${databases.%s.name}", dbName), dbDatabase)
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s.database}", dbName), dbDatabase)
			}
			resolvedEnv[k] = val
		}

		resMap := map[string]any{
			"gitRepoUrl":      req.RepoURL,
			"gitBranch":       branch,
			"rootDir":         svcInfo.RootDir,
			"rootDirectory":   svcInfo.RootDir,
			"buildCommand":    svcInfo.BuildCommand,
			"build_command":   svcInfo.BuildCommand,
			"startCommand":    svcInfo.StartCommand,
			"start_command":   svcInfo.StartCommand,
			"presetId":        svcInfo.Preset,
			"preset":          svcInfo.Preset,
			"runtimeVersion":  svcInfo.RuntimeVersion,
			"runtime_version": svcInfo.RuntimeVersion,
			"mem_limit":       svcInfo.MemoryLimit,
			"cpu_limit":       svcInfo.CPULimit,
			"routes":          svcInfo.Routes,
			"env":             resolvedEnv,
			"env_vars":        resolvedEnv,
		}
		resJSON, _ := json.Marshal(resMap)
		s.ResourceJSON = string(resJSON)

		if err := h.store.Services().Create(c.Context(), s); err != nil {
			continue
		}

		// Record explicit ServiceDatabaseLinks for detected database references
		for _, dbObj := range createdDatabases {
			if dbRec, ok := dbObj.(*domain.Database); ok && dbRec != nil {
				for k, v := range svcInfo.EnvVars {
					if strings.Contains(v, fmt.Sprintf("${databases.%s", dbRec.Name)) ||
						strings.Contains(v, fmt.Sprintf("${%s.", dbRec.Name)) ||
						strings.EqualFold(k, "DATABASE_URL") ||
						strings.EqualFold(k, "REDIS_URL") ||
						strings.EqualFold(k, "MONGODB_URI") {
						linkProp := "connectionString"
						if strings.Contains(v, ".host}") {
							linkProp = "host"
						} else if strings.Contains(v, ".port}") {
							linkProp = "port"
						} else if strings.Contains(v, ".user}") || strings.Contains(v, ".username}") {
							linkProp = "username"
						} else if strings.Contains(v, ".password}") || strings.Contains(v, ".pass}") {
							linkProp = "password"
						} else if strings.Contains(v, ".database}") || strings.Contains(v, ".databaseName}") {
							linkProp = "database"
						}
						_ = h.store.ServiceDatabaseLinks().Create(c.Context(), &domain.ServiceDatabaseLink{
							ID:             ids.NewV7(),
							ServiceID:      s.ID,
							DatabaseID:     dbRec.ID,
							EnvVarName:     k,
							ConnectionKind: domain.ConnectionInternal,
							Property:       linkProp,
							CreatedAt:      time.Now().UTC(),
							UpdatedAt:      time.Now().UTC(),
						})
					}
				}
			}
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

	clean := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(gitRepoUrl), "/"), ".git")
	cleanRepo := clean
	if strings.Contains(clean, "github.com/") {
		parts := strings.Split(clean, "github.com/")
		if len(parts) == 2 {
			sub := strings.Split(parts[1], "/")
			if len(sub) >= 2 {
				cleanRepo = fmt.Sprintf("%s/%s", sub[0], sub[1])
			}
		}
	}

	candidates := []string{"render.yaml", "render.yml", ".render.yaml", ".render.yml", "klouds.yaml", "klouds.yml", ".klouds.yaml", ".klouds.yml"}
	branchesToTry := []string{gitBranch, "main", "master", "HEAD"}
	var yamlContent string
	var detectedSource string

	client := &nethttp.Client{Timeout: 6 * time.Second}
	for _, filename := range candidates {
		for _, br := range branchesToTry {
			rawUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", cleanRepo, br, filename)
			req, err := nethttp.NewRequestWithContext(c.Context(), "GET", rawUrl, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "kloudsPanel-App/1.0")
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == 200 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				content := string(body)
				if strings.TrimSpace(content) != "" && !strings.Contains(content, "404: Not Found") {
					yamlContent = content
					detectedSource = filename
					break
				}
			}
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}
		if yamlContent != "" {
			break
		}
	}

	if yamlContent == "" {
		return c.Status(404).JSON(fiber.Map{"error": "No render.yaml or klouds.yaml detected in repository"})
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
