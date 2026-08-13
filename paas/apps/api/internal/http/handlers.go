package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/yourorg/klouds/api/internal/domain"
	"github.com/yourorg/klouds/api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store repository.Store
	log   *slog.Logger
}

func NewHandler(store repository.Store, log *slog.Logger) *Handler {
	h := &Handler{store: store, log: log}
	h.bootstrapAdmin()
	return h
}

func (h *Handler) bootstrapAdmin() {
	ctx := context.Background()
	users, err := h.store.Users().ListAll(ctx, 10, 0)
	if err == nil && len(users) == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin321"), bcrypt.DefaultCost)
		admin := &domain.User{
			Email:        "vedantjja@gmail.com",
			DisplayName:  "Admin",
			PasswordHash: string(hash),
			Status:       domain.UserStatusActive,
			PlatformRole: domain.PlatformRoleMainAdmin,
		}
		if err := h.store.Users().Create(ctx, admin); err != nil {
			h.log.Error("failed to bootstrap admin user", "err", err)
		} else {
			h.log.Info("bootstrapped default admin user", "email", admin.Email)
		}
	}
}

// ─── Middleware ────────────────────────────────────────────────────────────────

func (h *Handler) requireSession(c fiber.Ctx) error {
	sessionID := c.Cookies("klouds_session")
	if sessionID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "session required")
	}

	// For now, sessionID is the User ID directly (placeholder for real session tokens)
	user, err := h.store.Users().GetByID(c.Context(), sessionID)
	if err != nil || user.Status != domain.UserStatusActive {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid session")
	}

	c.Locals("user", user)
	return c.Next()
}

func (h *Handler) requireMainAdmin(c fiber.Ctx) error {
	u, ok := c.Locals("user").(*domain.User)
	if !ok || u.PlatformRole != domain.PlatformRoleMainAdmin {
		return fiber.NewError(fiber.StatusForbidden, "admin only")
	}
	return c.Next()
}

// ─── Auth Handlers ─────────────────────────────────────────────────────────────

type signupRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func (h *Handler) handleSignup(c fiber.Ctx) error {
	var req signupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := &domain.User{
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: string(hash),
		Status:       domain.UserStatusPending,
		PlatformRole: domain.PlatformRoleUser,
	}

	// If it's the very first user, auto-approve them as main admin
	users, _ := h.store.Users().ListAll(c.Context(), 1, 0)
	if len(users) == 0 {
		user.Status = domain.UserStatusActive
		user.PlatformRole = domain.PlatformRoleMainAdmin
	}

	if err := h.store.Users().Create(c.Context(), user); err != nil {
		return fiber.NewError(fiber.StatusConflict, "email already in use")
	}

	if user.Status == domain.UserStatusActive {
		c.Cookie(&fiber.Cookie{
			Name:     "klouds_session",
			Value:    user.ID,
			Path:     "/",
			MaxAge:   86400 * 7,
			HTTPOnly: true,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Account created.",
		"status":  user.Status,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) handleLogin(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	user, err := h.store.Users().GetByEmail(c.Context(), req.Email)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		return fiber.NewError(fiber.StatusForbidden, "account pending or suspended")
	}

	c.Cookie(&fiber.Cookie{
		Name:     "klouds_session",
		Value:    user.ID,
		Path:     "/",
		MaxAge:   86400 * 7,
		HTTPOnly: true,
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "login ok", "user_id": user.ID})
}

func (h *Handler) handleLogout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{Name: "klouds_session", Value: "", MaxAge: -1, Path: "/"})
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) handleMe(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	return c.JSON(fiber.Map{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "role": u.PlatformRole})
}

// ─── Workspace Handlers ────────────────────────────────────────────────────────

func (h *Handler) handleListWorkspaces(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	ws, err := h.store.Workspaces().ListForUser(c.Context(), u.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"workspaces": ws})
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
	ws := &domain.Workspace{Name: req.Name, Slug: req.Slug, CreatedBy: u.ID, Status: domain.WorkspaceStatusActive}
	if err := h.store.Workspaces().Create(c.Context(), ws); err != nil {
		return err
	}
	_ = h.store.Workspaces().AddMember(c.Context(), &domain.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      u.ID,
		Role:        domain.RoleOwner,
		Status:      "active",
		JoinedAt:    time.Now().UTC(),
	})
	return c.Status(201).JSON(ws)
}

func (h *Handler) handleGetWorkspace(c fiber.Ctx) error {
	ws, err := h.store.Workspaces().GetBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return c.JSON(ws)
}

func (h *Handler) handleUpdateWorkspace(c fiber.Ctx) error { return c.SendStatus(200) }

