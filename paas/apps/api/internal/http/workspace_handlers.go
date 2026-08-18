package http

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// --- Workspace Handlers -------------------------------------------------------

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
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Workspace name is required"})
	}

	// Disallow the SAME user from creating duplicate workspace names
	existingWorkspaces, err := h.store.Workspaces().ListForUser(c.Context(), u.ID)
	if err == nil {
		for _, existing := range existingWorkspaces {
			if strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(req.Name)) {
				return c.Status(400).JSON(fiber.Map{"error": "You already have a workspace named \"" + req.Name + "\""})
			}
		}
	}

	wsSlug := generateUniqueWorkspaceSlug(c.Context(), h.store, req.Name, req.Slug)
	ws := &domain.Workspace{
		Name:      req.Name,
		Slug:      wsSlug,
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

	ws, err := h.store.Workspaces().GetByID(c.Context(), id)
	if err != nil || ws == nil {
		ws, _ = h.store.Workspaces().GetBySlug(c.Context(), id)
	}
	actualID := id
	if ws != nil {
		actualID = ws.ID
	}

	// 1. Recursively find and delete all projects in this workspace
	projects, _ := h.store.Projects().ListForWorkspace(c.Context(), actualID, 1000, 0)
	for _, p := range projects {
		// Clean up all services in this project
		svcs, _ := h.store.Services().ListForProject(c.Context(), p.ID)
		for _, s := range svcs {
			cleanupServiceResources(s.Slug)
			_ = h.store.Services().Delete(c.Context(), s.ID)
		}
		// Clean up all databases in this project
		dbs, _ := h.store.Databases().ListForProject(c.Context(), p.ID)
		for _, db := range dbs {
			cleanupDatabaseResources(db.Name, db.InternalHostname)
			_ = h.store.Databases().Delete(c.Context(), db.ID)
		}
		// Delete project record
		_ = h.store.Projects().Delete(c.Context(), p.ID)
	}

	// 2. Clean up any databases linked directly to this workspace ID
	allDbs, _ := h.store.Databases().ListAll(c.Context())
	for _, db := range allDbs {
		if db.ProjectID == actualID {
			cleanupDatabaseResources(db.Name, db.InternalHostname)
			_ = h.store.Databases().Delete(c.Context(), db.ID)
		}
	}

	// 3. Delete the workspace record
	if err := h.store.Workspaces().Delete(c.Context(), actualID); err != nil {
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

func (h *Handler) handleListWorkspaceVariables(c fiber.Ctx) error {
	slugOrID := c.Params("slug")
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), slugOrID)
	if err != nil || ws == nil {
		ws, err = h.store.Workspaces().GetByID(c.Context(), slugOrID)
	}
	if err != nil || ws == nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}

	var data struct {
		SharedEnv []fiber.Map `json:"shared_env"`
		EnvGroups []fiber.Map `json:"env_groups"`
	}
	if ws.QuotaJSON != "" {
		_ = json.Unmarshal([]byte(ws.QuotaJSON), &data)
	}
	if data.SharedEnv == nil {
		data.SharedEnv = []fiber.Map{}
	}
	if data.EnvGroups == nil {
		data.EnvGroups = []fiber.Map{}
	}

	return c.JSON(fiber.Map{
		"variables": data.SharedEnv,
		"groups":    data.EnvGroups,
	})
}

func (h *Handler) handleSaveWorkspaceVariables(c fiber.Ctx) error {
	slugOrID := c.Params("slug")
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), slugOrID)
	if err != nil || ws == nil {
		ws, err = h.store.Workspaces().GetByID(c.Context(), slugOrID)
	}
	if err != nil || ws == nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}

	var req struct {
		Variables []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"variables"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	var data map[string]any
	if ws.QuotaJSON != "" {
		_ = json.Unmarshal([]byte(ws.QuotaJSON), &data)
	}
	if data == nil {
		data = make(map[string]any)
	}
	data["shared_env"] = req.Variables

	b, _ := json.Marshal(data)
	ws.QuotaJSON = string(b)
	if err := h.store.Workspaces().Update(c.Context(), ws); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"status": "ok", "variables": req.Variables})
}

func (h *Handler) handleSaveWorkspaceEnvGroup(c fiber.Ctx) error {
	slugOrID := c.Params("slug")
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), slugOrID)
	if err != nil || ws == nil {
		ws, err = h.store.Workspaces().GetByID(c.Context(), slugOrID)
	}
	if err != nil || ws == nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}

	var req struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		LinkedProjectIDs []string `json:"linkedProjectIds"`
		Variables        []struct {
			Key      string `json:"key"`
			Value    string `json:"value"`
			IsSecret bool   `json:"isSecret"`
		} `json:"variables"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "group name is required"})
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("grp_%d", time.Now().UnixNano())
	}
	if req.LinkedProjectIDs == nil {
		req.LinkedProjectIDs = []string{}
	}

	var data map[string]any
	if ws.QuotaJSON != "" {
		_ = json.Unmarshal([]byte(ws.QuotaJSON), &data)
	}
	if data == nil {
		data = make(map[string]any)
	}

	var groups []map[string]any
	if rawGroups, ok := data["env_groups"].([]any); ok {
		for _, rg := range rawGroups {
			if gm, ok := rg.(map[string]any); ok {
				groups = append(groups, gm)
			}
		}
	}

	newGroup := map[string]any{
		"id":               req.ID,
		"name":             req.Name,
		"description":      req.Description,
		"linkedProjectIds": req.LinkedProjectIDs,
		"variables":        req.Variables,
		"updatedAt":        time.Now().Format(time.RFC3339),
	}

	found := false
	for i, g := range groups {
		if g["id"] == req.ID {
			groups[i] = newGroup
			found = true
			break
		}
	}
	if !found {
		groups = append(groups, newGroup)
	}

	data["env_groups"] = groups
	b, _ := json.Marshal(data)
	ws.QuotaJSON = string(b)
	if err := h.store.Workspaces().Update(c.Context(), ws); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"status": "ok", "group": newGroup})
}

func (h *Handler) handleDeleteWorkspaceEnvGroup(c fiber.Ctx) error {
	slugOrID := c.Params("slug")
	groupID := c.Params("groupId")
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), slugOrID)
	if err != nil || ws == nil {
		ws, err = h.store.Workspaces().GetByID(c.Context(), slugOrID)
	}
	if err != nil || ws == nil {
		return c.Status(404).JSON(fiber.Map{"error": "workspace not found"})
	}

	var data map[string]any
	if ws.QuotaJSON != "" {
		_ = json.Unmarshal([]byte(ws.QuotaJSON), &data)
	}
	if data == nil {
		return c.JSON(fiber.Map{"status": "ok"})
	}

	var groups []map[string]any
	if rawGroups, ok := data["env_groups"].([]any); ok {
		for _, rg := range rawGroups {
			if gm, ok := rg.(map[string]any); ok {
				if gm["id"] != groupID {
					groups = append(groups, gm)
				}
			}
		}
	}

	data["env_groups"] = groups
	b, _ := json.Marshal(data)
	ws.QuotaJSON = string(b)
	if err := h.store.Workspaces().Update(c.Context(), ws); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"status": "ok"})
}
