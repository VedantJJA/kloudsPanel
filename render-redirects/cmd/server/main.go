// Command server runs the redirect/rewrite engine as a standalone HTTP
// service, meant to sit between Traefik and nginx:
//
//	Traefik (TLS, host routing) -> server (this binary) -> nginx (static files)
//
// Configure it with environment variables:
//
//	RULES_CONFIG   path to the JSON rules file (default /etc/redirector/rules.json)
//	STATIC_ROOT    path to the static files volume shared with nginx (default /var/www/html)
//	UPSTREAM_URL   nginx origin to proxy real requests to (default http://nginx:80)
//	LISTEN_ADDR    address to listen on (default :8080)
package main

import (
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/example/render-redirects/internal/config"
	"github.com/example/render-redirects/internal/middleware"
	"github.com/example/render-redirects/internal/rules"
)

func main() {
	configPath := env("RULES_CONFIG", "/etc/redirector/rules.json")
	staticRoot := env("STATIC_ROOT", "/var/www/html")
	upstream := env("UPSTREAM_URL", "http://nginx:80")
	addr := env("LISTEN_ADDR", ":8080")

	ruleDefs, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("loading rules: %v", err)
	}

	ruleSet, err := rules.NewSet(ruleDefs)
	if err != nil {
		log.Fatalf("compiling rules: %v", err)
	}

	upstreamURL, err := url.Parse(upstream)
	if err != nil {
		log.Fatalf("invalid UPSTREAM_URL %q: %v", upstream, err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	handler := middleware.New(middleware.Config{
		Rules:    ruleSet,
		Exister:  middleware.FileSystemExister{Root: staticRoot},
		Upstream: upstreamURL,
		Logger:   logger,
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("redirector listening on %s (rules=%d, static_root=%s, upstream=%s)",
		addr, len(ruleDefs), staticRoot, upstream)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
