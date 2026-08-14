# kloudsPanel Blueprint Specification (`klouds.yaml`)

`klouds.yaml` is the primary declarative blueprint and infrastructure-as-code specification for **kloudsPanel**. It allows any single repository or monorepo to declare multi-tier stacks (frontends, backends, databases, background workers, cron jobs) with automatic networking, SSL, health checks, and dynamic environment variable bindings.

Place this file in the **root directory** of your Git repository as `klouds.yaml` (or `.klouds.yaml`).

> [!NOTE]
> `klouds.yaml` is the **highest priority** blueprint format recognized by kloudsPanel, taking precedence over `devpanel.yaml` and `render.yaml`.

---

## 🌟 The 5-Phase Service Topology

kloudsPanel classifies application components into **5 distinct phases**:

| Phase | Service Type | Runtime Engines | Key Characteristics |
| :--- | :--- | :--- | :--- |
| **Phase 1** | **Static SPA** (`type: static`) | Vite, React, Vue, SvelteKit, Astro, Next.js Static | Compiled to static assets (`/dist`, `/build`), served by high-performance Nginx with automated `/api/` reverse-proxying. |
| **Phase 2** | **Dynamic Web & APIs** (`type: web`) | Node.js, Python, Go, Rust, Java, PHP, Ruby, Bun, Deno, Dockerfile | Long-running HTTP/gRPC server exposed on an internal container port and routed with auto-SSL. |
| **Phase 3** | **Managed Databases** (`type: database`) | PostgreSQL 16, MySQL 8.0, Redis 7.2, MongoDB 7.0, ClickHouse 24.3 | Containerized stateful engines with persistent Docker volumes and dedicated public/internal connection URIs. |
| **Phase 4** | **Background Workers** (`type: worker`) | Celery, BullMQ, Sidekiq, RabbitMQ/Kafka Consumers | Headless persistent background processes without public HTTP ingress. |
| **Phase 5** | **Scheduled Cron Jobs** (`type: cron`) | Any runtime | Ephemeral containers executed on a standard 5-field cron schedule (e.g. `0 * * * *`). |

---

## ⚡ Zero-Config Frontend-to-Backend Dynamic Auto-Wiring

When a `klouds.yaml` declares both a static/web frontend and a backend API:
1. **Dynamic URL Binding**: kloudsPanel automatically resolves backend service URLs into the frontend:
   - `${services.<backend-name>.url}` -> `https://<actual-allocated-slug>.<root-domain>`
   - `${services.<backend-name>.host}` -> `<actual-allocated-slug>.<root-domain>`
   - `${services.<backend-name>.internalUrl}` -> `http://paas-svc-<actual-slug>:<port>`
2. **Auto-Injected Environment Variables**:
   - `VITE_API_URL: ${services.<backend-name>.url}/api`
   - `NEXT_PUBLIC_API_URL: ${services.<backend-name>.url}/api`
   - `REACT_APP_API_URL: ${services.<backend-name>.url}/api`
   - `API_URL: ${services.<backend-name>.url}/api`
   - `BACKEND_URL: ${services.<backend-name>.url}`
   - `INTERNAL_API_URL: http://paas-svc-<backend-name>:<port>`
3. **Nginx High-Performance Reverse Proxy**: The frontend container's Nginx configuration is automatically generated with an internal `/api/` proxy route forwarding directly to `http://paas-svc-<backend-slug>:<port>/api/`, preventing CORS issues out of the box!

---

## 📋 Complete Multi-Tier Stack Example (`klouds.yaml`)

