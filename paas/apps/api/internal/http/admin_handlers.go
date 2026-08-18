package http

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/yourorg/klouds/api/internal/domain"
)

// Admin Handlers (main_admin only)

var (
	autoApproveMu    sync.RWMutex
	autoApproveUsers = false
)

func init() {
	if v := os.Getenv("AUTO_APPROVE_USERS"); v == "true" || v == "1" {
		autoApproveUsers = true
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

func (h *Handler) handleAdminDeleteUser(c fiber.Ctx) error {
	userID := c.Params("id")
	user, err := h.store.Users().GetByID(c.Context(), userID)
	if err != nil || user == nil {
		return c.Status(404).JSON(fiber.Map{"error": "user not found"})
	}
	if user.PlatformRole == domain.PlatformRoleMainAdmin {
		return c.Status(400).JSON(fiber.Map{"error": "cannot delete main admin account"})
	}

	// Clean up user workspaces, projects, services, containers, and databases
	if workspaces, err := h.store.Workspaces().ListForUser(c.Context(), user.ID); err == nil {
		for _, ws := range workspaces {
			if ws.CreatedBy == user.ID {
				if projects, err := h.store.Projects().ListForWorkspace(c.Context(), ws.ID, 1000, 0); err == nil {
					for _, p := range projects {
						// Remove all services & containers
						if services, err := h.store.Services().ListForProject(c.Context(), p.ID); err == nil {
							for _, s := range services {
								_ = exec.Command("docker", "rm", "-f", fmt.Sprintf("paas-svc-%s", s.Slug)).Run()
								removeTraefikDynamicConfig(s.Slug)
								_ = h.store.Services().Delete(c.Context(), s.ID)
							}
						}
						// Remove all databases & containers
						if dbs, err := h.store.Databases().ListForProject(c.Context(), p.ID); err == nil {
							for _, db := range dbs {
								_ = exec.Command("docker", "rm", "-f", fmt.Sprintf("paas-db-%s", db.Name)).Run()
								_ = exec.Command("docker", "rm", "-f", fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))).Run()
								_ = h.store.Databases().Delete(c.Context(), db.ID)
							}
						}
						_ = h.store.Projects().Delete(c.Context(), p.ID)
					}
				}
				_ = h.store.Workspaces().Delete(c.Context(), ws.ID)
			}
		}
	}

	// Delete git integration tokens
	_ = h.store.GitIntegrations().Delete(c.Context(), user.ID, "github")
	_ = h.store.GitIntegrations().Delete(c.Context(), user.ID, "gitlab")
	_ = h.store.GitIntegrations().Delete(c.Context(), user.ID, "bitbucket")

	// Delete user record
	if err := h.store.Users().Delete(c.Context(), user.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to delete user: %v", err)})
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "User and associated projects deleted successfully"})
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

func (h *Handler) handleOptimizeContainers(c fiber.Ctx) error {
	// 1. Gather all active services and databases registered in the database
	services, err := h.store.Services().ListAll(c.Context())
	if err != nil {
		services = []*domain.Service{}
	}
	databases, err := h.store.Databases().ListAll(c.Context())
	if err != nil {
		databases = []*domain.Database{}
	}

	knownContainers := make(map[string]bool)
	for _, s := range services {
		knownContainers[fmt.Sprintf("paas-svc-%s", s.Slug)] = true
	}
	for _, db := range databases {
		if db.InternalHostname != "" {
			knownContainers[db.InternalHostname] = true
		}
		dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(db.Name), "_", "-"))
		knownContainers[fmt.Sprintf("paas-db-%s", dbSlug)] = true
		knownContainers[fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))] = true
	}

	// 2. Query Docker for all existing container names on the host
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}").Output()
	var removedContainers []string
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			// Identify unmanaged paas service or database containers
			if strings.HasPrefix(name, "paas-svc-") || strings.HasPrefix(name, "paas-db-") || strings.HasPrefix(name, "paas-build-") {
				if !knownContainers[name] {
					// Orphan detected: terminate and purge
					_ = exec.Command("docker", "rm", "-f", name).Run()
					removedContainers = append(removedContainers, name)

					// If service container, remove orphaned Traefik dynamic routing
					if strings.HasPrefix(name, "paas-svc-") {
						slug := strings.TrimPrefix(name, "paas-svc-")
						removeTraefikDynamicConfig(slug)
					}
				}
			}
		}
	}

	// 3. Prune dangling layers and stopped containers
	_ = exec.Command("docker", "container", "prune", "-f").Run()
	_ = exec.Command("docker", "image", "prune", "-f").Run()

	return c.JSON(fiber.Map{
		"success":            true,
		"removed_containers": removedContainers,
		"count":              len(removedContainers),
		"message":            fmt.Sprintf("Optimizer scan complete. Removed %d orphan container(s) and synced routing state.", len(removedContainers)),
	})
}

