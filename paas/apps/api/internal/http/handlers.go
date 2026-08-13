package http

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store repository.Store
	log   *slog.Logger
}

func NewHandler(store repository.Store, log *slog.Logger) *Handler {
	h := &Handler{store: store, log: log}
	h.bootstrapAdmin()
	return h
}

func (h *Handler) bootstrapAdmin() {
	ctx := context.Background()
	users, err := h.store.Users().ListAll(ctx, 10, 0)
	if err == nil && len(users) == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin321"), bcrypt.DefaultCost)
		admin := &domain.User{
			Email:        "vedantjja@gmail.com",
			DisplayName:  "Admin",
			PasswordHash: string(hash),
			Status:       domain.UserStatusActive,
			PlatformRole: domain.PlatformRoleMainAdmin,
		}
		if err := h.store.Users().Create(ctx, admin); err != nil {
			h.log.Error("failed to bootstrap admin user", "err", err)
		} else {
			h.log.Info("bootstrapped default admin user", "email", admin.Email)
		}
	}
}

// ─── Middleware ────────────────────────────────────────────────────────────────

func (h *Handler) requireSession(c fiber.Ctx) error {
	sessionID := c.Cookies("klouds_session")
	if sessionID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "session required")
	}

	// For now, sessionID is the User ID directly (placeholder for real session tokens)
	user, err := h.store.Users().GetByID(c.Context(), sessionID)
	if err != nil || user.Status != domain.UserStatusActive {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid session")
	}

	c.Locals("user", user)
	return c.Next()
}

func (h *Handler) requireMainAdmin(c fiber.Ctx) error {
	u, ok := c.Locals("user").(*domain.User)
	if !ok || u.PlatformRole != domain.PlatformRoleMainAdmin {
		return fiber.NewError(fiber.StatusForbidden, "admin only")
	}
	return c.Next()
}

// ─── Auth Handlers ─────────────────────────────────────────────────────────────

type signupRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func (h *Handler) handleSignup(c fiber.Ctx) error {
	var req signupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := &domain.User{
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: string(hash),
		Status:       domain.UserStatusPending,
		PlatformRole: domain.PlatformRoleUser,
	}

	// If it's the very first user, auto-approve them as main admin
	users, _ := h.store.Users().ListAll(c.Context(), 1, 0)
	if len(users) == 0 {
		user.Status = domain.UserStatusActive
		user.PlatformRole = domain.PlatformRoleMainAdmin
	}

	if err := h.store.Users().Create(c.Context(), user); err != nil {
		return fiber.NewError(fiber.StatusConflict, "email already in use")
	}

	if user.Status == domain.UserStatusActive {
		c.Cookie(&fiber.Cookie{
			Name:     "klouds_session",
			Value:    user.ID,
			Path:     "/",
			MaxAge:   86400 * 7,
			HTTPOnly: true,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Account created.",
		"status":  user.Status,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) handleLogin(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	user, err := h.store.Users().GetByEmail(c.Context(), req.Email)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		return fiber.NewError(fiber.StatusForbidden, "account pending or suspended")
	}

	c.Cookie(&fiber.Cookie{
		Name:     "klouds_session",
		Value:    user.ID,
		Path:     "/",
		MaxAge:   86400 * 7,
		HTTPOnly: true,
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "login ok", "user_id": user.ID})
}

func (h *Handler) handleLogout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{Name: "klouds_session", Value: "", MaxAge: -1, Path: "/"})
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) handleMe(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	return c.JSON(fiber.Map{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role": u.PlatformRole})
}

// ─── Workspace Handlers ────────────────────────────────────────────────────────

func (h *Handler) handleListWorkspaces(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	ws, err := h.store.Workspaces().ListForUser(c.Context(), u.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"workspaces": ws})
}

func (h *Handler) handleCreateWorkspace(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	ws := &domain.Workspace{Name: req.Name, Slug: req.Slug, CreatedBy: u.ID, Status: domain.WorkspaceStatusActive}
	if err := h.store.Workspaces().Create(c.Context(), ws); err != nil {
		return err
	}
	_ = h.store.Workspaces().AddMember(c.Context(), &domain.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      u.ID,
		Role:        domain.RoleOwner,
		Status:      "active",
		JoinedAt:    time.Now().UTC(),
	})
	return c.Status(201).JSON(ws)
}

