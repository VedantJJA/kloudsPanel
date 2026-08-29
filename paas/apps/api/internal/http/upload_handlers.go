package http

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

// getUploadsBaseDir returns the base directory for storing uploaded project archives.
func getUploadsBaseDir() string {
	if dir := os.Getenv("UPLOADS_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0755)
		return dir
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath != "" {
		dir := filepath.Join(filepath.Dir(dbPath), "uploads")
		_ = os.MkdirAll(dir, 0755)
		return dir
	}
	dir := filepath.Join(".", "data", "uploads")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

// handleUploadProjectSource handles uploading source code (.zip or .tar.gz) for a project.
func (h *Handler) handleUploadProjectSource(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Project ID is required"})
	}

	ctx := context.Background()
	project, err := h.store.Projects().GetByID(ctx, projectID)
	if err != nil || project == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		file, err = c.FormFile("archive")
	}
	if err != nil {
		file, err = c.FormFile("source")
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use field name 'file', 'archive', or 'source'."})
	}

	// Max 500MB
	if file.Size > 500*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Archive too large (maximum size is 500MB)"})
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".zip" && ext != ".gz" && ext != ".tgz" && ext != ".tar" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid archive format. Supported formats: .zip, .tar.gz, .tgz, .tar"})
	}

	targetDir := filepath.Join(getUploadsBaseDir(), "projects", project.ID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to create upload directory: %v", err)})
	}

	saveName := "source.zip"
	if ext == ".gz" || ext == ".tgz" || strings.HasSuffix(strings.ToLower(file.Filename), ".tar.gz") {
		saveName = "source.tar.gz"
	} else if ext == ".tar" {
		saveName = "source.tar"
	}

	targetPath := filepath.Join(targetDir, saveName)
	if err := c.SaveFile(file, targetPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
	}

	project.SourceKind = domain.SourceKindUpload
	project.UpdatedAt = time.Now().UTC()
	_ = h.store.Projects().Update(ctx, project)

	h.log.Info("[upload] Project source code uploaded successfully",
		"projectID", project.ID,
		"filename", file.Filename,
		"size", file.Size,
		"savedTo", targetPath,
	)

	return c.JSON(fiber.Map{
		"message":   "Project source code uploaded successfully",
		"projectId": project.ID,
		"filename":  file.Filename,
		"size":      file.Size,
		"savedPath": targetPath,
	})
}

