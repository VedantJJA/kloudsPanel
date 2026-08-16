package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// --- Database Handlers -----------------------------------------------------

func inspectDatabaseRuntimeStatus(containerName string) domain.DatabaseStatus {
	if containerName == "" {
		return domain.DBStatusFailed
	}
	inspectCmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", containerName)
	out, err := inspectCmd.CombinedOutput()
	if err != nil {
		return domain.DBStatusFailed
	}
	status := strings.ToLower(strings.TrimSpace(string(out)))
	switch status {
	case "running":
		return domain.DBStatusReady
	case "restarting":
		return "restarting"
	case "created":
		return domain.DBStatusProvisioning
	case "paused":
		return "paused"
	case "exited", "dead":
		return "stopped"
	default:
		if status != "" {
			return domain.DatabaseStatus(status)
		}
		return domain.DBStatusFailed
	}
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func (h *Handler) handleListDatabases(c fiber.Ctx) error {
	projID := c.Query("projectId")
	var dbs []*domain.Database
	var err error
	if projID != "" {
		dbs, err = h.store.Databases().ListForProject(c.Context(), projID)
	} else {
		dbs, err = h.store.Databases().ListAll(c.Context())
	}
	if err != nil {
		return err
	}
	for _, db := range dbs {
		db.RuntimeStatus = inspectDatabaseRuntimeStatus(db.InternalHostname)
	}
	return c.JSON(fiber.Map{"databases": dbs})
}

func generateSecureDatabasePassword() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return fmt.Sprintf("kp_sec_%d", time.Now().UnixNano())
}