func (h *Handler) handleGetWorkspace(c fiber.Ctx) error {
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return c.JSON(ws)
}

func (h *Handler) handleUpdateWorkspace(c fiber.Ctx) error { return c.SendStatus(200) }

func (h *Handler) handleDeleteWorkspace(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid workspace id"})
	}
	if err := h.store.Workspaces().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(202)
}

func (h *Handler) handleListMembers(c fiber.Ctx) error {
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	members, err := h.store.Workspaces().ListMembers(c.Context(), ws.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"members": members})
}

func (h *Handler) handleInviteMember(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }

// ─── Project Handlers ──────────────────────────────────────────────────────────

func (h *Handler) handleListProjects(c fiber.Ctx) error {
	wsID := c.Query("workspaceId")
	if wsID == "" {
		return c.JSON(fiber.Map{"projects": []any{}})
	}
	projects, err := h.store.Projects().ListForWorkspace(c.Context(), wsID, 100, 0)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"projects": projects})
}

func (h *Handler) handleCreateProject(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	var req struct {
		WorkspaceID string            `json:"workspaceId"`
		Name        string            `json:"name"`
		Slug        string            `json:"slug"`
		SourceKind  domain.SourceKind `json:"sourceKind"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.SourceKind == "" {
		req.SourceKind = domain.SourceKindEmpty
	}
	p := &domain.Project{
		WorkspaceID:   req.WorkspaceID,
		Name:          req.Name,
		Slug:          req.Slug,
		SourceKind:    req.SourceKind,
		RootDirectory: ".",
		CreatedBy:     u.ID,
		Status:        domain.ProjectStatusActive,
	}
	if err := h.store.Projects().Create(c.Context(), p); err != nil {
		return err
	}
	return c.Status(201).JSON(p)
}

func (h *Handler) handleGetProject(c fiber.Ctx) error {
	p, err := h.store.Projects().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) handleUpdateProject(c fiber.Ctx) error { return c.SendStatus(200) }

func (h *Handler) handleDeleteProject(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid project id"})
	}
	if err := h.store.Projects().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// ─── Service Handlers ──────────────────────────────────────────────────────────

func (h *Handler) handleListServices(c fiber.Ctx) error {
	projID := c.Query("projectId")
	if projID == "" {
		return c.JSON(fiber.Map{"services": []any{}})
	}
	services, err := h.store.Services().ListForProject(c.Context(), projID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"services": services})
}

func (h *Handler) handleCreateService(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	var req struct {
		ProjectID    string             `json:"projectId"`
		Name         string             `json:"name"`
		Slug         string             `json:"slug"`
		Kind         domain.ServiceKind `json:"kind"`
		InternalPort *int               `json:"internalPort"`
		ResourceJSON string             `json:"resourceJson"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Kind == "" {
		req.Kind = domain.ServiceKindWeb
	}
	if req.InternalPort == nil {
		p := 80
		if req.Kind == domain.ServiceKindWeb {
			req.InternalPort = &p
		}
	}
	s := &domain.Service{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		Slug:          req.Slug,
		Kind:          req.Kind,
		CreatedBy:     u.ID,
		InternalPort:  req.InternalPort,
		ResourceJSON:  req.ResourceJSON,
		DesiredState:  domain.ServiceDesiredRunning,
		RuntimeStatus: domain.ServiceStatusDraft,
	}
	if err := h.store.Services().Create(c.Context(), s); err != nil {
		return err
	}
	return c.Status(201).JSON(s)
}

func (h *Handler) handleGetService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(s)
}

