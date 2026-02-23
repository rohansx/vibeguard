# VibeGuard MVP — Implementation Roadmap

**Created:** 2026-02-10 | **Target:** 9-week MVP build | **Status:** In Progress

---

## Overview

VibeGuard is a compliance-as-code tool that scans source code against regulatory frameworks. The MVP targets **GDPR only** with a full document ingestion pipeline, deterministic code scanning, and a compliance dashboard.

### Three Services

| Service | Language | Port | Role |
|---------|----------|------|------|
| **VGX** | Go | :8080 | CLI scanner + REST API + embedded React SPA |
| **Ingestion** | Python | :8001 (internal) | PDF parsing → structured compliance rules |
| **Dashboard** | TypeScript | embedded in VGX | Compliance dashboard, document upload, rule review |

### Key Tech Choices

- **Monorepo:** Turborepo + pnpm
- **Go router:** Chi
- **Frontend:** Vite + React 19 + TypeScript, embedded into Go binary via `go:embed`
- **Database:** PostgreSQL 17
- **Auth:** Clerk
- **Deployment:** Docker Compose on Hetzner CX33 (4 vCPU, 8 GB RAM)

---

## Monorepo Structure

```
vibeguard-io/
├── CLAUDE.md
├── README.md
├── package.json                      # Root (devDeps: turbo)
├── pnpm-workspace.yaml
├── turbo.json
├── .gitignore / .editorconfig / .tool-versions
│
├── apps/
│   ├── vgx/                          # Go binary
│   │   ├── cmd/vgx/                  # Cobra CLI entrypoints
│   │   ├── internal/
│   │   │   ├── api/                  # Chi router + handlers
│   │   │   ├── db/                   # pgx pool + migration runner
│   │   │   ├── embed/               # go:embed SPA serving
│   │   │   ├── scanner/             # ast-grep orchestration
│   │   │   └── config/              # Env-based config
│   │   ├── migrations/              # SQL migration files
│   │   ├── go.mod / Dockerfile / package.json / turbo.json
│   │
│   ├── ingestion/                    # Python FastAPI sidecar
│   │   ├── src/
│   │   │   ├── main.py              # FastAPI app
│   │   │   ├── config.py
│   │   │   ├── pipeline/            # Marker, PageIndex, LangExtract, Claude
│   │   │   ├── models/
│   │   │   └── db/
│   │   ├── tests/
│   │   ├── pyproject.toml / Dockerfile / package.json
│   │
│   └── dashboard/                    # React SPA
│       ├── src/ (pages, components, lib, types)
│       ├── vite.config.ts / tailwind.config.ts / package.json
│
├── docs/                             # All project documentation
├── docker/                           # Docker Compose configs
└── .github/workflows/                # CI/CD
```

---

## Implementation Steps

### Step 0: Planning Documents
- [x] Create `docs/planning/mvp-implementation-roadmap.md` (this file)
- [x] Create `docs/planning/tech-decisions.md`
- [x] Create `docs/planning/progress.md`
- [x] Create `docs/planning/product-roadmap.md`

### Step 1: Monorepo Root
- [ ] `package.json` with turbo devDependency
- [ ] `pnpm-workspace.yaml` defining `apps/*` and `packages/*`
- [ ] `turbo.json` with build/dev/test/lint/clean tasks
- [ ] `.gitignore` (Go, Python, Node, IDE)
- [ ] `.editorconfig` (2-space for TS/JSON, 4-space for Python, tab for Go)
- [ ] `.tool-versions` (go 1.23, python 3.12, node 20)
- [ ] `git init && pnpm install`

### Step 2: React Dashboard
- [ ] Scaffold with `pnpm create vite apps/dashboard --template react-ts`
- [ ] Install: tailwindcss, @clerk/clerk-react, react-router, @tanstack/react-query, lucide-react
- [ ] Configure `vite.config.ts` with API proxy to localhost:8080
- [ ] Set up Tailwind + shadcn/ui foundation
- [ ] Create minimal `App.tsx` with router + placeholder pages
- [ ] Verify: `pnpm --filter dashboard dev` starts on :5173

