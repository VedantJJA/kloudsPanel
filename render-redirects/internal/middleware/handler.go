// Package middleware adapts the rules package to net/http. It's meant to
// run as a small reverse proxy in front of nginx:
//
//	Traefik (TLS, routing) -> this handler -> nginx (serves static files)
//
// The handler decides redirect/rewrite/404 the same way Render's static
// site hosting does, then either issues a 301 itself or forwards the
// (possibly rewritten) request to nginx, which does the actual file
// serving, compression, caching headers, etc.
package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/example/render-redirects/internal/rules"
)

// Config wires the pieces Handler needs.
type Config struct {
	// Rules is the compiled rule set to evaluate for every request.
	Rules *rules.Set
	// Exister decides whether a real resource already exists at a given
	// path, so rules never shadow real files.
	Exister rules.Exister
	// Upstream is the nginx origin that actually serves static content.
	Upstream *url.URL
	// Logger receives one structured line per request. If nil, logging is
	// skipped.
	Logger *slog.Logger
}

// Handler is an http.Handler implementing Render's redirect/rewrite flow.
type Handler struct {
	cfg   Config
	proxy *httputil.ReverseProxy
}

// New builds a Handler ready to be mounted as an http.Handler (e.g. passed
// to http.ListenAndServe, or wrapped by other middleware).
func New(cfg Config) *Handler {
	return &Handler{
		cfg:   cfg,
		proxy: httputil.NewSingleHostReverseProxy(cfg.Upstream),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result := h.cfg.Rules.Resolve(h.cfg.Exister, r.URL.Path)

	h.log(r, result)

	switch result.Kind {
	case rules.ResultServe:
		h.serveUpstream(w, r, result.Path)

	case rules.ResultRedirect:
		location := withPreservedQuery(result.Location, r.URL.RawQuery)
		http.Redirect(w, r, location, http.StatusMovedPermanently)

	case rules.ResultProxyExternal:
		h.serveExternal(w, r, result.Location)

	default: // rules.ResultNotFound
		http.NotFound(w, r)
	}
}

// serveUpstream forwards the request to nginx with the path rewritten
// server-side to servePath. The client's original URL never changes.
func (h *Handler) serveUpstream(w http.ResponseWriter, r *http.Request, servePath string) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = servePath
	h.proxy.ServeHTTP(w, r2)
}

// serveExternal fetches an absolute URL named by a rewrite rule and streams
// it back, so the client still sees the original URL.
func (h *Handler) serveExternal(w http.ResponseWriter, r *http.Request, target string) {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		http.Error(w, "misconfigured rewrite destination", http.StatusBadGateway)
		return
	}

	p := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: u.Scheme, Host: u.Host})
	r2 := r.Clone(r.Context())
	r2.URL.Path = u.Path
	r2.URL.RawQuery = u.RawQuery
	r2.Host = u.Host
	p.ServeHTTP(w, r2)
}

func (h *Handler) log(r *http.Request, result rules.Result) {
	if h.cfg.Logger == nil {
		return
	}
	h.cfg.Logger.Info("resolved request",
		"method", r.Method,
		"path", r.URL.Path,
		"kind", resultKindName(result.Kind),
		"target", firstNonEmpty(result.Path, result.Location),
	)
}

func resultKindName(k rules.ResultKind) string {
	switch k {
	case rules.ResultServe:
		return "serve"
	case rules.ResultRedirect:
		return "redirect"
	case rules.ResultProxyExternal:
		return "proxy_external"
	default:
		return "not_found"
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// withPreservedQuery appends the original request's query string to a
// redirect location, unless the rule's own destination already specifies
// one.
func withPreservedQuery(location, rawQuery string) string {
	if rawQuery == "" || strings.Contains(location, "?") {
		return location
	}
	return location + "?" + rawQuery
}