```yaml
version: "1.0"
project: "my-startup-platform"

services:
  # --------------------------------------------------
  # Phase 1: Frontend Client (SvelteKit / React / Vite)
  # --------------------------------------------------
  frontend:
    type: static
    source:
      directory: "frontend"
    build:
      command: "npm ci && npm run build"
      output_dir: "dist"
    env:
      - key: VITE_API_URL
        fromService:
          name: "backend"
          property: "url" # Dynamically resolves to https://<backend-slug>.<root-domain>/api

  # --------------------------------------------------
  # Phase 2: REST / GraphQL Backend API (Go / Node / Python)
  # --------------------------------------------------
  backend:
    type: web
    source:
      directory: "backend"
    build:
      engine: "node" # node, go, python, rust, php, java, ruby, dockerfile
      command: "npm ci"
    deploy:
      port: 8080
      command: "node server.js"
    resources:
      cpu_limit: "1.0"
      mem_limit: "512m"
    env:
      # Auto-wire connection credentials from database
      - key: DATABASE_URL
        fromDatabase:
          name: "postgres-db"
          property: "connectionString"
      - key: DB_HOST
        fromDatabase:
          name: "postgres-db"
          property: "host"
      - key: DB_PORT
        fromDatabase:
          name: "postgres-db"
          property: "port"
      - key: REDIS_URL
        fromDatabase:
          name: "redis-cache"
          property: "connectionString"
      # Auto-generate cryptographically secure random secret
      - key: JWT_SECRET
        generateValue: true
      # Required variable prompting user on setup
      - key: OPENAI_API_KEY
        sync: false

  # --------------------------------------------------
  # Phase 3A: Relational Database (PostgreSQL 16)
  # --------------------------------------------------
  postgres-db:
    type: database
    image: "postgres:16-alpine"
    deploy:
      port: 5432
    volumes:
      - name: "pg_data"
        mount_path: "/var/lib/postgresql/data"
    env:
      - key: POSTGRES_DB
        value: "startup_prod"
      - key: POSTGRES_USER
        value: "postgres"

  # --------------------------------------------------
  # Phase 3B: In-Memory Cache & Broker (Redis 7.2)
  # --------------------------------------------------
  redis-cache:
    type: database
    image: "redis:7.2-alpine"
    deploy:
      port: 6379

  # --------------------------------------------------
  # Phase 4: Async Background Worker (Celery / BullMQ)
  # --------------------------------------------------
  queue-worker:
    type: worker
    source:
      directory: "backend"
    deploy:
      command: "node worker.js"
    env:
      - key: REDIS_URL
        fromDatabase:
          name: "redis-cache"
          property: "connectionString"

  # --------------------------------------------------
  # Phase 5: Scheduled Backup Cron Job
  # --------------------------------------------------
  nightly-backup:
    type: cron
    source:
      directory: "backend"
    schedule: "0 2 * * *" # Daily at 02:00 UTC
    deploy:
      command: "node backup.js"
```

---

## 🗄️ Supported Managed Databases

kloudsPanel natively deploys and wires **5 major database engines**:

| Engine | Version | Default Port | Internal Hostname Format | External Port Range |
| :--- | :--- | :--- | :--- | :--- |
| **PostgreSQL** | 16 Alpine | `5432` | `paas-db-<name>` | `15432-16432` |
| **MySQL** | 8.0 | `3306` | `paas-db-<name>` | `13306-14306` |
| **Redis** | 7.2 Alpine | `6379` | `paas-db-<name>` | `16379-17379` |
| **MongoDB** | 7.0 | `27017` | `paas-db-<name>` | `17017-18017` |
| **ClickHouse** | 24.3 Alpine | `8123` | `paas-db-<name>` | `18123-19123` |

---

## 🤖 AI Prompting Guide (Generate `klouds.yaml` for any Repo)

You can copy and paste this prompt into Claude, ChatGPT, or Gemini:

```text
Act as a Principal DevOps Architect. Generate a valid, production-ready `klouds.yaml` file for my repository:

Project Architecture:
- Project Name: "my-app"
- Frontend: Subdirectory "frontend" (React Vite static app, build with "npm run build", output "dist")
- Backend: Subdirectory "backend" (Node.js Express API on port 8080, start with "node index.js")
- Databases:
  - PostgreSQL 16 named "postgres-db"
  - Redis 7.2 named "redis-cache"

Rules:
1. Adhere strictly to kloudsPanel 1.0 schema specification.
2. Use type: static for frontend, type: web for backend, type: database for postgres-db and redis-cache.
3. Dynamically wire the backend URL into frontend VITE_API_URL using fromService: { name: "backend", property: "url" }.
4. Auto-wire database connections into backend environment using fromDatabase blocks.
5. Auto-generate JWT_SECRET with generateValue: true.
6. Output valid, clean YAML without markdown conversational text.
```
