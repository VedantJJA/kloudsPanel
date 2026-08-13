# Multi-stage Dockerfile for kloudsPanel API (Go)
# Produces a minimal Linux image for amd64 and arm64.

# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install CGO deps for sqlite3
RUN apk add --no-cache gcc musl-dev

# Copy module files first for cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with CGO enabled (required for go-sqlite3)
RUN CGO_ENABLED=1 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/api \
    ./cmd/api

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl git docker-cli

WORKDIR /app
COPY --from=builder /app/bin/api /app/api

# Data and build directory
RUN mkdir -p /data /tmp/builds

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/api"]