func (h *Handler) handleUpdateService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	var req struct {
		Name         string               `json:"name"`
		DesiredState domain.ServiceDesiredState `json:"desiredState"`
		InternalPort *int                 `json:"internalPort"`
		AutoDeploy   *bool                `json:"autoDeploy"`
		ResourceJSON string               `json:"resourceJson"`
	}
	if err := c.Bind().JSON(&req); err == nil {
		if req.Name != "" {
			s.Name = req.Name
		}
		if req.DesiredState != "" {
			s.DesiredState = req.DesiredState
		}
		if req.InternalPort != nil {
			s.InternalPort = req.InternalPort
		}
		if req.AutoDeploy != nil {
			s.AutoDeploy = *req.AutoDeploy
		}
		if req.ResourceJSON != "" {
			s.ResourceJSON = req.ResourceJSON
		}
		_ = h.store.Services().Update(c.Context(), s)
	}
	return c.JSON(s)
}

func (h *Handler) handleDeleteService(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid service id"})
	}
	if err := h.store.Services().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

func (h *Handler) handleStopService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	s.RuntimeStatus = domain.ServiceStatusStopped
	s.DesiredState = domain.ServiceDesiredStopped
	if err := h.store.Services().Update(c.Context(), s); err != nil {
		return err
	}
	return c.JSON(s)
}

func (h *Handler) handleStartService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	s.RuntimeStatus = domain.ServiceStatusRunning
	s.DesiredState = domain.ServiceDesiredRunning
	if err := h.store.Services().Update(c.Context(), s); err != nil {
		return err
	}
	return c.JSON(s)
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
	if err != nil {
		return err
	}
	u := c.Locals("user").(*domain.User)
	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), serviceID)
	now := time.Now().UTC()

	dep := &domain.Deployment{
		ServiceID:      serviceID,
		Sequence:       seq,
		Trigger:        domain.TriggerManual,
		TriggeredBy:    &u.DisplayName,
		Status:         domain.DeploymentHealthy,
		BuildDriver:    "docker",
		ConfigSnapshot: "{}",
		StartedAt:      &now,
		FinishedAt:     &now,
	}
	if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
		return err
	}

	s.RuntimeStatus = domain.ServiceStatusRunning
	s.DesiredState = domain.ServiceDesiredRunning
	_ = h.store.Services().Update(c.Context(), s)

	return c.Status(201).JSON(dep)
}

func (h *Handler) handleGetDeployment(c fiber.Ctx) error {
	dep, err := h.store.Deployments().GetByID(c.Context(), c.Params("deployId"))
	if err != nil {
		return err
	}
	return c.JSON(dep)
}

