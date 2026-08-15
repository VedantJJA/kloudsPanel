// Package rules implements the redirect/rewrite rule matching behavior
// described in Render's "Static Site Redirects and Rewrites" docs:
//
//   - Each rule has a Source (path, may contain "*" wildcards and
//     ":placeholder" segments), a Destination (path or absolute URL), and
//     an Action (redirect or rewrite).
//   - If a real resource exists at the incoming path, it's served as-is —
//     rules never shadow real files.
//   - Otherwise, rules are evaluated top to bottom; the first Source that
//     matches wins.
//   - A "redirect" sends the client a 301 to the Destination. A "rewrite"
//     serves the Destination's content transparently, without changing the
//     URL the client sees — and if the Destination itself doesn't exist and
//     matches another rule, that process repeats.
//
// The package has no HTTP dependency; see the sibling middleware package
// for the net/http glue.
package rules

import (
	"fmt"
	"strings"
)

// Action is the effect a matched rule has on a request.
type Action string

const (
	// ActionRedirect instructs the client to switch URLs via a 301.
	ActionRedirect Action = "redirect"
	// ActionRewrite serves the Destination's content at the original URL.
	ActionRewrite Action = "rewrite"
)

// Rule is a single redirect/rewrite rule, matching the shape configured in
// the Render Dashboard (and this package's JSON config format).
type Rule struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Action      Action `json:"action"`

	pattern *pattern
}

// Compile validates the rule and pre-compiles its Source pattern. It's
// called automatically by NewSet; exported so callers can validate a single
// rule (e.g. in a config editor) without building a full Set.
func (r *Rule) Compile() error {
	switch Action(strings.ToLower(string(r.Action))) {
	case ActionRedirect:
		r.Action = ActionRedirect
	case ActionRewrite:
		r.Action = ActionRewrite
	default:
		return fmt.Errorf("rule %q: action must be %q or %q, got %q", r.Source, ActionRedirect, ActionRewrite, r.Action)
	}
	if r.Destination == "" {
		return fmt.Errorf("rule %q: destination is required", r.Source)
	}

	p, err := compileSource(r.Source)
	if err != nil {
		return err
	}
	r.pattern = p
	return nil
}

// match reports whether path matches the rule's Source, returning the
// resolved Destination (with placeholders/wildcards substituted).
func (r *Rule) match(path string) (dest string, ok bool) {
	placeholders, wildcards, matched := r.pattern.match(path)
	if !matched {
		return "", false
	}
	return substituteDestination(r.Destination, placeholders, wildcards), true
}

// Set is an ordered, compiled collection of rules.
type Set struct {
	rules []Rule
}

// NewSet compiles rules into a Set, preserving their order (matching is
// first-match-wins, top to bottom, exactly as in the Dashboard).
func NewSet(defs []Rule) (*Set, error) {
	compiled := make([]Rule, len(defs))
	for i, r := range defs {
		rc := r
		if err := rc.Compile(); err != nil {
			return nil, fmt.Errorf("rule %d: %w", i, err)
		}
		compiled[i] = rc
	}
	return &Set{rules: compiled}, nil
}

// FirstMatch returns the first rule (in list order) whose Source matches
// path, and its resolved destination.
func (s *Set) FirstMatch(path string) (rule *Rule, destination string, ok bool) {
	for i := range s.rules {
		if dest, matched := s.rules[i].match(path); matched {
			return &s.rules[i], dest, true
		}
	}
	return nil, "", false
}

// Exister reports whether a real static resource already exists at path.
// Resolve consults this before ever applying a rule, mirroring "Render
// does not apply redirect or rewrite rules to a path if a resource exists
// at that path."
type Exister interface {
	Exists(path string) bool
}

// ExisterFunc adapts a plain function to the Exister interface.
type ExisterFunc func(path string) bool

// Exists implements Exister.
func (f ExisterFunc) Exists(path string) bool { return f(path) }

// ResultKind describes what a caller should do with a Resolve outcome.
type ResultKind int

const (
	// ResultServe means: a real resource exists (or a rewrite chain
	// bottomed out at one) — serve it from Path.
	ResultServe ResultKind = iota
	// ResultRedirect means: send the client a 301 to Location.
	ResultRedirect
	// ResultProxyExternal means: a rewrite rule's destination is an
	// absolute URL — fetch it and stream it back transparently.
	ResultProxyExternal
	// ResultNotFound means: nothing matched and no resource exists.
	ResultNotFound
)

// Result is the outcome of resolving a single request path.
type Result struct {
	Kind     ResultKind
	Path     string // for ResultServe: the local path to serve
	Location string // for ResultRedirect / ResultProxyExternal: path or URL
}

// maxRewriteDepth bounds internal rewrite chaining so a cyclical set of
// rules (e.g. "/a" rewrites to "/b" which rewrites back to "/a") can't spin
// forever.
const maxRewriteDepth = 10

// Resolve implements the documented path-matching flowchart:
//
//  1. If a resource exists at path, serve it.
//  2. Otherwise, find the first rule whose Source matches path.
//     - No match: 404.
//     - Action = redirect: tell the client to go to Location. (The
//       browser's subsequent request to that path naturally repeats this
//       whole process — Resolve doesn't need to loop for redirects.)
//     - Action = rewrite: the Destination becomes the new path, and the
//       process repeats server-side, transparently to the client. If the
//       Destination is itself an absolute URL, it's proxied instead.
func (s *Set) Resolve(exister Exister, path string) Result {
	seen := make(map[string]bool, maxRewriteDepth+1)

	for depth := 0; depth <= maxRewriteDepth; depth++ {
		if seen[path] {
			// Rewrite loop (e.g. two rules rewriting to each other).
			return Result{Kind: ResultNotFound}
		}
		seen[path] = true

		if exister != nil && exister.Exists(path) {
			return Result{Kind: ResultServe, Path: path}
		}

		rule, dest, matched := s.FirstMatch(path)
		if !matched {
			return Result{Kind: ResultNotFound}
		}

		switch rule.Action {
		case ActionRedirect:
			return Result{Kind: ResultRedirect, Location: dest}

		case ActionRewrite:
			if IsAbsoluteURL(dest) {
				return Result{Kind: ResultProxyExternal, Location: dest}
			}
			path = dest
			continue

		default:
			// Unreachable: Compile() rejects any other Action.
			return Result{Kind: ResultNotFound}
		}
	}

	return Result{Kind: ResultNotFound}
}
