package http

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yourorg/klouds/api/internal/domain"
)

// --- Runtime Version Resolver ------------------------------------------------
// Resolves Docker image tags dynamically from project files or live from
// container registries (Docker Hub / GHCR). Automatically queries the registry
// for the latest stable releases with intelligent TTL caching and instant fallback.

// RuntimeVersionInfo holds the resolved image tag and metadata.
type RuntimeVersionInfo struct {
	BaseImage    string // e.g. "node", "python", "golang"
	Version      string // e.g. "22", "3.12", "1.23"
	Tag          string // e.g. "22-alpine", "3.12-slim"
	FullImage    string // e.g. "node:22-alpine"
	Source       string // "user", "project-file", "registry-latest", "default"
	DetectedFrom string // e.g. ".node-version", "go.mod", "docker-hub-live"
}

// runtimeDefaults maps preset names to their baseline image configuration.
var runtimeDefaults = map[string]struct {
	base      string
	version   string
	tagSuffix string // "-bookworm-slim", "-slim", etc.
}{
	"node":       {"node", "22", "-bookworm-slim"},
	"nodejs":     {"node", "22", "-bookworm-slim"},
	"static":     {"node", "22", "-bookworm-slim"},
	"static-spa": {"node", "22", "-bookworm-slim"},
	"nginx":      {"nginx", "alpine", ""},
	"python":     {"python", "3.12", "-slim"},
	"go":         {"golang", "1.23", "-alpine"},
	"golang":     {"golang", "1.23", "-alpine"},
	"rust":       {"rust", "1.84", "-alpine"},
	"java":       {"eclipse-temurin", "21", "-jdk-alpine"},
	"php":        {"php", "8.3", "-apache"},
	"ruby":       {"ruby", "3.3", "-slim-bookworm"},
	"elixir":     {"elixir", "1.18", "-alpine"},
	"phoenix":    {"elixir", "1.18", "-alpine"},
	"deno":       {"denoland/deno", "latest", ""},
	"bun":        {"oven/bun", "latest", ""},
	"dotnet":     {"mcr.microsoft.com/dotnet/sdk", "9.0", "-alpine"},
	"csharp":     {"mcr.microsoft.com/dotnet/sdk", "9.0", "-alpine"},
	"aspnet":     {"mcr.microsoft.com/dotnet/sdk", "9.0", "-alpine"},
	"scala":      {"eclipse-temurin", "21", "-jdk-alpine"},
	"sbt":        {"eclipse-temurin", "21", "-jdk-alpine"},
	"kotlin":     {"eclipse-temurin", "21", "-jdk-alpine"},
	"ktor":       {"eclipse-temurin", "21", "-jdk-alpine"},
	"swift":      {"swift", "6.0", "-jammy"},
	"vapor":      {"swift", "6.0", "-jammy"},
	"haskell":    {"haskell", "9.10", "-slim"},
	"clojure":    {"clojure", "temurin-21-lein", "-alpine"},
	"crystal":    {"crystallang/crystal", "latest", ""},
	"zig":        {"alpine", "3.21", ""},
	"dart":       {"dart", "stable", ""},
}

// --- Dynamic Registry Tag Auto-Fetcher ---------------------------------------

type registryTagItem struct {
	Name string `json:"name"`
}

type registryTagsResponse struct {
	Results []registryTagItem `json:"results"`
}

type cachedDynamicTag struct {
	version   string
	tag       string
	fullImage string
	cachedAt  time.Time
}

var (
	registryCacheMu sync.RWMutex
	registryCache   = make(map[string]cachedDynamicTag)
	regHttpClient   = &nethttp.Client{Timeout: 5 * time.Second}
)

// compareVersionStrings compares two version strings numerically (e.g. "23" vs "22.1", "1.24" vs "1.23").
func compareVersionStrings(v1, v2 string) int {
	clean1 := strings.TrimPrefix(strings.ToLower(v1), "v")
	clean2 := strings.TrimPrefix(strings.ToLower(v2), "v")

	parts1 := strings.Split(clean1, ".")
	parts2 := strings.Split(clean2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			if v, err := strconv.Atoi(parts1[i]); err == nil {
				n1 = v
			}
		}
		if i < len(parts2) {
			if v, err := strconv.Atoi(parts2[i]); err == nil {
				n2 = v
			}
		}
		if n1 > n2 {
			return 1
		} else if n1 < n2 {
			return -1
		}
	}
	return 0
}

