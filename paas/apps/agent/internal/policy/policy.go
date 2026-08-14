// Package policy implements the agent's security allow-lists and validation.
// The policy layer rejects any Docker operation not explicitly authorized.
package policy

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// ErrUnauthorized is returned when a request fails policy validation.
	ErrUnauthorized = errors.New("policy: unauthorized operation")

	// allowedRegistries is the list of permitted container image registries.
	// Source images outside these prefixes are rejected.
	allowedRegistries = []string{
		"ghcr.io/klouds/",             // platform images
		"docker.io/library/",          // official Docker Hub
		"registry-1.docker.io/library/",
		"index.docker.io/library/",
		"gcr.io/buildpacks/",          // CNB buildpack builders
		"paketobuildpacks/",
	}

	// prohibited capability names
	prohibitedCaps = []string{
		"SYS_ADMIN", "SYS_PTRACE", "SYS_MODULE", "SYS_RAWIO",
		"NET_ADMIN", "NET_RAW", "DAC_OVERRIDE", "SYS_CHROOT",
	}

	// validEnvKeyRe validates environment variable names (shell identifier rules)
	validEnvKeyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

// ValidateImageRef checks that an image reference uses an allowed registry.
func ValidateImageRef(imageRef string) error {
	lower := strings.ToLower(imageRef)
	for _, prefix := range allowedRegistries {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return nil
		}
	}
	return ErrUnauthorized
}

// ValidateLabel checks that a label key/value pair is safe.
// Rejects labels that could override platform security or Traefik configuration
// if provided by an untrusted source.
func ValidateLabel(key, value string) error {
	// Prevent tenant containers from setting platform labels
	if strings.HasPrefix(key, "io.paas.") {
		return ErrUnauthorized
	}
	// Prevent arbitrary Traefik router overrides from tenant payloads
	if strings.HasPrefix(key, "traefik.") && strings.Contains(key, "router") {
		return ErrUnauthorized
	}
	return nil
}

// ValidateCapabilities checks that no prohibited capability is requested.
func ValidateCapabilities(caps []string) error {
	for _, cap := range caps {
		for _, prohibited := range prohibitedCaps {
			if strings.EqualFold(cap, prohibited) {
				return ErrUnauthorized
			}
		}
	}
	return nil
}

// ValidateMount ensures a bind-mount path is within allowed boundaries.
// User services cannot mount the Docker socket, host root, or system paths.
func ValidateMount(hostPath string) error {
	clean := filepath.Clean(hostPath)
	prohibited := []string{
		"/var/run/docker.sock",
		"/var/run",
		"/etc",
		"/sys",
		"/proc",
		"/dev",
		"/boot",
		"/run/klouds",
	}
	for _, p := range prohibited {
		if clean == p || strings.HasPrefix(clean, p+"/") {
			return ErrUnauthorized
		}
	}
	return nil
}

// ValidateEnvKey validates an environment variable key name.
func ValidateEnvKey(key string) error {
	if !validEnvKeyRe.MatchString(key) {
		return errors.New("policy: invalid env key: " + key)
	}
	// Reject overriding system/platform-injected variables
	reserved := []string{"PATH", "HOME", "USER", "HOSTNAME"}
	for _, r := range reserved {
		if key == r {
			return errors.New("policy: reserved env key: " + key)
		}
	}
	return nil
}

// ValidatePort ensures a port number is within safe bounds for user services.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("policy: port out of range")
	}
	if port < 1024 {
		return errors.New("policy: privileged port (<1024) not allowed")
	}
	return nil
}

// MinResources defines the minimum resources any user container must have.
var MinResources = ResourceLimits{
	MemoryBytes:    64 * 1024 * 1024, // 64 MB
	CPUNanoCores:   100_000_000,      // 0.1 CPU
	PidsLimit:      50,
}

// ResourceLimits defines resource ceiling values for a container.
type ResourceLimits struct {
	MemoryBytes  int64
	CPUNanoCores int64
	PidsLimit    int64
}

// Validate ensures resource limits are above the minimum.
func (r ResourceLimits) Validate() error {
	if r.MemoryBytes > 0 && r.MemoryBytes < MinResources.MemoryBytes {
		return errors.New("policy: memory limit below minimum 64MB")
	}
	if r.PidsLimit > 0 && r.PidsLimit < MinResources.PidsLimit {
		return errors.New("policy: pids limit below minimum 50")
	}
	return nil
}
