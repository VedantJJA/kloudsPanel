package http

import (
	"fmt"
	"os/exec"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

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
	s, err := h.store.Services().GetByID(c.Context(), id)
	if err == nil && s != nil {
		containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
		removeTraefikDynamicConfig(s.Slug)
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
	containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
	_ = exec.Command("docker", "stop", containerName).Run()

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
	containerName := fmt.Sprintf("paas-svc-%s", s.Slug)
	_ = exec.Command("docker", "start", containerName).Run()

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