// parseBestVersionFromTags inspects a list of registry tags and finds the latest numeric stable release.
func parseBestVersionFromTags(baseImage string, tags []registryTagItem, tagSuffix string, fallbackVersion string) string {
	var bestVer string

	isMajorOnly := !strings.Contains(fallbackVersion, ".")

	var re *regexp.Regexp
	escapedSuffix := regexp.QuoteMeta(tagSuffix)
	if isMajorOnly {
		if tagSuffix == "" {
			re = regexp.MustCompile(`^(\d+)$`)
		} else {
			re = regexp.MustCompile(fmt.Sprintf(`^(\d+)(?:%s)$`, escapedSuffix))
		}
	} else {
		if tagSuffix == "" {
			re = regexp.MustCompile(`^(\d+\.\d+)$`)
		} else {
			re = regexp.MustCompile(fmt.Sprintf(`^(\d+\.\d+)(?:%s)$`, escapedSuffix))
		}
	}

	for _, t := range tags {
		tagName := strings.ToLower(strings.TrimSpace(t.Name))

		// Ignore pre-releases, release candidates, and development builds
		if strings.Contains(tagName, "-rc") ||
			strings.Contains(tagName, "-beta") ||
			strings.Contains(tagName, "-alpha") ||
			strings.Contains(tagName, "-preview") ||
			strings.Contains(tagName, "-dev") ||
			strings.Contains(tagName, "-nightly") ||
			strings.Contains(tagName, "snapshot") ||
			strings.Contains(tagName, "bookworm") ||
			strings.Contains(tagName, "bullseye") ||
			strings.Contains(tagName, "windowsservercore") {
			continue
		}

		m := re.FindStringSubmatch(tagName)
		if len(m) > 1 {
			candidateVer := m[1]

			// Node: Stick to active LTS releases (22, 20, 18) for maximum package compatibility
			if baseImage == "node" {
				if candNum, err := strconv.Atoi(candidateVer); err == nil && candNum > 22 {
					continue
				}
			}

			// Sanity filters to avoid experimental/internal branch numbers
			if isMajorOnly {
				candNum, _ := strconv.Atoi(candidateVer)
				fallNum, _ := strconv.Atoi(fallbackVersion)
				if candNum > fallNum+5 || candNum < fallNum-5 {
					continue
				}
			} else {
				parts := strings.Split(candidateVer, ".")
				fallParts := strings.Split(fallbackVersion, ".")
				if len(parts) >= 1 && len(fallParts) >= 1 {
					candMajor, _ := strconv.Atoi(parts[0])
					fallMajor, _ := strconv.Atoi(fallParts[0])
					if candMajor > fallMajor+2 || candMajor < fallMajor-2 {
						continue
					}
				}
			}

			if bestVer == "" || compareVersionStrings(candidateVer, bestVer) > 0 {
				bestVer = candidateVer
			}
		}
	}

	if bestVer == "" {
		return fallbackVersion
	}
	return bestVer
}

// fetchLatestRegistryTag queries the Docker Hub API for the latest matching tag.
// Retries once on transient failure before falling back to the baseline default,
// so a single slow/dropped request doesn't silently pin an old version.
func fetchLatestRegistryTag(baseImage, tagSuffix string, fallbackVersion string) (version string, tag string) {
	cacheKey := fmt.Sprintf("%s:%s", baseImage, tagSuffix)

	// 1. Check in-memory cache (TTL: 6 hours)
	registryCacheMu.RLock()
	if c, ok := registryCache[cacheKey]; ok && time.Since(c.cachedAt) < 6*time.Hour {
		registryCacheMu.RUnlock()
		return c.version, c.tag
	}
	registryCacheMu.RUnlock()

	apiUrl := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/library/%s/tags?page_size=100&ordering=last_updated", baseImage)
	if strings.Contains(baseImage, "/") {
		apiUrl = fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/tags?page_size=100&ordering=last_updated", baseImage)
	}

	var data registryTagsResponse
	var lastErr error
	fetched := false

	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			time.Sleep(400 * time.Millisecond) // brief backoff before retry
		}

		req, err := nethttp.NewRequest("GET", apiUrl, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "kloudsPanel-VersionResolver/1.0")

		resp, err := regHttpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("registry returned status %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if decodeErr != nil || len(data.Results) == 0 {
			lastErr = decodeErr
			continue
		}

		fetched = true
		break
	}

	if !fetched {
		// Cache the fallback briefly (5 min) too, so a registry outage
		// doesn't cause a full-timeout retry on every single build in a
		// tight loop - but don't cache it for the full 6h, so we recover
		// automatically once the registry is healthy again.
		registryCacheMu.Lock()
		registryCache[cacheKey] = cachedDynamicTag{
			version:   fallbackVersion,
			tag:       fallbackVersion + tagSuffix,
			fullImage: fmt.Sprintf("%s:%s", baseImage, fallbackVersion+tagSuffix),
			cachedAt:  time.Now().Add(-6*time.Hour + 5*time.Minute),
		}
		registryCacheMu.Unlock()
		_ = lastErr // available for logging by caller if desired
		return fallbackVersion, fallbackVersion + tagSuffix
	}

	bestVer := parseBestVersionFromTags(baseImage, data.Results, tagSuffix, fallbackVersion)
	bestTag := bestVer + tagSuffix
	if tagSuffix == "" {
		bestTag = bestVer
	}

	registryCacheMu.Lock()
	registryCache[cacheKey] = cachedDynamicTag{
		version:   bestVer,
		tag:       bestTag,
		fullImage: fmt.Sprintf("%s:%s", baseImage, bestTag),
		cachedAt:  time.Now(),
	}
	registryCacheMu.Unlock()

	return bestVer, bestTag
}

