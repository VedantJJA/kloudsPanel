// Package handlers provides response formatters and utility helpers for HTTP handlers.
package handlers

import (
	"github.com/gofiber/fiber/v3"
)

// Response represents a standard JSON API envelope.
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// JSONOk writes a 200 OK JSON response with payload data.
func JSONOk(c fiber.Ctx, data any) error {
	return c.JSON(data)
}

// JSONCreated writes a 201 Created JSON response with payload data.
func JSONCreated(c fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(data)
}

// JSONError writes a structured error response with status code.
func JSONError(c fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error":  message,
		"status": status,
	})
}
