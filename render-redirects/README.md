# render-redirects

A Go implementation of Render's static-site "Redirects and Rewrites"
feature (Source / Destination / Action, `*` wildcards, `:placeholder`
segments, first-match-wins ordering, resource-exists precedence), built to
run as a small service between **Traefik** and **nginx**.

```
client -> Traefik (TLS, host routing) -> redirector (this binary) -> nginx (serves static files)
```

The redirector decides *what should happen* for a request — serve as-is,
301 redirect, or transparently rewrite to another path/URL — using exactly
the algorithm described in Render's docs. Actually serving files is left to
nginx, which is better at that (gzip, caching headers, range requests,
etc.) than a hand-rolled Go file server.

## Layout

```
internal/rules/       core matching engine — no HTTP dependency, fully unit tested
internal/middleware/  net/http glue: existence checks, 301s, reverse-proxying to nginx
internal/config/      loads rules from JSON
cmd/server/           standalone binary wiring it all together
```

`internal/rules` is the part worth reading first — it's a pure function of
"rules + path in" -> "what to do out" and has no framework baked in, so you
could also embed it in a Traefik plugin, an AWS Lambda@Edge-style handler,
or anywhere else.

## Rule syntax

Same as the Dashboard:

| Source                  | Destination              | Example                                   |
|--------------------------|---------------------------|--------------------------------------------|
| `/home`                  | `/`                        | `/home` -> `/`                              |
| `/blog/index.html`       | `/blog`                    | `/blog/index.html` -> `/blog`               |
| `/web-host`               | `https://render.com`      | `/web-host` -> `https://render.com`         |
| `/*`                      | `/blog/*`                 | `/path1/path2` -> `/blog/path1/path2`       |
| `/*`                      | `/index.html`             | any path -> `/index.html` (SPA fallback)    |
| `/blog/posts/:postid`    | `/blog/:postid`           | `/blog/posts/my-post` -> `/blog/my-post`    |
| `/updates/:month/:year`  | `/changelog/:year/:month`| `/updates/03/2024` -> `/changelog/2024/03`  |

Rules:

- `*` in a Source captures everything from that point on (can span `/`).
- `:name` in a Source captures a single path segment (stops at `/`).
- In a Destination, `*` is replaced by the wildcard capture, `:name` by the
  matching named capture.
- A bare `/` as a Source is rejected — you can't rule the domain root.
- Rules are evaluated **top to bottom**; the first `Source` that matches a
  path wins. Put more specific rules above more general/wildcard ones.
- **A real file always wins.** If a resource already exists at the
  incoming path, no rule is applied at all — this is checked before rules
  are evaluated, exactly like the Dashboard.

`Action` is `"redirect"` (301, browser URL changes, one quote of the client
round-trip naturally re-runs this whole process against the new path) or
`"rewrite"` (content is served from Destination, URL stays the same; if
Destination doesn't exist either and matches another rule, that's resolved
server-side, with a bounded chain length to guard against rule cycles).

## Config format

```json
{
  "rules": [
    { "source": "/home", "destination": "/", "action": "redirect" },
    { "source": "/*", "destination": "/index.html", "action": "rewrite" }
  ]
}
```

See `config.example.json` for one that mirrors every example above.

## Running it

Environment variables (all optional, shown with defaults):

| Var             | Default                        | Meaning                                    |
|------------------|----------------------------------|----------------------------------------------|
| `RULES_CONFIG`   | `/etc/redirector/rules.json`   | path to the rules JSON file                 |
| `STATIC_ROOT`    | `/var/www/html`                | static files volume, shared read-only with nginx — used for the exists-check |
| `UPSTREAM_URL`   | `http://nginx:80`              | nginx origin real requests get proxied to  |
| `LISTEN_ADDR`    | `:8080`                        | address the redirector listens on          |

```bash
go build ./...
go test ./...
```

`docker-compose.yml` wires up Traefik -> redirector -> nginx end to end;
`Dockerfile` builds a small static binary for the redirector. Traefik only
routes to the redirector (see the `traefik.*` labels) — nginx isn't
directly reachable from outside, so rules can't be bypassed.

### Why check the filesystem instead of asking nginx?

`middleware.FileSystemExister` stats the same volume nginx serves from
directly — it's a single `os.Stat`, no network hop, and it's what the
`STATIC_ROOT` volume mount in `docker-compose.yml` is for. If your
redirector and nginx don't share a filesystem (different hosts, no shared
volume), swap in `middleware.UpstreamExister`, which does the same check
via a `HEAD` request to nginx instead — same interface (`rules.Exister`),
one line to change in `cmd/server/main.go`.

## Extending

- **Hot-reloading rules**: `rules.NewSet` is cheap to rebuild; swap
  `Handler.cfg.Rules` behind a `sync/atomic` pointer and reload on file
  change or `SIGHUP` if you need this without a restart.
- **Traefik plugin instead of a sidecar**: `internal/rules` has zero
  external dependencies and no `net/http` import, so it should drop
  straight into a [Traefik Yaegi
  plugin](https://plugins.traefik.io/create) if you'd rather avoid the
  extra hop than run it as a standalone service — you'd reimplement
  `internal/middleware`'s `ServeHTTP` inside the plugin's `New`/`ServeHTTP`
  shape and swap in Traefik's own upstream forwarding for
  `httputil.ReverseProxy`.
- **YAML config**: swap `encoding/json` in `internal/config` for
  `gopkg.in/yaml.v3` if you'd rather author rules as YAML — the `Rule`
  struct's field names already match either way (add `yaml:"..."` tags
  alongside the existing `json:"..."` ones).

## A note on nginx alone

Nginx's own `rewrite`/`return`/`try_files` directives can express a good
chunk of this by hand. The value of this Go layer is having one
declarative rule format (JSON, could come from a database or an admin API)
that's unit-testable and behaves identically to Render's documented
semantics — including the "real file always wins" precedence and
placeholder substitution — rather than hand-maintaining regex rewrites in
nginx config.

## Testing note

This was written and its matching/substitution logic verified against
every example in Render's docs using an equivalent Python prototype (this
sandbox has no network access, so the Go toolchain itself couldn't be
installed to run `go build`/`go test` directly here). The `_test.go` files
are real Go tests — run `go test ./...` locally to confirm; nothing here
depends on anything beyond the standard library.
