package http

import (
	"os"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// Admin Handlers (main_admin only)

var (
	autoApproveMu    sync.RWMutex
	autoApproveUsers = true
)

func init() {
	if v := os.Getenv("AUTO_APPROVE_USERS"); v == "false" || v == "0" {
		autoApproveUsers = false
	}
}

func isAutoApproveEnabled() bool {
	autoApproveMu.RLock()
	defer autoApproveMu.RUnlock()
	return autoApproveUsers
}

func (h *Handler) handleAdminListUsers(c fiber.Ctx) error {
	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status")))
	users, err := h.store.Users().ListAll(c.Context(), 100, 0)
	if err != nil {
		return err
	}
	type userItem struct {
		ID            string              `json:"id"`
		Email         string              `json:"email"`
		DisplayName   string              `json:"displayName"`
		Display_Name  string              `json:"display_name"`
		Status        domain.UserStatus   `json:"status"`
		PlatformRole  domain.PlatformRole `json:"platformRole"`
		Platform_Role string              `json:"platform_role"`
		IsMainAdmin   bool                `json:"isMainAdmin"`
		CreatedAt     string              `json:"createdAt"`
		Created_At    string              `json:"created_at"`
		LastLoginAt   string              `json:"lastLoginAt,omitempty"`
		Last_Login_At string              `json:"last_login_at,omitempty"`
	}
	var out []userItem
	for _, u := range users {
		if statusFilter != "" && strings.ToLower(string(u.Status)) != statusFilter {
			continue
		}
		var lastLogin string
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format("2006-01-02 15:04:05")
		}
		out = append(out, userItem{
			ID:            u.ID,
			Email:         u.Email,
			DisplayName:   u.DisplayName,
			Display_Name:  u.DisplayName,
			Status:        u.Status,
			PlatformRole:  u.PlatformRole,
			Platform_Role: string(u.PlatformRole),
			IsMainAdmin:   u.PlatformRole == domain.PlatformRoleMainAdmin,
			CreatedAt:     u.CreatedAt.Format("2006-01-02 15:04:05"),
			Created_At:    u.CreatedAt.Format("2006-01-02 15:04:05"),
			LastLoginAt:   lastLogin,
			Last_Login_At: lastLogin,
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
	autoApproveMu.RLock()
	autoApprove := autoApproveUsers
	autoApproveMu.RUnlock()

	return c.JSON(fiber.Map{
		"settings": fiber.Map{
			"root_domain":        getRootDomain(),
			"acme_email":         "",
			"dns_mode":           "http-01",
			"auto_approve_users": autoApprove,
		},
	})
}

func (h *Handler) handleUpdatePlatformSettings(c fiber.Ctx) error {
	var req struct {
		AutoApproveUsers *bool `json:"auto_approve_users"`
	}
	if err := c.Bind().JSON(&req); err == nil && req.AutoApproveUsers != nil {
		autoApproveMu.Lock()
		autoApproveUsers = *req.AutoApproveUsers
		autoApproveMu.Unlock()
	}
	return h.handleGetPlatformSettings(c)
}

func (h *Handler) handleAdminSetup(c fiber.Ctx) error {
	return h.handleUpdatePlatformSettings(c)
}
