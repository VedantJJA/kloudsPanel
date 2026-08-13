package http

import (
	"github.com/gofiber/fiber/v3"
)

// ─── Middleware ────────────────────────────────────────────────────────────────

// requireSession validates the session cookie and sets the user in context.
func requireSession(c fiber.Ctx) error {
	// TODO: implement full session validation in Phase 2
	// For now, check for Authorization header or session cookie
	cookie := c.Cookies("klouds_session")
	if cookie == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "session required")
	}
	// Session lookup and user injection will be wired in Phase 2
	return c.Next()
}

// requireMainAdmin ensures the caller is the main_admin.
func requireMainAdmin(c fiber.Ctx) error {
	// TODO: check user role from context
	return c.Next()
}

// ─── Auth Handlers ─────────────────────────────────────────────────────────────

type signupRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func handleSignup(c fiber.Ctx) error {
	var req signupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	// TODO: wire to user service in Phase 2
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "Account created. Awaiting admin approval.",
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	// TODO: wire to auth service
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "login ok"})
}

func handleLogout(c fiber.Ctx) error {
	// TODO: revoke session
	c.Cookie(&fiber.Cookie{Name: "klouds_session", Value: "", MaxAge: -1})
	return c.SendStatus(fiber.StatusNoContent)
}

func handleMe(c fiber.Ctx) error {
	// TODO: return current user from context
	return c.JSON(fiber.Map{"id": "placeholder"})
}

// ─── Workspace Handlers ────────────────────────────────────────────────────────

func handleListWorkspaces(c fiber.Ctx) error  { return c.JSON(fiber.Map{"workspaces": []any{}}) }
func handleCreateWorkspace(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleGetWorkspace(c fiber.Ctx) error    { return c.JSON(fiber.Map{"slug": c.Params("slug")}) }
func handleUpdateWorkspace(c fiber.Ctx) error { return c.SendStatus(200) }
func handleListMembers(c fiber.Ctx) error     { return c.JSON(fiber.Map{"members": []any{}}) }
func handleInviteMember(c fiber.Ctx) error    { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }

// ─── Project Handlers ──────────────────────────────────────────────────────────

func handleListProjects(c fiber.Ctx) error  { return c.JSON(fiber.Map{"projects": []any{}}) }
func handleCreateProject(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleGetProject(c fiber.Ctx) error    { return c.JSON(fiber.Map{"id": c.Params("id")}) }
func handleUpdateProject(c fiber.Ctx) error { return c.SendStatus(200) }
func handleDeleteProject(c fiber.Ctx) error { return c.SendStatus(202) }

// ─── Service Handlers ──────────────────────────────────────────────────────────

func handleListServices(c fiber.Ctx) error   { return c.JSON(fiber.Map{"services": []any{}}) }
func handleCreateService(c fiber.Ctx) error  { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleGetService(c fiber.Ctx) error     { return c.JSON(fiber.Map{"id": c.Params("id")}) }
func handleUpdateService(c fiber.Ctx) error  { return c.SendStatus(200) }
func handleDeleteService(c fiber.Ctx) error  { return c.SendStatus(202) }

// ─── Deployment Handlers ───────────────────────────────────────────────────────

func handleListDeployments(c fiber.Ctx) error   { return c.JSON(fiber.Map{"deployments": []any{}}) }
func handleTriggerDeployment(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleGetDeployment(c fiber.Ctx) error     { return c.JSON(fiber.Map{"id": c.Params("deployId")}) }
func handleGetLogs(c fiber.Ctx) error           { return c.JSON(fiber.Map{"entries": []any{}}) }
func handleWSLogs(c fiber.Ctx) error            { return c.SendStatus(501) }

// ─── Terminal Handlers ─────────────────────────────────────────────────────────

func handleCreateTerminalSession(c fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{"grant": "todo"})
}

// ─── Database Handlers ─────────────────────────────────────────────────────────

func handleListDatabases(c fiber.Ctx) error  { return c.JSON(fiber.Map{"databases": []any{}}) }
func handleCreateDatabase(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleGetDatabase(c fiber.Ctx) error    { return c.JSON(fiber.Map{"id": c.Params("id")}) }
func handleDeleteDatabase(c fiber.Ctx) error { return c.SendStatus(202) }

// ─── Domain Handlers ───────────────────────────────────────────────────────────

func handleListDomains(c fiber.Ctx) error  { return c.JSON(fiber.Map{"domains": []any{}}) }
func handleCreateDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func handleVerifyDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"status": "pending"}) }

// ─── Admin Handlers ────────────────────────────────────────────────────────────

func handleGetTelemetry(c fiber.Ctx) error        { return c.JSON(fiber.Map{"host": fiber.Map{}}) }
func handleAdminListUsers(c fiber.Ctx) error      { return c.JSON(fiber.Map{"users": []any{}}) }
func handleApproveUser(c fiber.Ctx) error         { return c.SendStatus(200) }
func handleSuspendUser(c fiber.Ctx) error         { return c.SendStatus(200) }
func handleListAuditEvents(c fiber.Ctx) error     { return c.JSON(fiber.Map{"events": []any{}}) }
func handleGetPlatformSettings(c fiber.Ctx) error { return c.JSON(fiber.Map{"settings": fiber.Map{}}) }
func handleAdminSetup(c fiber.Ctx) error          { return c.Status(202).JSON(fiber.Map{"status": "ok"}) }