type AdminContainerItem struct {
	ID               string `json:"id"`
	Names            string `json:"names"`
	Image            string `json:"image"`
	Status           string `json:"status"`
	State            string `json:"state"`
	CreatedAt        string `json:"created_at"`
	Ports            string `json:"ports"`
	Size             string `json:"size"`
	Type             string `json:"type"` // service, database, build, system, other
	Slug             string `json:"slug,omitempty"`
	IsOrphan         bool   `json:"is_orphan"`
	HasTraefikConfig bool   `json:"has_traefik_config"`
	WorkspaceName    string `json:"workspace_name,omitempty"`
	ProjectName      string `json:"project_name,omitempty"`
	ServiceName      string `json:"service_name,omitempty"`
	ServiceID        string `json:"service_id,omitempty"`
	DatabaseID       string `json:"database_id,omitempty"`
}

func (h *Handler) handleListAllContainers(c fiber.Ctx) error {
	// 1. Fetch all services, databases, projects, and workspaces for indexing
	services, _ := h.store.Services().ListAll(c.Context())
	databases, _ := h.store.Databases().ListAll(c.Context())
	projects, _ := h.store.Projects().ListAll(c.Context())
	users, _ := h.store.Users().ListAll(c.Context(), 1000, 0)

	var workspaces []*domain.Workspace
	for _, u := range users {
		wsList, err := h.store.Workspaces().ListForUser(c.Context(), u.ID)
		if err == nil {
			workspaces = append(workspaces, wsList...)
		}
	}

	projectMap := make(map[string]*domain.Project)
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	workspaceMap := make(map[string]*domain.Workspace)
	for _, w := range workspaces {
		workspaceMap[w.ID] = w
	}

	serviceContainerMap := make(map[string]*domain.Service)
	for _, s := range services {
		serviceContainerMap[fmt.Sprintf("paas-svc-%s", s.Slug)] = s
		serviceContainerMap[s.Slug] = s
	}

	databaseContainerMap := make(map[string]*domain.Database)
	for _, db := range databases {
		if db.InternalHostname != "" {
			databaseContainerMap[db.InternalHostname] = db
		}
		dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(db.Name), "_", "-"))
		databaseContainerMap[fmt.Sprintf("paas-db-%s", dbSlug)] = db
		databaseContainerMap[fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))] = db
		databaseContainerMap[db.Name] = db
	}

	// 2. Query Docker CLI for all containers
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.State}}\t{{.CreatedAt}}\t{{.Ports}}\t{{.Size}}").Output()
	var containers []AdminContainerItem
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 5 {
				continue
			}

			id := parts[0]
			names := parts[1]
			image := parts[2]
			status := parts[3]
			state := parts[4]
			createdAt := ""
			if len(parts) > 5 {
				createdAt = parts[5]
			}
			ports := ""
			if len(parts) > 6 {
				ports = parts[6]
			}
			size := ""
			if len(parts) > 7 {
				size = parts[7]
			}

			// Clean primary name (Docker names can have leading /)
			cleanName := strings.TrimPrefix(names, "/")

			item := AdminContainerItem{
				ID:        id,
				Names:     cleanName,
				Image:     image,
				Status:    status,
				State:     state,
				CreatedAt: createdAt,
				Ports:     ports,
				Size:      size,
				Type:      "other",
			}

			// Determine container type
			if strings.HasPrefix(cleanName, "paas-svc-") {
				item.Type = "service"
				item.Slug = strings.TrimPrefix(cleanName, "paas-svc-")
			} else if strings.HasPrefix(cleanName, "paas-db-") {
				item.Type = "database"
				item.Slug = strings.TrimPrefix(cleanName, "paas-db-")
			} else if strings.HasPrefix(cleanName, "paas-build-") || strings.HasPrefix(cleanName, "paas-builder-") {
				item.Type = "build"
				item.Slug = strings.TrimPrefix(strings.TrimPrefix(cleanName, "paas-build-"), "paas-builder-")
			} else if cleanName == "paas-traefik" || cleanName == "traefik" || strings.Contains(image, "traefik") {
				item.Type = "system"
			}

			// Check Traefik configuration existence
			if item.Slug != "" {
				dynamicDir := "/traefik/dynamic"
				if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
					dynamicDir = "./paas/deploy/traefik/dynamic"
				}
				traefikFile := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", item.Slug))
				if _, err := os.Stat(traefikFile); err == nil {
					item.HasTraefikConfig = true
				}
			}

			// Check if mapped to active database record or if it is an orphan
			if item.Type == "service" {
				if s, exists := serviceContainerMap[cleanName]; exists {
					item.ServiceName = s.Name
					item.ServiceID = s.ID
					if p, pExists := projectMap[s.ProjectID]; pExists {
						item.ProjectName = p.Name
						if w, wExists := workspaceMap[p.WorkspaceID]; wExists {
							item.WorkspaceName = w.Name
						}
					}
					item.IsOrphan = false
				} else {
					item.IsOrphan = true
				}
			} else if item.Type == "database" {
				if db, exists := databaseContainerMap[cleanName]; exists {
					item.ServiceName = db.Name
					item.DatabaseID = db.ID
					if p, pExists := projectMap[db.ProjectID]; pExists {
						item.ProjectName = p.Name
						if w, wExists := workspaceMap[p.WorkspaceID]; wExists {
							item.WorkspaceName = w.Name
						}
					}
					item.IsOrphan = false
				} else {
					item.IsOrphan = true
				}
			} else if item.Type == "build" {
				item.IsOrphan = true
			}

			containers = append(containers, item)
		}
	}

	return c.JSON(fiber.Map{
		"containers": containers,
		"total":      len(containers),
	})
}

