package http

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Workspace Handlers ───────────────────────────────────────────────────────

func (h *Handler) handleListWorkspaces(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	workspaces, err := h.store.Workspaces().ListForUser(c.Context(), u.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"workspaces": workspaces})
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
	ws := &domain.Workspace{
		Name:      req.Name,
		Slug:      req.Slug,
		CreatedBy: u.ID,
		Status:    domain.WorkspaceStatusActive,
	}
	if err := h.store.Workspaces().Create(c.Context(), ws); err != nil {
		return err
	}
	member := &domain.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      u.ID,
		Role:        domain.RoleOwner,
		Status:      "active",
		JoinedAt:    time.Now().UTC(),
	}
	_ = h.store.Workspaces().AddMember(c.Context(), member)
	return c.Status(201).JSON(ws)
}

func (h *Handler) handleGetWorkspace(c fiber.Ctx) error {
	slugOrID := c.Params("slug")
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), slugOrID)
	if err != nil || ws == nil {
		ws, err = h.store.Workspaces().GetByID(c.Context(), slugOrID)
	}
	if err != nil || ws == nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}
	return c.JSON(ws)
}

func (h *Handler) handleUpdateWorkspace(c fiber.Ctx) error {
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind().JSON(&req); err == nil && req.Name != "" {
		ws.Name = req.Name
		_ = h.store.Workspaces().Update(c.Context(), ws)
	}
	return c.JSON(ws)
}

func (h *Handler) handleDeleteWorkspace(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid workspace id"})
	}
	if err := h.store.Workspaces().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
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

func (h *Handler) handleInviteMember(c fiber.Ctx) error {
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	var req struct {
		Email string                     `json:"email"`
		Role  domain.WorkspaceMemberRole `json:"role"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	user, err := h.store.Users().GetByEmail(c.Context(), req.Email)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	if req.Role == "" {
		req.Role = domain.RoleDeveloper
	}
	member := &domain.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        req.Role,
		Status:      "active",
		JoinedAt:    time.Now().UTC(),
	}
	if err := h.store.Workspaces().AddMember(c.Context(), member); err != nil {
		return err
	}
	return c.Status(201).JSON(fiber.Map{"status": "ok"})
}
