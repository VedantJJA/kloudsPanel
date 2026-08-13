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
	"github.com/yourorg/klouds/api/internal/repository"
)

// NewServer creates and configures the Fiber application with all middleware
// and routes registered.
func NewServer(log *slog.Logger, store repository.Store, addr string) *fiber.App {
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
		AllowOriginsFunc: func(origin string) bool {
			return true // Traefik handles edge security; API is internal-only
		},
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

	h := NewHandler(store, log)

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
	auth.Post("/signup", authLimiter, h.handleSignup)
	auth.Post("/login", authLimiter, h.handleLogin)
	auth.Post("/logout", h.handleLogout)
	auth.Get("/me", h.requireSession, h.handleMe)

	// Workspace routes
	ws := v1.Group("/workspaces", h.requireSession)
	ws.Get("/", h.handleListWorkspaces)
	ws.Post("/", h.handleCreateWorkspace)
	ws.Get("/:slug", h.handleGetWorkspace)
	ws.Patch("/:slug", h.handleUpdateWorkspace)
	ws.Delete("/:id", h.handleDeleteWorkspace)
	ws.Get("/:slug/members", h.handleListMembers)
	ws.Post("/:slug/members", h.handleInviteMember)

	// Project routes
	proj := v1.Group("/projects", h.requireSession)
	proj.Get("/", h.handleListProjects)
	proj.Post("/", h.handleCreateProject)
	proj.Get("/:id", h.handleGetProject)
	proj.Patch("/:id", h.handleUpdateProject)
	proj.Delete("/:id", h.handleDeleteProject)

	// Service routes
	svc := v1.Group("/services", h.requireSession)
	svc.Get("/", h.handleListServices)
	svc.Post("/", h.handleCreateService)
	svc.Get("/:id", h.handleGetService)
	svc.Patch("/:id", h.handleUpdateService)
	svc.Delete("/:id", h.handleDeleteService)
	svc.Post("/:id/deploy", h.handleTriggerDeployment)
	svc.Post("/:id/stop", h.handleStopService)
	svc.Post("/:id/start", h.handleStartService)
	svc.Get("/:id/logs", h.handleGetLogs)

	// Deployment routes
	dep := v1.Group("/services/:id/deployments", h.requireSession)
	dep.Get("/", h.handleListDeployments)
	dep.Post("/", h.handleTriggerDeployment)
	dep.Get("/:deployId", h.handleGetDeployment)

	// Log streaming
	v1.Get("/deployments/:id/logs", h.requireSession, h.handleGetLogs)
	v1.Get("/services/:id/logs", h.requireSession, h.handleGetLogs)
	// WebSocket log stream handled by Fiber's built-in WS support
	v1.Get("/ws/deployments/:id/logs", h.requireSession, h.handleWSLogs)

	// Terminal sessions
	v1.Post("/services/:id/terminal-sessions", h.requireSession, h.handleCreateTerminalSession)

	// Database routes
	db := v1.Group("/databases", h.requireSession)
	db.Get("/", h.handleListDatabases)
	db.Post("/", h.handleCreateDatabase)
	db.Get("/:id", h.handleGetDatabase)
	db.Delete("/:id", h.handleDeleteDatabase)

	// Domain routes
	dom := v1.Group("/domains", h.requireSession)
	dom.Get("/", h.handleListDomains)
	dom.Post("/", h.handleCreateDomain)
	dom.Post("/:id/verify", h.handleVerifyDomain)

	// Admin routes (main_admin only)
	admin := v1.Group("/admin", h.requireSession, h.requireMainAdmin)
	admin.Get("/telemetry", h.handleGetTelemetry)
	admin.Get("/users", h.handleAdminListUsers)
	admin.Post("/users/:id/approve", h.handleApproveUser)
	admin.Post("/users/:id/suspend", h.handleSuspendUser)
	admin.Get("/audit", h.handleListAuditEvents)
	admin.Get("/platform", h.handleGetPlatformSettings)
	admin.Post("/setup", h.handleAdminSetup)

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
