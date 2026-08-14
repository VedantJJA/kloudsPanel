package http

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Database Handlers ─────────────────────────────────────────────────────────

func (h *Handler) handleListDatabases(c fiber.Ctx) error {
	projID := c.Query("projectId")
	if projID != "" {
		dbs, err := h.store.Databases().ListForProject(c.Context(), projID)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"databases": dbs})
	}
	dbs, err := h.store.Databases().ListAll(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"databases": dbs})
}

func (h *Handler) handleCreateDatabase(c fiber.Ctx) error {
	var req struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
		Engine    string `json:"engine"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "database name is required"})
	}

	db, err := h.provisionDatabaseInternal(c.Context(), req.ProjectID, req.Name, req.Engine)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(db)
}

func (h *Handler) allocateExternalPort(ctx context.Context, engine string) int {
	basePort := 15432
	switch engine {
	case "mysql":
		basePort = 13306
	case "redis":
		basePort = 16379
	case "mongodb":
		basePort = 17017
	case "clickhouse":
		basePort = 18123
	}

	dbs, _ := h.store.Databases().ListAll(ctx)
	usedPorts := make(map[int]bool)
	for _, d := range dbs {
		var meta map[string]any
		if d.ResourceJSON != "" {
			_ = json.Unmarshal([]byte(d.ResourceJSON), &meta)
			if p, ok := meta["externalPort"].(float64); ok && p > 0 {
				usedPorts[int(p)] = true
			}
		}
	}

	for p := basePort; p < basePort+1000; p++ {
		if !usedPorts[p] {
			return p
		}
	}
	return basePort
}

func (h *Handler) provisionDatabaseInternal(ctx context.Context, projectID, name, engine string) (*domain.Database, error) {
	if name == "" {
		return nil, fmt.Errorf("database name is required")
	}
	if engine == "" {
		engine = "postgres"
	}

	dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", "-"))
	port := 5432
	version := "16"
	defaultUser := "postgres"

	if engine == "mysql" {
		port = 3306
		version = "8.0"
		defaultUser = "root"
	} else if engine == "redis" {
		port = 6379
		version = "7.2"
		defaultUser = "default"
	} else if engine == "mongodb" {
		port = 27017
		version = "7.0"
		defaultUser = "admin"
	} else if engine == "clickhouse" {
		port = 8123
		version = "24.3"
		defaultUser = "default"
	}

	externalPort := h.allocateExternalPort(ctx, engine)
	externalHost := getRootDomain()
	if externalHost == "" {
		externalHost = "klouds.online"
	}

	dbName := dbSlug
	password := fmt.Sprintf("kp_sec_%d", time.Now().UnixNano()%1000000)
	hostname := fmt.Sprintf("paas-db-%s", dbSlug)

	var internalConnURI, externalConnURI string
	switch engine {
	case "postgres":
		internalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, externalHost, externalPort, dbName)
	case "mysql":
		internalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, externalHost, externalPort, dbName)
	case "redis":
		internalConnURI = fmt.Sprintf("redis://:%s@%s:%d", password, hostname, port)
		externalConnURI = fmt.Sprintf("redis://:%s@%s:%d", password, externalHost, externalPort)
	case "mongodb":
		internalConnURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", defaultUser, password, externalHost, externalPort, dbName)
	case "clickhouse":
		internalConnURI = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", defaultUser, password, externalHost, externalPort, dbName)
	default:
		internalConnURI = fmt.Sprintf("%s://%s:%s@%s:%d/%s", engine, defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("%s://%s:%s@%s:%d/%s", engine, defaultUser, password, externalHost, externalPort, dbName)
	}

	metaMap := map[string]any{
		"username":              defaultUser,
		"password":              password,
		"databaseName":          dbName,
		"connectionUri":         internalConnURI,
		"internalConnectionUri": internalConnURI,
		"externalConnectionUri": externalConnURI,
		"externalHost":          externalHost,
		"externalPort":          externalPort,
		"psqlCommand":           fmt.Sprintf("psql \"%s\"", externalConnURI),
	}
	metaBytes, _ := json.Marshal(metaMap)

	db := &domain.Database{
		ProjectID:        projectID,
		Name:             name,
		Engine:           domain.DatabaseEngine(engine),
		EngineVersion:    version,
		RuntimeStatus:    domain.DBStatusReady,
		InternalHostname: hostname,
		InternalPort:     port,
		DatabaseName:     &dbName,
		ResourceJSON:     string(metaBytes),
	}
	if err := h.store.Databases().Create(ctx, db); err != nil {
		return nil, err
	}

	// Launch Real Docker Database Container asynchronously with published external port
	go h.startDatabaseContainer(dbSlug, hostname, defaultUser, password, dbName, engine, port, externalPort)

	return db, nil
}

func (h *Handler) startDatabaseContainer(dbSlug, containerName, defaultUser, password, dbName, engine string, internalPort, externalPort int) {
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	var runArgs []string
	switch engine {
	case "postgres":
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("POSTGRES_USER=%s", defaultUser),
			"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			"-e", fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/postgresql/data", dbSlug),
			"postgres:16-alpine",
		}
	case "mysql":
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", password),
			"-e", fmt.Sprintf("MYSQL_DATABASE=%s", dbName),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/mysql", dbSlug),
			"mysql:8.0",
		}
	case "redis":
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-v", fmt.Sprintf("paas-db-data-%s:/data", dbSlug),
			"redis:7.2-alpine",
			"redis-server", "--requirepass", password,
		}
	case "mongodb":
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-e", fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", defaultUser),
			"-e", fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", password),
			"-v", fmt.Sprintf("paas-db-data-%s:/data/db", dbSlug),
			"mongo:7.0",
		}
	case "clickhouse":
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/clickhouse", dbSlug),
			"clickhouse/clickhouse-server:24.3-alpine",
		}
	default:
		runArgs = []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"-p", fmt.Sprintf("%d:%d", externalPort, internalPort),
			"postgres:16-alpine",
		}
	}

	_ = exec.Command("docker", runArgs...).Run()
}

func (h *Handler) handleGetDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
	}

	// Self-heal: ensure externalPort exists in meta and container is exposed
	var meta map[string]any
	if db.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
	} else {
		meta = make(map[string]any)
	}

	extPort, hasPort := meta["externalPort"].(float64)
	if !hasPort || extPort == 0 {
		externalPort := h.allocateExternalPort(c.Context(), string(db.Engine))
		externalHost := getRootDomain()
		if externalHost == "" {
			externalHost = "klouds.online"
		}

		user, _ := meta["username"].(string)
		if user == "" {
			user = "postgres"
		}
		pass, _ := meta["password"].(string)
		dbName := db.Name
		if db.DatabaseName != nil && *db.DatabaseName != "" {
			dbName = *db.DatabaseName
		}

		var externalConnURI string
		switch db.Engine {
		case "postgres":
			externalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", user, pass, externalHost, externalPort, dbName)
		case "mysql":
			externalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", user, pass, externalHost, externalPort, dbName)
		case "redis":
			externalConnURI = fmt.Sprintf("redis://:%s@%s:%d", pass, externalHost, externalPort)
		default:
			externalConnURI = fmt.Sprintf("%s://%s:%s@%s:%d/%s", db.Engine, user, pass, externalHost, externalPort, dbName)
		}

		meta["externalPort"] = externalPort
		meta["externalHost"] = externalHost
		meta["externalConnectionUri"] = externalConnURI
		meta["psqlCommand"] = fmt.Sprintf("psql \"%s\"", externalConnURI)
		metaBytes, _ := json.Marshal(meta)
		db.ResourceJSON = string(metaBytes)
		_ = h.store.Databases().Update(c.Context(), db)

		dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(db.Name), "_", "-"))
		containerName := db.InternalHostname
		if containerName == "" {
			containerName = fmt.Sprintf("paas-db-%s", dbSlug)
		}
		go h.startDatabaseContainer(dbSlug, containerName, user, pass, dbName, string(db.Engine), db.InternalPort, externalPort)
	}

	return c.JSON(db)
}

func (h *Handler) handleExecuteDatabaseQuery(c fiber.Ctx) error {
	id := c.Params("id")
	db, err := h.store.Databases().GetByID(c.Context(), id)
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query cannot be empty"})
	}

	var meta struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		DatabaseName string `json:"databaseName"`
	}
	_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)

	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}

	startTime := time.Now()

	var cmd *exec.Cmd
	switch db.Engine {
	case "postgres":
		cmd = exec.Command("docker", "exec", containerName, "psql", "-U", meta.Username, "-d", meta.DatabaseName, "-c", query, "--csv")
	case "mysql":
		cmd = exec.Command("docker", "exec", containerName, "mysql", "-u", meta.Username, fmt.Sprintf("-p%s", meta.Password), meta.DatabaseName, "-e", query, "--batch", "--raw")
	case "redis":
		args := append([]string{"exec", containerName, "redis-cli", "-a", meta.Password}, strings.Fields(query)...)
		cmd = exec.Command("docker", args...)
	case "mongodb":
		cmd = exec.Command("docker", "exec", containerName, "mongosh", "-u", meta.Username, "-p", meta.Password, "--authenticationDatabase", "admin", meta.DatabaseName, "--eval", query)
	case "clickhouse":
		cmd = exec.Command("docker", "exec", containerName, "clickhouse-client", "--query", query, "--format", "CSVWithNames")
	default:
		cmd = exec.Command("docker", "exec", containerName, "psql", "-U", meta.Username, "-d", meta.DatabaseName, "-c", query, "--csv")
	}

	out, err := cmd.CombinedOutput()
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":      string(out),
			"durationMs": durationMs,
		})
	}

	rawOut := string(out)

	// Parse CSV for tabular output if applicable
	if db.Engine == "postgres" || db.Engine == "clickhouse" {
		reader := csv.NewReader(bytes.NewReader(out))
		records, parseErr := reader.ReadAll()
		if parseErr == nil && len(records) > 0 {
			columns := records[0]
			var rows [][]string
			if len(records) > 1 {
				rows = records[1:]
			}
			return c.JSON(fiber.Map{
				"columns":    columns,
				"rows":       rows,
				"rowCount":   len(rows),
				"durationMs": durationMs,
				"rawOutput":  rawOut,
			})
		}
	} else if db.Engine == "mysql" {
		lines := strings.Split(strings.TrimSpace(rawOut), "\n")
		if len(lines) > 0 {
			columns := strings.Split(lines[0], "\t")
			var rows [][]string
			for _, line := range lines[1:] {
				if strings.TrimSpace(line) != "" {
					rows = append(rows, strings.Split(line, "\t"))
				}
			}
			return c.JSON(fiber.Map{
				"columns":    columns,
				"rows":       rows,
				"rowCount":   len(rows),
				"durationMs": durationMs,
				"rawOutput":  rawOut,
			})
		}
	}

	return c.JSON(fiber.Map{
		"columns":    []string{"Result"},
		"rows":       [][]string{{rawOut}},
		"rowCount":   1,
		"durationMs": durationMs,
		"rawOutput":  rawOut,
	})
}

func (h *Handler) handleGetDatabaseLogs(c fiber.Ctx) error {
	id := c.Params("id")
	db, err := h.store.Databases().GetByID(c.Context(), id)
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
	}

	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}

	cmd := exec.Command("docker", "logs", "--tail", "100", containerName)
	out, err := cmd.CombinedOutput()
	var entries []LogEntry
	now := time.Now().UTC()

	if err == nil && len(out) > 0 {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) != "" {
				entries = append(entries, LogEntry{
					Timestamp: now.Format("15:04:05"),
					Stream:    "stdout",
					Message:   line,
				})
			}
		}
	}

	if len(entries) == 0 {
		entries = append(entries, LogEntry{
			Timestamp: now.Format("15:04:05"),
			Stream:    "system",
			Message:   fmt.Sprintf("Database container '%s' is running. Awaiting database query activity.", containerName),
		})
	}

	return c.JSON(fiber.Map{"entries": entries})
}

func (h *Handler) handleRestartDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
	}

	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}
	_ = exec.Command("docker", "restart", containerName).Run()

	db.RuntimeStatus = domain.DBStatusReady
	_ = h.store.Databases().Update(c.Context(), db)
	return c.JSON(db)
}

func (h *Handler) handleDeleteDatabase(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid database id"})
	}

	db, _ := h.store.Databases().GetByID(c.Context(), id)
	if db != nil {
		containerName := db.InternalHostname
		if containerName == "" {
			containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
		}
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	}

	if err := h.store.Databases().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
