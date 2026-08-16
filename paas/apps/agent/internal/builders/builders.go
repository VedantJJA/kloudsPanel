// Package builders defines the BuildDriver interface and implements all
// supported build strategies: cnb, nixpacks, dockerfile, manual, image.
package builders

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// --- Build Driver Interface ----------------------------------------------------

// BuildRequest carries the input to a build driver.
type BuildRequest struct {
	ServiceID    string
	DeploymentID string
	Strategy     string // cnb | nixpacks | dockerfile | manual | image

	// Source
	SourcePath   string // path to checked-out source on disk
	RootDir      string // subdirectory within source
	BuildContext string // relative build context path

	// Driver-specific
	DockerfilePath string
	BuildCommand   string
	StartCommand   string
	ManualStack    string // python | node | go | rust | static
	ImageReference string // for image strategy (must include digest)
	BuilderImage   string // CNB builder image
	Buildpacks     []string

	// Environment (build-scope only, no runtime secrets)
	BuildEnv map[string]string

	// Limits
	TimeoutSeconds int
	MemoryBytes    int64
	CPUNanoCores   int64
}

// BuildPlan is the resolved, validated plan before execution.
type BuildPlan struct {
	Request              BuildRequest
	Strategy             string
	ResolvedBuilderImage string
	GeneratedDockerfile  string
}

// ImageArtifact is the result of a successful build.
type ImageArtifact struct {
	ImageRef string
	Digest   string
	SBOMRef  string
}

// LogSink receives build log lines.
type LogSink interface {
	Write(stream, message string) error
	io.Closer
}

// BuildDriver is the interface every build strategy must implement.
type BuildDriver interface {
	Validate(req BuildRequest) error
	Plan(ctx context.Context, req BuildRequest) (BuildPlan, error)
	Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error)
}

// --- CNB Driver ---------------------------------------------------------------

type CNBDriver struct{ defaultBuilderImage string }

func NewCNBDriver(builderImage string) *CNBDriver {
	return &CNBDriver{defaultBuilderImage: builderImage}
}

func (d *CNBDriver) Validate(req BuildRequest) error {
	if req.SourcePath == "" {
		return fmt.Errorf("cnb: source path required")
	}
	return nil
}

func (d *CNBDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	builder := req.BuilderImage
	if builder == "" {
		builder = d.defaultBuilderImage
	}
	return BuildPlan{Request: req, Strategy: "cnb", ResolvedBuilderImage: builder}, nil
}