// handleUploadServiceSource handles uploading source code directly for a specific service.
func (h *Handler) handleUploadServiceSource(c fiber.Ctx) error {
	serviceID := c.Params("id")
	if serviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Service ID is required"})
	}

	ctx := context.Background()
	service, err := h.store.Services().GetByID(ctx, serviceID)
	if err != nil || service == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Service not found"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		file, err = c.FormFile("archive")
	}
	if err != nil {
		file, err = c.FormFile("source")
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded. Use field name 'file', 'archive', or 'source'."})
	}

	if file.Size > 500*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Archive too large (maximum size is 500MB)"})
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".zip" && ext != ".gz" && ext != ".tgz" && ext != ".tar" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid archive format. Supported formats: .zip, .tar.gz, .tgz, .tar"})
	}

	targetDir := filepath.Join(getUploadsBaseDir(), "services", service.ID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to create upload directory: %v", err)})
	}

	saveName := "source.zip"
	if ext == ".gz" || ext == ".tgz" || strings.HasSuffix(strings.ToLower(file.Filename), ".tar.gz") {
		saveName = "source.tar.gz"
	} else if ext == ".tar" {
		saveName = "source.tar"
	}

	targetPath := filepath.Join(targetDir, saveName)
	if err := c.SaveFile(file, targetPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
	}

	h.log.Info("[upload] Service source code uploaded successfully",
		"serviceID", service.ID,
		"filename", file.Filename,
		"size", file.Size,
		"savedTo", targetPath,
	)

	// If deploy=true query or form parameter is set, trigger deployment immediately
	autoDeploy := c.Query("deploy") == "true" || c.FormValue("deploy") == "true"
	var dep *domain.Deployment
	if autoDeploy {
		var userID *string
		if u, ok := c.Locals("user").(*domain.User); ok && u != nil {
			userID = &u.ID
		}

		seq, _ := h.store.Deployments().GetNextSequence(c.Context(), service.ID)
		now := time.Now().UTC()
		dep = &domain.Deployment{
			ServiceID:      service.ID,
			Sequence:       seq,
			Trigger:        domain.TriggerManual,
			TriggeredBy:    userID,
			Status:         domain.DeploymentBuilding,
			BuildDriver:    "docker",
			ConfigSnapshot: service.ResourceJSON,
			StartedAt:      &now,
		}
		if err := h.store.Deployments().Create(c.Context(), dep); err == nil {
			service.RuntimeStatus = domain.ServiceStatusBuilding
			_ = h.store.Services().Update(c.Context(), service)
			rootDomain := os.Getenv("ROOT_DOMAIN")
			go h.executeDeployment(service, dep, rootDomain)
		}
	}

	resp := fiber.Map{
		"message":   "Service source code uploaded successfully",
		"serviceId": service.ID,
		"filename":  file.Filename,
		"size":      file.Size,
		"savedPath": targetPath,
	}
	if dep != nil {
		resp["deploymentId"] = dep.ID
		resp["deploymentStatus"] = dep.Status
	}
	return c.JSON(resp)
}

// detectPresetFromFiles inspects a directory for standard manifests and returns the matching preset.
func detectPresetFromFiles(dir string) string {
	checks := []struct {
		file   string
		preset string
	}{
		{"package.json", "node"},
		{"requirements.txt", "python"},
		{"pyproject.toml", "python"},
		{"Pipfile", "python"},
		{"main.py", "python"},
		{"app.py", "python"},
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"build.gradle.kts", "java"},
		{"Gemfile", "ruby"},
		{"composer.json", "php"},
		{"mix.exs", "elixir"},
		{"deno.json", "deno"},
		{"bun.lockb", "bun"},
		{"Dockerfile", "dockerfile"},
		{"index.html", "static-spa"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.preset
		}
	}
	return "node"
}

// handleParseUploadedArchive inspects an uploaded archive to auto-detect blueprints or frameworks.
func (h *Handler) handleParseUploadedArchive(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		file, err = c.FormFile("archive")
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
	}

	tempDir, err := os.MkdirTemp("", "paas-upload-parse-*")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create temp directory"})
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, file.Filename)
	if err := c.SaveFile(file, tempFile); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save temp file"})
	}

	extractDir := filepath.Join(tempDir, "extracted")
	if err := ExtractArchive(tempFile, extractDir); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Failed to extract archive: %v", err)})
	}

	// Check for blueprint
	blueprintFile := filepath.Join(extractDir, "klouds.yaml")
	if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
		blueprintFile = filepath.Join(extractDir, "render.yaml")
	}
	if _, err := os.Stat(blueprintFile); os.IsNotExist(err) {
		blueprintFile = filepath.Join(extractDir, "render.yml")
	}

	if _, err := os.Stat(blueprintFile); err == nil {
		if yamlBytes, err := os.ReadFile(blueprintFile); err == nil {
			parsed := parseRenderYAMLString(string(yamlBytes))
			return c.JSON(fiber.Map{
				"hasBlueprint": true,
				"blueprint":    string(yamlBytes),
				"services":     parsed.Services,
				"databases":    parsed.Databases,
			})
		}
	}

	// No blueprint, resolve runtime preset from files
	detected := detectPresetFromFiles(extractDir)
	return c.JSON(fiber.Map{
		"hasBlueprint": false,
		"preset":       detected,
		"version":      resolveRuntimeVersion(detected, extractDir, "").Version,
	})
}
