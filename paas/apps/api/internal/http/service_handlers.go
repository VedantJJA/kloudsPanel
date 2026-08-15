package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

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

func cleanupServiceResources(slug string) {
	if slug == "" {
		return
	}
	containerName := fmt.Sprintf("paas-svc-%s", slug)
	// Stop & remove container
	_ = exec.Command("docker", "rm", "-f", containerName).Run()
	// Remove service images (current and tagged versions)
	_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-svc-%s:latest", slug)).Run()
	_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-app-%s:latest", slug)).Run()
	_ = exec.Command("docker", "rmi", "-f", fmt.Sprintf("paas-svc-%s", slug)).Run()
	// Remove volume if created
	_ = exec.Command("docker", "volume", "rm", "-f", fmt.Sprintf("paas-svc-data-%s", slug)).Run()
	// Remove Traefik dynamic routing
	removeTraefikDynamicConfig(slug)
	// Prune dangling build cache
	go func() {
		_ = exec.Command("docker", "builder", "prune", "-f", "--filter", "until=2h").Run()
		_ = exec.Command("docker", "image", "prune", "-f").Run()
	}()
}

func (h *Handler) handleDeleteService(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" || id == "undefined" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid service id"})
	}
	s, err := h.store.Services().GetByID(c.Context(), id)
	if err == nil && s != nil {
		cleanupServiceResources(s.Slug)
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

func (h *Handler) handleListServiceDomains(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}
	rootDomain := getRootDomain()
	primaryDomain := fmt.Sprintf("%s.%s", s.Slug, rootDomain)

	var resMap map[string]any
	_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	if resMap == nil {
		resMap = make(map[string]any)
	}

	var customDomains []string
	if cds, ok := resMap["customDomains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" {
				customDomains = append(customDomains, str)
			}
		}
	} else if cds, ok := resMap["custom_domains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" {
				customDomains = append(customDomains, str)
			}
		}
	}

	type DomainItem struct {
		Domain    string `json:"domain"`
		IsPrimary bool   `json:"isPrimary"`
		SSLStatus string `json:"sslStatus"`
		Target    string `json:"target"`
	}

	items := []DomainItem{
		{
			Domain:    primaryDomain,
			IsPrimary: true,
			SSLStatus: "active",
			Target:    primaryDomain,
		},
	}

	for _, cd := range customDomains {
		items = append(items, DomainItem{
			Domain:    cd,
			IsPrimary: false,
			SSLStatus: "active",
			Target:    primaryDomain,
		})
	}

	return c.JSON(fiber.Map{
		"primaryDomain": primaryDomain,
		"domains":       items,
	})
}

func (h *Handler) syncServiceTraefikConfig(ctx context.Context, s *domain.Service, resMap map[string]any) {
	if s == nil {
		return
	}
	if (resMap == nil || len(resMap) == 0) && s.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}

	port := 80
	if s.InternalPort != nil && *s.InternalPort > 0 {
		port = *s.InternalPort
	}
	rootDomain := getRootDomain()

	var customDomains []string
	if cds, ok := resMap["customDomains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" {
				customDomains = append(customDomains, str)
			}
		}
	} else if cdsStr, ok := resMap["customDomains"].([]string); ok {
		for _, str := range cdsStr {
			if str != "" {
				customDomains = append(customDomains, str)
			}
		}
	}

	var routes []ServiceRouteItem
	if rts, ok := resMap["routes"].([]any); ok {
		for _, rt := range rts {
			if rMap, ok := rt.(map[string]any); ok {
				src, _ := rMap["source"].(string)
				dst, _ := rMap["destination"].(string)
				t, _ := rMap["type"].(string)
				if src != "" && dst != "" {
					routes = append(routes, ServiceRouteItem{
						Source:      src,
						Destination: dst,
						Type:        t,
					})
				}
			}
		}
	} else if rtsItems, ok := resMap["routes"].([]ServiceRouteItem); ok {
		routes = append(routes, rtsItems...)
	}

	var siblingStaticSlugs []string
	if s.ProjectID != "" {
		if siblings, err := h.store.Services().ListForProject(ctx, s.ProjectID); err == nil {
			for _, sib := range siblings {
				if sib.ID != s.ID && (sib.Kind == domain.ServiceKindStatic || strings.Contains(strings.ToLower(sib.Name), "front") || strings.Contains(strings.ToLower(sib.Name), "web") || strings.Contains(strings.ToLower(sib.Name), "ui") || strings.Contains(strings.ToLower(sib.Name), "client")) {
					siblingStaticSlugs = append(siblingStaticSlugs, sib.Slug)
				}
			}
		}
	}

	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(s.Slug, port, rootDomain, customDomains, routes, siblingStaticSlugs)
}

