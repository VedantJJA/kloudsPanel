package http

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ─── Runtime Version Resolver ────────────────────────────────────────────────
// Resolves Docker image tags dynamically from project files instead of
// hardcoding versions. Supports user-specified versions, auto-detection
// from project files, and fallback to latest stable.

// RuntimeVersionInfo holds the resolved image tag and metadata.
type RuntimeVersionInfo struct {
	BaseImage      string // e.g. "node", "python", "golang"
	Version        string // e.g. "20", "3.12", "1.23"
	Tag            string // e.g. "20-alpine", "3.12-slim"
	FullImage      string // e.g. "node:20-alpine"
	Source         string // "user", "project-file", "default"
	DetectedFrom   string // e.g. ".node-version", "go.mod", "package.json engines"
}

// runtimeDefaults maps preset names to their default image configuration.
var runtimeDefaults = map[string]struct {
	base       string
	version    string
	tagSuffix  string // "-alpine", "-slim", etc.
}{
	"node":    {"node", "22", "-alpine"},
	"nodejs":  {"node", "22", "-alpine"},
	"python":  {"python", "3.12", "-slim"},
	"go":      {"golang", "1.23", "-alpine"},
	"golang":  {"golang", "1.23", "-alpine"},
	"rust":    {"rust", "1.82", "-alpine"},
	"java":    {"eclipse-temurin", "21", "-jdk-alpine"},
	"php":     {"php", "8.3", "-apache"},
	"ruby":    {"ruby", "3.3", "-alpine"},
	"elixir":  {"elixir", "1.17", "-alpine"},
	"phoenix": {"elixir", "1.17", "-alpine"},
	"deno":    {"denoland/deno", "latest", ""},
	"bun":     {"oven/bun", "latest", ""},
	"dotnet":  {"mcr.microsoft.com/dotnet/sdk", "8.0", "-alpine"},
	"csharp":  {"mcr.microsoft.com/dotnet/sdk", "8.0", "-alpine"},
	"aspnet":  {"mcr.microsoft.com/dotnet/sdk", "8.0", "-alpine"},
	"scala":   {"eclipse-temurin", "21", "-jdk-alpine"},
	"sbt":     {"eclipse-temurin", "21", "-jdk-alpine"},
	"kotlin":  {"eclipse-temurin", "21", "-jdk-alpine"},
	"ktor":    {"eclipse-temurin", "21", "-jdk-alpine"},
	"swift":   {"swift", "5.10", "-jammy"},
	"vapor":   {"swift", "5.10", "-jammy"},
	"haskell": {"haskell", "9.8", "-slim"},
	"clojure": {"clojure", "temurin-21-lein", "-alpine"},
	"crystal": {"crystallang/crystal", "latest", ""},
	"zig":     {"alpine", "3.21", ""},
	"dart":    {"dart", "stable", ""},
}

// resolveRuntimeVersion determines the Docker image tag for a given preset.
// Priority: requestedVersion (user/blueprint) > project file detection > default.
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

	// 3. Fallback to defaults
	tag := defaults.version + defaults.tagSuffix
	return RuntimeVersionInfo{
		BaseImage: defaults.base,
		Version:   defaults.version,
		Tag:       tag,
		FullImage: fmt.Sprintf("%s:%s", defaults.base, tag),
		Source:    "default",
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

// ─── Node.js Version Detection ───────────────────────────────────────────────

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

// ─── Python Version Detection ────────────────────────────────────────────────

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

// ─── Go Version Detection ────────────────────────────────────────────────────

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

// ─── Rust Version Detection ─────────────────────────────────────────────────

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

// ─── Ruby Version Detection ─────────────────────────────────────────────────

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

// ─── Java Version Detection ─────────────────────────────────────────────────

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

// ─── PHP Version Detection ──────────────────────────────────────────────────

func detectPHPVersion(readFile func(string) string, fileExists func(string) bool) *RuntimeVersionInfo {
	if content := readFile("composer.json"); content != "" {
		re := regexp.MustCompile(`"php"\s*:\s*"[><=^~!]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "composer.json require.php"}
		}
	}
	return nil
}

// ─── Elixir Version Detection ───────────────────────────────────────────────

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

// ─── .NET Version Detection ─────────────────────────────────────────────────

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

// ─── Swift Version Detection ────────────────────────────────────────────────

func detectSwiftVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("Package.swift"); content != "" {
		re := regexp.MustCompile(`(?i)swift-tools-version:\s*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "Package.swift swift-tools-version"}
		}
	}
	return nil
}

// ─── Dart Version Detection ─────────────────────────────────────────────────

func detectDartVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("pubspec.yaml"); content != "" {
		re := regexp.MustCompile(`sdk:\s*['"]?[><=!~^]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "pubspec.yaml sdk constraint"}
		}
	}
	return nil
}

// ─── Crystal Version Detection ──────────────────────────────────────────────

func detectCrystalVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("shard.yml"); content != "" {
		re := regexp.MustCompile(`crystal:\s*['"]?[><=!~^]*(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "shard.yml crystal version"}
		}
	}
	return nil
}

// ─── Zig Version Detection ──────────────────────────────────────────────────

func detectZigVersion(readFile func(string) string) *RuntimeVersionInfo {
	if content := readFile("build.zig.zon"); content != "" {
		re := regexp.MustCompile(`\.minimum_zig_version\s*=\s*"(\d+\.\d+)`)
		if m := re.FindStringSubmatch(content); len(m) > 1 {
			return &RuntimeVersionInfo{Version: m[1], DetectedFrom: "build.zig.zon minimum_zig_version"}
		}
	}
	return nil
}

// ─── Version String Utilities ────────────────────────────────────────────────

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

// ─── Image Tag Helpers ───────────────────────────────────────────────────────

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

// ─── Database Version Resolver ───────────────────────────────────────────────

// resolveDatabaseVersion returns the full Docker image and resolved version
// for supported database engines, honoring user version selection or falling back to stable defaults.
func resolveDatabaseVersion(engine, requestedVersion string) (imageTag string, resolvedVersion string) {
	engine = strings.ToLower(strings.TrimSpace(engine))
	cleanVer := sanitizeVersionString(requestedVersion)

	switch engine {
	case "postgres", "postgresql":
		if cleanVer == "" {
			cleanVer = "16"
		}
		return fmt.Sprintf("postgres:%s-alpine", cleanVer), cleanVer

	case "mysql":
		if cleanVer == "" {
			cleanVer = "8.0"
		}
		return fmt.Sprintf("mysql:%s", cleanVer), cleanVer

	case "redis":
		if cleanVer == "" {
			cleanVer = "7.2"
		}
		return fmt.Sprintf("redis:%s-alpine", cleanVer), cleanVer

	case "mongodb", "mongo":
		if cleanVer == "" {
			cleanVer = "7.0"
		}
		return fmt.Sprintf("mongo:%s", cleanVer), cleanVer

	case "clickhouse":
		if cleanVer == "" {
			cleanVer = "24.3"
		}
		return fmt.Sprintf("clickhouse/clickhouse-server:%s-alpine", cleanVer), cleanVer

	default:
		if cleanVer != "" {
			return fmt.Sprintf("%s:%s", engine, cleanVer), cleanVer
		}
		return fmt.Sprintf("%s:latest", engine), "latest"
	}
}