// resolveRuntimeVersion determines the Docker image tag for a given preset.
// Priority: requestedVersion (user/blueprint) > project file detection > live registry auto-fetch > default.
func resolveRuntimeVersion(preset, contextDir, requestedVersion string) RuntimeVersionInfo {
	preset = strings.ToLower(preset)
	defaults, ok := runtimeDefaults[preset]
	if !ok {
		return RuntimeVersionInfo{
			BaseImage: "alpine",
			Version:   "3.21",
			Tag:       "3.21",
			FullImage: "alpine:3.21",
			Source:    "default",
		}
	}

	// 1. User/blueprint-specified version takes highest priority
	if requestedVersion != "" {
		cleanVersion := sanitizeVersionString(requestedVersion)
		if cleanVersion != "" {
			tag := cleanVersion + defaults.tagSuffix
			return RuntimeVersionInfo{
				BaseImage:    defaults.base,
				Version:      cleanVersion,
				Tag:          tag,
				FullImage:    fmt.Sprintf("%s:%s", defaults.base, tag),
				Source:       "user",
				DetectedFrom: "blueprint or service config",
			}
		}
	}

	// 2. Auto-detect from project files
	if contextDir != "" {
		if detected := detectVersionFromProject(preset, contextDir); detected != nil {
			tag := detected.Version + defaults.tagSuffix
			return RuntimeVersionInfo{
				BaseImage:    defaults.base,
				Version:      detected.Version,
				Tag:          tag,
				FullImage:    fmt.Sprintf("%s:%s", defaults.base, tag),
				Source:       "project-file",
				DetectedFrom: detected.DetectedFrom,
			}
		}
	}

	// 3. Dynamically fetch latest version from container registry API (with instant cache & baseline fallback)
	dynVer, dynTag := fetchLatestRegistryTag(defaults.base, defaults.tagSuffix, defaults.version)
	return RuntimeVersionInfo{
		BaseImage:    defaults.base,
		Version:      dynVer,
		Tag:          dynTag,
		FullImage:    fmt.Sprintf("%s:%s", defaults.base, dynTag),
		Source:       "registry-latest",
		DetectedFrom: "docker-hub-registry",
	}
}