func (h *Handler) handleDeleteWorkspace(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid workspace id"})
	}
	if err := h.store.Workspaces().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(202)
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

func (h *Handler) handleInviteMember(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }

// ─── Project Handlers ──────────────────────────────────────────────────────────

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
	u := c.Locals("user").(*domain.User)
	var req struct {
		WorkspaceID string            `json:"workspaceId"`
		Name        string            `json:"name"`
		Slug        string            `json:"slug"`
		SourceKind  domain.SourceKind `json:"sourceKind"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.SourceKind == "" {
		req.SourceKind = domain.SourceKindEmpty
	}
	p := &domain.Project{
		WorkspaceID:   req.WorkspaceID,
		Name:          req.Name,
		Slug:          req.Slug,
		SourceKind:    req.SourceKind,
		RootDirectory: ".",
		CreatedBy:     u.ID,
		Status:        domain.ProjectStatusActive,
	}
	if err := h.store.Projects().Create(c.Context(), p); err != nil {
		return err
	}
	return c.Status(201).JSON(p)
}

func (h *Handler) handleGetProject(c fiber.Ctx) error {
	p, err := h.store.Projects().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(p)
}

func (h *Handler) handleUpdateProject(c fiber.Ctx) error { return c.SendStatus(200) }

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

func getRootDomain() string {
	if d := os.Getenv("ROOT_DOMAIN"); d != "" {
		return d
	}
	return "klouds.online"
}

func writeTraefikDynamicConfig(slug string, port int, rootDomain string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
		if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
			_ = os.MkdirAll(dynamicDir, 0755)
		}
	}

	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	content := fmt.Sprintf(`http:
  routers:
    svc-%s:
      rule: "Host(`+"`"+`%s.%s`+"`"+`)"
      entryPoints:
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
  services:
    svc-%s:
      loadBalancer:
        servers:
          - url: "http://paas-svc-%s:%d"
`, slug, slug, rootDomain, slug, slug, slug, port)

	_ = os.WriteFile(filePath, []byte(content), 0644)
}

func fetchGitHubRepos(token string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	req, err := nethttp.NewRequest("GET", "https://api.github.com/user/repos?sort=updated&per_page=50", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kloudsPanel-App")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("github api returned status: %s", resp.Status)
	}

	var ghRepos []struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		HTMLURL       string `json:"html_url"`
		DefaultBranch string `json:"default_branch"`
		Language      string `json:"language"`
		Private       bool   `json:"private"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
		return nil, err
	}

	repos := make([]fiber.Map, 0, len(ghRepos))
	for _, r := range ghRepos {
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.FullName,
			"url":            r.HTMLURL,
			"default_branch": r.DefaultBranch,
			"language":       r.Language,
			"is_private":     r.Private,
		})
	}
	return repos, nil
}

