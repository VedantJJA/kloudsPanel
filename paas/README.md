# kloudsPanel - Self-Hosted PaaS

A lightweight, single-node, self-hosted PaaS built in Go + SvelteKit. Deploy Git repositories automatically via Cloud Native Buildpacks, Nixpacks, Dockerfile, or manual stacks. Full HTTPS, scale-to-zero, managed databases, real-time logs, web terminal, and an MCP server for AI-assisted deployment.

## Quick Start

### Prerequisites
- Linux `amd64` or `arm64` host (or macOS/Windows for development)
- Docker Engine 24+
- Go 1.22+
- Node.js 20+ and pnpm 9+
- A public IPv4/IPv6 address with `*.yourdomain.com` DNS

### Development

```bash
# Clone and enter
git clone https://github.com/VedantJJA/kloudsPanel.git paas
cd paas

# Install Node deps
pnpm install

# Initialize Go workspace
go work sync

# Start development environment
cp deploy/compose/.env.example deploy/compose/.env
# Edit .env with your settings

docker compose -f deploy/compose/compose.dev.yaml up
```

See [docs/architecture.md](docs/architecture.md) for full system design.

## Structure

```
apps/api/     - Go Fiber control API
apps/agent/   - Go node agent (Docker authority)
apps/web/     - SvelteKit 2 + TypeScript dashboard
packages/     - Shared TypeScript contracts and UI components
deploy/       - Docker Compose, Traefik, Sablier configs
docs/         - Architecture, ADRs, runbooks
```

## License

[MIT](LICENSE)