func (h *Handler) handleCreateDatabase(c fiber.Ctx) error {
	var req struct {
		ProjectID    string `json:"projectId"`
		Name         string `json:"name"`
		Engine       string `json:"engine"`
		Version      string `json:"version"`
		Password     string `json:"password"`
		DatabaseName string `json:"databaseName"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "database name is required"})
	}

	db, err := h.provisionDatabaseInternal(c.Context(), req.ProjectID, req.Name, req.Engine, req.Version, req.Password, req.DatabaseName)
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
	case "mongodb", "mongo":
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
		if !usedPorts[p] && isPortAvailable(p) {
			return p
		}
	}
	return basePort
}

func (h *Handler) provisionDatabaseInternal(ctx context.Context, projectID, name, engine, requestedVersion, customPassword, customDbName string) (*domain.Database, error) {
	if name == "" {
		return nil, fmt.Errorf("database name is required")
	}
	if engine == "" {
		engine = "postgres"
	}

	// Disambiguate database name and slug to avoid UNIQUE constraint collisions on (project_id, name) and internal_hostname
	existingDbs, _ := h.store.Databases().ListAll(ctx)
	existingNamesInProj := make(map[string]bool)
	existingHostnames := make(map[string]bool)
	for _, d := range existingDbs {
		existingHostnames[strings.ToLower(d.InternalHostname)] = true
		if d.ProjectID == projectID {
			existingNamesInProj[strings.ToLower(d.Name)] = true
		}
	}

	baseName := strings.TrimSpace(name)
	baseSlug := strings.ToLower(strings.ReplaceAll(baseName, "_", "-"))

	finalName := baseName
	finalSlug := baseSlug
	hostname := fmt.Sprintf("paas-db-%s", finalSlug)

	counter := 1
	for existingNamesInProj[strings.ToLower(finalName)] || existingHostnames[strings.ToLower(hostname)] {
		counter++
		finalName = fmt.Sprintf("%s-%d", baseName, counter)
		finalSlug = fmt.Sprintf("%s-%d", baseSlug, counter)
		hostname = fmt.Sprintf("paas-db-%s", finalSlug)
	}

	// Security: Validate generated slug against command injection
	if err := ValidateSlug(finalSlug); err != nil {
		return nil, fmt.Errorf("invalid database name: %w", err)
	}

	dbSlug := finalSlug
	name = finalName
	port := 5432
	defaultUser := "postgres"

	if engine == "mysql" {
		port = 3306
		defaultUser = "root"
	} else if engine == "redis" {
		port = 6379
		defaultUser = "default"
	} else if engine == "mongodb" || engine == "mongo" {
		port = 27017
		defaultUser = "admin"
	} else if engine == "clickhouse" {
		port = 8123
		defaultUser = "default"
	}

	// Resolve image and version dynamically
	imageTag, resolvedVer := resolveDatabaseVersion(engine, requestedVersion)

	externalPort := h.allocateExternalPort(ctx, engine)
	externalHost := getRootDomain()
	if externalHost == "" {
		externalHost = "yourdomain.com"
	}

	dbName := dbSlug
	if customDbName != "" {
		dbName = customDbName
	}

	password := customPassword
	if password == "" {
		password = generateSecureDatabasePassword()
	}

	var internalConnURI, externalConnURI string
	switch engine {
	case "postgres", "postgresql":
		internalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, externalHost, externalPort, dbName)
	case "mysql":
		internalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
		externalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, externalHost, externalPort, dbName)
	case "redis":
		internalConnURI = fmt.Sprintf("redis://:%s@%s:%d", password, hostname, port)
		externalConnURI = fmt.Sprintf("redis://:%s@%s:%d", password, externalHost, externalPort)
	case "mongodb", "mongo":
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
		"version":               resolvedVer,
		"image":                 imageTag,
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
		EngineVersion:    resolvedVer,
		RuntimeStatus:    domain.DBStatusProvisioning,
		InternalHostname: hostname,
		InternalPort:     port,
		DatabaseName:     &dbName,
		ResourceJSON:     string(metaBytes),
	}
	if err := h.store.Databases().Create(ctx, db); err != nil {
		return nil, err
	}

	// Launch Real Docker Database Container asynchronously with published external port & security hardening
	go h.startDatabaseContainer(db.ID, dbSlug, hostname, defaultUser, password, dbName, engine, resolvedVer, port, externalPort)

	return db, nil
}

func (h *Handler) ensureDatabaseContainerRunning(ctx context.Context, db *domain.Database) error {
	containerName := db.InternalHostname
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))
	}

	inspectCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	out, err := inspectCmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "true" {
		return nil
	}

	var meta struct {
		Username     string  `json:"username"`
		Password     string  `json:"password"`
		DatabaseName string  `json:"databaseName"`
		Version      string  `json:"version"`
		ExternalPort float64 `json:"externalPort"`
	}
	if db.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(db.ResourceJSON), &meta)
	}

	defaultUser := meta.Username
	if defaultUser == "" {
		defaultUser = "postgres"
		if db.Engine == "mysql" {
			defaultUser = "root"
		} else if db.Engine == "mongodb" {
			defaultUser = "admin"
		} else if db.Engine == "redis" || db.Engine == "clickhouse" {
			defaultUser = "default"
		}
	}

	password := meta.Password
	if password == "" {
		password = fmt.Sprintf("kp_sec_%d", time.Now().UnixNano()%1000000)
	}

	dbName := meta.DatabaseName
	if dbName == "" {
		dbName = strings.ToLower(strings.ReplaceAll(db.Name, "_", "-"))
	}

	port := db.InternalPort
	if port <= 0 {
		switch db.Engine {
		case "mysql":
			port = 3306
		case "redis":
			port = 6379
		case "mongodb":
			port = 27017
		case "clickhouse":
			port = 8123
		default:
			port = 5432
		}
	}

	externalPort := int(meta.ExternalPort)
	if externalPort <= 0 {
		externalPort = h.allocateExternalPort(ctx, string(db.Engine))
	}

	version := db.EngineVersion
	if meta.Version != "" {
		version = meta.Version
	}

	dbSlug := strings.ToLower(strings.ReplaceAll(db.Name, "_", "-"))
	h.startDatabaseContainer(db.ID, dbSlug, containerName, defaultUser, password, dbName, string(db.Engine), version, port, externalPort)

	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
		if chkOut, chkErr := checkCmd.CombinedOutput(); chkErr == nil && strings.TrimSpace(string(chkOut)) == "true" {
			return nil
		}
	}
	return nil
}

func (h *Handler) startDatabaseContainer(dbID, dbSlug, containerName, defaultUser, password, dbName, engine, version string, internalPort, externalPort int) {
	_ = exec.Command("docker", "network", "create", "platform-control").Run()
	_ = exec.Command("docker", "rm", "-f", containerName).Run()

	imageTag, _ := resolveDatabaseVersion(engine, version)

	// Pre-pull container image if needed to prevent container run timing out
	_ = exec.Command("docker", "pull", imageTag).Run()

	allocatedPort := externalPort
	var runErr error

	for attempt := 0; attempt < 5; attempt++ {
		baseArgs := []string{
			"run", "-d",
			"--name", containerName,
			"--network", "platform-control",
			"--restart", "unless-stopped",
			"--pids-limit", "256",
			"--memory", "1g",
			"--memory-swap", "1g",
			"--label", "io.paas.managed=true",
			"--label", "io.paas.type=database",
			"--label", fmt.Sprintf("io.paas.engine=%s", engine),
			"-p", fmt.Sprintf("%d:%d", allocatedPort, internalPort),
		}

		var engineArgs []string
		switch engine {
		case "postgres", "postgresql":
			engineArgs = []string{
				"-e", fmt.Sprintf("POSTGRES_USER=%s", defaultUser),
				"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
				"-e", fmt.Sprintf("POSTGRES_DB=%s", dbName),
				"-e", "PGDATA=/var/lib/postgresql/data/pgdata",
				"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/postgresql/data", dbSlug),
				imageTag,
			}
		case "mysql":
			engineArgs = []string{
				"-e", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", password),
				"-e", fmt.Sprintf("MYSQL_DATABASE=%s", dbName),
				"-e", "MYSQL_ROOT_HOST=%",
				"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/mysql", dbSlug),
				imageTag,
			}
		case "redis":
			engineArgs = []string{
				"-v", fmt.Sprintf("paas-db-data-%s:/data", dbSlug),
				imageTag,
				"redis-server", "--requirepass", password,
			}
		case "mongodb", "mongo":
			engineArgs = []string{
				"-e", fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", defaultUser),
				"-e", fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", password),
				"-v", fmt.Sprintf("paas-db-data-%s:/data/db", dbSlug),
				imageTag,
			}
		case "clickhouse":
			engineArgs = []string{
				"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/clickhouse", dbSlug),
				imageTag,
			}
		default:
			engineArgs = []string{
				"-e", fmt.Sprintf("POSTGRES_USER=%s", defaultUser),
				"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
				"-e", fmt.Sprintf("POSTGRES_DB=%s", dbName),
				"-v", fmt.Sprintf("paas-db-data-%s:/var/lib/postgresql/data", dbSlug),
				imageTag,
			}
		}

		runArgs := append(baseArgs, engineArgs...)
		runOut, err := exec.Command("docker", runArgs...).CombinedOutput()
		if err == nil {
			runErr = nil
			break
		}
		runErr = err
		outStr := string(runOut)
		if strings.Contains(outStr, "port is already allocated") || strings.Contains(outStr, "address already in use") || strings.Contains(outStr, "Bind for") {
			allocatedPort = allocatedPort + 1
			_ = exec.Command("docker", "rm", "-f", containerName).Run()
			continue
		}
		break
	}

	// Update database record in SQLite store
	ctx := context.Background()
	if dbID != "" {
		if db, err := h.store.Databases().GetByID(ctx, dbID); err == nil && db != nil {
			if runErr != nil {
				db.RuntimeStatus = domain.DBStatusFailed
				_ = h.store.Databases().Update(ctx, db)
			} else {
				// Wait for container to be ready
				for i := 0; i < 15; i++ {
					time.Sleep(1 * time.Second)
					checkCmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
					if chkOut, chkErr := checkCmd.CombinedOutput(); chkErr == nil && strings.TrimSpace(string(chkOut)) == "true" {
						db.RuntimeStatus = domain.DBStatusReady
						break
					}
				}
				if db.RuntimeStatus != domain.DBStatusReady {
					db.RuntimeStatus = inspectDatabaseRuntimeStatus(containerName)
				}

				if allocatedPort != externalPort && db.ResourceJSON != "" {
					var meta map[string]any
					if json.Unmarshal([]byte(db.ResourceJSON), &meta) == nil {
						meta["externalPort"] = allocatedPort
						externalHost := getRootDomain()
						if externalHost == "" {
							externalHost = "yourdomain.com"
						}
						var externalConnURI string
						switch engine {
						case "postgres", "postgresql":
							externalConnURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, externalHost, allocatedPort, dbName)
						case "mysql":
							externalConnURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, externalHost, allocatedPort, dbName)
						case "redis":
							externalConnURI = fmt.Sprintf("redis://:%s@%s:%d", password, externalHost, allocatedPort)
						case "mongodb", "mongo":
							externalConnURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", defaultUser, password, externalHost, allocatedPort, dbName)
						case "clickhouse":
							externalConnURI = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", defaultUser, password, externalHost, allocatedPort, dbName)
						}
						meta["externalConnectionUri"] = externalConnURI
						meta["psqlCommand"] = fmt.Sprintf("psql \"%s\"", externalConnURI)
						metaBytes, _ := json.Marshal(meta)
						db.ResourceJSON = string(metaBytes)
					}
				}
				_ = h.store.Databases().Update(ctx, db)
			}
		}
	}
}

func (h *Handler) handleGetDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
	}

	db.RuntimeStatus = inspectDatabaseRuntimeStatus(db.InternalHostname)

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
			externalHost = "yourdomain.com"
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
		go h.startDatabaseContainer(db.ID, dbSlug, containerName, user, pass, dbName, string(db.Engine), db.EngineVersion, db.InternalPort, externalPort)
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

	// 1. Ensure container is running, or auto-heal/start it immediately
	_ = h.ensureDatabaseContainerRunning(c.Context(), db)

	startTime := time.Now()

	var out []byte
	var execErr error

	// Retry loop for databases that are starting up (e.g. MySQL initializing InnoDB buffer, Postgres recovery)
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
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

		out, execErr = cmd.CombinedOutput()
		if execErr == nil {
			break
		}

		outStr := string(out)
		// If MySQL is initializing, or Postgres starting up, or container restarting/not ready yet, wait and retry
		if strings.Contains(outStr, "is restarting") || strings.Contains(outStr, "is not running") || strings.Contains(outStr, "No such container") || strings.Contains(outStr, "Can't connect to local MySQL server") || strings.Contains(outStr, "the database system is starting up") || strings.Contains(outStr, "the database system is in recovery mode") {
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		// If MongoDB mongosh failed, attempt mongo fallback
		if db.Engine == "mongodb" && (strings.Contains(outStr, "mongosh: not found") || strings.Contains(outStr, "executable file not found")) {
			fallbackCmd := exec.Command("docker", "exec", containerName, "mongo", "-u", meta.Username, "-p", meta.Password, "--authenticationDatabase", "admin", meta.DatabaseName, "--eval", query)
			out, execErr = fallbackCmd.CombinedOutput()
			if execErr == nil {
				break
			}
		}
		break
	}

	durationMs := time.Since(startTime).Milliseconds()

	if execErr != nil {
		outStr := string(out)
		if strings.Contains(outStr, "is restarting") {
			logCmd := exec.Command("docker", "logs", "--tail", "10", containerName)
			logOut, _ := logCmd.CombinedOutput()
			reason := strings.TrimSpace(string(logOut))
			if reason != "" {
				return c.Status(400).JSON(fiber.Map{
					"error": fmt.Sprintf("Database container is restarting/initializing. Recent container logs:\n%s", reason),
					"durationMs": durationMs,
				})
			}
			return c.Status(400).JSON(fiber.Map{
				"error": "Database container is currently restarting / initializing. Please wait a few seconds for the engine to become ready.",
				"durationMs": durationMs,
			})
		}
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

func cleanupDatabaseResources(name, internalHostname string) {
	containerName := internalHostname
	dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", "-"))
	if containerName == "" {
		containerName = fmt.Sprintf("paas-db-%s", dbSlug)
	}
	// Stop and remove Docker container
	_ = exec.Command("docker", "rm", "-f", containerName).Run()
	// Remove persistent data volume
	_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-db-data-%s", dbSlug)).Run()
	if name != dbSlug {
		_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-db-data-%s", strings.ToLower(name))).Run()
	}
}

type SchemaColumn struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsPrimary    bool   `json:"is_primary"`
	IsForeign    bool   `json:"is_foreign"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"default_value,omitempty"`
}

type SchemaTable struct {
	Name        string         `json:"name"`
	Columns     []SchemaColumn `json:"columns"`
	ColumnCount int            `json:"column_count"`
}

type SchemaRelationship struct {
	FromTable      string `json:"from_table"`
	FromColumn     string `json:"from_column"`
	ToTable        string `json:"to_table"`
	ToColumn       string `json:"to_column"`
	ConstraintName string `json:"constraint_name,omitempty"`
}

type DatabaseSchemaResponse struct {
	Engine        string               `json:"engine"`
	DatabaseName  string               `json:"database_name"`
	Tables        []SchemaTable        `json:"tables"`
	Relationships []SchemaRelationship `json:"relationships"`
	TableCount    int                  `json:"table_count"`
}

func (h *Handler) handleGetDatabaseSchema(c fiber.Ctx) error {
	id := c.Params("id")
	db, err := h.store.Databases().GetByID(c.Context(), id)
	if err != nil || db == nil {
		return c.Status(404).JSON(fiber.Map{"error": "database not found"})
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

	_ = h.ensureDatabaseContainerRunning(c.Context(), db)

	tablesMap := make(map[string]*SchemaTable)
	var tablesOrder []string
	var relationships []SchemaRelationship

	if db.Engine == "postgres" || db.Engine == "" {
		// 1. Fetch columns and primary keys
		colsQuery := `SELECT c.table_name, c.column_name, c.data_type, c.is_nullable, COALESCE(c.column_default, '') as column_default, CASE WHEN tc.constraint_type = 'PRIMARY KEY' THEN 'true' ELSE 'false' END as is_primary FROM information_schema.columns c LEFT JOIN information_schema.key_column_usage kcu ON c.table_name = kcu.table_name AND c.column_name = kcu.column_name AND c.table_schema = kcu.table_schema LEFT JOIN information_schema.table_constraints tc ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema AND tc.constraint_type = 'PRIMARY KEY' WHERE c.table_schema = 'public' ORDER BY c.table_name, c.ordinal_position;`
		cmd := exec.Command("docker", "exec", containerName, "psql", "-U", meta.Username, "-d", meta.DatabaseName, "-c", colsQuery, "--csv")
		out, err := cmd.CombinedOutput()
		if err == nil {
			r := csv.NewReader(bytes.NewReader(out))
			rows, _ := r.ReadAll()
			if len(rows) > 1 {
				for _, row := range rows[1:] {
					if len(row) >= 6 {
						tableName := row[0]
						colName := row[1]
						dataType := row[2]
						isNullable := row[3] == "YES"
						colDef := row[4]
						isPrimary := row[5] == "true"

						tbl, exists := tablesMap[tableName]
						if !exists {
							tbl = &SchemaTable{Name: tableName, Columns: []SchemaColumn{}}
							tablesMap[tableName] = tbl
							tablesOrder = append(tablesOrder, tableName)
						}
						tbl.Columns = append(tbl.Columns, SchemaColumn{
							Name:         colName,
							Type:         dataType,
							IsPrimary:    isPrimary,
							Nullable:     isNullable,
							DefaultValue: colDef,
						})
					}
				}
			}
		}

		// 2. Fetch foreign key relationships
		fkQuery := `SELECT tc.table_name as from_table, kcu.column_name as from_column, ccu.table_name AS to_table, ccu.column_name AS to_column, tc.constraint_name FROM information_schema.table_constraints AS tc JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public';`
		fkCmd := exec.Command("docker", "exec", containerName, "psql", "-U", meta.Username, "-d", meta.DatabaseName, "-c", fkQuery, "--csv")
		fkOut, fkErr := fkCmd.CombinedOutput()
		if fkErr == nil {
			r := csv.NewReader(bytes.NewReader(fkOut))
			rows, _ := r.ReadAll()
			if len(rows) > 1 {
				for _, row := range rows[1:] {
					if len(row) >= 4 {
						fromTbl := row[0]
						fromCol := row[1]
						toTbl := row[2]
						toCol := row[3]
						cName := ""
						if len(row) >= 5 {
							cName = row[4]
						}
						relationships = append(relationships, SchemaRelationship{
							FromTable:      fromTbl,
							FromColumn:     fromCol,
							ToTable:        toTbl,
							ToColumn:       toCol,
							ConstraintName: cName,
						})
						// Mark column as foreign in tablesMap
						if tbl, ok := tablesMap[fromTbl]; ok {
							for i := range tbl.Columns {
								if tbl.Columns[i].Name == fromCol {
									tbl.Columns[i].IsForeign = true
								}
							}
						}
					}
				}
			}
		}
	} else if db.Engine == "mysql" {
		// MySQL schema extraction
		colsQuery := fmt.Sprintf("SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COALESCE(COLUMN_DEFAULT, ''), CASE WHEN COLUMN_KEY = 'PRI' THEN '1' ELSE '0' END FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = '%s' ORDER BY TABLE_NAME, ORDINAL_POSITION;", meta.DatabaseName)
		cmd := exec.Command("docker", "exec", containerName, "mysql", "-u", meta.Username, fmt.Sprintf("-p%s", meta.Password), meta.DatabaseName, "-e", colsQuery, "--batch", "--raw")
		out, err := cmd.CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 1 {
				for _, line := range lines[1:] {
					parts := strings.Split(line, "\t")
					if len(parts) >= 6 {
						tableName := parts[0]
						colName := parts[1]
						dataType := parts[2]
						isNullable := parts[3] == "YES"
						colDef := parts[4]
						isPrimary := parts[5] == "1"

						tbl, exists := tablesMap[tableName]
						if !exists {
							tbl = &SchemaTable{Name: tableName, Columns: []SchemaColumn{}}
							tablesMap[tableName] = tbl
							tablesOrder = append(tablesOrder, tableName)
						}
						tbl.Columns = append(tbl.Columns, SchemaColumn{
							Name:         colName,
							Type:         dataType,
							IsPrimary:    isPrimary,
							Nullable:     isNullable,
							DefaultValue: colDef,
						})
					}
				}
			}
		}

		fkQuery := fmt.Sprintf("SELECT TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME, CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = '%s' AND REFERENCED_TABLE_NAME IS NOT NULL;", meta.DatabaseName)
		fkCmd := exec.Command("docker", "exec", containerName, "mysql", "-u", meta.Username, fmt.Sprintf("-p%s", meta.Password), meta.DatabaseName, "-e", fkQuery, "--batch", "--raw")
		fkOut, fkErr := fkCmd.CombinedOutput()
		if fkErr == nil {
			lines := strings.Split(string(fkOut), "\n")
			if len(lines) > 1 {
				for _, line := range lines[1:] {
					parts := strings.Split(line, "\t")
					if len(parts) >= 4 {
						fromTbl := parts[0]
						fromCol := parts[1]
						toTbl := parts[2]
						toCol := parts[3]
						cName := ""
						if len(parts) >= 5 {
							cName = parts[4]
						}
						relationships = append(relationships, SchemaRelationship{
							FromTable:      fromTbl,
							FromColumn:     fromCol,
							ToTable:        toTbl,
							ToColumn:       toCol,
							ConstraintName: cName,
						})
						if tbl, ok := tablesMap[fromTbl]; ok {
							for i := range tbl.Columns {
								if tbl.Columns[i].Name == fromCol {
									tbl.Columns[i].IsForeign = true
								}
							}
						}
					}
				}
			}
		}
	} else if db.Engine == "mongodb" {
		// MongoDB collections
		cmd := exec.Command("docker", "exec", containerName, "mongosh", "-u", meta.Username, "-p", meta.Password, "--authenticationDatabase", "admin", meta.DatabaseName, "--quiet", "--eval", "db.getCollectionNames().join(',')")
		out, err := cmd.CombinedOutput()
		if err == nil {
			colls := strings.Split(strings.TrimSpace(string(out)), ",")
			for _, cName := range colls {
				cName = strings.TrimSpace(cName)
				if cName != "" {
					tablesMap[cName] = &SchemaTable{
						Name: cName,
						Columns: []SchemaColumn{
							{Name: "_id", Type: "ObjectId", IsPrimary: true},
							{Name: "document", Type: "BSON / Object", IsPrimary: false},
						},
					}
					tablesOrder = append(tablesOrder, cName)
				}
			}
		}
	}

	var tables []SchemaTable
	for _, name := range tablesOrder {
		if t, ok := tablesMap[name]; ok {
			t.ColumnCount = len(t.Columns)
			tables = append(tables, *t)
		}
	}

	return c.JSON(DatabaseSchemaResponse{
		Engine:        string(db.Engine),
		DatabaseName:  meta.DatabaseName,
		Tables:        tables,
		Relationships: relationships,
		TableCount:    len(tables),
	})
}

func (h *Handler) handleDeleteDatabase(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid database id"})
	}

	db, _ := h.store.Databases().GetByID(c.Context(), id)
	if db != nil {
		cleanupDatabaseResources(db.Name, db.InternalHostname)
	}

	if err := h.store.Databases().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
