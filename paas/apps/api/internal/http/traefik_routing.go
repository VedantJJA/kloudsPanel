package http

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// --- Traefik Dynamic Ingress Configuration ------------------------------------

func getRootDomain() string {
	if d := os.Getenv("ROOT_DOMAIN"); d != "" {
		return d
	}
	return "yourdomain.com"
}

func writeTraefikDynamicConfigWithDomainsRoutesAndSiblings(slug string, port int, rootDomain string, customDomains []string, routes []ServiceRouteItem, siblingStaticSlugs []string) {
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
		if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
			_ = os.MkdirAll(dynamicDir, 0755)
		}
	}

	ruleParts := []string{fmt.Sprintf("Host(`%s.%s`)", slug, rootDomain)}
	for _, cd := range customDomains {
		cd = strings.TrimSpace(cd)
		if cd != "" {
			ruleParts = append(ruleParts, fmt.Sprintf("Host(`%s`)", cd))
		}
	}
	baseHostRule := strings.Join(ruleParts, " || ")

	var routersYAML strings.Builder
	var middlewaresYAML strings.Builder
	var servicesYAML strings.Builder

	// Primary container backend service
	servicesYAML.WriteString(fmt.Sprintf(`    svc-%s:
      loadBalancer:
        servers:
          - url: "http://paas-svc-%s:%d"
`, slug, slug, port))

	// Render-Style Redirect & Rewrite Rules
	for i, r := range routes {
		idx := i + 1
		src := strings.TrimSpace(r.Source)
		dest := strings.TrimSpace(r.Destination)
		rType := strings.ToLower(strings.TrimSpace(r.Type))
		if src == "" || dest == "" {
			continue
		}

		cleanSrc := src
		// Skip internal SPA fallbacks (e.g. /* -> /index.html) in Traefik edge router
		// The container's static web server (nginx) handles SPA fallbacks natively via try_files.
		// Intercepting /* in Traefik rewrites static asset paths or triggers infinite redirect loops.
		if (cleanSrc == "/*" || cleanSrc == "*" || cleanSrc == "/") && (dest == "/index.html" || dest == "index.html" || strings.HasSuffix(dest, "/index.html") || strings.HasSuffix(dest, ".html")) && !strings.HasPrefix(dest, "http://") && !strings.HasPrefix(dest, "https://") {
			continue
		}

		var pathRule string
		if cleanSrc == "/*" || cleanSrc == "*" || cleanSrc == "/" {
			pathRule = ""
		} else if strings.HasSuffix(cleanSrc, "/*") {
			prefix := strings.TrimSuffix(cleanSrc, "/*")
			pathRule = fmt.Sprintf(" && PathPrefix(`%s`)", prefix)
		} else if strings.HasSuffix(cleanSrc, "*") {
			prefix := strings.TrimSuffix(cleanSrc, "*")
			pathRule = fmt.Sprintf(" && PathPrefix(`%s`)", prefix)
		} else {
			pathRule = fmt.Sprintf(" && Path(`%s`)", cleanSrc)
		}

		if rType == "rewrite" || rType == "rewrite_200" {
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				parsedDest, err := url.Parse(dest)
				if err == nil {
					targetBaseUrl := fmt.Sprintf("%s://%s", parsedDest.Scheme, parsedDest.Host)
					targetServiceKey := fmt.Sprintf("svc-%s-target-%d", slug, idx)
					targetRouterKey := fmt.Sprintf("svc-%s-rewrite-%d", slug, idx)
					priority := 1000 - (i * 10)

					var mwList []string
					destPath := strings.TrimSuffix(parsedDest.Path, "/*")
					srcPrefix := strings.TrimSuffix(src, "/*")

					// If destination path differs from source prefix, add a replacePathRegex middleware
					if destPath != srcPrefix && (srcPrefix != "" || destPath != "") {
						pathMwKey := fmt.Sprintf("svc-%s-rewrite-path-%d", slug, idx)
						var mwRegex, mwReplacement string
						if strings.HasSuffix(src, "/*") {
							mwRegex = fmt.Sprintf("^%s/(.*)", srcPrefix)
							if destPath == "" || destPath == "/" {
								mwReplacement = "/${1}"
							} else {
								mwReplacement = fmt.Sprintf("%s/${1}", destPath)
							}
						} else {
							mwRegex = fmt.Sprintf("^%s$", src)
							mwReplacement = destPath
						}
						middlewaresYAML.WriteString(fmt.Sprintf(`    %s:
      replacePathRegex:
        regex: "%s"
        replacement: "%s"
`, pathMwKey, mwRegex, mwReplacement))
						mwList = append(mwList, pathMwKey)
					}

					var mwSection string
					if len(mwList) > 0 {
						mwSection = fmt.Sprintf("      middlewares:\n        - \"%s\"\n", strings.Join(mwList, "\"\n        - \""))
					}

					routersYAML.WriteString(fmt.Sprintf(`    %s:
      rule: "(%s)%s"
      priority: %d
%s      entryPoints:
        - "web"
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "%s"
`, targetRouterKey, baseHostRule, pathRule, priority, mwSection, targetServiceKey))

					servicesYAML.WriteString(fmt.Sprintf(`    %s:
      loadBalancer:
        passHostHeader: false
        servers:
          - url: "%s"
`, targetServiceKey, targetBaseUrl))
				}
			} else {
				// Subpath internal rewrite (e.g. /docs/* -> /documentation/*)
				internalRouterKey := fmt.Sprintf("svc-%s-internal-rtr-%d", slug, idx)
				internalMwKey := fmt.Sprintf("svc-%s-internal-mw-%d", slug, idx)
				priority := 1000 - (i * 10)

				var mwRegex, mwReplacement string
				if strings.HasSuffix(src, "/*") {
					mwRegex = fmt.Sprintf("^%s/(.*)", strings.TrimSuffix(src, "/*"))
					if strings.HasSuffix(dest, "/*") {
						mwReplacement = fmt.Sprintf("%s/${1}", strings.TrimSuffix(dest, "/*"))
					} else {
						mwReplacement = dest
					}
				} else {
					mwRegex = fmt.Sprintf("^%s$", src)
					mwReplacement = dest
				}

				middlewaresYAML.WriteString(fmt.Sprintf(`    %s:
      replacePathRegex:
        regex: "%s"
        replacement: "%s"
`, internalMwKey, mwRegex, mwReplacement))

				routersYAML.WriteString(fmt.Sprintf(`    %s:
      rule: "(%s)%s"
      priority: %d
      middlewares:
        - "%s"
      entryPoints:
        - "web"
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
`, internalRouterKey, baseHostRule, pathRule, priority, internalMwKey, slug))
			}
		} else {
			// Redirect action (301, 302, 307, 308)
			isPermanent := rType == "redirect" || rType == "redirect_301" || rType == "redirect_308"
			redirMiddlewareKey := fmt.Sprintf("svc-%s-redir-%d", slug, idx)
			redirRouterKey := fmt.Sprintf("svc-%s-redir-rtr-%d", slug, idx)

			var regex, replacement string
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") {
				targetUrl := strings.TrimSuffix(dest, "/*")
				if strings.HasSuffix(src, "/*") {
					prefix := strings.TrimSuffix(cleanSrc, "/*")
					regex = fmt.Sprintf(`^https?://[^/]+%s/(.*)`, prefix)
					replacement = fmt.Sprintf(`%s/${1}`, targetUrl)
				} else {
					regex = fmt.Sprintf(`^https?://[^/]+%s/?$`, cleanSrc)
					replacement = targetUrl
				}
			} else {
				targetPath := strings.TrimSuffix(dest, "/*")
				if strings.HasSuffix(src, "/*") {
					prefix := strings.TrimSuffix(cleanSrc, "/*")
					regex = fmt.Sprintf(`^(https?://[^/]+)%s/(.*)`, prefix)
					replacement = fmt.Sprintf(`${1}%s/${2}`, targetPath)
				} else {
					regex = fmt.Sprintf(`^(https?://[^/]+)%s/?$`, cleanSrc)
					replacement = fmt.Sprintf(`${1}%s`, targetPath)
				}
			}

			middlewaresYAML.WriteString(fmt.Sprintf(`    %s:
      redirectRegex:
        regex: "%s"
        replacement: "%s"
        permanent: %t
`, redirMiddlewareKey, regex, replacement, isPermanent))

			priority := 1000 - (i * 10)
			routersYAML.WriteString(fmt.Sprintf(`    %s:
      rule: "(%s)%s"
      priority: %d
      middlewares:
        - "%s"
      entryPoints:
        - "web"
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
`, redirRouterKey, baseHostRule, pathRule, priority, redirMiddlewareKey, slug))
		}
	}

	// Sibling static frontends auto proxying
	for _, fSlug := range siblingStaticSlugs {
		fSlug = strings.TrimSpace(fSlug)
		if fSlug != "" {
			routersYAML.WriteString(fmt.Sprintf("    svc-%s-api-proxy-%s:\n      rule: \"Host(`%s.%s`) && PathPrefix(`/api`)\"\n      priority: 100\n      entryPoints:\n        - \"web\"\n        - \"websecure\"\n      tls:\n        certResolver: \"letsencrypt\"\n      service: \"svc-%s\"\n", slug, fSlug, fSlug, rootDomain, slug))
		}
	}

	// Base fallback router
	routersYAML.WriteString(fmt.Sprintf(`    svc-%s:
      rule: "%s"
      priority: 10
      entryPoints:
        - "web"
        - "websecure"
      tls:
        certResolver: "letsencrypt"
      service: "svc-%s"
`, slug, baseHostRule, slug))

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

	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	_ = os.WriteFile(filePath, []byte(output.String()), 0644)
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
	dynamicDir := "/traefik/dynamic"
	if _, err := os.Stat(dynamicDir); os.IsNotExist(err) {
		dynamicDir = "./paas/deploy/traefik/dynamic"
	}
	filePath := filepath.Join(dynamicDir, fmt.Sprintf("svc-%s.yaml", slug))
	_ = os.Remove(filePath)
}
