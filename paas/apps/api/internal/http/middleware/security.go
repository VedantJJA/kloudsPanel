package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// DefaultCORS returns the standard CORS middleware configuration for kloudsPanel.
func DefaultCORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return true // Traefik handles edge TLS and security; API is internal
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-CSRF-Token", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           600,
	})
}