### Step 3: Go Module (VGX)
- [ ] `go mod init github.com/vibeguard/vgx`
- [ ] Install deps: chi, pgx, clerk-sdk-go, cobra, godotenv
- [ ] Create thin `package.json` + `turbo.json` (with `dependsOn: ["dashboard#build"]`)
- [ ] `cmd/vgx/main.go` — Cobra root + serve + migrate commands
- [ ] `internal/api/router.go` — Chi router + Clerk middleware
- [ ] `internal/api/handlers/health.go`
- [ ] `internal/embed/spa.go` — go:embed directive + SPA handler
- [ ] `internal/config/config.go` — env-based config struct
- [ ] `internal/db/db.go` — pgx connection pool
- [ ] Verify: `go run ./cmd/vgx serve` → `/api/v1/health` returns JSON

### Step 4: Python Ingestion Service
- [ ] `pyproject.toml` with FastAPI, uvicorn, marker-pdf, langextract, pageindex, anthropic
- [ ] Thin `package.json` for Turborepo
- [ ] `src/main.py` — FastAPI app with `/health` and `/ingest` endpoints
- [ ] `src/config.py` — pydantic-settings
- [ ] Pipeline stubs: orchestrator, pdf_parser, tree_indexer, rule_extractor, technical_translator
- [ ] `tests/test_health.py`
- [ ] Verify: `uvicorn src.main:app --port 8001` → `/health` returns JSON

### Step 5: Database Migration
- [ ] `migrations/001_initial_schema.sql` — 7 tables with indexes + audit triggers
  - organizations, users, documents, proposed_rules, compliance_rules, scan_results, audit_log
- [ ] Implement `runMigrate` in Go binary (read + execute SQL files)
- [ ] Verify against running PostgreSQL

### Step 6: Docker Setup
- [ ] `docker/docker-compose.yml` — 3 services + PostgreSQL
- [ ] `docker/docker-compose.dev.yml` — dev overrides
- [ ] `docker/.env.example`
- [ ] `apps/vgx/Dockerfile` — multi-stage (dashboard → Go → alpine)
- [ ] `apps/ingestion/Dockerfile`
- [ ] Verify: `docker compose up --build` → all healthy

### Step 7: Wire Embedded SPA
- [ ] VGX build script copies `../dashboard/dist` → `internal/embed/dist`
- [ ] `turbo run build` chains: dashboard build → copy → Go compile
- [ ] Verify: Go binary serves React at `/` and API at `/api/v1/`

### Step 8: CLAUDE.md + Documentation
- [ ] `CLAUDE.md` — architecture, conventions, workflow, security
- [ ] `README.md` — project overview + quickstart
- [ ] Move + reorganize spec docs into `docs/architecture/`
- [ ] `docs/development/getting-started.md`

### Step 9: GitHub Actions CI
- [ ] `.github/workflows/ci.yml` — lint + test + build (Go, Python, TypeScript)

### Step 10: Git Init + Initial Commit
- [ ] Initialize git repository
- [ ] Initial commit with full monorepo scaffold

---

## Key Design Decisions

1. **Thin `package.json` wrappers** — Go and Python apps get package.json files where scripts delegate to native commands (`go build`, `pytest`). This lets Turborepo orchestrate all three languages.

2. **`dependsOn: ["dashboard#build"]`** — VGX's turbo.json declares a dependency on the dashboard build. Turborepo ensures React builds first so Go can embed the output.

3. **Migrations owned by Go** — Single source of truth for schema. Both services read from PostgreSQL, but only VGX manages migrations.

4. **SPA via `go:embed`** — Single binary serves both API and UI. No CORS, no separate deployment. During dev, Vite's proxy sends API calls to Go.

5. **Ingestion is internal-only** — Uses Docker `expose` (not `ports`). Never accessible from the internet. Go proxies uploads to it.

6. **Append-only audit log** — PostgreSQL triggers prevent UPDATE and DELETE on the audit_log table at the database level.

---

## Verification Checklist

After all steps complete:
- [ ] `pnpm install && turbo run build` succeeds from clean clone
- [ ] `docker compose -f docker/docker-compose.yml up` → all services healthy
- [ ] `curl localhost:8080/api/v1/health` → `{"status":"ok","service":"vgx"}`
- [ ] `curl localhost:8001/health` (Docker internal) → `{"status":"ok","service":"ingestion"}`
- [ ] Browser at `localhost:8080/` loads React SPA
- [ ] `vgx migrate` creates all 7 tables in PostgreSQL
