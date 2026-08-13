package http

import (
	"context"
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
	var req struct{ Name, Slug string }
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
		WorkspaceID string `json:"workspaceId"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	p := &domain.Project{WorkspaceID: req.WorkspaceID, Name: req.Name, Slug: req.Slug, CreatedBy: u.ID, Status: domain.ProjectStatusActive}
	if err := h.store.Projects().Create(c.Context(), p); err != nil {
		return err
	}
	return c.Status(201).JSON(p)
}

func (h *Handler) handleGetProject(c fiber.Ctx) error {
	// id here could be slug in the UI, we should support both. For now assuming ID.
	p, err := h.store.Projects().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(p)
}
func (h *Handler) handleUpdateProject(c fiber.Ctx) error { return c.SendStatus(200) }
func (h *Handler) handleDeleteProject(c fiber.Ctx) error { return c.SendStatus(202) }

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
		ProjectID string               `json:"projectId"`
		Name      string               `json:"name"`
		Slug      string               `json:"slug"`
		Kind      domain.ServiceKind   `json:"kind"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Kind == "" {
		req.Kind = domain.ServiceKindWeb
	}
	s := &domain.Service{
		ProjectID: req.ProjectID, Name: req.Name, Slug: req.Slug, Kind: req.Kind, CreatedBy: u.ID,
		DesiredState: domain.ServiceDesiredRunning, RuntimeStatus: domain.ServiceStatusDraft,
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
func (h *Handler) handleUpdateService(c fiber.Ctx) error { return c.SendStatus(200) }
func (h *Handler) handleDeleteService(c fiber.Ctx) error { return c.SendStatus(202) }

// ─── Deployment Handlers ───────────────────────────────────────────────────────

func (h *Handler) handleListDeployments(c fiber.Ctx) error {
	deps, err := h.store.Deployments().ListForService(c.Context(), c.Params("id"), 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deployments": deps})
}
func (h *Handler) handleTriggerDeployment(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func (h *Handler) handleGetDeployment(c fiber.Ctx) error {
	dep, err := h.store.Deployments().GetByID(c.Context(), c.Params("deployId"))
	if err != nil {
		return err
	}
	return c.JSON(dep)
}
func (h *Handler) handleGetLogs(c fiber.Ctx) error { return c.JSON(fiber.Map{"entries": []any{}}) }
func (h *Handler) handleWSLogs(c fiber.Ctx) error  { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"grant": "todo"}) }

// ─── Database Handlers ─────────────────────────────────────────────────────────

func (h *Handler) handleListDatabases(c fiber.Ctx) error {
	projID := c.Query("projectId")
	if projID == "" {
		return c.JSON(fiber.Map{"databases": []any{}})
	}
	dbs, err := h.store.Databases().ListForProject(c.Context(), projID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"databases": dbs})
}
func (h *Handler) handleCreateDatabase(c fiber.Ctx) error {
	var req struct{ ProjectID, Name, Engine string }
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	db := &domain.Database{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		Engine:        domain.DatabaseEngine(req.Engine),
		EngineVersion: "latest",
		RuntimeStatus: domain.DBStatusProvisioning,
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
func (h *Handler) handleDeleteDatabase(c fiber.Ctx) error { return c.SendStatus(202) }

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
