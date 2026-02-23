# CLAUDE.md — VibeGuard Project Conventions

## Architecture

VibeGuard is a compliance-as-code tool with three services in a Turborepo + pnpm monorepo:

1. **VGX (Go)** — `apps/vgx/` — CLI scanner + REST API (:8080) + embedded React SPA
2. **Ingestion (Python)** — `apps/ingestion/` — FastAPI sidecar (:8001, internal only) for regulatory PDF parsing
3. **Dashboard (React)** — `apps/dashboard/` — Vite + React 19 + TypeScript, built and embedded into VGX binary

All services share **PostgreSQL 17**. Go proxies document uploads to the Python service.

## Quick Start

```bash
pnpm install                    # Install JS dependencies
turbo run build                 # Build everything (dashboard → VGX)
turbo run dev                   # Start all services in dev mode

# Podman (full stack with Postgres):
podman compose -f docker/docker-compose.yml up

# Or use Makefile (recommended):
make setup                      # First-time: install + db + migrate
make dev                        # Start all services
make db-migrate                 # Run migrations
make db-status                  # Check migration status
```

## Monorepo Orchestration

- Turborepo + pnpm workspaces manage all three services
- Go and Python apps have thin `package.json` wrappers so Turborepo can orchestrate them
- `turbo run build` builds dashboard first (Turborepo `dependsOn`), copies dist into Go embed dir, then compiles Go binary
- VGX's turbo.json has `"dependsOn": ["dashboard#build"]`

## Code Conventions

### Go (apps/vgx/)
- Standard layout: `cmd/` for entrypoints, `internal/` for private packages
- `pgx/v5` for database, Chi for routing, Cobra for CLI
- Clerk SDK v2 for auth middleware
- All handlers: JSON responses with `Content-Type: application/json`
- Error format: `{"error": "message", "code": "ERROR_CODE"}`
- Linting: `golangci-lint run ./...`

### Python (apps/ingestion/)
- Python 3.12+, type hints everywhere
- FastAPI + Pydantic v2, asyncpg for database
- Ruff for linting/formatting (`ruff check`, `ruff format`)
- Pytest + pytest-asyncio for tests
- Config via pydantic-settings (environment variables)

### TypeScript (apps/dashboard/)
- React 19 + TypeScript strict mode
- Vite for bundling, Tailwind CSS v4 for styling
- TanStack Query for server state
- Path alias: `@/` maps to `src/`
- ESLint with typescript-eslint

## Database

- PostgreSQL 17, connection via `DATABASE_URL` env var
- Migrations managed by **dbmate** — files in `apps/vgx/db/migrations/`
- Run migrations: `make db-migrate` (or `dbmate -d ./apps/vgx/db/migrations up`)
- Create new migration: `make db-new name=<name>` (uses `-- migrate:up` / `-- migrate:down` markers)
- Never modify existing migrations; always create new ones
- Schema owned by Go service only; Python reads/writes but never modifies schema

## Git Workflow

- `main` is always deployable
- Feature branches: `feat/short-description`
- Bug fixes: `fix/short-description`
- Conventional commits: `feat:`, `fix:`, `docs:`, `chore:`
- Squash merge to main

## Testing

- **Go:** `go test ./...` in `apps/vgx/`
- **Python:** `pytest` in `apps/ingestion/`
- **TypeScript:** `pnpm --filter dashboard lint` (tests added later)

## Security

- All API routes (except /health) require Clerk JWT
- Secrets via environment variables, never committed
- Audit log is append-only (DB triggers prevent UPDATE/DELETE)
- Ingestion service: internal-only (Podman `expose`, not `ports`)
- File uploads: PDF only, max 50MB
- SQL: parameterized queries only (pgx, asyncpg)

## Ingestion Pipeline (runs inside ingestion container)

All pipeline tools are Python libraries running in the `apps/ingestion/` service:

- **Marker API** (`httpx`) — converts regulatory PDFs to structured markdown (cloud API via MARKER_API_KEY)
- **PageIndex** — builds hierarchical tree index for cross-reference traversal (local)
- **LangExtract** — extracts rules from text with character-level source offsets (local)
- **OpenAI API** (`openai`) — translates legal rules into technical requirements (remote API call)

Pipeline stubs are in `apps/ingestion/src/pipeline/`. Implementation follows `docs/architecture/ingestion-pipeline.md`.

## Container Runtime

- **Podman 5.x** (aliased as `docker`) — compose files in `docker/`
- `podman compose -f docker/docker-compose.yml up` for full stack
- Ingestion service uses `expose` (internal only), never `ports`

## Key Paths

| Path | Purpose |
|------|---------|
| `apps/vgx/cmd/` | Go CLI entrypoints |
| `apps/vgx/internal/api/` | REST API handlers + router |
| `apps/vgx/internal/embed/` | Embedded SPA filesystem |
| `apps/vgx/db/migrations/` | SQL migration files (dbmate format) |
| `apps/ingestion/src/pipeline/` | PDF parsing + extraction pipeline |
| `apps/dashboard/src/pages/` | React page components |
| `docs/` | Architecture and development docs |
| `docker/` | Podman/Docker Compose configs |
