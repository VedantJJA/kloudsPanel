package http

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/yourorg/klouds/api/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type SessionInfo struct {
	UserID    string
	ExpiresAt time.Time
}

var (
	sessionMu     sync.RWMutex
	sessionTokens = make(map[string]SessionInfo)
)

// ─── Auth Middleware ──────────────────────────────────────────────────────────

func (h *Handler) requireSession(c fiber.Ctx) error {
	token := c.Cookies("session_token")
	if token == "" {
		token = c.Cookies("klouds_session")
	}
	if token == "" {
		token = c.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
	}
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	sessionMu.RLock()
	info, exists := sessionTokens[token]
	sessionMu.RUnlock()

	var userID string
	if exists && info.ExpiresAt.After(time.Now()) {
		userID = info.UserID
	} else {
		// Persistent session lookup across container restarts
		dbUserID, exp, err := h.store.AuthSessions().GetByToken(c.Context(), token)
		if err != nil || exp.Before(time.Now()) {
			return c.Status(401).JSON(fiber.Map{"error": "session expired or invalid"})
		}
		userID = dbUserID
		sessionMu.Lock()
		sessionTokens[token] = SessionInfo{
			UserID:    dbUserID,
			ExpiresAt: exp,
		}
		sessionMu.Unlock()
	}

	user, err := h.store.Users().GetByID(c.Context(), userID)
	if err != nil || user.Status != domain.UserStatusActive {
		return c.Status(401).JSON(fiber.Map{"error": "user not found or inactive"})
	}

	c.Locals("user", user)
	return c.Next()
}

func (h *Handler) requireMainAdmin(c fiber.Ctx) error {
	u, ok := c.Locals("user").(*domain.User)
	if !ok || u.PlatformRole != domain.PlatformRoleMainAdmin {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden: main_admin required"})
	}
	return c.Next()
}

// ─── Auth Handlers ────────────────────────────────────────────────────────────

func (h *Handler) handleSignup(c fiber.Ctx) error {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	userStatus := domain.UserStatusPending
	if isAutoApproveEnabled() {
		userStatus = domain.UserStatusActive
	}

	user := &domain.User{
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: string(hash),
		Status:       userStatus,
		PlatformRole: domain.PlatformRoleUser,
	}

	if err := h.store.Users().Create(c.Context(), user); err != nil {
		return err
	}

	if userStatus == domain.UserStatusActive {
		return c.Status(201).JSON(fiber.Map{
			"message": "Account created and activated. You can now log in.",
			"status":  "active",
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "User registered. An administrator must approve your account before you can log in.",
		"status":  "pending",
	})
}

func (h *Handler) handleLogin(c fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return err
	}
	user, err := h.store.Users().GetByEmail(c.Context(), req.Email)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if user.Status == domain.UserStatusPending {
		return c.Status(403).JSON(fiber.Map{"error": "Account pending approval by an administrator."})
	}
	if user.Status == domain.UserStatusSuspended {
		return c.Status(403).JSON(fiber.Map{"error": "Account suspended."})
	}

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	expires := time.Now().Add(30 * 24 * time.Hour)

	// Persist session to SQLite database
	_ = h.store.AuthSessions().Create(c.Context(), token, user.ID, token, expires)

	sessionMu.Lock()
	sessionTokens[token] = SessionInfo{
		UserID:    user.ID,
		ExpiresAt: expires,
	}
	sessionMu.Unlock()

	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   30 * 24 * 3600,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   false,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "klouds_session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   30 * 24 * 3600,
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   false,
	})

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":          user.ID,
			"email":       user.Email,
			"displayName": user.DisplayName,
			"isMainAdmin": user.PlatformRole == domain.PlatformRoleMainAdmin,
		},
		"token": token,
	})
}

func (h *Handler) handleLogout(c fiber.Ctx) error {
	token := c.Cookies("session_token")
	if token == "" {
		token = c.Cookies("klouds_session")
	}
	if token != "" {
		_ = h.store.AuthSessions().DeleteByToken(c.Context(), token)
		sessionMu.Lock()
		delete(sessionTokens, token)
		sessionMu.Unlock()
	}
	c.Cookie(&fiber.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		SameSite: "Lax",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "klouds_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		SameSite: "Lax",
	})
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *Handler) handleMe(c fiber.Ctx) error {
	u := c.Locals("user").(*domain.User)
	return c.JSON(fiber.Map{
		"id":          u.ID,
		"email":       u.Email,
		"displayName": u.DisplayName,
		"isMainAdmin": u.PlatformRole == domain.PlatformRoleMainAdmin,
		"status":      u.Status,
	})
}
