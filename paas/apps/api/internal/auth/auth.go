// Package auth implements password hashing, secure sessions, CSRF protection,
// and RBAC policy checks.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	sessionBytes   = 32
	csrfBytes      = 32
	SessionMaxAge  = 30 * 24 * time.Hour
	CSRFHeaderName = "X-CSRF-Token"
)

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(plaintext string) (string, error) {
	if len(plaintext) < 8 {
		return "", errors.New("password must be at least 8 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(plaintext, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}

// GenerateSessionToken creates a cryptographically random session token.
// Returns the raw token (set in cookie) and its SHA-256 hash (stored in DB).
func GenerateSessionToken() (token, tokenHash string, err error) {
	b := make([]byte, sessionBytes)
	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(b)
	tokenHash = hashString(token)
	return token, tokenHash, nil
}

// GenerateCSRFSecret creates a CSRF secret and its hash.
func GenerateCSRFSecret() (secret, secretHash string, err error) {
	b := make([]byte, csrfBytes)
	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	secret = hex.EncodeToString(b)
	secretHash = hashString(secret)
	return secret, secretHash, nil
}

// HashToken computes SHA-256 of a token for storage.
func HashToken(token string) string {
	return hashString(token)
}

// ConstantTimeCompare performs a constant-time string comparison.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// HashIP hashes an IP address for privacy-preserving audit logs.
func HashIP(ip string) string {
	return hashString("ip:" + ip)
}

// ─── RBAC ─────────────────────────────────────────────────────────────────────

// Permission represents an action a user can perform.
type Permission string

const (
	PermViewService    Permission = "service:view"
	PermDeployService  Permission = "service:deploy"
	PermManageService  Permission = "service:manage"
	PermDeleteService  Permission = "service:delete"
	PermViewSecrets    Permission = "secrets:view"
	PermManageSecrets  Permission = "secrets:manage"
	PermViewDomain     Permission = "domain:view"
	PermManageDomain   Permission = "domain:manage"
	PermTerminal       Permission = "service:terminal"
	PermManageDatabase Permission = "database:manage"
	PermRevealCreds    Permission = "database:reveal_credentials"
	PermAdminRead      Permission = "admin:read"
	PermAdminManage    Permission = "admin:manage"
)

// RolePermissions maps workspace roles to their allowed permissions.
var RolePermissions = map[string][]Permission{
	"owner": {
		PermViewService, PermDeployService, PermManageService, PermDeleteService,
		PermViewSecrets, PermManageSecrets, PermViewDomain, PermManageDomain,
		PermTerminal, PermManageDatabase, PermRevealCreds,
	},
	"admin": {
		PermViewService, PermDeployService, PermManageService,
		PermViewSecrets, PermManageSecrets, PermViewDomain, PermManageDomain,
		PermTerminal, PermManageDatabase, PermRevealCreds,
	},
	"developer": {
		PermViewService, PermDeployService, PermViewDomain, PermTerminal,
	},
	"viewer": {
		PermViewService, PermViewDomain,
	},
}

// HasPermission checks if a workspace role grants the requested permission.
func HasPermission(role string, perm Permission) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// ErrUnauthenticated is returned when no valid session is found.
var ErrUnauthenticated = fmt.Errorf("unauthenticated")

// ErrForbidden is returned when the user lacks required permission.
var ErrForbidden = fmt.Errorf("forbidden")
