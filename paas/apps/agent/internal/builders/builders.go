// Package builders defines the BuildDriver interface and implements all
// supported build strategies: cnb, nixpacks, dockerfile, manual, image.
package builders

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// ─── Build Driver Interface ────────────────────────────────────────────────────

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

// ─── CNB Driver ───────────────────────────────────────────────────────────────

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
	sink.Write("system", "CNB build driver (stub) — Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// ─── Nixpacks Driver ──────────────────────────────────────────────────────────

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
	sink.Write("system", "Nixpacks build driver (stub) — Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// ─── Dockerfile Driver ────────────────────────────────────────────────────────

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
	sink.Write("system", "Dockerfile build driver (stub) — Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

// ─── Manual Stack Driver ──────────────────────────────────────────────────────

type ManualDriver struct{}

func NewManualDriver() *ManualDriver { return &ManualDriver{} }

func (d *ManualDriver) Validate(req BuildRequest) error {
	validStacks := map[string]bool{"python": true, "node": true, "go": true, "rust": true, "static": true}
	if !validStacks[req.ManualStack] {
		return fmt.Errorf("manual: invalid stack %q; must be one of python, node, go, rust, static", req.ManualStack)
	}
	return nil
}

func (d *ManualDriver) Plan(ctx context.Context, req BuildRequest) (BuildPlan, error) {
	dockerfile := generateManualDockerfile(req.ManualStack, req.BuildCommand, req.StartCommand)
	return BuildPlan{Request: req, Strategy: "manual", GeneratedDockerfile: dockerfile}, nil
}

func (d *ManualDriver) Build(ctx context.Context, plan BuildPlan, sink LogSink) (ImageArtifact, error) {
	sink.Write("system", "Manual stack build driver (stub) — Phase 5 implementation pending")
	return ImageArtifact{}, nil
}

func generateManualDockerfile(stack, buildCmd, startCmd string) string {
	bc := buildCmd
	if bc == "" {
		bc = "true"
	}
	sc := startCmd
	templates := map[string]string{
		"node":   fmt.Sprintf("FROM node:22-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --production\nCOPY . .\nRUN %s\nCMD [\"%s\"]", bc, orDefault(sc, "node index.js")),
		"python": fmt.Sprintf("FROM python:3.13-slim\nWORKDIR /app\nCOPY requirements*.txt ./\nRUN pip install -r requirements.txt\nCOPY . .\nCMD [\"%s\"]", orDefault(sc, "python main.py")),
		"go":     "FROM golang:1.26-alpine AS build\nWORKDIR /app\nCOPY go.* .\nRUN go mod download\nCOPY . .\nRUN go build -o /app/main .\nFROM alpine:3.21\nCOPY --from=build /app/main /main\nCMD [\"/main\"]",
		"rust":   "FROM rust:1.87-alpine AS build\nWORKDIR /app\nCOPY . .\nRUN cargo build --release\nFROM alpine:3.21\nCOPY --from=build /app/target/release/app /app\nCMD [\"/app\"]",
		"static": "FROM nginx:alpine\nCOPY . /usr/share/nginx/html\n",
	}
	if t, ok := templates[stack]; ok {
		return t
	}
	return ""
}

// ─── Image Driver ─────────────────────────────────────────────────────────────

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
	sink.Write("system", "Image pull driver (stub) — Phase 5 implementation pending")
	return ImageArtifact{ImageRef: plan.Request.ImageReference}, nil
}

// ─── Registry ─────────────────────────────────────────────────────────────────

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
