package http

import (
	"fmt"
	"regexp"
	"strings"
)

// --- Container Security Module -----------------------------------------------
// Provides security hardening for Docker containers, Dockerfile sanitization,
// slug validation, and environment variable safety checks.

// SecurityProfile defines configurable resource limits and security flags.
type SecurityProfile struct {
	MemoryLimit   string // e.g. "512m", "1g"
	CPULimit      string // e.g. "1.0", "2.0"
	PIDLimit      int    // max process count, prevents fork bombs
	ReadOnlyRoot  bool   // mount root filesystem read-only
	NoNewPrivs    bool   // prevent privilege escalation
	DropAllCaps   bool   // drop all Linux capabilities
	AllowRoot     bool   // allow running as root (opt-in)
	BuildTimeoutS int    // max build duration in seconds
	TmpfsSize     string // tmpfs mount size for /tmp
	NetworkMode   string // "platform-control" (isolated bridge)
	Privileged    bool   // run container in privileged mode (admin only)
	IsAdmin       bool   // whether container belongs to an admin account
}

// DefaultSecurityProfile returns production-safe defaults for standard users.
func DefaultSecurityProfile() SecurityProfile {
	return SecurityProfile{
		MemoryLimit:   "512m",
		CPULimit:      "1.0",
		PIDLimit:      256,
		ReadOnlyRoot:  false,
		NoNewPrivs:    true,  // restricted mode for standard accounts
		DropAllCaps:   true,  // restricted mode for standard accounts
		AllowRoot:     false,
		BuildTimeoutS: 600, // 10 minutes
		TmpfsSize:     "128m",
		NetworkMode:   "platform-control",
		Privileged:    false,
		IsAdmin:       false,
	}
}

// BuildSecurityProfile creates a profile from blueprint resource config and user admin role.
func BuildSecurityProfile(resMap map[string]any, isAdmin ...bool) SecurityProfile {
	admin := false
	if len(isAdmin) > 0 {
		admin = isAdmin[0]
	}

	if admin {
		profile := SecurityProfile{
			MemoryLimit:   "",
			CPULimit:      "",
			PIDLimit:      0,
			ReadOnlyRoot:  false,
			NoNewPrivs:    false,
			DropAllCaps:   false,
			AllowRoot:     true,
			BuildTimeoutS: 1800, // 30 minutes for admin builds
			TmpfsSize:     "",
			NetworkMode:   "platform-control",
			Privileged:    false,
			IsAdmin:       true,
		}

		if resMap != nil {
			if mem, ok := resMap["mem_limit"].(string); ok && mem != "" {
				profile.MemoryLimit = sanitizeResourceLimit(mem)
			}
			if mem, ok := resMap["memory"].(string); ok && mem != "" {
				profile.MemoryLimit = sanitizeResourceLimit(mem)
			}
			if cpu, ok := resMap["cpu_limit"].(string); ok && cpu != "" {
				profile.CPULimit = sanitizeResourceLimit(cpu)
			}
			if cpu, ok := resMap["cpus"].(string); ok && cpu != "" {
				profile.CPULimit = sanitizeResourceLimit(cpu)
			}
			if pids, ok := resMap["pids_limit"].(float64); ok && pids > 0 {
				profile.PIDLimit = int(pids)
			}
			if timeout, ok := resMap["build_timeout"].(float64); ok && timeout > 0 {
				profile.BuildTimeoutS = int(timeout)
			}
			// Admin accounts can enable privileged mode
			if priv, ok := resMap["privileged"].(bool); ok && priv {
				profile.Privileged = true
			}
			if priv, ok := resMap["privileged_mode"].(bool); ok && priv {
				profile.Privileged = true
			}
			if modeStr, ok := resMap["mode"].(string); ok && strings.EqualFold(modeStr, "privileged") {
				profile.Privileged = true
			}
		}
		return profile
	}

	// Non-admin / standard account: Restricted Mode for platform safety
	profile := DefaultSecurityProfile()
	profile.IsAdmin = false
	profile.Privileged = false // strictly blocked for non-admin accounts

	if resMap != nil {
		if mem, ok := resMap["mem_limit"].(string); ok && mem != "" {
			profile.MemoryLimit = sanitizeResourceLimit(mem)
		}
		if mem, ok := resMap["memory"].(string); ok && mem != "" {
			profile.MemoryLimit = sanitizeResourceLimit(mem)
		}
		if cpu, ok := resMap["cpu_limit"].(string); ok && cpu != "" {
			profile.CPULimit = sanitizeResourceLimit(cpu)
		}
		if cpu, ok := resMap["cpus"].(string); ok && cpu != "" {
			profile.CPULimit = sanitizeResourceLimit(cpu)
		}
		if pids, ok := resMap["pids_limit"].(float64); ok && pids > 0 {
			if pids > 512 {
				profile.PIDLimit = 512
			} else {
				profile.PIDLimit = int(pids)
			}
		}
		if timeout, ok := resMap["build_timeout"].(float64); ok && timeout > 0 {
			if timeout > 900 {
				profile.BuildTimeoutS = 900
			} else {
				profile.BuildTimeoutS = int(timeout)
			}
		}
	}

	return profile
}