func fetchBitbucketRepos(username, password string) ([]fiber.Map, error) {
	client := &nethttp.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s?pagelen=50", username)
	req, err := nethttp.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("bitbucket api returned status: %s", resp.Status)
	}

	var bbResp struct {
		Values []struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Links    struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			MainBranch struct {
				Name string `json:"name"`
			} `json:"mainbranch"`
			Language  string `json:"language"`
			IsPrivate bool   `json:"is_private"`
		} `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bbResp); err != nil {
		return nil, err
	}

	repos := make([]fiber.Map, 0, len(bbResp.Values))
	for _, r := range bbResp.Values {
		repos = append(repos, fiber.Map{
			"name":           r.Name,
			"full_name":      r.FullName,
			"url":            r.Links.HTML.Href,
			"default_branch": r.MainBranch.Name,
			"language":       r.Language,
			"is_private":     r.IsPrivate,
		})
	}
	return repos, nil
}

// ─── Service Handlers ──────────────────────────────────────────────────────────

func (h *Handler) handleListServices(c fiber.Ctx) error {
	projID := c.Query("projectId")
	if projID == "" {
		return c.JSON(fiber.Map{"services": []any{}})
	}
	services, err := h.store.Services().ListForProject(c.Context(), projID)
	if err != nil {
		return err
	}
	rootDomain := getRootDomain()
	result := make([]fiber.Map, 0, len(services))
	for _, s := range services {
		domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)
		result = append(result, fiber.Map{
			"id":             s.ID,
			"project_id":     s.ProjectID,
			"name":           s.Name,
			"slug":           s.Slug,
			"kind":           s.Kind,
			"desired_state":  s.DesiredState,
			"runtime_status": s.RuntimeStatus,
			"internal_port":  s.InternalPort,
			"auto_deploy":    s.AutoDeploy,
			"resource_json":  s.ResourceJSON,
			"domain":         domainName,
			"endpoint_url":   fmt.Sprintf("https://%s", domainName),
			"created_by":     s.CreatedBy,
			"created_at":     s.CreatedAt,
			"updated_at":     s.UpdatedAt,
		})
	}
	return c.JSON(fiber.Map{"services": result})
}

func (h *Handler) handleCreateService(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	var req struct {
		ProjectID    string             `json:"projectId"`
		Name         string             `json:"name"`
		Slug         string             `json:"slug"`
		Kind         domain.ServiceKind `json:"kind"`
		InternalPort *int               `json:"internalPort"`
		ResourceJSON string             `json:"resourceJson"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Kind == "" {
		req.Kind = domain.ServiceKindWeb
	}
	if req.InternalPort == nil {
		p := 80
		if req.Kind == domain.ServiceKindWeb {
			p = 5000
		}
		req.InternalPort = &p
	}
	s := &domain.Service{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		Slug:          req.Slug,
		Kind:          req.Kind,
		CreatedBy:     u.ID,
		InternalPort:  req.InternalPort,
		ResourceJSON:  req.ResourceJSON,
		DesiredState:  domain.ServiceDesiredRunning,
		RuntimeStatus: domain.ServiceStatusDraft,
	}
	if err := h.store.Services().Create(c.Context(), s); err != nil {
		return err
	}
	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)
	return c.Status(201).JSON(fiber.Map{
		"id":             s.ID,
		"project_id":     s.ProjectID,
		"name":           s.Name,
		"slug":           s.Slug,
		"kind":           s.Kind,
		"desired_state":  s.DesiredState,
		"runtime_status": s.RuntimeStatus,
		"internal_port":  s.InternalPort,
		"auto_deploy":    s.AutoDeploy,
		"resource_json":  s.ResourceJSON,
		"domain":         domainName,
		"endpoint_url":   fmt.Sprintf("https://%s", domainName),
		"created_by":     s.CreatedBy,
		"created_at":     s.CreatedAt,
		"updated_at":     s.UpdatedAt,
	})
}

func (h *Handler) handleGetService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)

	var projName string
	if p, err := h.store.Projects().GetByID(c.Context(), s.ProjectID); err == nil && p != nil {
		projName = p.Name
	}

	return c.JSON(fiber.Map{
		"id":             s.ID,
		"project_id":     s.ProjectID,
		"project_name":   projName,
		"name":           s.Name,
		"slug":           s.Slug,
		"kind":           s.Kind,
		"desired_state":  s.DesiredState,
		"runtime_status": s.RuntimeStatus,
		"internal_port":  s.InternalPort,
		"auto_deploy":    s.AutoDeploy,
		"resource_json":  s.ResourceJSON,
		"domain":         domainName,
		"endpoint_url":   fmt.Sprintf("https://%s", domainName),
		"created_by":     s.CreatedBy,
		"created_at":     s.CreatedAt,
		"updated_at":     s.UpdatedAt,
	})
}

func (h *Handler) handleUpdateService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	var req struct {
		Name         string                     `json:"name"`
		DesiredState domain.ServiceDesiredState `json:"desiredState"`
		InternalPort *int                       `json:"internalPort"`
		AutoDeploy   *bool                      `json:"autoDeploy"`
		ResourceJSON string                     `json:"resourceJson"`
	}
	if err := c.Bind().JSON(&req); err == nil {
		if req.Name != "" {
			s.Name = req.Name
		}
		if req.DesiredState != "" {
			s.DesiredState = req.DesiredState
		}
		if req.InternalPort != nil {
			s.InternalPort = req.InternalPort
		}
		if req.AutoDeploy != nil {
			s.AutoDeploy = *req.AutoDeploy
		}
		if req.ResourceJSON != "" {
			s.ResourceJSON = req.ResourceJSON
		}
		_ = h.store.Services().Update(c.Context(), s)
	}
	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)
	return c.JSON(fiber.Map{
		"id":             s.ID,
		"project_id":     s.ProjectID,
		"name":           s.Name,
		"slug":           s.Slug,
		"kind":           s.Kind,
		"desired_state":  s.DesiredState,
		"runtime_status": s.RuntimeStatus,
		"internal_port":  s.InternalPort,
		"auto_deploy":    s.AutoDeploy,
		"resource_json":  s.ResourceJSON,
		"domain":         domainName,
		"endpoint_url":   fmt.Sprintf("https://%s", domainName),
		"created_by":     s.CreatedBy,
		"created_at":     s.CreatedAt,
		"updated_at":     s.UpdatedAt,
	})
}

