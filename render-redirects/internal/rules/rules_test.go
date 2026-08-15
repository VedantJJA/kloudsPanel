package rules

import "testing"

// TestDocExamples pins the exact examples from Render's docs so the
// matching/substitution logic can't silently drift from spec.
func TestDocExamples(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		destination string
		path        string
		want        string
	}{
		{"basic-home", "/home", "/", "/home", "/"},
		{"basic-blog-index", "/blog/index.html", "/blog", "/blog/index.html", "/blog"},
		{"basic-external", "/web-host", "https://render.com", "/web-host", "https://render.com"},
		{"wildcard-prefix", "/*", "/blog/*", "/path1/path2", "/blog/path1/path2"},
		{"wildcard-catchall", "/*", "/index.html", "/anything/at/all", "/index.html"},
		{"placeholder-single", "/blog/posts/:postid", "/blog/:postid", "/blog/posts/my-post", "/blog/my-post"},
		{"placeholder-multi", "/updates/:month/:year", "/changelog/:year/:month", "/updates/03/2024", "/changelog/2024/03"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set, err := NewSet([]Rule{{Source: tc.source, Destination: tc.destination, Action: ActionRedirect}})
			if err != nil {
				t.Fatalf("NewSet: %v", err)
			}
			_, dest, ok := set.FirstMatch(tc.path)
			if !ok {
				t.Fatalf("path %q did not match source %q", tc.path, tc.source)
			}
			if dest != tc.want {
				t.Errorf("got destination %q, want %q", dest, tc.want)
			}
		})
	}
}

func TestRootSourceRejected(t *testing.T) {
	_, err := NewSet([]Rule{{Source: "/", Destination: "/home", Action: ActionRedirect}})
	if err == nil {
		t.Fatal("expected an error compiling a rule whose source is the domain root")
	}
}

func TestSourceMustBeAbsolutePath(t *testing.T) {
	_, err := NewSet([]Rule{{Source: "blog", Destination: "/blog", Action: ActionRedirect}})
	if err == nil {
		t.Fatal("expected an error for a source that isn't an absolute path")
	}
}

func TestFirstMatchWins(t *testing.T) {
	set, err := NewSet([]Rule{
		{Source: "/blog/*", Destination: "/blog-specific", Action: ActionRedirect},
		{Source: "/*", Destination: "/catch-all", Action: ActionRedirect},
	})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}

	_, dest, ok := set.FirstMatch("/blog/hello")
	if !ok || dest != "/blog-specific" {
		t.Fatalf("got (%q, %v), want (/blog-specific, true)", dest, ok)
	}

	_, dest, ok = set.FirstMatch("/other")
	if !ok || dest != "/catch-all" {
		t.Fatalf("got (%q, %v), want (/catch-all, true)", dest, ok)
	}
}

func TestNoMatch(t *testing.T) {
	set, err := NewSet([]Rule{{Source: "/blog/*", Destination: "/x", Action: ActionRedirect}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	if _, _, ok := set.FirstMatch("/docs/intro"); ok {
		t.Fatal("expected no match for an unrelated path")
	}
}

// --- Resolve() (exists-check + ordering + rewrite chaining) ---

type fakeExister map[string]bool

func (f fakeExister) Exists(path string) bool { return f[path] }

func TestResolveServesRealResourceBeforeRules(t *testing.T) {
	set, err := NewSet([]Rule{{Source: "/*", Destination: "/index.html", Action: ActionRewrite}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	exister := fakeExister{"/about.html": true}

	got := set.Resolve(exister, "/about.html")
	if got.Kind != ResultServe || got.Path != "/about.html" {
		t.Fatalf("got %+v, want ResultServe /about.html", got)
	}
}

func TestResolveRedirect(t *testing.T) {
	set, err := NewSet([]Rule{{Source: "/home", Destination: "/", Action: ActionRedirect}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	got := set.Resolve(fakeExister{}, "/home")
	if got.Kind != ResultRedirect || got.Location != "/" {
		t.Fatalf("got %+v, want ResultRedirect /", got)
	}
}

func TestResolveRewriteChaining(t *testing.T) {
	// /old -> /middle (rewrite), /middle doesn't exist but matches another
	// rewrite rule -> /final, which exists as a real resource.
	set, err := NewSet([]Rule{
		{Source: "/old", Destination: "/middle", Action: ActionRewrite},
		{Source: "/middle", Destination: "/final.html", Action: ActionRewrite},
	})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	exister := fakeExister{"/final.html": true}

	got := set.Resolve(exister, "/old")
	if got.Kind != ResultServe || got.Path != "/final.html" {
		t.Fatalf("got %+v, want ResultServe /final.html", got)
	}
}

func TestResolveSPAFallback(t *testing.T) {
	// Classic client-side-routing setup: anything not a real file rewrites
	// to /index.html.
	set, err := NewSet([]Rule{{Source: "/*", Destination: "/index.html", Action: ActionRewrite}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	exister := fakeExister{"/index.html": true, "/app.js": true}

	got := set.Resolve(exister, "/dashboard/settings")
	if got.Kind != ResultServe || got.Path != "/index.html" {
		t.Fatalf("got %+v, want ResultServe /index.html", got)
	}

	got = set.Resolve(exister, "/app.js")
	if got.Kind != ResultServe || got.Path != "/app.js" {
		t.Fatalf("real asset should be served directly, got %+v", got)
	}
}

func TestResolveRewriteLoopIsBounded(t *testing.T) {
	set, err := NewSet([]Rule{
		{Source: "/a", Destination: "/b", Action: ActionRewrite},
		{Source: "/b", Destination: "/a", Action: ActionRewrite},
	})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}

	got := set.Resolve(fakeExister{}, "/a")
	if got.Kind != ResultNotFound {
		t.Fatalf("expected a rewrite cycle to resolve to ResultNotFound, got %+v", got)
	}
}

func TestResolveRewriteToExternalURL(t *testing.T) {
	set, err := NewSet([]Rule{{Source: "/proxied/*", Destination: "https://example.com/*", Action: ActionRewrite}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}

	got := set.Resolve(fakeExister{}, "/proxied/api/things")
	if got.Kind != ResultProxyExternal || got.Location != "https://example.com/api/things" {
		t.Fatalf("got %+v, want ResultProxyExternal https://example.com/api/things", got)
	}
}

func TestResolveNotFound(t *testing.T) {
	set, err := NewSet([]Rule{{Source: "/blog/*", Destination: "/x", Action: ActionRedirect}})
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	got := set.Resolve(fakeExister{}, "/nope")
	if got.Kind != ResultNotFound {
		t.Fatalf("got %+v, want ResultNotFound", got)
	}
}