// detectVersionFromProject scans project files for version hints.
func detectVersionFromProject(preset, contextDir string) *RuntimeVersionInfo {
	readFile := func(name string) string {
		data, err := os.ReadFile(filepath.Join(contextDir, name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}

	fileExists := func(name string) bool {
		_, err := os.Stat(filepath.Join(contextDir, name))
		return err == nil
	}

	switch preset {
	case "node", "nodejs":
		return detectNodeVersion(readFile, fileExists)
	case "python":
		return detectPythonVersion(readFile, fileExists)
	case "go", "golang":
		return detectGoVersion(readFile)
	case "rust":
		return detectRustVersion(readFile, fileExists)
	case "ruby":
		return detectRubyVersion(readFile, fileExists)
	case "java", "kotlin", "ktor", "scala", "sbt":
		return detectJavaVersion(readFile, fileExists)
	case "php":
		return detectPHPVersion(readFile, fileExists)
	case "elixir", "phoenix":
		return detectElixirVersion(readFile, fileExists)
	case "dotnet", "csharp", "aspnet":
		return detectDotnetVersion(readFile, fileExists)
	case "swift", "vapor":
		return detectSwiftVersion(readFile)
	case "dart":
		return detectDartVersion(readFile)
	case "crystal":
		return detectCrystalVersion(readFile)
	case "zig":
		return detectZigVersion(readFile)
	}
	return nil
}

// --- Node.js Version Detection -----------------------------------------------

func detectNodeVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	// .node-version (e.g. "20.11.0" or "v20" or "20")
	if content := readFile(".node-version"); content != "" {
		if v := extractMajorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".node-version"}
		}
	}

	// .nvmrc (e.g. "20", "lts/iron", "v18.17.0")
	if content := readFile(".nvmrc"); content != "" {
		if strings.HasPrefix(content, "lts/") {
			// Map LTS codenames to versions
			ltsMap := map[string]string{
				"lts/iron":     "20",
				"lts/hydrogen": "18",
				"lts/gallium":  "16",
				"lts/*":        "20",
			}
			if v, ok := ltsMap[strings.ToLower(content)]; ok {
				return &RuntimeVersionInfo{Version: v, DetectedFrom: ".nvmrc (LTS)"}
			}
		}
		if v := extractMajorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".nvmrc"}
		}
	}

	// package.json "engines.node" (e.g. ">=18", "^20", "18.x", "20.11.0")
	if content := readFile("package.json"); content != "" {
		if v := extractEnginesNode(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: "package.json engines.node"}
		}
		// Volta pinning
		if v := extractVoltaNode(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: "package.json volta.node"}
		}
	}

	return nil
}

// --- Python Version Detection ------------------------------------------------

func detectPythonVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	// .python-version (e.g. "3.11.4" or "3.12")
	if content := readFile(".python-version"); content != "" {
		if v := extractPythonMajorMinor(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".python-version"}
		}
	}

	// runtime.txt (Heroku format: "python-3.11.4")
	if content := readFile("runtime.txt"); content != "" {
		content = strings.TrimPrefix(strings.ToLower(content), "python-")
		if v := extractPythonMajorMinor(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: "runtime.txt"}
		}
	}

	// pyproject.toml requires-python
	if content := readFile("pyproject.toml"); content != "" {
		re := regexp.MustCompile(`requires-python\s*=\s*"[><=!~]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "pyproject.toml requires-python"}
		}
	}

	return nil
}

// --- Go Version Detection ----------------------------------------------------

func detectGoVersion(readFile func(string) string) *RuntimeVersionInfo {
	// go.mod "go 1.22" directive
	if content := readFile("go.mod"); content != "" {
		re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "go.mod"}
		}
	}
	return nil
}

// --- Rust Version Detection -------------------------------------------------

func detectRustVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	// rust-toolchain.toml [toolchain] channel = "1.78"
	if content := readFile("rust-toolchain.toml"); content != "" {
		re := regexp.MustCompile(`channel\s*=\s*"(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "rust-toolchain.toml"}
		}
	}
	// rust-toolchain (plain text: "1.78" or "stable" or "nightly")
	if content := readFile("rust-toolchain"); content != "" {
		if v := extractMajorMinorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: "rust-toolchain"}
		}
	}
	return nil
}

// --- Ruby Version Detection -------------------------------------------------

func detectRubyVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	if content := readFile(".ruby-version"); content != "" {
		if v := extractMajorMinorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".ruby-version"}
		}
	}
	if content := readFile("Gemfile"); content != "" {
		re := regexp.MustCompile(`ruby\s+['"](\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "Gemfile ruby constraint"}
		}
	}
	return nil
}

// --- Java Version Detection -------------------------------------------------

func detectJavaVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	if content := readFile(".java-version"); content != "" {
		if v := extractMajorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".java-version"}
		}
	}
	// pom.xml java.version property
	if content := readFile("pom.xml"); content != "" {
		re := regexp.MustCompile(`<java\.version>(\d+)</java\.version>`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "pom.xml java.version"}
		}
		re2 := regexp.MustCompile(`<maven\.compiler\.source>(\d+)</maven\.compiler\.source>`)
		if m := re2.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "pom.xml maven.compiler.source"}
		}
	}
	// build.gradle sourceCompatibility
	for _, gf := range []string{"build.gradle", "build.gradle.kts"} {
		if content := readFile(gf); content != "" {
			re := regexp.MustCompile(`(?:sourceCompatibility|targetCompatibility|jvmTarget)\s*=\s*['"]?(\d+)`)
			if m := re.FindStringSubmatch(content); len(m) > 1 {
				return &RuntimeVersionInfo{Version: m[1], DetectedFrom: gf + " compatibility"}
			}
		}
	}
	return nil
}

