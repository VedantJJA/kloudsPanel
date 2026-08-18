// Package http provides the Fiber HTTP server, router, and middleware.
package http

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/yourorg/klouds/api/internal/http/middleware"
	"github.com/yourorg/klouds/api/internal/repository"
)

// NewServer creates and configures the Fiber application with all middleware
// and routes registered.
func NewServer(log *slog.Logger, store repository.Store, addr string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "kloudsPanel API v1",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,

		// RFC 9457 problem+json errors
		ErrorHandler: middleware.ProblemErrorHandler,
	})

	// -- Global middleware ---------------------------------------------------
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${method} ${path} ${status} ${latency} req-id=${locals:requestid}\n",
	}))
	app.Use(helmet.New())
	app.Use(compress.New())
	app.Use(middleware.DefaultCORS())

	// -- Health endpoints ----------------------------------------------------
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "time": time.Now().UTC()})
	})

	// -- API v1 --------------------------------------------------------------
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
	ws.Get("/:slug/variables", h.handleListWorkspaceVariables)
	ws.Post("/:slug/variables", h.handleSaveWorkspaceVariables)
	ws.Get("/:slug/env-groups", h.handleListWorkspaceVariables)
	ws.Post("/:slug/env-groups", h.handleSaveWorkspaceEnvGroup)
	ws.Delete("/:slug/env-groups/:groupId", h.handleDeleteWorkspaceEnvGroup)

	// Project routes
	proj := v1.Group("/projects", h.requireSession)
	proj.Get("/", h.handleListProjects)
	proj.Post("/", h.handleCreateProject)
	proj.Get("/:id", h.handleGetProject)
	proj.Patch("/:id", h.handleUpdateProject)
	proj.Delete("/:id", h.handleDeleteProject)
	proj.Post("/:id/blueprint/deploy", h.handleDeployBlueprint)

	// Service routes
	svc := v1.Group("/services", h.requireSession)
	svc.Get("/", h.handleListServices)
	svc.Post("/", h.handleCreateService)
	svc.Post("/parse-render-yaml", h.handleParseRenderYaml)
	svc.Get("/:id", h.handleGetService)
	svc.Patch("/:id", h.handleUpdateService)
	svc.Delete("/:id", h.handleDeleteService)
	svc.Post("/:id/deploy", h.handleTriggerDeployment)
	svc.Post("/:id/stop", h.handleStopService)
	svc.Post("/:id/start", h.handleStartService)
	svc.Get("/:id/logs", h.handleGetLogs)
	svc.Get("/:id/domains", h.handleListServiceDomains)
	svc.Post("/:id/domains", h.handleAddServiceDomain)
	svc.Delete("/:id/domains/:domain", h.handleDeleteServiceDomain)
	svc.Get("/:id/routes", h.handleGetServiceRoutes)
	svc.Post("/:id/routes", h.handleUpdateServiceRoutes)
	svc.Get("/:id/blueprint", h.handleGetServiceBlueprint)

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
	db.Post("/:id/restart", h.handleRestartDatabase)
	db.Post("/:id/query", h.handleExecuteDatabaseQuery)
	db.Get("/:id/schema", h.handleGetDatabaseSchema)
	db.Get("/:id/logs", h.handleGetDatabaseLogs)
	db.Delete("/:id", h.handleDeleteDatabase)

	// Git Integrations routes (GitHub, GitLab, Bitbucket)
	v1.Get("/integrations/git/:provider/callback", h.handleGitOAuthCallback)
	git := v1.Group("/integrations/git", h.requireSession)
	git.Get("/", h.handleListGitIntegrations)
	git.Get("/:provider/authorize", h.handleGitOAuthAuthorize)
	git.Post("/", h.handleSaveGitIntegration)
	git.Delete("/:provider", h.handleDeleteGitIntegration)
	git.Get("/:provider/repos", h.handleListGitRepos)

	// Webhook endpoints for auto-deploy on commit (GitHub, GitLab, Bitbucket, Gitea)
	v1.Post("/webhooks/deploy/:serviceId", h.handleServiceDeployWebhook)
	v1.Post("/webhooks/git", h.handleGenericGitWebhook)
	v1.Post("/webhooks/git/:provider", h.handleGenericGitWebhook)
	v1.Post("/integrations/git/webhook", h.handleGenericGitWebhook)

	// Admin routes (main_admin only)
	admin := v1.Group("/admin", h.requireSession, h.requireMainAdmin)
	admin.Get("/telemetry", h.handleGetTelemetry)
	admin.Get("/users", h.handleAdminListUsers)
	admin.Post("/users/:id/approve", h.handleApproveUser)
	admin.Post("/users/:id/suspend", h.handleSuspendUser)
	admin.Delete("/users/:id", h.handleAdminDeleteUser)
	admin.Get("/audit", h.handleListAuditEvents)
	admin.Get("/settings", h.handleGetPlatformSettings)
	admin.Patch("/settings", h.handleUpdatePlatformSettings)
	admin.Post("/setup", h.handleAdminSetup)
	admin.Post("/maintenance/prune-storage", h.handlePruneStorage)
	admin.Post("/maintenance/optimize-containers", h.handleOptimizeContainers)
	admin.Get("/containers", h.handleListAllContainers)
	admin.Delete("/containers/:nameOrId", h.handleDeleteContainerInstance)
	admin.Post("/containers/prune-orphans", h.handlePruneAllFloatingContainers)

	return app
}
