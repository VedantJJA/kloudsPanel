package http

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// --- Traefik Dynamic Ingress Configuration ------------------------------------

func getRootDomain() string {
	if d := os.Getenv("ROOT_DOMAIN"); d != "" {
		return d
	}
	return "yourdomain.com"
}

func getDynamicTraefikDirs() []string {
	var candidates []string
	if d := os.Getenv("TRAEFIK_DYNAMIC_DIR"); d != "" {
		candidates = append(candidates, d)
	}
	candidates = append(candidates, "/traefik/dynamic", "./paas/deploy/traefik/dynamic", "./deploy/traefik/dynamic")

	var validDirs []string
	seen := make(map[string]bool)
	for _, dir := range candidates {
		_ = os.MkdirAll(dir, 0755)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			if !seen[abs] {
				seen[abs] = true
				validDirs = append(validDirs, dir)
			}
		}
	}
	return validDirs
}

// cleanRoutePath ensures proper leading slash and standardizes glob/prefix characters.
func cleanRoutePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "http://") && !strings.HasPrefix(p, "https://") {
		p = "/" + p
	}
	return p
}

func writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug string, port int, rootDomain string, customDomains []string, routes []ServiceRouteItem, siblingStaticSlugs []string) {
	if slug == "" {
		return
	}
	if port <= 0 {
		port = 80
	}

	cleanSlug := strings.ToLower(strings.TrimSpace(slug))

	ruleParts := []string{fmt.Sprintf("Host(`%s.%s`)", cleanSlug, rootDomain)}
	for _, cd := range customDomains {
		cd = strings.TrimSpace(strings.ToLower(cd))
		if cd != "" {
			ruleParts = append(ruleParts, fmt.Sprintf("Host(`%s`)", cd))
		}
	}
	baseHostRule := strings.Join(ruleParts, " || ")

	var routersYAML strings.Builder
	var middlewaresYAML strings.Builder
	var servicesYAML strings.Builder

	// Primary container backend service
	servicesYAML.WriteString(fmt.Sprintf("    svc-%s:\n      loadBalancer:\n        servers:\n          - url: \"http://paas-svc-%s:%d\"\n", cleanSlug, cleanSlug, port))

	// Render-Style Redirect & Rewrite Rules
	for i, r := range routes {
		idx := i + 1
		src := cleanRoutePath(r.Source)
		dest := strings.TrimSpace(r.Destination)
		rType := strings.ToLower(strings.TrimSpace(r.Type))
		if src == "" || dest == "" {
			continue
		}

		// Skip internal SPA fallbacks (e.g. /* -> /index.html) in Traefik edge router
		// The container's static web server (nginx) handles SPA fallbacks natively via try_files.
		// Intercepting /* in Traefik rewrites static asset paths or triggers infinite redirect loops.
		if (src == "/*" || src == "*" || src == "/") && (dest == "/index.html" || dest == "index.html" || strings.HasSuffix(dest, "/index.html") || strings.HasSuffix(dest, ".html")) && !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
			continue
		}

		var pathRule string
		isWildcard := strings.HasSuffix(src, "/*") || strings.HasSuffix(src, "*")
		srcPrefix := src
		if strings.HasSuffix(src, "/*") {
			srcPrefix = strings.TrimSuffix(src, "/*")
		} else if strings.HasSuffix(src, "*") {
			srcPrefix = strings.TrimSuffix(src, "*")
		}

		if src == "/*" || src == "*" || src == "/" {
			pathRule = ""
		} else if isWildcard {
			pathRule = fmt.Sprintf(" && PathPrefix(`%s`)", srcPrefix)
		} else {
			pathRule = fmt.Sprintf(" && Path(`%s`)", src)
		}

		priority := 1000 - (i * 10)
		if priority < 200 {
			priority = 200
		}

		if rType == "rewrite" || rType == "rewrite_200" {
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				parsedDest, err := url.Parse(dest)
				if err == nil && parsedDest.Host != "" {
					targetBaseUrl := fmt.Sprintf("%s://%s", parsedDest.Scheme, parsedDest.Host)
					targetServiceKey := fmt.Sprintf("svc-%s-target-%d", cleanSlug, idx)
					targetRouterKey := fmt.Sprintf("svc-%s-rewrite-%d", cleanSlug, idx)

					var mwList []string
					destPath := strings.TrimSuffix(parsedDest.Path, "/*")
					destPath = strings.TrimSuffix(destPath, "*")

					// If destination path differs from source prefix, add a replacePathRegex middleware
					if destPath != srcPrefix && (srcPrefix != "" || destPath != "") {
						pathMwKey := fmt.Sprintf("svc-%s-rewrite-path-%d", cleanSlug, idx)
						var mwRegex, mwReplacement string
						if isWildcard {
							mwRegex = fmt.Sprintf("^%s/(.*)", regexp.QuoteMeta(srcPrefix))
							if destPath == "" || destPath == "/" {
								mwReplacement = "/${1}"
							} else {
								mwReplacement = fmt.Sprintf("%s/${1}", destPath)
							}
						} else {
							mwRegex = fmt.Sprintf("^%s/?$", regexp.QuoteMeta(src))
							if destPath == "" || destPath == "/" {
								mwReplacement = "/"
							} else {
								mwReplacement = destPath
							}
						}
						middlewaresYAML.WriteString(fmt.Sprintf("    %s:\n      replacePathRegex:\n        regex: \"%s\"\n        replacement: \"%s\"\n", pathMwKey, mwRegex, mwReplacement))
						mwList = append(mwList, pathMwKey)
					}

					var mwSection string
					if len(mwList) > 0 {
						mwSection = fmt.Sprintf("      middlewares:\n        - \"%s\"\n", strings.Join(mwList, "\"\n        - \""))
					}

					routersYAML.WriteString(fmt.Sprintf("    %s:\n      rule: \"(%s)%s\"\n      priority: %d\n%s      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"%s\"\n", targetRouterKey, baseHostRule, pathRule, priority, mwSection, targetServiceKey))

					servicesYAML.WriteString(fmt.Sprintf("    %s:\n      loadBalancer:\n        passHostHeader: false\n        servers:\n          - url: \"%s\"\n", targetServiceKey, targetBaseUrl))
				}
			} else {
				// Subpath internal rewrite (e.g. /docs/* -> /documentation/*)
				cleanDest := cleanRoutePath(dest)
				internalRouterKey := fmt.Sprintf("svc-%s-internal-rtr-%d", cleanSlug, idx)
				internalMwKey := fmt.Sprintf("svc-%s-internal-mw-%d", cleanSlug, idx)

				var mwRegex, mwReplacement string
				if isWildcard {
					destPrefix := strings.TrimSuffix(cleanDest, "/*")
					destPrefix = strings.TrimSuffix(destPrefix, "*")
					mwRegex = fmt.Sprintf("^%s/(.*)", regexp.QuoteMeta(srcPrefix))
					if destPrefix == "" || destPrefix == "/" {
						mwReplacement = "/${1}"
					} else {
						mwReplacement = fmt.Sprintf("%s/${1}", destPrefix)
					}
				} else {
					mwRegex = fmt.Sprintf("^%s/?$", regexp.QuoteMeta(src))
					mwReplacement = cleanDest
				}

				middlewaresYAML.WriteString(fmt.Sprintf("    %s:\n      replacePathRegex:\n        regex: \"%s\"\n        replacement: \"%s\"\n", internalMwKey, mwRegex, mwReplacement))

				routersYAML.WriteString(fmt.Sprintf("    %s:\n      rule: \"(%s)%s\"\n      priority: %d\n      middlewares:\n        - \"%s\"\n      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", internalRouterKey, baseHostRule, pathRule, priority, internalMwKey, cleanSlug))
			}
		} else {
			// Redirect action (301, 302, 307, 308)
			isPermanent := rType == "redirect" || rType == "redirect_301" || rType == "redirect_308"
			redirMiddlewareKey := fmt.Sprintf("svc-%s-redir-%d", cleanSlug, idx)
			redirRouterKey := fmt.Sprintf("svc-%s-redir-rtr-%d", cleanSlug, idx)

			var regex, replacement string
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				targetUrl := strings.TrimSuffix(dest, "/*")
				targetUrl = strings.TrimSuffix(targetUrl, "*")
				if isWildcard {
					regex = fmt.Sprintf(`^https?://[^/]+%s/(.*)`, regexp.QuoteMeta(srcPrefix))
					replacement = fmt.Sprintf(`%s/${1}`, targetUrl)
				} else {
					regex = fmt.Sprintf(`^https?://[^/]+%s/?$`, regexp.QuoteMeta(src))
					replacement = targetUrl
				}
			} else {
				cleanDest := cleanRoutePath(dest)
				targetPath := strings.TrimSuffix(cleanDest, "/*")
				targetPath = strings.TrimSuffix(targetPath, "*")
				if isWildcard {
					regex = fmt.Sprintf(`^(https?://[^/]+)%s/(.*)`, regexp.QuoteMeta(srcPrefix))
					if targetPath == "" || targetPath == "/" {
						replacement = `${1}/${2}`
					} else {
						replacement = fmt.Sprintf(`${1}%s/${2}`, targetPath)
					}
				} else {
					regex = fmt.Sprintf(`^(https?://[^/]+)%s/?$`, regexp.QuoteMeta(src))
					replacement = fmt.Sprintf(`${1}%s`, targetPath)
				}
			}

			middlewaresYAML.WriteString(fmt.Sprintf("    %s:\n      redirectRegex:\n        regex: \"%s\"\n        replacement: \"%s\"\n        permanent: %t\n", redirMiddlewareKey, regex, replacement, isPermanent))

			routersYAML.WriteString(fmt.Sprintf("    %s:\n      rule: \"(%s)%s\"\n      priority: %d\n      middlewares:\n        - \"%s\"\n      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", redirRouterKey, baseHostRule, pathRule, priority, redirMiddlewareKey, cleanSlug))
		}
	}

	// Sibling static frontends auto proxying
	for _, fSlug := range siblingStaticSlugs {
		fSlug = strings.TrimSpace(strings.ToLower(fSlug))
		if fSlug != "" && fSlug != cleanSlug {
			routersYAML.WriteString(fmt.Sprintf("    svc-%s-api-proxy-%s:\n      rule: \"Host(`%s.%s`) && PathPrefix(`/api`)\"\n      priority: 150\n      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", cleanSlug, fSlug, fSlug, rootDomain, cleanSlug))
		}
	}

	// Base fallback router
	routersYAML.WriteString(fmt.Sprintf("    svc-%s:\n      rule: \"%s\"\n      priority: 10\n      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", cleanSlug, baseHostRule, cleanSlug))

	var output strings.Builder
	output.WriteString("http:\n")
	output.WriteString("  routers:\n")
	output.WriteString(routersYAML.String())
	if middlewaresYAML.Len() > 0 {
		output.WriteString("  middlewares:\n")
		output.WriteString(middlewaresYAML.String())
	}
	output.WriteString("  services:\n")
	output.WriteString(servicesYAML.String())

	configContent := []byte(output.String())
	for _, dir := range getDynamicTraefikDirs() {
		filePath := filepath.Join(dir, fmt.Sprintf("svc-%s.yaml", cleanSlug))
		_ = os.WriteFile(filePath, configContent, 0644)
	}
}

func writeTraefikDynamicConfigWithDomainsAndSiblings(slug string, port int, rootDomain string, customDomains []string, siblingStaticSlugs []string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, customDomains, nil, siblingStaticSlugs)
}

func writeTraefikDynamicConfigWithDomains(slug string, port int, rootDomain string, customDomains []string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, customDomains, nil, nil)
}

func writeTraefikDynamicConfig(slug string, port int, rootDomain string) {
	writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug, port, rootDomain, nil, nil, nil)
}

func removeTraefikDynamicConfig(slug string) {
	if slug == "" {
		return
	}
	cleanSlug := strings.ToLower(strings.TrimSpace(slug))
	for _, dir := range getDynamicTraefikDirs() {
		filePath := filepath.Join(dir, fmt.Sprintf("svc-%s.yaml", cleanSlug))
		_ = os.Remove(filePath)
	}
}
