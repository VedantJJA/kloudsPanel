package http

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Project Handlers ─────────────────────────────────────────────────────────

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
	var req struct {
		WorkspaceID string `json:"workspaceId"`
		WorkspaceSlug string `json:"workspace_id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		SourceKind  string `json:"source_kind"`
		SourceKindCamel string `json:"sourceKind"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	wsID := req.WorkspaceID
	if wsID == "" {
		wsID = req.WorkspaceSlug
	}
	if ws, err := h.store.Workspaces().GetByID(c.Context(), wsID); err == nil && ws != nil {
		wsID = ws.ID
	}
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}
	sourceStr := strings.ToLower(strings.TrimSpace(req.SourceKind))
	if sourceStr == "" {
		sourceStr = strings.ToLower(strings.TrimSpace(req.SourceKindCamel))
	}
	sk := domain.SourceKindEmpty
	if sourceStr == "git" {
		sk = domain.SourceKindGit
	} else if sourceStr == "upload" {
		sk = domain.SourceKindUpload
	}
	var createdBy string
	if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
		createdBy = u.ID
	}
	pSlug := generateUniqueProjectSlug(c.Context(), h.store, req.Name, req.Slug)
	p := &domain.Project{
		WorkspaceID:   wsID,
		Name:          req.Name,
		Slug:          pSlug,
		Description:   desc,
		SourceKind:    sk,
		RootDirectory: ".",
		Status:        domain.ProjectStatusActive,
		CreatedBy:     createdBy,
	}
	if err := h.store.Projects().Create(c.Context(), p); err != nil {
		return err
	}
	return c.Status(201).JSON(p)
}

func (h *Handler) handleGetProject(c fiber.Ctx) error {
	slugOrID := c.Params("id")
	p, err := h.store.Projects().GetByID(c.Context(), slugOrID)
	if err != nil || p == nil {
		return c.Status(404).JSON(fiber.Map{"error": "project not found"})
	}
	wsName := ""
	wsSlug := ""
	if ws, err := h.store.Workspaces().GetByID(c.Context(), p.WorkspaceID); err == nil && ws != nil {
		wsName = ws.Name
		wsSlug = ws.Slug
	}
	return c.JSON(fiber.Map{
		"id":             p.ID,
		"workspace_id":   p.WorkspaceID,
		"workspace_name": wsName,
		"workspace_slug": wsSlug,
		"name":           p.Name,
		"slug":           p.Slug,
		"description":    p.Description,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	})
}

func (h *Handler) handleUpdateProject(c fiber.Ctx) error {
	p, err := h.store.Projects().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.Bind().JSON(&req); err == nil {
		if req.Name != "" {
			p.Name = req.Name
		}
		if req.Description != "" {
			p.Description = &req.Description
		}
		_ = h.store.Projects().Update(c.Context(), p)
	}
	return c.JSON(p)
}

func (h *Handler) handleDeleteProject(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid project id"})
	}

	// Clean up all services in this project
	svcs, _ := h.store.Services().ListForProject(c.Context(), id)
	for _, s := range svcs {
		cleanupServiceResources(s.Slug)
	}

	// Clean up all databases in this project
	dbs, _ := h.store.Databases().ListForProject(c.Context(), id)
	for _, db := range dbs {
		cleanupDatabaseResources(db.Name, db.InternalHostname)
	}

	if err := h.store.Projects().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