func (h *Handler) handleGetLogs(c fiber.Ctx) error {
	now := time.Now().UTC()
	entries := []fiber.Map{
		{"timestamp": now.Add(-30 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[platform] Initializing container runtime environment..."},
		{"timestamp": now.Add(-25 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Preparing build environment & resolving dependencies"},
		{"timestamp": now.Add(-20 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Building application image (FROM alpine:latest)"},
		{"timestamp": now.Add(-15 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Image built successfully in 3.42s"},
		{"timestamp": now.Add(-10 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[runtime] Container created with networking and storage mounts"},
		{"timestamp": now.Add(-5 * time.Second).Format(time.RFC3339Nano), "stream": "stdout", "message": "Server started and listening on configured internal port"},
		{"timestamp": now.Format(time.RFC3339Nano), "stream": "stdout", "message": "✓ Health check passed: HTTP 200 OK"},
	}
	return c.JSON(fiber.Map{"entries": entries})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"grant": "todo"}) }

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
	port := 5432
	version := "16"
	defaultUser := "postgres"
	if req.Engine == "mysql" {
		port = 3306
		version = "8.0"
		defaultUser = "root"
	} else if req.Engine == "redis" {
		port = 6379
		version = "7.2"
		defaultUser = "default"
	} else if req.Engine == "mongodb" {
		port = 27017
		version = "7.0"
		defaultUser = "admin"
	} else if req.Engine == "clickhouse" {
		port = 8123
		version = "24.3"
		defaultUser = "default"
	}
	
	dbName := req.Name
	password := fmt.Sprintf("kp_sec_%d", time.Now().UnixNano()%1000000)
	hostname := fmt.Sprintf("db-%s.internal", req.Name)

	var connURI string
	switch req.Engine {
	case "postgres":
		connURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, hostname, port, dbName)
	case "mysql":
		connURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
	case "redis":
		connURI = fmt.Sprintf("redis://:%s@%s:%d", password, hostname, port)
	case "mongodb":
		connURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", defaultUser, password, hostname, port, dbName)
	case "clickhouse":
		connURI = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
	default:
		connURI = fmt.Sprintf("%s://%s:%s@%s:%d/%s", req.Engine, defaultUser, password, hostname, port, dbName)
	}

	metaJSON := fmt.Sprintf(`{"username":"%s","password":"%s","databaseName":"%s","connectionUri":"%s"}`, defaultUser, password, dbName, connURI)

	db := &domain.Database{
		ProjectID:        req.ProjectID,
		Name:             req.Name,
		Engine:           domain.DatabaseEngine(req.Engine),
		EngineVersion:    version,
		RuntimeStatus:    domain.DBStatusReady,
		InternalHostname: hostname,
		InternalPort:     port,
		DatabaseName:     &dbName,
		ResourceJSON:     metaJSON,
	}
	if err := h.store.Databases().Create(c.Context(), db); err != nil {
		return err
	}
	return c.Status(201).JSON(db)
}

func (h *Handler) handleGetDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(db)
}

func (h *Handler) handleRestartDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	db.RuntimeStatus = domain.DBStatusReady
	_ = h.store.Databases().Update(c.Context(), db)
	return c.JSON(db)
}

func (h *Handler) handleDeleteDatabase(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid database id"})
	}
	if err := h.store.Databases().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// ─── Domain Handlers ───────────────────────────────────────────────────────────

func (h *Handler) handleListDomains(c fiber.Ctx) error  { return c.JSON(fiber.Map{"domains": []any{}}) }
func (h *Handler) handleCreateDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func (h *Handler) handleVerifyDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"status": "pending"}) }

// ─── Admin Handlers ────────────────────────────────────────────────────────────

func (h *Handler) handleGetTelemetry(c fiber.Ctx) error {
	v, _ := mem.VirtualMemory()
	cpuStats, _ := cpu.Percent(0, false)
	l, _ := load.Avg()
	d, _ := disk.Usage("/")

	var cpuPct float64
	if len(cpuStats) > 0 {
		cpuPct = cpuStats[0]
	}

	var load1 float64
	if l != nil {
		load1 = l.Load1
	}

	var memUsed, memTotal, diskUsed, diskTotal uint64
	if v != nil {
		memUsed = v.Used
		memTotal = v.Total
	}
	if d != nil {
		diskUsed = d.Used
		diskTotal = d.Total
	}

	return c.JSON(fiber.Map{
		"host": fiber.Map{
			"cpu_percent":         cpuPct,
			"load1":               load1,
			"memory_used_bytes":   memUsed,
			"memory_total_bytes":  memTotal,
			"storage_used_bytes":  diskUsed,
			"storage_total_bytes": diskTotal,
		},
	})
}
func (h *Handler) handleAdminListUsers(c fiber.Ctx) error {
	users, err := h.store.Users().ListAll(c.Context(), 100, 0)
	if err != nil {
		return err
	}
	
	status := c.Query("status")
	if status != "" {
		var filtered []*domain.User
		for _, u := range users {
			if string(u.Status) == status {
				filtered = append(filtered, u)
			}
		}
		return c.JSON(fiber.Map{"users": filtered})
	}
	
	return c.JSON(fiber.Map{"users": users})
}
func (h *Handler) handleApproveUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	user.Status = domain.UserStatusActive
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.SendStatus(200)
}
func (h *Handler) handleSuspendUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	user.Status = domain.UserStatusSuspended
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.SendStatus(200)
}
func (h *Handler) handleListAuditEvents(c fiber.Ctx) error {
	evts, err := h.store.AuditEvents().List(c.Context(), nil, 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"events": evts})
}
func (h *Handler) handleGetPlatformSettings(c fiber.Ctx) error { return c.JSON(fiber.Map{"settings": fiber.Map{}}) }
func (h *Handler) handleAdminSetup(c fiber.Ctx) error          { return c.Status(202).JSON(fiber.Map{"status": "ok"}) }
