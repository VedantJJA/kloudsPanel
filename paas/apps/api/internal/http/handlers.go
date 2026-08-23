package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"

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
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		if adminPassword == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			adminPassword = hex.EncodeToString(b)[:16]
			h.log.Info("=============================================================")
			h.log.Info("FIRST BOOT: Auto-generated admin password", "password", adminPassword)
			h.log.Info("Save this password now. It will not be shown again.")
			h.log.Info("You can set ADMIN_PASSWORD env var to control this on next fresh install.")
			h.log.Info("=============================================================")
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		adminEmail := os.Getenv("ADMIN_EMAIL")
		if adminEmail == "" {
			adminEmail = "admin@klouds.local"
		}
		admin := &domain.User{
			Email:        adminEmail,
			DisplayName:  "Admin",
			PasswordHash: string(hash),
			Status:       domain.UserStatusActive,
			PlatformRole: domain.PlatformRoleMainAdmin,
		}
		_ = h.store.Users().Create(ctx, admin)
	}
}
