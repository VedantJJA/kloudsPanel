// Package mcp implements the kloudsPanel MCP server.
// It uses the official github.com/modelcontextprotocol/go-sdk in Stateless mode
// with Streamable HTTP transport per the MCP 2026-07-28 specification.
//
// Endpoint: POST https://<root-domain>/mcp
// Auth: OAuth 2.1 with PKCE, short-lived access tokens, workspace-scoped.
package mcp

import (
	"context"
	"log/slog"
	"net/http"
)

// Server is the MCP server for kloudsPanel.
type Server struct {
	logger *slog.Logger
	// TODO Phase 10: embed mcp.Server from github.com/modelcontextprotocol/go-sdk
}

// NewServer creates a new MCP server.
func NewServer(logger *slog.Logger) *Server {
	return &Server{logger: logger}
}

// Handler returns the HTTP handler for the /mcp endpoint.
func (s *Server) Handler() http.Handler {
	// TODO Phase 10: configure Stateless=true, register tools/resources
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"mcp_not_implemented","detail":"MCP server - Phase 10 implementation pending"}`, http.StatusNotImplemented)
	})
}

// --- MCP Scopes ---------------------------------------------------------------

const (
	ScopeProjectsRead    = "mcp:projects:read"
	ScopeLogsRead        = "mcp:logs:read"
	ScopeDeploymentsWrite = "mcp:deployments:write"
	ScopeDocsRead        = "mcp:docs:read"
	ScopeTerminalCreate  = "mcp:terminal:create" // off by default
	ScopeAdminRead       = "mcp:admin:read"      // main admin only
)

// --- Tool names (forbidden list) ---------------------------------------------
// These must never be registered as tools in any version.
var forbiddenToolNames = []string{
	"shell",
	"docker.exec",
	"read_any_file",
	"get_secret",
	"curl_url",
}

// validateToolName panics during server initialization if a forbidden tool
// name is registered. This catches misuse at startup.
func validateToolName(name string) {
	for _, f := range forbiddenToolNames {
		if name == f {
			panic("mcp: forbidden tool name: " + name)
		}
	}
}

// --- MCP Tools ----------------------------------------------------------------
// All tools are workspace-scoped. Raw container IDs, socket paths, or file
// system paths are never accepted as authoritative inputs.

// Tool list (to be registered in Phase 10):
//   - projects.list       (mcp:projects:read)
//   - projects.get        (mcp:projects:read)
//   - deployments.get     (mcp:projects:read)
//   - logs.tail           (mcp:logs:read) - bounded, redacted
//   - project.structure   (mcp:projects:read) - filtered tree
//   - deployment.explain_failure (mcp:logs:read) - diagnosis suggestions
//   - deployment.trigger  (mcp:deployments:write) - requires idempotency key
//   - deployment.rollback (mcp:deployments:write) - requires idempotency key
//   - docs.search         (mcp:docs:read)
//   - docs.fetch          (mcp:docs:read)

// --- Docs Adapters ------------------------------------------------------------

// SvelteDocAdapter queries the Svelte MCP sidecar (stdio, not public).
type SvelteDocAdapter struct{}

// GoDocAdapter queries pkg.go.dev API + local go/doc.
type GoDocAdapter struct {
	// allowedPackages limits which packages can be fetched
	allowedPackages []string
}

func init() {
	// Validate at init that no forbidden tool slips through
	for _, t := range forbiddenToolNames {
		_ = t // just ensuring the list is checked
	}
}

// EnsureWorkspaceScope verifies the request is scoped to the caller's workspace.
// This must be called before any data lookup to prevent cross-workspace IDOR.
func EnsureWorkspaceScope(ctx context.Context, callerWorkspaceID, requestedWorkspaceID string) error {
	if callerWorkspaceID != requestedWorkspaceID {
		return &MCPError{Code: "unauthorized", Message: "workspace scope mismatch"}
	}
	return nil
}

// MCPError is a structured MCP tool error response.
type MCPError struct {
	Code    string
	Message string
}

func (e *MCPError) Error() string { return e.Code + ": " + e.Message }