func (h *Handler) handleDeleteContainerInstance(c fiber.Ctx) error {
	nameOrId := c.Params("nameOrId")
	if nameOrId == "" || nameOrId == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid container identifier"})
	}

	cleanName := strings.TrimPrefix(nameOrId, "/")
	var slug string
	if strings.HasPrefix(cleanName, "paas-svc-") {
		slug = strings.TrimPrefix(cleanName, "paas-svc-")
	} else if strings.HasPrefix(cleanName, "paas-db-") {
		slug = strings.TrimPrefix(cleanName, "paas-db-")
	}

	// 1. Forcefully remove Docker container
	_ = exec.Command("docker", "rm", "-f", cleanName).Run()
	_ = exec.Command("docker", "rm", "-f", nameOrId).Run()

	// 2. Wipe associated images
	if slug != "" {
		_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-svc-%s:latest", slug)).Run()
		_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-app-%s:latest", slug)).Run()
		_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-svc-%s", slug)).Run()
	}

	// 3. Wipe persistent volumes
	if slug != "" {
		_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-svc-data-%s", slug)).Run()
		_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-db-data-%s", slug)).Run()
		_ = exec.Command("docker", "volume", "rm", "-f", cleanName).Run()
	}

	// 4. Remove Traefik dynamic routing configuration
	if slug != "" {
		removeTraefikDynamicConfig(slug)
	}

	// 5. Clean temp builds and caches
	if slug != "" {
		_ = exec.Command("rm", "-rf", fmt.Sprintf("/tmp/builds/%s", slug)).Run()
		_ = exec.Command("rm", "-rf", fmt.Sprintf("/tmp/paas-%s*", slug)).Run()
	}

	// 6. Clean residual DB rows if matched
	if slug != "" {
		if s, err := h.store.Services().GetByID(c.Context(), slug); err == nil && s != nil {
			_ = h.store.Services().Delete(c.Context(), s.ID)
		} else {
			allSvcs, _ := h.store.Services().ListAll(c.Context())
			for _, s := range allSvcs {
				if s.Slug == slug || s.ID == slug {
					_ = h.store.Services().Delete(c.Context(), s.ID)
				}
			}
		}

		allDbs, _ := h.store.Databases().ListAll(c.Context())
		for _, db := range allDbs {
			dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(db.Name), "_", "-"))
			if db.InternalHostname == cleanName || dbSlug == slug || strings.ToLower(db.Name) == slug {
				_ = h.store.Databases().Delete(c.Context(), db.ID)
			}
		}
	}

	// 7. Prune dangling build cache
	go func() {
		_ = exec.Command("docker", "container", "prune", "-f").Run()
		_ = exec.Command("docker", "image", "prune", "-f").Run()
	}()

	return c.JSON(fiber.Map{
		"success":   true,
		"container": cleanName,
		"message":   fmt.Sprintf("Container '%s' terminated, Traefik routing removed, and volumes purged.", cleanName),
	})
}

