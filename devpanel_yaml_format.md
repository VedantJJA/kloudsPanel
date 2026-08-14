# DevPanel & kloudsPanel Specification (`devpanel.yaml`)

`devpanel.yaml` is the declarative infrastructure-as-code specification for **kloudsPanel / DevPanel PaaS**. It allows any single repository or monorepo to declare multi-tier stacks (frontends, backends, databases, background workers, cron jobs) with automatic networking, SSL, health checks, and environment variable bindings.

Place this file in the **root directory** of your Git repository as `devpanel.yaml` (or `render.yaml`).

---

## 🌟 The 5-Phase Service Topology

DevPanel classifies application components into **5 distinct phases**:

| Phase | Service Type | Runtime Engines | Key Characteristics |
| :--- | :--- | :--- | :--- |
| **Phase 1** | **Static SPA** (`type: static`) | Vite, React, Vue, SvelteKit, Astro, Next.js Static | Compiled to static assets (`/dist`, `/build`), served by high-performance Nginx with automated `/api/` reverse-proxying. |
| **Phase 2** | **Dynamic Web & APIs** (`type: web`) | Node.js, Python, Go, Rust, Java, PHP, Ruby, Bun, Deno, Dockerfile | Long-running HTTP/gRPC server exposed on an internal container port and routed with auto-SSL. |
| **Phase 3** | **Managed Databases** (`type: database`) | PostgreSQL 16, MySQL 8.0, Redis 7.2, MongoDB 7.0, ClickHouse 24.3 | Containerized stateful engines with persistent Docker volumes and dedicated public/internal connection URIs. |
| **Phase 4** | **Background Workers** (`type: worker`) | Celery, BullMQ, Sidekiq, RabbitMQ/Kafka Consumers | Headless persistent background processes without public HTTP ingress. |
| **Phase 5** | **Scheduled Cron Jobs** (`type: cron`) | Any runtime | Ephemeral containers executed on a standard 5-field cron schedule (e.g. `0 * * * *`). |

---

## ⚡ Zero-Config Frontend-to-Backend Auto-Wiring

When a `devpanel.yaml` declares both a static/web frontend and a backend API:
1. **Build-Time Ingress Injection**: DevPanel automatically injects the backend's public and internal URLs into the frontend build context:
   - `VITE_API_URL: https://<backend-slug>.klouds.online/api`
   - `NEXT_PUBLIC_API_URL: https://<backend-slug>.klouds.online/api`
   - `REACT_APP_API_URL: https://<backend-slug>.klouds.online/api`
   - `API_URL: https://<backend-slug>.klouds.online/api`
   - `BACKEND_URL: https://<backend-slug>.klouds.online`
   - `INTERNAL_API_URL: http://paas-svc-<backend-slug>:<port>`
2. **Nginx High-Performance Reverse Proxy**: The frontend container's Nginx configuration is automatically generated with an internal `/api/` proxy route forwarding directly to `http://paas-svc-<backend-slug>:<port>/api/`, preventing CORS issues out of the box!

---

## 📋 Complete Multi-Tier Stack Example (`devpanel.yaml`)

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
    domains:
      - "app.klouds.online"

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

DevPanel natively deploys and wires **5 major database engines**:

| Engine | Version | Default Port | Internal Hostname Format | External Port Range |
| :--- | :--- | :--- | :--- | :--- |
| **PostgreSQL** | 16 Alpine | `5432` | `paas-db-<name>` | `15432-16432` |
| **MySQL** | 8.0 | `3306` | `paas-db-<name>` | `13306-14306` |
| **Redis** | 7.2 Alpine | `6379` | `paas-db-<name>` | `16379-17379` |
| **MongoDB** | 7.0 | `27017` | `paas-db-<name>` | `17017-18017` |
| **ClickHouse** | 24.3 Alpine | `8123` | `paas-db-<name>` | `18123-19123` |

---

## 🛠️ Schema Reference Guide

### 1. Service Definition (`services.<name>`)
* **`type`**: `"static"`, `"web"`, `"database"`, `"worker"`, `"cron"`
* **`source.directory`**: Monorepo subfolder path (e.g. `frontend`, `api`, `web`). Default: `.` (root).
* **`source.branch`**: Target branch. Default: `main`.
* **`build.engine`**: Preset engine (`node`, `go`, `python`, `rust`, `java`, `php`, `ruby`, `static`, `dockerfile`).
* **`build.command`**: Shell command executed during build phase (`npm run build`, `go build`, `cargo build --release`).
* **`build.output_dir`**: Static asset directory (`dist`, `build`, `public`).
* **`deploy.port`**: Internal TCP listening port (`8080`, `3000`, `5000`, `80`).
* **`deploy.command`**: Runtime startup command (`node index.js`, `./server`, `gunicorn app:app`).
* **`schedule`**: Cron schedule expression for cron services (e.g. `*/15 * * * *`).

### 2. Environment Variables (`env`)
* **Plain value**: `- key: PORT\n  value: "8080"`
* **Auto-generated secret**: `- key: JWT_SECRET\n  generateValue: true`
* **Required input prompt**: `- key: STRIPE_KEY\n  sync: false`
* **From Database reference**:
  ```yaml
  - key: DATABASE_URL
    fromDatabase:
      name: "postgres-db"
      property: "connectionString" # host, port, user, password, database
  ```
* **From Service reference**:
  ```yaml
  - key: API_BASE_URL
    fromService:
      name: "backend"
      property: "url" # host, port, internalUrl
  ```

---

## 🤖 AI Prompting Guide (Generate `devpanel.yaml` for any Repo)

You can copy and paste this prompt into Claude, ChatGPT, or Gemini:

```text
Act as a Principal DevOps Architect. Generate a valid, production-ready `devpanel.yaml` file for my repository:

Project Architecture:
- Project Name: "my-app"
- Frontend: Subdirectory "frontend" (React Vite static app, build with "npm run build", output "dist")
- Backend: Subdirectory "backend" (Node.js Express API on port 8080, start with "node index.js")
- Databases:
  - PostgreSQL 16 named "postgres-db"
  - Redis 7.2 named "redis-cache"

Rules:
1. Adhere strictly to DevPanel 1.0 schema specification.
2. Use type: static for frontend, type: web for backend, type: database for postgres-db and redis-cache.
3. Auto-wire database connections into backend environment using fromDatabase blocks.
4. Auto-generate JWT_SECRET with generateValue: true.
5. Output valid, clean YAML without markdown conversational text.
```