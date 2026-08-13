// Package middleware provides HTTP middlewares for error handling, authentication, and security.
package middleware

import (
	"github.com/gofiber/fiber/v3"
)

// ProblemErrorHandler converts Go and Fiber errors into RFC 9457 problem+json responses.
func ProblemErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	title := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		title = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"type":      "about:blank",
		"title":     title,
		"status":    code,
		"detail":    err.Error(),
		"requestId": c.Locals("requestid"),
	})
}
