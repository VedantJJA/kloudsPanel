package rules

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// tokenKind identifies what a captured group in a compiled source pattern
// corresponds to.
type tokenKind int

const (
	tokenWildcard tokenKind = iota
	tokenPlaceholder
)

// captureToken records, in the order capturing groups appear in the
// compiled regexp, whether that group came from a "*" or a ":name".
type captureToken struct {
	kind tokenKind
	name string // only set for tokenPlaceholder
}

// pattern is a compiled Source string, e.g. "/blog/posts/:postid" or "/*".
type pattern struct {
	raw      string
	re       *regexp.Regexp
	captures []captureToken
}

// compileSource compiles a Render-style Source path into a pattern.
//
// Syntax (see "Static Site Redirects and Rewrites"):
//   - "*" matches any string starting at that position (may span "/").
//   - ":name" matches a single path segment (no "/") and binds it to name.
//   - Everything else is matched literally.
//
// A bare "/" is rejected: rules cannot be applied to the domain root.
func compileSource(src string) (*pattern, error) {
	if src == "" || src[0] != '/' {
		return nil, fmt.Errorf("source %q must be an absolute path starting with /", src)
	}
	if src == "/" {
		return nil, fmt.Errorf("source %q: rules cannot be applied to the domain root", src)
	}

	var re strings.Builder
	re.WriteString("^")

	var captures []captureToken

	i := 0
	for i < len(src) {
		c := src[i]
		switch c {
		case '*':
			re.WriteString("(.*)")
			captures = append(captures, captureToken{kind: tokenWildcard})
			i++

		case ':':
			j := i + 1
			for j < len(src) && src[j] != '/' {
				j++
			}
			name := src[i+1 : j]
			if name == "" {
				return nil, fmt.Errorf("source %q: empty placeholder name at position %d", src, i)
			}
			re.WriteString("([^/]+)")
			captures = append(captures, captureToken{kind: tokenPlaceholder, name: name})
			i = j

		default:
			j := i
			for j < len(src) && src[j] != '*' && src[j] != ':' {
				j++
			}
			re.WriteString(regexp.QuoteMeta(src[i:j]))
			i = j
		}
	}
	re.WriteString("$")

	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return nil, fmt.Errorf("source %q: compiling pattern: %w", src, err)
	}

	return &pattern{raw: src, re: compiled, captures: captures}, nil
}

// match reports whether path satisfies the pattern, returning the named
// placeholder values and the ordered wildcard captures.
func (p *pattern) match(path string) (placeholders map[string]string, wildcards []string, ok bool) {
	m := p.re.FindStringSubmatch(path)
	if m == nil {
		return nil, nil, false
	}
	groups := m[1:]

	placeholders = make(map[string]string, len(p.captures))
	for idx, tok := range p.captures {
		switch tok.kind {
		case tokenWildcard:
			wildcards = append(wildcards, groups[idx])
		case tokenPlaceholder:
			placeholders[tok.name] = groups[idx]
		}
	}
	return placeholders, wildcards, true
}

// substituteDestination expands a Destination string using the values
// captured from a matching Source pattern.
//
//   - ":name" is replaced with the matching placeholder's value. A ":name"
//     that wasn't captured by the Source is left as-is.
//   - "*" is replaced with the wildcard captures, consumed left to right.
//     If the Destination uses "*" more times than the Source captured one,
//     the last captured value is reused. If the Source had no wildcard at
//     all, "*" in the Destination expands to nothing.
func substituteDestination(dst string, placeholders map[string]string, wildcards []string) string {
	var out strings.Builder
	wi := 0
	i := 0
	for i < len(dst) {
		c := dst[i]
		switch c {
		case '*':
			if wi < len(wildcards) {
				out.WriteString(wildcards[wi])
				wi++
			} else if len(wildcards) > 0 {
				out.WriteString(wildcards[len(wildcards)-1])
			}
			i++

		case ':':
			j := i + 1
			for j < len(dst) && dst[j] != '/' {
				j++
			}
			name := dst[i+1 : j]
			if v, ok := placeholders[name]; ok {
				out.WriteString(v)
			} else {
				out.WriteString(dst[i:j])
			}
			i = j

		default:
			j := i
			for j < len(dst) && dst[j] != '*' && dst[j] != ':' {
				j++
			}
			out.WriteString(dst[i:j])
			i = j
		}
	}
	return out.String()
}

// IsAbsoluteURL reports whether dest is a full, publicly addressable URL
// (as opposed to a path on the same site).
func IsAbsoluteURL(dest string) bool {
	u, err := url.Parse(dest)
	if err != nil {
		return false
	}
	return u.IsAbs()
}
