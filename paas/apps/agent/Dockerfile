# Multi-stage Dockerfile for kloudsPanel Agent (Go)
# No CGO needed — pure Go binary.

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/bin/agent \
    ./cmd/agent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1001 klouds && adduser -u 1001 -G klouds -s /bin/sh -D klouds

WORKDIR /app
COPY --from=builder /app/bin/agent /app/agent

USER klouds

ENTRYPOINT ["/app/agent"]