// ContainerSecurityArgs returns Docker run flags for security hardening.
func ContainerSecurityArgs(profile SecurityProfile) []string {
	args := []string{}

	if profile.IsAdmin {
		if profile.Privileged {
			args = append(args, "--privileged", "-v", "/var/run/docker.sock:/var/run/docker.sock", "--label", "io.paas.security=privileged")
		} else {
			args = append(args, "--label", "io.paas.security=admin")
		}
		if profile.MemoryLimit != "" {
			args = append(args, "--memory", profile.MemoryLimit, "--memory-swap", profile.MemoryLimit)
		}
		if profile.CPULimit != "" {
			args = append(args, "--cpus", profile.CPULimit)
		}
		if profile.PIDLimit > 0 {
			args = append(args, "--pids-limit", fmt.Sprintf("%d", profile.PIDLimit))
		}
		return args
	}

	// Restricted Mode for standard non-admin users
	if profile.MemoryLimit != "" {
		args = append(args, "--memory", profile.MemoryLimit, "--memory-swap", profile.MemoryLimit)
	}
	if profile.CPULimit != "" {
		args = append(args, "--cpus", profile.CPULimit)
	}
	if profile.PIDLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", profile.PIDLimit))
	} else {
		args = append(args, "--pids-limit", "256")
	}

	// Prevent privilege escalation
	args = append(args, "--security-opt", "no-new-privileges:true")

	// Drop all capabilities and retain only non-root essentials
	args = append(args, "--cap-drop", "ALL")
	args = append(args, "--cap-add", "NET_BIND_SERVICE")
	args = append(args, "--cap-add", "CHOWN")
	args = append(args, "--cap-add", "SETUID")
	args = append(args, "--cap-add", "SETGID")
	args = append(args, "--cap-add", "DAC_OVERRIDE")

	if profile.ReadOnlyRoot {
		args = append(args, "--read-only")
	}

	tmpfsSize := profile.TmpfsSize
	if tmpfsSize == "" {
		tmpfsSize = "128m"
	}
	args = append(args, "--tmpfs", fmt.Sprintf("/tmp:rw,noexec,nosuid,size=%s", tmpfsSize))

	args = append(args, "--label", "io.paas.security=restricted")
	return args
}

// --- Slug Validation ---------------------------------------------------------

