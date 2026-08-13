# Multi-stage Dockerfile for kloudsPanel Agent (Go)
# No CGO needed — pure Go binary.
# Build context must be the repo root (contains go.work + go.work.sum).

FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy workspace files and agent module
COPY go.work go.work.sum ./
COPY apps/agent/go.mod apps/agent/
COPY apps/api/go.mod apps/api/
RUN go work sync

# Copy agent source
COPY apps/agent/ apps/agent/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/bin/agent \
    ./apps/agent/cmd/agent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1001 klouds && adduser -u 1001 -G klouds -s /bin/sh -D klouds

WORKDIR /app
COPY --from=builder /app/bin/agent /app/agent

USER klouds

ENTRYPOINT ["/app/agent"]
