package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// ─── Admin Handlers (main_admin only) ─────────────────────────────────────────

func (h *Handler) handleAdminListUsers(c fiber.Ctx) error {
	users, err := h.store.Users().ListAll(c.Context(), 100, 0)
	if err != nil {
		return err
	}
	type userItem struct {
		ID           string              `json:"id"`
		Email        string              `json:"email"`
		DisplayName  string              `json:"displayName"`
		Status       domain.UserStatus   `json:"status"`
		PlatformRole domain.PlatformRole `json:"platformRole"`
		IsMainAdmin  bool                `json:"isMainAdmin"`
		CreatedAt    string              `json:"createdAt"`
	}
	var out []userItem
	for _, u := range users {
		out = append(out, userItem{
			ID:           u.ID,
			Email:        u.Email,
			DisplayName:  u.DisplayName,
			Status:       u.Status,
			PlatformRole: u.PlatformRole,
			IsMainAdmin:  u.PlatformRole == domain.PlatformRoleMainAdmin,
			CreatedAt:    u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return c.JSON(fiber.Map{"users": out})
}

func (h *Handler) handleApproveUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	user.Status = domain.UserStatusActive
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"status": "ok", "user": user})
}

func (h *Handler) handleSuspendUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	if user.PlatformRole == domain.PlatformRoleMainAdmin {
		return c.Status(400).JSON(fiber.Map{"error": "cannot suspend main admin"})
	}
	user.Status = domain.UserStatusSuspended
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"status": "ok", "user": user})
}

func (h *Handler) handleListAuditEvents(c fiber.Ctx) error {
	events, err := h.store.AuditEvents().List(c.Context(), nil, 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"events": events})
}

func (h *Handler) handleGetPlatformSettings(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"settings": fiber.Map{
			"root_domain": getRootDomain(),
			"acme_email":  "",
			"dns_mode":    "http-01",
		},
	})
}

func (h *Handler) handleAdminSetup(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"message": "Platform settings saved",
	})
}
