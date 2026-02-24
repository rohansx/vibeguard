# CLAUDE.md — VibeGuard Project Conventions

## Product

VibeGuard is a **persistent, live Security Property Graph (SPG)** that encodes the full security
semantics of a codebase — taint sources, sinks, sanitizers, trust boundaries, attack paths — and
exposes them to AI coding agents (Cursor, Claude Code) via MCP, so agents write secure code by
construction, not scan for vulnerabilities after the fact.

## Architecture

Two services in a Turborepo + pnpm monorepo:

1. **VGX (Go)** — `apps/vgx/` — SPG daemon + MCP server (stdio) + CLI + team sync REST API (:8080)
2. **Dashboard (React)** — `apps/dashboard/` — Vite + React 19 + TypeScript, embedded into VGX binary

Both share **PostgreSQL 17** (team sync metadata only — local SPG lives on-disk at `SPG_STORE_PATH`).

**Phase 2 addition:** `apps/vscode-extension/` — TypeScript + D3.js VS Code extension

## Quick Start

```bash
pnpm install                    # Install JS dependencies
turbo run build                 # Build everything (dashboard → VGX binary)
turbo run dev                   # Start dev mode

# MCP mode (for Cursor / Claude Code):
vgx init                        # Build initial SPG for current repo
vgx serve                       # Start SPG daemon + MCP server on stdio

# Team sync API + Postgres:
podman compose -f docker/docker-compose.yml up

# Makefile:
make setup                      # First-time: install + db + migrate
make dev                        # Start all services
make db-migrate                 # Run migrations
make db-status                  # Check migration status
```

## CLI Commands

| Command | Purpose |
|---------|---------|
| `vgx init [path]` | Build initial SPG for a repository |
| `vgx serve` | Start SPG daemon + MCP server (stdio, local only) |
| `vgx serve --api` | Also start team sync REST API on :8080 |
| `vgx query <tool> [args]` | Run a security query against the live SPG |
| `vgx report` | Generate structured security posture report |
| `vgx diff [ref]` | Show SPG changes introduced since a git ref |
| `vgx ci` | CI/CD mode: exit non-zero if PR introduces new taint paths |

## MCP Tools (8 deterministic query tools)

All tools return structured JSON. Same query + same code = same result always. No LLM in the
verification path — pure graph traversal.

| Tool | Security Question |
|------|------------------|
| `query_taint_paths(sink)` | Unsanitized paths from any SourceNode to this sink? |
| `get_attack_surface(module)` | All untrusted entry points for this module? |
| `calculate_blast_radius(function)` | What security properties change if I modify this function? |
| `find_missing_sanitizers()` | Where does tainted data reach a sink without sanitization? |
| `get_trust_boundary_violations()` | Which paths cross trust boundaries without auth checks? |
| `trace_data_flow(variable, file, line)` | Where does this variable travel through the codebase? |
| `check_auth_coverage(endpoint)` | Does this endpoint enforce auth before sensitive operations? |
| `get_security_context(file)` | Full security posture of this file? |

## Monorepo Orchestration

- Turborepo + pnpm workspaces manage both services
- `turbo run build` builds dashboard first, copies dist into Go embed dir, then compiles VGX binary
- VGX's `turbo.json` has `"dependsOn": ["dashboard#build"]`

## Code Conventions

### Go (apps/vgx/)
- Standard layout: `cmd/` for entrypoints, `internal/` for private packages
- `pgx/v5` for database, Chi for routing, Cobra for CLI
- All handlers: JSON responses with `Content-Type: application/json`
- Error format: `{"error": "message", "code": "ERROR_CODE"}`
- Linting: `golangci-lint run ./...`

### TypeScript (apps/dashboard/, apps/vscode-extension/)
- React 19 + TypeScript strict mode
- Vite for bundling, Tailwind CSS v4 for styling
- TanStack Query for server state
- Path alias: `@/` maps to `src/`
- ESLint with typescript-eslint

## Database (team sync metadata only)

- PostgreSQL 17, connection via `DATABASE_URL` env var
- Local SPG stored on-disk (Neo4j embedded) at `SPG_STORE_PATH` — NOT in PostgreSQL
- PostgreSQL holds: `repositories`, `spg_nodes`, `taint_paths`, `calibration_events`, `framework_classifiers`
- Migrations managed by **dbmate** — files in `apps/vgx/db/migrations/`
- Run migrations: `make db-migrate` (or `dbmate -d ./apps/vgx/db/migrations up`)
- Create new migration: `make db-new name=<name>` (uses `-- migrate:up` / `-- migrate:down` markers)
- Never modify existing migrations; always create new ones

## Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `DATABASE_URL` | For team sync API | PostgreSQL connection string |
| `ANTHROPIC_API_KEY` | Phase 2+ | Claude Sonnet for hybrid sanitizer classification |
| `CLERK_SECRET_KEY` | For team sync | Authentication for Pro/Team tier |
| `SPG_STORE_PATH` | No | Override local graph store path (default: `~/.vibeguard/graph`) |
| `SENTRY_DSN` | No | Error tracking |

## Git Workflow

- `main` is always deployable
- Feature branches: `feat/short-description`
- Bug fixes: `fix/short-description`
- Conventional commits: `feat:`, `fix:`, `docs:`, `chore:`
- Squash merge to main
- Compliance-as-code baseline preserved at `archive/compliance-v0`
- **Commit messages: single line only**, conventional commit format (e.g. `feat: add taint propagation engine`)
- **Never add `Co-Authored-By` lines** or multi-line commit bodies — keep it to one concise line

## Testing

- **Go:** `go test ./...` in `apps/vgx/`
- **TypeScript:** `pnpm --filter dashboard lint`

## Security

- MCP server runs on stdio transport only — no network exposure, no code leaves the machine
- All team sync API routes (except /health) require Clerk JWT
- `calibration_events` table is append-only (DB triggers prevent UPDATE/DELETE)
- SQL: parameterized queries only (pgx)
- Secrets via environment variables, never committed

## Key Paths

| Path | Purpose |
|------|---------|
| `apps/vgx/cmd/vgx/` | CLI entrypoints (init, serve, query, report, diff, ci) |
| `apps/vgx/internal/parser/` | Tree-sitter incremental AST (Phase 1) |
| `apps/vgx/internal/graph/` | SPG construction + Neo4j persistence (Phase 1) |
| `apps/vgx/internal/taint/` | Taint propagation + differential dataflow (Phase 1) |
| `apps/vgx/internal/mcp/` | MCP server + 8 security query tools (Phase 1) |
| `apps/vgx/internal/watcher/` | File watcher — inotify/FSEvents (Phase 1) |
| `apps/vgx/internal/api/` | Team sync REST API handlers |
| `apps/vgx/internal/embed/` | Embedded SPA filesystem |
| `apps/vgx/db/migrations/` | SQL migration files (dbmate format) |
| `apps/dashboard/src/pages/` | React page components |
| `apps/vscode-extension/` | VS Code extension — D3.js attack path viz (Phase 2) |
| `docs/` | Architecture and development docs |
| `docker/` | Podman/Docker Compose configs |
