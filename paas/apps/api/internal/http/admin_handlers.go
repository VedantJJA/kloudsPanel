package http

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/disk"
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

	ghClient, ghSecret := getProviderOAuthCredentials("github")
	glClient, glSecret := getProviderOAuthCredentials("gitlab")
	bbClient, bbSecret := getProviderOAuthCredentials("bitbucket")

	return c.JSON(fiber.Map{
		"settings": fiber.Map{
			"root_domain":             getRootDomain(),
			"acme_email":              "",
			"dns_mode":                "http-01",
			"auto_approve_users":      autoApprove,
			"github_client_id":        ghClient,
			"github_client_secret":    ghSecret,
			"gitlab_client_id":        glClient,
			"gitlab_client_secret":    glSecret,
			"bitbucket_client_id":     bbClient,
			"bitbucket_client_secret": bbSecret,
		},
	})
}

func (h *Handler) handleUpdatePlatformSettings(c fiber.Ctx) error {
	var req struct {
		AutoApproveUsers      *bool  `json:"auto_approve_users"`
		GitHubClientID        string `json:"github_client_id"`
		GitHubClientSecret    string `json:"github_client_secret"`
		GitLabClientID        string `json:"gitlab_client_id"`
		GitLabClientSecret    string `json:"gitlab_client_secret"`
		BitbucketClientID     string `json:"bitbucket_client_id"`
		BitbucketClientSecret string `json:"bitbucket_client_secret"`
	}
	if err := c.Bind().JSON(&req); err == nil {
		if req.AutoApproveUsers != nil {
			autoApproveMu.Lock()
			autoApproveUsers = *req.AutoApproveUsers
			autoApproveMu.Unlock()
		}
		if req.GitHubClientID != "" {
			setProviderOAuthCredentials("github", req.GitHubClientID, req.GitHubClientSecret)
		}
		if req.GitLabClientID != "" {
			setProviderOAuthCredentials("gitlab", req.GitLabClientID, req.GitLabClientSecret)
		}
		if req.BitbucketClientID != "" {
			setProviderOAuthCredentials("bitbucket", req.BitbucketClientID, req.BitbucketClientSecret)
		}
	}
	return h.handleGetPlatformSettings(c)
}

func (h *Handler) handleAdminSetup(c fiber.Ctx) error {
	return h.handleUpdatePlatformSettings(c)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (h *Handler) handlePruneStorage(c fiber.Ctx) error {
	diskBefore, _ := disk.Usage("/")
	beforeUsed := uint64(0)
	if diskBefore != nil {
		beforeUsed = diskBefore.Used
	}

	var logs []string

	// 1. Prune Docker build cache (BuildKit)
	out1, err1 := exec.Command("docker", "builder", "prune", "-a", "-f").CombinedOutput()
	if err1 == nil {
		logs = append(logs, "Docker BuildKit cache cleared.")
	} else if len(out1) > 0 {
		logs = append(logs, fmt.Sprintf("BuildKit: %s", strings.TrimSpace(string(out1))))
	}

	// 2. Prune dangling and unused Docker images
	out2, _ := exec.Command("docker", "image", "prune", "-f").CombinedOutput()
	if len(out2) > 0 {
		logs = append(logs, "Dangling image layers pruned.")
	}

	// 3. Prune stopped ephemeral build containers
	out3, _ := exec.Command("docker", "container", "prune", "-f").CombinedOutput()
	if len(out3) > 0 {
		logs = append(logs, "Stopped build containers removed.")
	}

	// NOTE: Volumes are NEVER pruned during storage reclamation to guarantee all database and persistent volumes remain 100% intact.

	// 4. Clean systemd journal logs (Linux VPS)
	_ = exec.Command("journalctl", "--vacuum-time=2d").Run()

	// 6. Clean APT package cache
	_ = exec.Command("apt-get", "clean").Run()

	// 7. Clean temp build directories
	_ = exec.Command("rm", "-rf", "/tmp/paas-*").Run()

	diskAfter, _ := disk.Usage("/")
	afterUsed := uint64(0)
	totalBytes := uint64(0)
	if diskAfter != nil {
		afterUsed = diskAfter.Used
		totalBytes = diskAfter.Total
	}

	var reclaimedBytes int64
	if beforeUsed > afterUsed {
		reclaimedBytes = int64(beforeUsed - afterUsed)
	}

	reclaimedFormatted := formatBytes(uint64(reclaimedBytes))

	return c.JSON(fiber.Map{
		"success":             true,
		"reclaimed_bytes":     reclaimedBytes,
		"reclaimed_formatted": reclaimedFormatted,
		"used_before":         beforeUsed,
		"used_after":          afterUsed,
		"storage_total":       totalBytes,
		"logs":                logs,
		"message":             fmt.Sprintf("Storage reclamation complete. Reclaimed %s of build cache, dangling layers, and logs.", reclaimedFormatted),
	})
}