func (h *Handler) handleAddServiceDomain(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}

	domainToAdd := strings.TrimSpace(strings.ToLower(req.Domain))
	if domainToAdd == "" {
		return c.Status(400).JSON(fiber.Map{"error": "domain is required"})
	}

	var resMap map[string]any
	_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	if resMap == nil {
		resMap = make(map[string]any)
	}

	var customDomains []string
	if cds, ok := resMap["customDomains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" {
				customDomains = append(customDomains, str)
			}
		}
	}

	exists := false
	for _, cd := range customDomains {
		if strings.EqualFold(cd, domainToAdd) {
			exists = true
			break
		}
	}
	if !exists {
		customDomains = append(customDomains, domainToAdd)
	}

	resMap["customDomains"] = customDomains
	updatedJSON, _ := json.Marshal(resMap)
	s.ResourceJSON = string(updatedJSON)
	_ = h.store.Services().Update(c.Context(), s)

	h.syncServiceTraefikConfig(c.Context(), s, nil)

	return h.handleListServiceDomains(c)
}

func (h *Handler) handleDeleteServiceDomain(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}

	domainToDelete := strings.TrimSpace(strings.ToLower(c.Params("domain")))

	var resMap map[string]any
	_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	if resMap == nil {
		resMap = make(map[string]any)
	}

	var customDomains []string
	if cds, ok := resMap["customDomains"].([]any); ok {
		for _, cd := range cds {
			if str, ok := cd.(string); ok && str != "" && !strings.EqualFold(str, domainToDelete) {
				customDomains = append(customDomains, str)
			}
		}
	}

	resMap["customDomains"] = customDomains
	updatedJSON, _ := json.Marshal(resMap)
	s.ResourceJSON = string(updatedJSON)
	_ = h.store.Services().Update(c.Context(), s)

	h.syncServiceTraefikConfig(c.Context(), s, nil)

	return h.handleListServiceDomains(c)
}

// ─── Service Redirect and Rewrite Rules ──────────────────────────────────────

type ServiceRouteItem struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func (h *Handler) handleGetServiceRoutes(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}
	var resMap map[string]any
	if s.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}
	routes, _ := resMap["routes"].([]any)
	if routes == nil {
		routes = []any{}
	}
	return c.JSON(fiber.Map{"routes": routes})
}

func (h *Handler) handleUpdateServiceRoutes(c fiber.Ctx) error {
	s, err := h.store.Services().GetByID(c.Context(), c.Params("id"))
	if err != nil || s == nil {
		return c.Status(404).JSON(fiber.Map{"error": "service not found"})
	}
	var req struct {
		Routes []ServiceRouteItem `json:"routes"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	var resMap map[string]any
	if s.ResourceJSON != "" {
		_ = json.Unmarshal([]byte(s.ResourceJSON), &resMap)
	}
	if resMap == nil {
		resMap = make(map[string]any)
	}
	resMap["routes"] = req.Routes

	updatedJSON, _ := json.Marshal(resMap)
	s.ResourceJSON = string(updatedJSON)
	if err := h.store.Services().Update(c.Context(), s); err != nil {
		return err
	}

	h.syncServiceTraefikConfig(c.Context(), s, nil)

	return c.JSON(fiber.Map{
		"message": "Redirect and rewrite rules saved successfully",
		"routes":  req.Routes,
	})
}