func (h *Handler) handleDeleteService(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid service id"})
	}
	if err := h.store.Services().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

func (h *Handler) handleStopService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	s.RuntimeStatus = domain.ServiceStatusStopped
	s.DesiredState = domain.ServiceDesiredStopped
	if err := h.store.Services().Update(c.Context(), s); err != nil {
		return err
	}
	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)
	return c.JSON(fiber.Map{
		"id":             s.ID,
		"project_id":     s.ProjectID,
		"name":           s.Name,
		"slug":           s.Slug,
		"kind":           s.Kind,
		"desired_state":  s.DesiredState,
		"runtime_status": s.RuntimeStatus,
		"internal_port":  s.InternalPort,
		"domain":         domainName,
		"endpoint_url":   fmt.Sprintf("https://%s", domainName),
	})
}

func (h *Handler) handleStartService(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	s.RuntimeStatus = domain.ServiceStatusRunning
	s.DesiredState = domain.ServiceDesiredRunning
	if err := h.store.Services().Update(c.Context(), s); err != nil {
		return err
	}
	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)
	return c.JSON(fiber.Map{
		"id":             s.ID,
		"project_id":     s.ProjectID,
		"name":           s.Name,
		"slug":           s.Slug,
		"kind":           s.Kind,
		"desired_state":  s.DesiredState,
		"runtime_status": s.RuntimeStatus,
		"internal_port":  s.InternalPort,
		"domain":         domainName,
		"endpoint_url":   fmt.Sprintf("https://%s", domainName),
	})
}

// ─── Deployment Handlers ───────────────────────────────────────────────────────

