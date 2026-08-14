package http

import (
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
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		SourceKind  string `json:"sourceKind"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}
	sk := domain.SourceKind(req.SourceKind)
	if sk != domain.SourceKindGit && sk != domain.SourceKindUpload && sk != domain.SourceKindEmpty {
		sk = domain.SourceKindEmpty
	}
	var createdBy string
	if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
		createdBy = u.ID
	}
	p := &domain.Project{
		WorkspaceID:   req.WorkspaceID,
		Name:          req.Name,
		Slug:          req.Slug,
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
	if err := h.store.Projects().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
