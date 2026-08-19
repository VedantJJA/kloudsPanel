package http

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/domain/ids"
)

// resolveLinkValue resolves a specific database property based on connection kind.
func resolveLinkValue(db *domain.Database, link *domain.ServiceDatabaseLink) string {
	if db == nil || link == nil {
		return ""
	}

	var meta struct {
		ConnectionURI         string `json:"connectionUri"`
		InternalConnectionURI string `json:"internalConnectionUri"`
		ExternalConnectionURI string `json:"externalConnectionUri"`
		ExternalHost          string `json:"externalHost"`
		ExternalPort          int    `json:"externalPort"`
		Username              string `json:"username"`
		Password              string `json:"password"`
		DatabaseName          string `json:"databaseName"`
	}
	if db.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
	}

	isExternal := link.ConnectionKind == domain.ConnectionExternal

	// Resolve URI
	uri := meta.InternalConnectionURI
	if isExternal {
		if meta.ExternalConnectionURI != "" {
			uri = meta.ExternalConnectionURI
		} else {
			uri = meta.ConnectionURI
		}
	} else if uri == "" {
		uri = meta.ConnectionURI
	}

	if uri == "" && !isExternal {
		// Construct internal URI if not set
		engine := domain.CanonicalizeEngine(string(db.Engine))
		user := meta.Username
		if user == "" {
			if engine == "mysql" {
				user = "root"
			} else {
				user = "postgres"
			}
		}
		host := db.InternalHostname
		if host == "" {
			host = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
		}
		port := db.InternalPort
		if port == 0 {
			port = 5432
		}
		dbName := meta.DatabaseName
		if dbName == "" {
			dbName = db.Name
		}
		if engine == "redis" {
			if meta.Password != "" {
				uri = fmt.Sprintf("redis://:%s@%s:%d", meta.Password, host, port)
			} else {
				uri = fmt.Sprintf("redis://%s:%d", host, port)
			}
		} else {
			uri = fmt.Sprintf("%s://%s:%s@%s:%d/%s?sslmode=disable", engine, user, meta.Password, host, port, dbName)
		}
	}

	// Resolve Host
	host := db.InternalHostname
	if host == "" {
		host = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}
	if isExternal {
		if meta.ExternalHost != "" {
			host = meta.ExternalHost
		} else {
			host = getRootDomain()
		}
	}

	// Resolve Port
	port := fmt.Sprintf("%d", db.InternalPort)
	if isExternal {
		if meta.ExternalPort > 0 {
			port = fmt.Sprintf("%d", meta.ExternalPort)
		}
	} else if db.InternalPort == 0 {
		engine := domain.CanonicalizeEngine(string(db.Engine))
		if engine == "mysql" {
			port = "3306"
		} else if engine == "redis" {
			port = "6379"
		} else if engine == "mongodb" {
			port = "27017"
		} else if engine == "clickhouse" {
			port = "8123"
		} else {
			port = "5432"
		}
	}

	// Resolve User
	user := meta.Username
	if user == "" {
		engine := domain.CanonicalizeEngine(string(db.Engine))
		if engine == "mysql" {
			user = "root"
		} else {
			user = "postgres"
		}
	}

	// Resolve Database name
	dbName := meta.DatabaseName
	if dbName == "" {
		dbName = db.Name
	}

	switch strings.ToLower(link.Property) {
	case "host":
		return host
	case "port":
		return port
	case "user", "username":
		return user
	case "password", "pass":
		return meta.Password
	case "database", "databasename", "name":
		return dbName
	case "connectionstring", "url", "":
		return uri
	default:
		return uri
	}
}

// redactSensitiveValue masks passwords and connection strings containing passwords.
func redactSensitiveValue(val string, property string) string {
	if val == "" {
		return ""
	}
	if strings.EqualFold(property, "password") || strings.EqualFold(property, "pass") {
		return "********"
	}
	if strings.Contains(val, "://") {
		if u, err := url.Parse(val); err == nil && u.User != nil {
			if pass, hasPass := u.User.Password(); hasPass && pass != "" {
				user := u.User.Username()
				target := fmt.Sprintf("%s:%s@", user, pass)
				replacement := fmt.Sprintf("%s:********@", user)
				return strings.Replace(val, target, replacement, 1)
			}
		}
	}
	return val
}