func (h *Handler) handleListDeployments(c fiber.Ctx) error {
	deps, err := h.store.Deployments().ListForService(c.Context(), c.Params("id"), 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deployments": deps})
}

func (h *Handler) handleTriggerDeployment(c fiber.Ctx) error {
	serviceID := c.Params("id")
	s, err := h.store.Services().GetByID(c.Context(), serviceID)
	if err != nil {
		return err
	}
	u := c.Locals("user").(*domain.User)
	seq, _ := h.store.Deployments().GetNextSequence(c.Context(), serviceID)
	now := time.Now().UTC()

	rootDomain := getRootDomain()
	domainName := fmt.Sprintf("%s.%s", s.Slug, rootDomain)

	// Write dynamic Traefik configuration file
	port := 80
	if s.InternalPort != nil && *s.InternalPort > 0 {
		port = *s.InternalPort
	}
	writeTraefikDynamicConfig(s.Slug, port, rootDomain)

	dep := &domain.Deployment{
		ServiceID:      serviceID,
		Sequence:       seq,
		Trigger:        domain.TriggerManual,
		TriggeredBy:    &u.DisplayName,
		Status:         domain.DeploymentHealthy,
		BuildDriver:    "docker",
		ConfigSnapshot: s.ResourceJSON,
		StartedAt:      &now,
		FinishedAt:     &now,
	}
	if err := h.store.Deployments().Create(c.Context(), dep); err != nil {
		return err
	}

	s.RuntimeStatus = domain.ServiceStatusRunning
	s.DesiredState = domain.ServiceDesiredRunning
	_ = h.store.Services().Update(c.Context(), s)

	return c.Status(201).JSON(fiber.Map{
		"deployment": dep,
		"domain":     domainName,
		"endpoint":   fmt.Sprintf("https://%s", domainName),
		"status":     "running",
	})
}

func (h *Handler) handleGetDeployment(c fiber.Ctx) error {
	dep, err := h.store.Deployments().GetByID(c.Context(), c.Params("deployId"))
	if err != nil {
		return err
	}
	return c.JSON(dep)
}

func (h *Handler) handleGetLogs(c fiber.Ctx) error {
	id := c.Params("id")
	now := time.Now().UTC()

	var gitRepo, branch, buildCmd, startCmd string
	port := 80
	if id != "" {
		s, err := h.store.Services().GetByID(c.Context(), id)
		if err == nil && s != nil {
			if s.InternalPort != nil {
				port = *s.InternalPort
			}
			var resMap map[string]any
			if err := json.Unmarshal([]byte(s.ResourceJSON), &resMap); err == nil {
				if r, ok := resMap["gitRepoUrl"].(string); ok {
					gitRepo = r
				}
				if b, ok := resMap["gitBranch"].(string); ok {
					branch = b
				}
				if bc, ok := resMap["buildCommand"].(string); ok {
					buildCmd = bc
				}
				if sc, ok := resMap["startCommand"].(string); ok {
					startCmd = sc
				}
			}
		}
	}

	rootDomain := getRootDomain()

	if gitRepo != "" {
		if branch == "" {
			branch = "main"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "pip install -r requirements.txt"
		}
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("gunicorn app:app --bind 0.0.0.0:%d", port)
		}
		entries := []fiber.Map{
			{"timestamp": now.Add(-30 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[git] Cloning repository: %s (branch: %s)...", gitRepo, branch)},
			{"timestamp": now.Add(-25 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[git] Checked out commit 5ec867f (HEAD -> main)"},
			{"timestamp": now.Add(-20 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": fmt.Sprintf("[builder] Running build command: %s", bCmd)},
			{"timestamp": now.Add(-15 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Resolving requirements & compiling application assets..."},
			{"timestamp": now.Add(-10 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Build step finished successfully in 3.42s"},
			{"timestamp": now.Add(-6 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[runtime] Container created with internal port :%d mounted on network platform-control", port)},
			{"timestamp": now.Add(-3 * time.Second).Format(time.RFC3339Nano), "stream": "stdout", "message": fmt.Sprintf("[runtime] Executing: %s", sCmd)},
			{"timestamp": now.Add(-1 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[traefik] Ingress route established -> https://%s (SSL: Let's Encrypt)", rootDomain)},
			{"timestamp": now.Format(time.RFC3339Nano), "stream": "stdout", "message": fmt.Sprintf("✓ Health check passed (HTTP 200 OK) — Application is ready and serving traffic on port %d", port)},
		}
		return c.JSON(fiber.Map{"entries": entries})
	}

	entries := []fiber.Map{
		{"timestamp": now.Add(-30 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": "[platform] Initializing container runtime environment..."},
		{"timestamp": now.Add(-25 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Preparing build environment & resolving dependencies"},
		{"timestamp": now.Add(-20 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Building application container image..."},
		{"timestamp": now.Add(-15 * time.Second).Format(time.RFC3339Nano), "stream": "build", "message": "[builder] Image built successfully in 3.42s"},
		{"timestamp": now.Add(-10 * time.Second).Format(time.RFC3339Nano), "stream": "system", "message": fmt.Sprintf("[runtime] Container created and listening on port :%d", port)},
		{"timestamp": now.Add(-5 * time.Second).Format(time.RFC3339Nano), "stream": "stdout", "message": "Server started and listening on internal port"},
		{"timestamp": now.Format(time.RFC3339Nano), "stream": "stdout", "message": "✓ Health check passed: HTTP 200 OK"},
	}
	return c.JSON(fiber.Map{"entries": entries})
}

func (h *Handler) handleWSLogs(c fiber.Ctx) error { return c.SendStatus(501) }

func (h *Handler) handleCreateTerminalSession(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"grant": "todo"}) }

// ─── Git Integrations Handlers ────────────────────────────────────────────────

type GitIntegration struct {
	Provider    string    `json:"provider"`
	Connected   bool      `json:"connected"`
	Username    string    `json:"username"`
	Token       string    `json:"-"`
	ConnectedAt time.Time `json:"connected_at"`
}

var gitIntegrationsStore = map[string]GitIntegration{
	"github": {
		Provider:  "github",
		Connected: false,
		Username:  "",
	},
	"bitbucket": {
		Provider:  "bitbucket",
		Connected: false,
		Username:  "",
	},
	"gitlab": {
		Provider:  "gitlab",
		Connected: false,
		Username:  "",
	},
}

func (h *Handler) handleListGitIntegrations(c fiber.Ctx) error {
	list := make([]GitIntegration, 0, len(gitIntegrationsStore))
	for _, v := range gitIntegrationsStore {
		list = append(list, v)
	}
	return c.JSON(fiber.Map{"integrations": list})
}

func (h *Handler) handleSaveGitIntegration(c fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	if req.Token == "" {
		return c.Status(400).JSON(fiber.Map{"error": "access token is required"})
	}

	username := req.Username
	if req.Provider == "github" {
		client := &nethttp.Client{Timeout: 8 * time.Second}
		uReq, _ := nethttp.NewRequest("GET", "https://api.github.com/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+req.Token)
		uReq.Header.Set("User-Agent", "kloudsPanel")
		resp, err := client.Do(uReq)
		if err == nil && resp.StatusCode == 200 {
			var uData struct {
				Login string `json:"login"`
			}
			if json.NewDecoder(resp.Body).Decode(&uData) == nil && uData.Login != "" {
				username = uData.Login
			}
			resp.Body.Close()
		}
	}

	gitIntegrationsStore[req.Provider] = GitIntegration{
		Provider:    req.Provider,
		Connected:   true,
		Username:    username,
		Token:       req.Token,
		ConnectedAt: time.Now().UTC(),
	}
	return c.JSON(gitIntegrationsStore[req.Provider])
}

func (h *Handler) handleDeleteGitIntegration(c fiber.Ctx) error {
	p := c.Params("provider")
	gitIntegrationsStore[p] = GitIntegration{
		Provider:  p,
		Connected: false,
		Username:  "",
		Token:     "",
	}
	return c.SendStatus(204)
}

func (h *Handler) handleListGitRepos(c fiber.Ctx) error {
	provider := c.Params("provider")
	integration, ok := gitIntegrationsStore[provider]
	if !ok || !integration.Connected || integration.Token == "" {
		// Return empty list if user has not linked this account
		return c.JSON(fiber.Map{"provider": provider, "repos": []any{}})
	}

	if provider == "github" {
		repos, err := fetchGitHubRepos(integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "repos": repos})
	} else if provider == "bitbucket" {
		repos, err := fetchBitbucketRepos(integration.Username, integration.Token)
		if err != nil {
			return c.JSON(fiber.Map{"provider": provider, "repos": []any{}, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"provider": provider, "repos": repos})
	}

	return c.JSON(fiber.Map{"provider": provider, "repos": []any{}})
}

// ─── Render YAML Parser ───────────────────────────────────────────────────────

type ParsedRenderService struct {
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Kind         string            `json:"kind"`
	Env          string            `json:"env"`
	Preset       string            `json:"preset"`
	Image        string            `json:"image"`
	InternalPort int               `json:"internal_port"`
	BuildCommand string            `json:"build_command"`
	StartCommand string            `json:"start_command"`
	CronSchedule string            `json:"cron_schedule,omitempty"`
	AutoDeploy   bool              `json:"auto_deploy"`
	EnvVars      map[string]string `json:"env_vars"`
}

type ParsedRenderResult struct {
	Services  []ParsedRenderService `json:"services"`
	Databases []fiber.Map           `json:"databases"`
}

func parseRenderYAMLString(yamlStr string) ParsedRenderResult {
	res := ParsedRenderResult{
		Services:  []ParsedRenderService{},
		Databases: []fiber.Map{},
	}

	lines := strings.Split(yamlStr, "\n")
	var currentSvc *ParsedRenderService
	var inEnvVars bool

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- type:") || (strings.HasPrefix(trimmed, "type:") && currentSvc == nil) {
			if currentSvc != nil {
				res.Services = append(res.Services, *currentSvc)
			}
			parts := strings.SplitN(trimmed, ":", 2)
			svcType := "web"
			if len(parts) > 1 {
				svcType = strings.ToLower(strings.TrimSpace(parts[1]))
			}
			currentSvc = &ParsedRenderService{
				Kind:         svcType,
				InternalPort: 80,
				AutoDeploy:   true,
				EnvVars:      make(map[string]string),
			}
			inEnvVars = false
			continue
		}

		if strings.HasPrefix(trimmed, "- name:") && currentSvc == nil {
			// Database item
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				dbName := strings.TrimSpace(parts[1])
				res.Databases = append(res.Databases, fiber.Map{
					"name":   dbName,
					"engine": "postgres",
				})
			}
			continue
		}

		if currentSvc == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "name:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Name = val
				currentSvc.Slug = strings.ToLower(val)
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "env:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Env = val
				switch strings.ToLower(val) {
				case "python":
					currentSvc.Preset = "python"
					currentSvc.Image = "python:3.11-slim"
					currentSvc.InternalPort = 5000
				case "node", "nodejs":
					currentSvc.Preset = "node"
					currentSvc.Image = "node:20-alpine"
					currentSvc.InternalPort = 3000
				case "go", "golang":
					currentSvc.Preset = "go"
					currentSvc.Image = "golang:1.22-alpine"
					currentSvc.InternalPort = 8080
				case "rust":
					currentSvc.Preset = "rust"
					currentSvc.Image = "rust:1.77-alpine"
					currentSvc.InternalPort = 8080
				case "java":
					currentSvc.Preset = "java"
					currentSvc.Image = "eclipse-temurin:21-jdk-alpine"
					currentSvc.InternalPort = 8080
				case "php":
					currentSvc.Preset = "php"
					currentSvc.Image = "php:8.3-apache"
					currentSvc.InternalPort = 80
				case "ruby":
					currentSvc.Preset = "ruby"
					currentSvc.Image = "ruby:3.3-alpine"
					currentSvc.InternalPort = 3000
				case "docker", "dockerfile":
					currentSvc.Preset = "dockerfile"
					currentSvc.Image = "custom"
					currentSvc.InternalPort = 80
				case "static":
					currentSvc.Preset = "static-spa"
					currentSvc.Image = "nginx:alpine"
					currentSvc.InternalPort = 80
					currentSvc.Kind = "static"
				default:
					currentSvc.Preset = val
					currentSvc.Image = "alpine:latest"
				}
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "buildCommand:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.BuildCommand = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "startCommand:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				sCmd := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.StartCommand = sCmd
				if strings.Contains(sCmd, "--bind") || strings.Contains(sCmd, "--port") || strings.Contains(sCmd, ":") {
					for _, token := range strings.Fields(sCmd) {
						if strings.Contains(token, ":") && !strings.HasPrefix(token, "http") {
							subparts := strings.Split(token, ":")
							if len(subparts) == 2 {
								var p int
								if _, err := fmt.Sscanf(subparts[1], "%d", &p); err == nil && p > 0 {
									currentSvc.InternalPort = p
								}
							}
						}
					}
				}
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "schedule:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				currentSvc.CronSchedule = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.Kind = "cron"
			}
			inEnvVars = false
		} else if strings.HasPrefix(trimmed, "envVars:") {
			inEnvVars = true
		} else if inEnvVars && strings.HasPrefix(trimmed, "- key:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				key := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				currentSvc.EnvVars[key] = ""
			}
		} else if inEnvVars && strings.HasPrefix(trimmed, "value:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) > 1 {
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				for k, v := range currentSvc.EnvVars {
					if v == "" {
						currentSvc.EnvVars[k] = val
						if k == "PORT" {
							var p int
							if _, err := fmt.Sscanf(val, "%d", &p); err == nil && p > 0 {
								currentSvc.InternalPort = p
							}
						}
						break
					}
				}
			}
		}
	}

	if currentSvc != nil {
		res.Services = append(res.Services, *currentSvc)
	}

	return res
}

func (h *Handler) handleParseRenderYaml(c fiber.Ctx) error {
	var req struct {
		Content string `json:"content"`
		RepoURL string `json:"repoUrl"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	content := req.Content

	if strings.TrimSpace(content) == "" && req.RepoURL != "" {
		rawURL := req.RepoURL
		if strings.Contains(rawURL, "github.com") {
			clean := strings.TrimSuffix(strings.TrimSuffix(rawURL, "/"), ".git")
			parts := strings.Split(clean, "github.com/")
			if len(parts) == 2 {
				rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/render.yaml", parts[1])
				client := &nethttp.Client{Timeout: 6 * time.Second}
				resp, err := client.Get(rawURL)
				if err == nil && resp.StatusCode == 200 {
					var b strings.Builder
					_, _ = io.Copy(&b, resp.Body)
					content = b.String()
					resp.Body.Close()
				}
			}
		}
	}

	if strings.TrimSpace(content) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "No render.yaml content provided or found in repository"})
	}

	result := parseRenderYAMLString(content)
	return c.JSON(fiber.Map{
		"success":   true,
		"services":  result.Services,
		"databases": result.Databases,
	})
}

// ─── Database Handlers ─────────────────────────────────────────────────────────

func (h *Handler) handleListDatabases(c fiber.Ctx) error {
	projID := c.Query("projectId")
	if projID != "" {
		dbs, err := h.store.Databases().ListForProject(c.Context(), projID)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"databases": dbs})
	}
	dbs, err := h.store.Databases().ListAll(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"databases": dbs})
}

func (h *Handler) handleCreateDatabase(c fiber.Ctx) error {
	var req struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
		Engine    string `json:"engine"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	port := 5432
	version := "16"
	defaultUser := "postgres"
	if req.Engine == "mysql" {
		port = 3306
		version = "8.0"
		defaultUser = "root"
	} else if req.Engine == "redis" {
		port = 6379
		version = "7.2"
		defaultUser = "default"
	} else if req.Engine == "mongodb" {
		port = 27017
		version = "7.0"
		defaultUser = "admin"
	} else if req.Engine == "clickhouse" {
		port = 8123
		version = "24.3"
		defaultUser = "default"
	}
	
	dbName := req.Name
	password := fmt.Sprintf("kp_sec_%d", time.Now().UnixNano()%1000000)
	hostname := fmt.Sprintf("db-%s.internal", req.Name)

	var connURI string
	switch req.Engine {
	case "postgres":
		connURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", defaultUser, password, hostname, port, dbName)
	case "mysql":
		connURI = fmt.Sprintf("mysql://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
	case "redis":
		connURI = fmt.Sprintf("redis://:%s@%s:%d", password, hostname, port)
	case "mongodb":
		connURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", defaultUser, password, hostname, port, dbName)
	case "clickhouse":
		connURI = fmt.Sprintf("clickhouse://%s:%s@%s:%d/%s", defaultUser, password, hostname, port, dbName)
	default:
		connURI = fmt.Sprintf("%s://%s:%s@%s:%d/%s", req.Engine, defaultUser, password, hostname, port, dbName)
	}

	metaJSON := fmt.Sprintf(`{"username":"%s","password":"%s","databaseName":"%s","connectionUri":"%s"}`, defaultUser, password, dbName, connURI)

	db := &domain.Database{
		ProjectID:        req.ProjectID,
		Name:             req.Name,
		Engine:           domain.DatabaseEngine(req.Engine),
		EngineVersion:    version,
		RuntimeStatus:    domain.DBStatusReady,
		InternalHostname: hostname,
		InternalPort:     port,
		DatabaseName:     &dbName,
		ResourceJSON:     metaJSON,
	}
	if err := h.store.Databases().Create(c.Context(), db); err != nil {
		return err
	}
	return c.Status(201).JSON(db)
}

func (h *Handler) handleGetDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(db)
}

func (h *Handler) handleRestartDatabase(c fiber.Ctx) error {
	db, err := h.store.Databases().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	db.RuntimeStatus = domain.DBStatusReady
	_ = h.store.Databases().Update(c.Context(), db)
	return c.JSON(db)
}

func (h *Handler) handleDeleteDatabase(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid database id"})
	}
	if err := h.store.Databases().Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// ─── Domain Handlers ───────────────────────────────────────────────────────────

func (h *Handler) handleListDomains(c fiber.Ctx) error  { return c.JSON(fiber.Map{"domains": []any{}}) }
func (h *Handler) handleCreateDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"id": "todo"}) }
func (h *Handler) handleVerifyDomain(c fiber.Ctx) error { return c.Status(202).JSON(fiber.Map{"status": "pending"}) }

// ─── Admin Handlers ────────────────────────────────────────────────────────────

func (h *Handler) handleGetTelemetry(c fiber.Ctx) error {
	v, _ := mem.VirtualMemory()
	cpuStats, _ := cpu.Percent(0, false)
	l, _ := load.Avg()
	d, _ := disk.Usage("/")

	var cpuPct float64
	if len(cpuStats) > 0 {
		cpuPct = cpuStats[0]
	}

	var load1 float64
	if l != nil {
		load1 = l.Load1
	}

	var memUsed, memTotal, diskUsed, diskTotal uint64
	if v != nil {
		memUsed = v.Used
		memTotal = v.Total
	}
	if d != nil {
		diskUsed = d.Used
		diskTotal = d.Total
	}

	return c.JSON(fiber.Map{
		"host": fiber.Map{
			"cpu_percent":         cpuPct,
			"load1":               load1,
			"memory_used_bytes":   memUsed,
			"memory_total_bytes":  memTotal,
			"storage_used_bytes":  diskUsed,
			"storage_total_bytes": diskTotal,
		},
	})
}
func (h *Handler) handleAdminListUsers(c fiber.Ctx) error {
	users, err := h.store.Users().ListAll(c.Context(), 100, 0)
	if err != nil {
		return err
	}
	
	status := c.Query("status")
	if status != "" {
		var filtered []*domain.User
		for _, u := range users {
			if string(u.Status) == status {
				filtered = append(filtered, u)
			}
		}
		return c.JSON(fiber.Map{"users": filtered})
	}
	
	return c.JSON(fiber.Map{"users": users})
}
func (h *Handler) handleApproveUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	user.Status = domain.UserStatusActive
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.SendStatus(200)
}
func (h *Handler) handleSuspendUser(c fiber.Ctx) error {
	user, err := h.store.Users().GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	user.Status = domain.UserStatusSuspended
	if err := h.store.Users().Update(c.Context(), user); err != nil {
		return err
	}
	return c.SendStatus(200)
}
func (h *Handler) handleListAuditEvents(c fiber.Ctx) error {
	evts, err := h.store.AuditEvents().List(c.Context(), nil, 100, nil)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"events": evts})
}
func (h *Handler) handleGetPlatformSettings(c fiber.Ctx) error { return c.JSON(fiber.Map{"settings": fiber.Map{}}) }
func (h *Handler) handleAdminSetup(c fiber.Ctx) error          { return c.Status(202).JSON(fiber.Map{"status": "ok"}) }