// --- PHP Version Detection --------------------------------------------------

func detectPHPVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	if content := readFile("composer.json"); content != "" {
		re := regexp.MustCompile(`"php"\s*:\s*"[><=^~!]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "composer.json require.php"}
		}
	}
	return nil
}

// --- Elixir Version Detection -----------------------------------------------

func detectElixirVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	if content := readFile(".elixir-version"); content != "" {
		if v := extractMajorMinorVersion(content); v != "" {
			return &RuntimeVersionInfo{Version: v, DetectedFrom: ".elixir-version"}
		}
	}
	if content := readFile("mix.exs"); content != "" {
		re := regexp.MustCompile(`elixir:\s*"~>\s*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "mix.exs elixir constraint"}
		}
	}
	return nil
}

// --- .NET Version Detection -------------------------------------------------

func detectDotnetVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	// global.json
	if content := readFile("global.json"); content != "" {
		re := regexp.MustCompile(`"version"\s*:\s*"(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "global.json sdk.version"}
		}
	}
	// .csproj TargetFramework
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".csproj") {
			if content := readFile(e.Name()); content != "" {
				re := regexp.MustCompile(`<TargetFramework>net(\d+\.\d+)</TargetFramework>`)
				if m := re.FindStringSubmatch(content); len(m) > 1 {
					return &RuntimeVersionInfo{Version: m[1], DetectedFrom: e.Name() + " TargetFramework"}
				}
			}
		}
	}
	return nil
}

// --- Swift Version Detection ------------------------------------------------

func detectSwiftVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("Package.swift"); content != "" {
		re := regexp.MustCompile(`(?i)swift-tools-version:\s*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "Package.swift swift-tools-version"}
		}
	}
	return nil
}

// --- Dart Version Detection -------------------------------------------------

func detectDartVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("pubspec.yaml"); content != "" {
		re := regexp.MustCompile(`sdk:\s*['"]?[><=!~^]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "pubspec.yaml sdk constraint"}
		}
	}
	return nil
}

// --- Crystal Version Detection ----------------------------------------------

func detectCrystalVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("shard.yml"); content != "" {
		re := regexp.MustCompile(`crystal:\s*['"]?[><=!~^]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "shard.yml crystal version"}
		}
	}
	return nil
}

// --- Zig Version Detection --------------------------------------------------

func detectZigVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("build.zig.zon"); content != "" {
		re := regexp.MustCompile(`\.minimum_zig_version\s*=\s*"(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "build.zig.zon minimum_zig_version"}
		}
	}
	return nil
}

// --- Version String Utilities ------------------------------------------------

// sanitizeVersionString cleans and validates a user-specified version string.
// Prevents injection through version strings (e.g. "20; rm -rf /").
func sanitizeVersionString(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	// Only allow digits, dots, and hyphens (e.g. "20", "3.12", "21-jdk")
	re := regexp.MustCompile(`^[\d]+[\d.\-a-zA-Z]*$`)
	if re.MatchString(version) {
		return version
	}
	return ""
}

// extractMajorVersion extracts the major version from strings like "v20.11.0", "20", "18.x"
func extractMajorVersion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	re := regexp.MustCompile(`^(\d+)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}

// extractMajorMinorVersion extracts "X.Y" from strings like "3.12.4", "1.82"
func extractMajorMinorVersion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	re := regexp.MustCompile(`^(\d+\.\d+)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}

// extractPythonMajorMinor extracts "3.12" from "3.12.4" or "3.12"
func extractPythonMajorMinor(s string) string {
	return extractMajorMinorVersion(s)
}

// extractEnginesNode extracts a Node major version from package.json engines.node.
// Handles: ">=18", "^20", "20.x", "~18.0", ">=18 <21"
func extractEnginesNode(packageJSON string) string {
	re := regexp.MustCompile(`"engines"\s*:\s*\{[^}]*"node"\s*:\s*"([^"]+)"`)
	m := re.FindStringSubmatch(packageJSON)
	if len(m) < 2 {
		return ""
	}
	constraint := m[1]
	// Extract the first numeric version from the constraint
	numRe := regexp.MustCompile(`(\d+)`)
	if nm := numRe.FindStringSubmatch(constraint); len(nm) > 1 {
		return nm[1]
	}
	return ""
}

