package http

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
)

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
