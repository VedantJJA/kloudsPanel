# ADR-0001: SQLite as Primary Database

## Status
Accepted

## Context
kloudsPanel targets a single Linux host with no external database dependency requirement. The platform must work out-of-the-box without a DBA, without PostgreSQL, and without connection pooling infrastructure.

## Decision
Use SQLite with WAL journal mode as the primary database for the control plane state (users, workspaces, projects, services, deployments, jobs, secrets, audit log).

Key configuration:
- `journal_mode=WAL` — concurrent readers + single writer, no blocking
- `synchronous=NORMAL` — safe with WAL
- `busy_timeout=5000` — 5-second retry window for write contention
- `foreign_keys=ON`
- `cache_size=-64000` — 64 MB in-memory page cache

Migration strategy: append-only DDL in Go `embed.FS` via forward-only migrations.

## Consequences

**Positive:**
- Zero external dependency — `apt install` not required
- Trivial backup: `cp klouds.db klouds.db.bak` or VACUUM INTO
- Excellent read performance for typical PaaS control-plane loads
- `go-sqlite3` (CGO) provides battle-tested SQLite bindings

**Negative:**
- Single-host only — no built-in replication
- CGO required for go-sqlite3 (adds compilation complexity)
- Write throughput limited to single writer

**Mitigation:**
- PostgreSQL driver interface is designed and co-exists behind `DB_DRIVER=postgres`
- Write throughput for control plane (config mutations, not data) is low in practice
- SQLite Litestream can provide continuous WAL replication to S3 if required

## Alternatives Considered
- **PostgreSQL** — excellent, but requires external process, more operational complexity
- **bbolt/buntdb** — embedded KV, not relational enough for complex queries
- **TiDB serverless** — too complex for self-hosted target