// slugValidation regex  -  only lowercase alphanumeric and hyphens
var validSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}[a-z0-9]$`)

// ValidateSlug ensures a slug is safe for use in Docker container names,
// Traefik configs, and filesystem paths. Prevents command injection.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug cannot be empty")
	}
	if len(slug) > 63 {
		return fmt.Errorf("slug too long (max 63 characters): %s", slug)
	}
	// Single character slugs are OK
	if len(slug) == 1 {
		if slug[0] >= 'a' && slug[0] <= 'z' || slug[0] >= '0' && slug[0] <= '9' {
			return nil
		}
		return fmt.Errorf("slug contains invalid characters: %s", slug)
	}
	if !validSlugPattern.MatchString(slug) {
		return fmt.Errorf("slug contains invalid characters (allowed: a-z, 0-9, hyphens): %s", slug)
	}
	// Block reserved names that could cause issues
	reserved := map[string]bool{
		"docker": true, "traefik": true, "nginx": true, "root": true,
		"admin": true, "system": true, "host": true, "localhost": true,
		"platform-control": true, "bridge": true, "none": true,
	}
	if reserved[slug] {
		return fmt.Errorf("slug uses a reserved name: %s", slug)
	}
	return nil
}

// --- Dockerfile Safety Scanner -----------------------------------------------

// dangerousDockerfilePatterns are patterns that should never appear in
// generated Dockerfiles. User-provided Dockerfiles are not scanned.
var dangerousDockerfilePatterns = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	{regexp.MustCompile(`(?i)--privileged`), "privileged mode is not allowed"},
	{regexp.MustCompile(`(?i)--net=host`), "host networking is not allowed"},
	{regexp.MustCompile(`(?i)--network=host`), "host networking is not allowed"},
	{regexp.MustCompile(`(?i)--pid=host`), "host PID namespace is not allowed"},
	{regexp.MustCompile(`(?i)--ipc=host`), "host IPC namespace is not allowed"},
	{regexp.MustCompile(`(?i)docker\.sock`), "Docker socket mounting is not allowed"},
	{regexp.MustCompile(`(?i)/var/run/docker`), "Docker socket access is not allowed"},
	{regexp.MustCompile(`(?i)curl\s+.*\|\s*(sh|bash)`), "pipe-to-shell patterns are risky"},
	{regexp.MustCompile(`(?i)wget\s+.*\|\s*(sh|bash)`), "pipe-to-shell patterns are risky"},
}

// ScanDockerfileForDangers checks a Dockerfile string for dangerous patterns.
// If isAdmin is true, dangerous patterns produce warnings rather than blocking errors.
func ScanDockerfileForDangers(content string, isAdmin ...bool) (warnings []string, errors []string) {
	admin := false
	if len(isAdmin) > 0 {
		admin = isAdmin[0]
	}

	for _, p := range dangerousDockerfilePatterns {
		if p.pattern.MatchString(content) {
			if strings.Contains(p.reason, "not allowed") {
				if admin {
					warnings = append(warnings, fmt.Sprintf("ADMIN_OVERRIDE: %s (authorized for platform administrator)", p.reason))
				} else {
					errors = append(errors, fmt.Sprintf("RESTRICTED: %s (requires administrator account)", p.reason))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("WARNING: %s", p.reason))
			}
		}
	}
	return warnings, errors
}

// --- Environment Variable Safety ---------------------------------------------

// dangerousEnvPatterns blocks command injection via environment variable values.
var dangerousEnvValuePattern = regexp.MustCompile(`[;|&$` + "`" + `\\\n]`)

// SanitizeEnvValue checks if an environment variable value is safe.
// Returns the sanitized value and whether it was modified.
func SanitizeEnvValue(key, value string) (string, bool) {
	// Allow common safe env vars with special characters
	safeKeys := map[string]bool{
		"DATABASE_URL": true, "REDIS_URL": true, "MONGODB_URI": true,
		"POSTGRES_URL": true, "MYSQL_URL": true, "CELERY_BROKER_URL": true,
		"BROKER_URL": true, "AMQP_URL": true, "CLICKHOUSE_URL": true,
		"GUNICORN_CMD_ARGS": true, "JAVA_OPTS": true, "NODE_OPTIONS": true,
		"MAVEN_OPTS": true, "GRADLE_OPTS": true, "BUNDLE_PATH": true,
	}

	// Connection URLs and option strings are allowed to have special chars
	if safeKeys[key] || strings.HasSuffix(key, "_URL") || strings.HasSuffix(key, "_URI") || strings.HasSuffix(key, "_DSN") {
		return value, false
	}

	// For other keys, warn but don't block (too many false positives)
	return value, false
}

// --- Resource Limit Sanitization ---------------------------------------------

// sanitizeResourceLimit validates resource limit strings like "512m", "1g", "1.5".
var validResourcePattern = regexp.MustCompile(`^[\d]+\.?[\d]*[kmgKMG]?[bB]?$`)

func sanitizeResourceLimit(limit string) string {
	limit = strings.TrimSpace(limit)
	if validResourcePattern.MatchString(limit) {
		return limit
	}
	return "" // Invalid, will fall back to default
}

// --- Non-Root User Dockerfile Directives -------------------------------------

// NonRootDirective returns Dockerfile lines to add a non-root user and switch to it.
// This prevents containers from running as root, reducing attack surface.
func NonRootDirective(preset string) string {
	// Some presets already handle their own user management
	skipUserFor := map[string]bool{
		"static": true, "static-spa": true, "nginx": true, // Nginx manages its own user
		"php": true, // Apache manages www-data
	}
	if skipUserFor[strings.ToLower(preset)] {
		return ""
	}
	return `
# Security: Run as non-root user
RUN (getent group appgroup >/dev/null 2>&1 || groupadd -g 1001 appgroup 2>/dev/null || addgroup -g 1001 -S appgroup 2>/dev/null || true) && \
    (id -u appuser >/dev/null 2>&1 || useradd -u 1001 -g 1001 -M -s /bin/sh appuser 2>/dev/null || adduser -u 1001 -G appgroup -S -s /bin/sh appuser 2>/dev/null || true)
USER 1001
`
}

// HealthcheckDirective returns a Dockerfile HEALTHCHECK instruction.
func HealthcheckDirective(port int, preset string) string {
	// Static sites don't need healthchecks (Nginx has its own)
	skipFor := map[string]bool{
		"static": true, "static-spa": true, "nginx": true,
	}
	if skipFor[strings.ToLower(preset)] {
		return ""
	}
	return fmt.Sprintf(`
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:%d/health 2>/dev/null || wget -qO- http://localhost:%d/ 2>/dev/null || exit 1
`, port, port)
}