// extractVoltaNode extracts the Node version from Volta pinning in package.json.
func extractVoltaNode(packageJSON string) string {
	re := regexp.MustCompile(`"volta"\s*:\s*\{[^}]*"node"\s*:\s*"(\d+)`)
	if m := re.FindStringSubmatch(packageJSON); len(m) > 1 {
		return m[1]
	}
	return ""
}

// --- Image Tag Helpers -------------------------------------------------------

// getJavaBaseImage returns the appropriate JDK/JRE image for a Java version.
func getJavaBaseImage(version string, stage string) string {
	if stage == "runtime" {
		return fmt.Sprintf("eclipse-temurin:%s-jre-alpine", version)
	}
	return fmt.Sprintf("eclipse-temurin:%s-jdk-alpine", version)
}

// getDotnetBaseImage returns the SDK or ASP.NET runtime image.
func getDotnetBaseImage(version string, stage string) string {
	if stage == "runtime" {
		return fmt.Sprintf("mcr.microsoft.com/dotnet/aspnet:%s-alpine", version)
	}
	return fmt.Sprintf("mcr.microsoft.com/dotnet/sdk:%s-alpine", version)
}

// --- Database Version Resolver -----------------------------------------------

// resolveDatabaseVersion returns the full Docker image and resolved version
// for supported database engines, honoring user version selection or dynamically fetching from registry.
func resolveDatabaseVersion(engine, requestedVersion string) (imageTag string, resolvedVersion string) {
	engine = domain.CanonicalizeEngine(engine)
	cleanVer := sanitizeVersionString(requestedVersion)

	switch engine {
	case "postgres":
		if cleanVer != "" && cleanVer != "auto" {
			return fmt.Sprintf("postgres:%s-alpine", cleanVer), cleanVer
		}
		return "postgres:16-alpine", "16"

	case "mysql":
		if cleanVer != "" {
			return fmt.Sprintf("mysql:%s", cleanVer), cleanVer
		}
		ver, tag := fetchLatestRegistryTag("mysql", "", "8.4")
		return fmt.Sprintf("mysql:%s", tag), ver

	case "redis":
		if cleanVer != "" {
			return fmt.Sprintf("redis:%s-alpine", cleanVer), cleanVer
		}
		ver, tag := fetchLatestRegistryTag("redis", "-alpine", "7.4")
		return fmt.Sprintf("redis:%s", tag), ver

	case "mongodb", "mongo":
		if cleanVer != "" {
			return fmt.Sprintf("mongo:%s", cleanVer), cleanVer
		}
		ver, tag := fetchLatestRegistryTag("mongo", "", "8.0")
		return fmt.Sprintf("mongo:%s", tag), ver

	case "clickhouse":
		if cleanVer != "" {
			return fmt.Sprintf("clickhouse/clickhouse-server:%s-alpine", cleanVer), cleanVer
		}
		ver, tag := fetchLatestRegistryTag("clickhouse/clickhouse-server", "-alpine", "24.8")
		return fmt.Sprintf("clickhouse/clickhouse-server:%s", tag), ver

	default:
		if cleanVer != "" {
			return fmt.Sprintf("%s:%s", engine, cleanVer), cleanVer
		}
		return fmt.Sprintf("%s:latest", engine), "latest"
	}
}

// detectGoMainTarget finds the relative path to the Go main entrypoint in a project directory.
func detectGoMainTarget(dir string) string {
	if dir == "" {
		return "."
	}

	// 1. Check root directory for package main
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				if bytes, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
					if strings.Contains(string(bytes), "package main") {
						return "."
					}
				}
			}
		}
	}

	// 2. Check cmd/* subdirectories
	cmdDir := filepath.Join(dir, "cmd")
	if entries, err := os.ReadDir(cmdDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				subPath := filepath.Join(cmdDir, e.Name())
				if subEntries, err := os.ReadDir(subPath); err == nil {
					for _, se := range subEntries {
						if strings.HasSuffix(se.Name(), ".go") {
							return fmt.Sprintf("./cmd/%s", e.Name())
						}
					}
				}
			}
		}
	}

	// 3. Fallback: check all subdirectories for package main
	var foundMain string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || foundMain != "" {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			if bytes, err := os.ReadFile(path); err == nil {
				if strings.Contains(string(bytes), "package main") {
					rel, _ := filepath.Rel(dir, filepath.Dir(path))
					if rel == "." {
						foundMain = "."
					} else {
						foundMain = fmt.Sprintf("./%s", filepath.ToSlash(rel))
					}
				}
			}
		}
		return nil
	})

	return foundMain
}

