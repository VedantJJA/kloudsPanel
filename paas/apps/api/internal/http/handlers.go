package http

import (
	"context"
	"log/slog"

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
		_ = h.store.Users().Create(ctx, admin)
	}
}