// POST /api/v1/services/:id/database-links
func (h *Handler) handleCreateServiceDatabaseLink(c fiber.Ctx) error {
	serviceID := c.Params("id")
	svc, err := h.store.Services().GetByID(c.Context(), serviceID)
	if err != nil || svc == nil {
		return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Service '%s' not found", serviceID)})
	}

	var req struct {
		DatabaseID     string `json:"databaseId"`
		EnvVarName     string `json:"envVarName"`
		ConnectionKind string `json:"connectionKind"`
		Property       string `json:"property"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.DatabaseID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "databaseId is required"})
	}
	if strings.TrimSpace(req.EnvVarName) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "envVarName is required"})
	}

	db, err := h.store.Databases().GetByID(c.Context(), req.DatabaseID)
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Database '%s' not found", req.DatabaseID)})
	}

	// Project-scoping check: verify database belongs to the same project as the service
	if db.ProjectID != svc.ProjectID {
		return c.Status(400).JSON(fiber.Map{"error": "Database must belong to the same project as the service"})
	}

	connKind := domain.ConnectionInternal
	if strings.EqualFold(req.ConnectionKind, "external") {
		connKind = domain.ConnectionExternal
	}

	prop := req.Property
	if prop == "" {
		prop = "connectionString"
	}

	now := time.Now().UTC()
	link := &domain.ServiceDatabaseLink{
		ID:             ids.NewV7(),
		ServiceID:      svc.ID,
		DatabaseID:     db.ID,
		EnvVarName:     strings.TrimSpace(req.EnvVarName),
		ConnectionKind: connKind,
		Property:       prop,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.store.ServiceDatabaseLinks().Create(c.Context(), link); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "constraint") {
			return c.Status(409).JSON(fiber.Map{
				"error": fmt.Sprintf("An environment variable named '%s' is already linked to this service", link.EnvVarName),
			})
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create database link: " + err.Error()})
	}

	// Write resolved value into service's resource_json.env immediately so it shows in the UI
	resolvedVal := resolveLinkValue(db, link)
	resMap := make(map[string]any)
	if svc.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(svc.ResourceJSON), &resMap)
	}
	envMap := make(map[string]string)
	if rawEnv, ok := resMap["env"].(map[string]any); ok {
		for k, v := range rawEnv {
			envMap[k] = fmt.Sprintf("%v", v)
		}
	} else if rawEnv2, ok := resMap["env_vars"].(map[string]any); ok {
		for k, v := range rawEnv2 {
			envMap[k] = fmt.Sprintf("%v", v)
		}
	}
	envMap[link.EnvVarName] = resolvedVal
	resMap["env"] = envMap
	resMap["env_vars"] = envMap
	updatedBytes, _ := json.Marshal(resMap)
	svc.ResourceJSON = string(updatedBytes)
	_ = h.store.Services().Update(c.Context(), svc)

	// Trigger deployment for the linked service
	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), svc.ID)
	nowTime := time.Now().UTC()
	dep := &domain.Deployment{
		ID:          ids.NewV7(),
		ServiceID:   svc.ID,
		Sequence:    seq,
		Trigger:     domain.TriggerManual,
		Status:      domain.DeploymentQueued,
		BuildDriver: "docker",
		StartedAt:   &nowTime,
		CreatedAt:   nowTime,
		UpdatedAt:   nowTime,
	}
	_ = h.store.Deployments().Create(c.Context(), dep)
	go h.executeDeployment(svc, dep, getRootDomain())

	return c.Status(201).JSON(fiber.Map{
		"link": link,
		"preview": fiber.Map{
			"envVarName":    link.EnvVarName,
			"resolvedValue": redactSensitiveValue(resolvedVal, link.Property),
			"databaseName":  db.Name,
		},
		"deploymentId": dep.ID,
	})
}

// GET /api/v1/services/:id/database-links
func (h *Handler) handleListServiceDatabaseLinks(c fiber.Ctx) error {
	serviceID := c.Params("id")
	links, err := h.store.ServiceDatabaseLinks().ListForService(c.Context(), serviceID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	reveal := c.Query("reveal") == "true"

	type linkResponseItem struct {
		ID             string                `json:"id"`
		ServiceID      string                `json:"service_id"`
		DatabaseID     string                `json:"database_id"`
		DatabaseName   string                `json:"database_name"`
		DatabaseEngine string                `json:"database_engine"`
		EnvVarName     string                `json:"env_var_name"`
		ConnectionKind domain.ConnectionKind `json:"connection_kind"`
		Property       string                `json:"property"`
		PreviewValue   string                `json:"preview_value"`
		CreatedAt      time.Time             `json:"created_at"`
		UpdatedAt      time.Time             `json:"updated_at"`
	}

	out := make([]linkResponseItem, 0, len(links))
	for _, l := range links {
		db, _ := h.store.Databases().GetByID(c.Context(), l.DatabaseID)
		dbName := ""
		dbEngine := ""
		val := ""
		if db != nil {
			dbName = db.Name
			dbEngine = string(db.Engine)
			val = resolveLinkValue(db, l)
			if !reveal {
				val = redactSensitiveValue(val, l.Property)
			}
		}

		out = append(out, linkResponseItem{
			ID:             l.ID,
			ServiceID:      l.ServiceID,
			DatabaseID:     l.DatabaseID,
			DatabaseName:   dbName,
			DatabaseEngine: dbEngine,
			EnvVarName:     l.EnvVarName,
			ConnectionKind: l.ConnectionKind,
			Property:       l.Property,
			PreviewValue:   val,
			CreatedAt:      l.CreatedAt,
			UpdatedAt:      l.UpdatedAt,
		})
	}

	return c.JSON(fiber.Map{"links": out})
}

// DELETE /api/v1/services/:id/database-links/:linkId
func (h *Handler) handleDeleteServiceDatabaseLink(c fiber.Ctx) error {
	linkID := c.Params("linkId")
	if err := h.store.ServiceDatabaseLinks().Delete(c.Context(), linkID); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Database link not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"note":    "Link removed. The env var was left in place; edit service environment variables to remove it if desired.",
	})
}

// GET /api/v1/databases/:id/linked-services
func (h *Handler) handleGetDatabaseLinkedServices(c fiber.Ctx) error {
	databaseID := c.Params("id")
	links, err := h.store.ServiceDatabaseLinks().ListForDatabase(c.Context(), databaseID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	type linkedServiceItem struct {
		LinkID         string                `json:"link_id"`
		ServiceID      string                `json:"service_id"`
		ServiceName    string                `json:"service_name"`
		ServiceSlug    string                `json:"service_slug"`
		ProjectID      string                `json:"project_id"`
		EnvVarName     string                `json:"env_var_name"`
		ConnectionKind domain.ConnectionKind `json:"connection_kind"`
		Property       string                `json:"property"`
		CreatedAt      time.Time             `json:"created_at"`
	}

	out := make([]linkedServiceItem, 0, len(links))
	for _, l := range links {
		svc, _ := h.store.Services().GetByID(c.Context(), l.ServiceID)
		svcName := ""
		svcSlug := ""
		projID := ""
		if svc != nil {
			svcName = svc.Name
			svcSlug = svc.Slug
			projID = svc.ProjectID
		}
		out = append(out, linkedServiceItem{
			LinkID:         l.ID,
			ServiceID:      l.ServiceID,
			ServiceName:    svcName,
			ServiceSlug:    svcSlug,
			ProjectID:      projID,
			EnvVarName:     l.EnvVarName,
			ConnectionKind: l.ConnectionKind,
			Property:       l.Property,
			CreatedAt:      l.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{"linkedServices": out})
}