func (d *CNBDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "CNB build driver (stub) - Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// --- Nixpacks Driver ----------------------------------------------------------

type NixpacksDriver struct{}

func NewNixpacksDriver() *NixpacksDriver { return &NixpacksDriver{} }

func (d *NixpacksDriver) Validate(req BuildRequest) error {
	if req.SourcePath == "" {
		return fmt.Errorf("nixpacks: source path required")
	}
	return nil
}

func (d *NixpacksDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	return BuildPlan{Request: req, Strategy: "nixpacks"}, nil
}

func (d *NixpacksDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "Nixpacks build driver (stub) - Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// --- Dockerfile Driver --------------------------------------------------------

type DockerfileDriver struct{}

func NewDockerfileDriver() *DockerfileDriver { return &DockerfileDriver{} }

func (d *DockerfileDriver) Validate(req BuildRequest) error {
	if req.DockerfilePath == "" {
		return fmt.Errorf("dockerfile: dockerfile_path required")
	}
	if req.SourcePath == "" {
		return fmt.Errorf("dockerfile: source path required")
	}
	return nil
}

func (d *DockerfileDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	return BuildPlan{Request: req, Strategy: "dockerfile"}, nil
}

func (d *DockerfileDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "Dockerfile build driver (stub) - Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// --- Manual Stack Driver ------------------------------------------------------

type ManualDriver struct{}

func NewManualDriver() *ManualDriver { return &ManualDriver{} }

func (d *ManualDriver) Validate(req BuildRequest) error {
	validStacks := map[string]bool{
		"python": true, "node": true, "go": true, "rust": true, "static": true,
		"java": true, "php": true, "ruby": true, "elixir": true, "deno": true,
		"bun": true, "dotnet": true, "scala": true, "kotlin": true, "swift": true,
		"crystal": true, "dart": true,
	}
	if !validStacks[req.ManualStack] {
		return fmt.Errorf("manual: invalid stack %q; must be one of python, node, go, rust, static, java, php, ruby, elixir, deno, bun, dotnet, scala, kotlin, swift, crystal, dart", req.ManualStack)
	}
	return nil
}

func (d *ManualDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	dockerfile := generateManualDockerfile(req.ManualStack, req.BuildCommand, req.StartCommand)
	return BuildPlan{Request: req, Strategy: "manual", GeneratedDockerfile: dockerfile}, nil
}

func (d *ManualDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "Manual stack build driver (stub) - Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

func generateManualDockerfile(stack, buildCmd, startCmd string) string {
	bc := buildCmd
	if bc == "" {
		bc = "true"
	}
	sc := startCmd
	templates := map[string]string{
		"node":    fmt.Sprintf("FROM node:20-alpine\nWORKDIR /app\nCOPY . .\nRUN if [ -f package-lock.json ]; then npm ci; elif [ -f package.json ]; then npm install; fi\nRUN %s\nCMD [\"sh\", \"-c\", \"%s\"]", bc, orDefault(sc, "npm start || node index.js")),
		"python":  fmt.Sprintf("FROM python:3.12-slim\nWORKDIR /app\nCOPY . .\nRUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(sc, "python main.py || python app.py")),
		"go":      "FROM golang:1.23-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN if [ -f go.mod ]; then go mod download; fi\nRUN CGO_ENABLED=0 go build -o /app/main .\nFROM alpine:3.21\nRUN apk add --no-cache ca-certificates\nCOPY --from=build /app/main /main\nCMD [\"/main\"]",
		"rust":    "FROM rust:1.82-alpine AS build\nWORKDIR /app\nRUN apk add --no-cache musl-dev\nCOPY . .\nRUN cargo build --release && cp $(find target/release -maxdepth 1 -type f -executable | head -1) /app/server\nFROM alpine:3.21\nRUN apk add --no-cache libgcc ca-certificates\nCOPY --from=build /app/server /app/server\nCMD [\"/app/server\"]",
		"java":    fmt.Sprintf("FROM eclipse-temurin:21-jdk-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM eclipse-temurin:21-jre-alpine\nWORKDIR /app\nCOPY --from=build /app /app\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "if [ -f pom.xml ]; then mvn clean package -DskipTests; elif [ -f build.gradle ]; then ./gradlew build -x test; fi"), orDefault(sc, "java -jar target/*.jar || java -jar build/libs/*.jar")),
		"php":     fmt.Sprintf("FROM php:8.3-apache\nWORKDIR /var/www/html\nRUN docker-php-ext-install pdo pdo_mysql\nCOPY . /var/www/html/\nRUN %s\nRUN a2enmod rewrite\nCMD [\"%s\"]", orDefault(bc, "true"), orDefault(sc, "apache2-foreground")),
		"ruby":    fmt.Sprintf("FROM ruby:3.3-alpine\nWORKDIR /app\nRUN apk add --no-cache build-base\nCOPY . .\nRUN if [ -f Gemfile ]; then bundle install --without development test; fi\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(sc, "bundle exec puma || ruby app.rb")),
		"elixir":  fmt.Sprintf("FROM elixir:1.17-alpine\nWORKDIR /app\nENV MIX_ENV=prod\nCOPY . .\nRUN mix local.hex --force && mix local.rebar --force && if [ -f mix.exs ]; then mix deps.get; fi\nRUN %s\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "mix compile"), orDefault(sc, "mix phx.server || mix run --no-halt")),
		"deno":    fmt.Sprintf("FROM denoland/deno:alpine\nWORKDIR /app\nCOPY . .\nRUN %s\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "deno cache main.ts 2>/dev/null || true"), orDefault(sc, "deno run --allow-net --allow-env --allow-read main.ts")),
		"bun":     fmt.Sprintf("FROM oven/bun:alpine\nWORKDIR /app\nCOPY . .\nRUN if [ -f package.json ]; then bun install; fi\nRUN %s\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "true"), orDefault(sc, "bun run start || bun run index.ts")),
		"dotnet":  fmt.Sprintf("FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine\nCOPY --from=build /app/publish /app\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "dotnet publish -c Release -o /app/publish"), orDefault(sc, "dotnet $(find /app -name '*.dll' -maxdepth 1 | head -1)")),
		"scala":   fmt.Sprintf("FROM eclipse-temurin:21-jdk-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM eclipse-temurin:21-jre-alpine\nCOPY --from=build /app/target /app/target\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "sbt stage || sbt assembly"), orDefault(sc, "java -jar target/universal/stage/lib/*.jar")),
		"kotlin":  fmt.Sprintf("FROM eclipse-temurin:21-jdk-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM eclipse-temurin:21-jre-alpine\nCOPY --from=build /app/build/libs/*.jar /app/\nCMD [\"sh\", \"-c\", \"%s\"]", orDefault(bc, "./gradlew build -x test"), orDefault(sc, "java -jar /app/*-all.jar || java -jar /app/*.jar")),
		"swift":   fmt.Sprintf("FROM swift:5.10-jammy AS build\nWORKDIR /app\nCOPY . .\nRUN %s && cp $(find .build/release -maxdepth 1 -type f -executable | head -1) /app/Run\nFROM ubuntu:22.04\nCOPY --from=build /app/Run /app/Run\nCMD [\"/app/Run\"]", orDefault(bc, "swift build -c release")),
		"crystal": fmt.Sprintf("FROM crystallang/crystal:latest AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM ubuntu:22.04\nCOPY --from=build /app/app /app/app\nCMD [\"/app/app\"]", orDefault(bc, "shards install && crystal build src/*.cr --release -o app")),
		"dart":    fmt.Sprintf("FROM dart:stable AS build\nWORKDIR /app\nCOPY . .\nRUN %s\nFROM alpine:3.21\nRUN apk add --no-cache libgcc gcompat\nCOPY --from=build /app/server /app/server\nCMD [\"/app/server\"]", orDefault(bc, "dart pub get && dart compile exe bin/server.dart -o server")),
		"static":  "FROM nginx:alpine\nCOPY . /usr/share/nginx/html\n",
	}
	if t, ok := templates[stack]; ok {
		return t
	}
	return ""
}

// --- Image Driver -------------------------------------------------------------

type ImageDriver struct{}

func NewImageDriver() *ImageDriver { return &ImageDriver{} }

func (d *ImageDriver) Validate(req BuildRequest) error {
	if req.ImageReference == "" {
		return fmt.Errorf("image: image_reference required")
	}
	if !strings.Contains(req.ImageReference, "@sha256:") {
		return fmt.Errorf("image: image_reference must include @sha256: digest for immutability")
	}
	return nil
}

func (d *ImageDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	return BuildPlan{Request: req, Strategy: "image"}, nil
}

func (d *ImageDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "Image pull driver (stub) - Phase 5 implementation pending")
	return ImageArtifact{ImageRef: plan.Request.ImageReference}, nil
}

// --- Registry -----------------------------------------------------------------

type Registry map[string]BuildDriver

func NewRegistry(cnbBuilderImage string) Registry {
	return Registry{
		"cnb":        NewCNBDriver(cnbBuilderImage),
		"nixpacks":   NewNixpacksDriver(),
		"dockerfile": NewDockerfileDriver(),
		"manual":     NewManualDriver(),
		"image":      NewImageDriver(),
	}
}

func (r Registry) Get(strategy string) (BuildDriver, error) {
	d, ok := r[strategy]
	if !ok {
		return nil, fmt.Errorf("unknown build strategy: %s", strategy)
	}
	return d, nil
}

func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