func (h *Handler) handlePruneAllFloatingContainers(c fiber.Ctx) error {
	// 1. Gather all active services and databases registered in the database
	services, _ := h.store.Services().ListAll(c.Context())
	databases, _ := h.store.Databases().ListAll(c.Context())

	knownContainers := make(map[string]bool)
	for _, s := range services {
		knownContainers[fmt.Sprintf("paas-svc-%s", s.Slug)] = true
	}
	for _, db := range databases {
		if db.InternalHostname != "" {
			knownContainers[db.InternalHostname] = true
		}
		dbSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(db.Name), "_", "-"))
		knownContainers[fmt.Sprintf("paas-db-%s", dbSlug)] = true
		knownContainers[fmt.Sprintf("paas-db-%s", strings.ToLower(db.Name))] = true
	}

	// 2. Query Docker for all existing container names on the host
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}").Output()
	var removedContainers []string
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, "paas-svc-") || strings.HasPrefix(cleanName, "paas-db-") || strings.HasPrefix(cleanName, "paas-build-") || strings.HasPrefix(cleanName, "paas-builder-") {
				if !knownContainers[cleanName] {
					// Orphan detected: terminate container
					_ = exec.Command("docker", "rm", "-f", cleanName).Run()
					removedContainers = append(removedContainers, cleanName)

					// If service container, wipe Traefik config, volume, and temp files
					if strings.HasPrefix(cleanName, "paas-svc-") {
						slug := strings.TrimPrefix(cleanName, "paas-svc-")
						removeTraefikDynamicConfig(slug)
						_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-svc-data-%s", slug)).Run()
						_ = exec.Command("rm", "-rf", fmt.Sprintf("/tmp/builds/%s", slug)).Run()
						_ = exec.Command("rm", "-rf", fmt.Sprintf("/tmp/paas-%s*", slug)).Run()
					} else if strings.HasPrefix(cleanName, "paas-db-") {
						slug := strings.TrimPrefix(cleanName, "paas-db-")
						_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-db-data-%s", slug)).Run()
					}
				}
			}
		}
	}

	// 3. Prune dangling layers and builder caches
	_ = exec.Command("docker", "container", "prune", "-f").Run()
	_ = exec.Command("docker", "image", "prune", "-f").Run()
	_ = exec.Command("docker", "builder", "prune", "-f").Run()

	return c.JSON(fiber.Map{
		"success":            true,
		"removed_containers": removedContainers,
		"count":              len(removedContainers),
		"message":            fmt.Sprintf("Pruned %d floating container(s) and cleared their networking and volumes.", len(removedContainers)),
	})
}
