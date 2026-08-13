// Package http provides the Fiber HTTP server, router, and middleware.
package http

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// NewServer creates and configures the Fiber application with all middleware
// and routes registered.
func NewServer(log *slog.Logger, addr string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "kloudsPanel API v1",
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          60 * time.Second,
		IdleTimeout:           120 * time.Second,

		// RFC 9457 problem+json errors
		ErrorHandler: problemErrorHandler,
	})

	// ── Global middleware ───────────────────────────────────────────────────
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${method} ${path} ${status} ${latency} req-id=${locals:requestid}\n",
	}))
	app.Use(helmet.New())
	app.Use(compress.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-CSRF-Token", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           600,
	}))

	// ── Health endpoints ────────────────────────────────────────────────────
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now().UTC()})
	})

	// ── API v1 ──────────────────────────────────────────────────────────────
	v1 := app.Group("/api/v1")

	// Rate limit auth endpoints aggressively
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
	})

	// Auth routes
	auth := v1.Group("/auth")
	auth.Post("/signup", authLimiter, handleSignup)
	auth.Post("/login", authLimiter, handleLogin)
	auth.Post("/logout", handleLogout)
	auth.Get("/me", requireSession, handleMe)

	// Workspace routes
	ws := v1.Group("/workspaces", requireSession)
	ws.Get("/", handleListWorkspaces)
	ws.Post("/", handleCreateWorkspace)
	ws.Get("/:slug", handleGetWorkspace)
	ws.Patch("/:slug", handleUpdateWorkspace)
	ws.Get("/:slug/members", handleListMembers)
	ws.Post("/:slug/members", handleInviteMember)

	// Project routes
	proj := v1.Group("/projects", requireSession)
	proj.Get("/", handleListProjects)
	proj.Post("/", handleCreateProject)
	proj.Get("/:id", handleGetProject)
	proj.Patch("/:id", handleUpdateProject)
	proj.Delete("/:id", handleDeleteProject)

	// Service routes
	svc := v1.Group("/services", requireSession)
	svc.Get("/", handleListServices)
	svc.Post("/", handleCreateService)
	svc.Get("/:id", handleGetService)
	svc.Patch("/:id", handleUpdateService)
	svc.Delete("/:id", handleDeleteService)

	// Deployment routes
	dep := v1.Group("/services/:id/deployments", requireSession)
	dep.Get("/", handleListDeployments)
	dep.Post("/", handleTriggerDeployment)
	dep.Get("/:deployId", handleGetDeployment)

	// Log streaming
	v1.Get("/deployments/:id/logs", requireSession, handleGetLogs)
	// WebSocket log stream handled by Fiber's built-in WS support
	v1.Get("/ws/deployments/:id/logs", requireSession, handleWSLogs)

	// Terminal sessions
	v1.Post("/services/:id/terminal-sessions", requireSession, handleCreateTerminalSession)

	// Database routes
	db := v1.Group("/databases", requireSession)
	db.Get("/", handleListDatabases)
	db.Post("/", handleCreateDatabase)
	db.Get("/:id", handleGetDatabase)
	db.Delete("/:id", handleDeleteDatabase)

	// Domain routes
	dom := v1.Group("/domains", requireSession)
	dom.Get("/", handleListDomains)
	dom.Post("/", handleCreateDomain)
	dom.Post("/:id/verify", handleVerifyDomain)

	// Admin routes (main_admin only)
	admin := v1.Group("/admin", requireSession, requireMainAdmin)
	admin.Get("/telemetry", handleGetTelemetry)
	admin.Get("/users", handleAdminListUsers)
	admin.Post("/users/:id/approve", handleApproveUser)
	admin.Post("/users/:id/suspend", handleSuspendUser)
	admin.Get("/audit", handleListAuditEvents)
	admin.Get("/platform", handleGetPlatformSettings)
	admin.Post("/setup", handleAdminSetup)

	return app
}

// problemErrorHandler converts errors to RFC 9457 problem+json responses.
func problemErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	title := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		title = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"type":      "about:blank",
		"title":     title,
		"status":    code,
		"detail":    err.Error(),
		"requestId": c.Locals("requestid"),
	})
}
