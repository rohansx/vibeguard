# VibeGuard — Build Progress Tracker

**Last updated:** 2026-02-11 | **Current phase:** MVP Feature Implementation

---

## Phase 1: MVP Scaffolding (Complete)

### Step 0: Planning Documents
- [x] `docs/planning/mvp-implementation-roadmap.md`
- [x] `docs/planning/tech-decisions.md`
- [x] `docs/planning/progress.md` (this file)
- [x] `docs/planning/product-roadmap.md`

### Step 1: Monorepo Root
- [x] `package.json` (root)
- [x] `pnpm-workspace.yaml`
- [x] `turbo.json`
- [x] `.gitignore`
- [x] `.editorconfig`
- [x] `.tool-versions`
- [x] `git init && pnpm install`

### Step 2: React Dashboard
- [x] Vite + React + TypeScript scaffold
- [x] Tailwind CSS v4 setup
- [x] Vite proxy to Go API
- [x] App.tsx with router + layout + sidebar
- [x] Pages: Dashboard, Rules, Documents, Scans
- [x] Verify: `pnpm --filter dashboard build` succeeds

### Step 3: Go Module (VGX)
- [x] `go.mod` initialized (chi, pgx, cobra)
- [x] Cobra CLI (serve, migrate subcommands)
- [x] Chi router + health handler + placeholder routes
- [x] go:embed SPA serving with index.html fallback
- [x] Config + DB connection pool (pgx/v5)
- [x] Verify: `go run ./cmd/vgx serve` starts, `/api/v1/health` returns JSON

### Step 4: Python Ingestion Service
- [x] `pyproject.toml` + FastAPI app
- [x] `/health` + `/ingest` endpoints
- [x] Pipeline stubs (Marker, PageIndex, LangExtract, TechnicalTranslator, Orchestrator)
- [x] Pydantic models (Document, ProposedRule)
- [x] Tests (test_health.py)

### Step 5: Database Migration
- [x] 001_initial_schema.sql (7 tables: organizations, users, documents, proposed_rules, compliance_rules, scan_results, audit_log)
- [x] Migration runner in Go (embedded SQL via migrations package)
- [x] Append-only audit log triggers

### Step 6: Docker Setup
- [x] docker-compose.yml (app + ingestion + postgres)
- [x] docker-compose.dev.yml (hot reload overrides)
- [x] .env.example
- [x] VGX Dockerfile (multi-stage: node → go → debian-slim)
- [x] Ingestion Dockerfile (python:3.12-slim)

### Step 7: Wire Embedded SPA
- [x] VGX prebuild script copies dashboard dist → Go embed dir
- [x] turbo.json `dependsOn: ["dashboard#build"]` chains correctly
- [x] `turbo run build` succeeds (~8.7s) — dashboard → copy → Go compile
- [x] Go binary serves SPA at `/` and API at `/api/v1/`
- [x] `vgx --version` → `vgx version 0.1.0`

### Step 8: CLAUDE.md + Documentation
- [x] CLAUDE.md (conventions, architecture, code standards, security)
- [x] README.md (quickstart, architecture, project structure)
- [x] Architecture docs reorganized (`docs/architecture/overview.md`, `ingestion-pipeline.md`)

### Step 9: GitHub Actions CI
- [x] `.github/workflows/ci.yml` (PostgreSQL service, pnpm/node/go/python setup, lint, test, build)

---

## Phase 2: Dashboard + API Wiring (Complete)

### Step 10: Dashboard UI
- [x] Real data from PostgreSQL via TanStack Query
- [x] Monochrome black/white design with dark mode toggle
- [x] Dashboard page: compliance score, recent scans, findings by severity
- [x] Rules page: list all compliance rules with regulation/severity badges
- [x] Documents page: uploaded documents with status badges
- [x] Scans page: scan history with findings breakdown

### Step 11: Go API Handlers
- [x] `GET /api/v1/rules` — list active compliance rules
- [x] `GET /api/v1/documents` — list documents
- [x] `GET /api/v1/scans` — list scans (most recent first)
- [x] `GET /api/v1/health` — service health check
- [x] Seed data migration (demo org, users, rules, documents, scans)

### Step 12: OpenAI Migration
- [x] Switched all LLM calls from Anthropic/Claude to OpenAI (gpt-4o)
- [x] Updated `docker/.env.example` and `docker/.env` with `OPENAI_API_KEY`
- [x] Updated Python config to use `OPENAI_API_KEY`

---

## Phase 3: Scanner + Ingestion Pipeline (Complete)

### Step 13: Python Ingestion Pipeline
- [x] `pdf_parser.py` — Marker cloud API integration via httpx (uses `MARKER_API_KEY`)
- [x] `tree_indexer.py` — heading-based markdown tree parser (Chapters → Articles → Paragraphs)
- [x] `rule_extractor.py` — OpenAI gpt-4o structured output extraction with chunk splitting + dedup
- [x] `technical_translator.py` — OpenAI legal→technical translation (generates ast-grep patterns)
- [x] `cross_references.py` — regex-based Article/Recital/Section reference resolver
- [x] `orchestrator.py` — chains all 5 stages, stores proposed_rules in DB, updates document status
- [x] `main.py` — wired `BackgroundTasks` for async extraction after upload

### Step 14: Go Scanner
- [x] `scanner.go` — dual-strategy scanner: ast-grep for AST patterns + grep-based fallback
  - Loads active rules from DB, writes temporary YAML rule files
  - Runs `sg scan --config sgconfig.yml` as subprocess, parses NDJSON output
  - Falls back to `grep -rlE` for rules without ast-grep patterns
  - Computes weighted compliance score, stores results in `scan_results` table
- [x] `scan.go` — `vgx scan <path>` CLI command (Cobra) with `--branch`, `--org-id`, `--repo-url`, `--output` flags
- [x] `main.go` — registered `scanCmd` in root command

### Step 15: Scan API
- [x] `POST /api/v1/scans` — accepts `{repository_url, branch, org_id}`, clones repo, runs scanner in background goroutine
- [x] `GET /api/v1/scans/{scanID}` — returns scan details with full findings JSONB
- [x] `POST /api/v1/documents` — proxies multipart upload to ingestion service

### Step 16: Dockerfile + Runtime
- [x] Switched runtime from `alpine:3.21` to `debian:bookworm-slim` (glibc compatibility for ast-grep)
- [x] Installs ast-grep v0.40.5 from GitHub releases (both `sg` and `ast-grep` binaries)
- [x] Installs `git` and `grep` for scanner operations

### Step 17: Seed Data with ast-grep Rules
- [x] Migration `20260212000000_add_ast_grep_rules.sql`
- [x] GDPR-ENC-001: detects unencrypted DB connections, localStorage usage
- [x] GDPR-ENC-002: detects HTTP URLs, disabled TLS verification
- [x] GDPR-CNS-001: detects cookies without consent, tracking pixels
- [x] PCI-LOG-001: detects print/fmt.Println instead of structured logging

### Verification
- [x] `sg --version` → ast-grep 0.40.5 (inside container)
- [x] `vgx scan /tmp/test-project` → 7 findings (2 ast-grep + 5 grep), score 54.5%
- [x] `GET /api/v1/scans` → lists all scans with metadata
- [x] `GET /api/v1/scans/{id}` → full scan details with findings JSON
- [x] `POST /api/v1/documents` → proxied to ingestion, background extraction runs
- [x] All 3 containers build and run (`make up`)

---

## Current Blockers

_None._

---

## Next Actions

1. Initial git commit
2. Human review UI for proposed rules (approve/reject/edit workflow)
3. Dashboard integration with scan API (trigger scans from UI)
4. VS Code extension (inline compliance diagnostics)
5. GitHub Action for CI/CD pipeline checks

---

## Session Log

| Date | Work Done | Notes |
|------|-----------|-------|
| 2026-02-10 | Steps 0-9: full monorepo scaffolded | Turborepo + pnpm, Go/Python/React services, Docker, CI, docs |
| 2026-02-10 | Switched to dbmate, Podman, added pipeline deps | Removed custom Go migration runner, updated docs to Podman |
| 2026-02-11 | Steps 10-12: dashboard + API wiring | Real data from DB, monochrome UI with dark mode, 4 API endpoints, OpenAI migration |
| 2026-02-11 | Steps 13-17: scanner + ingestion pipeline | Full Python pipeline (Marker → OpenAI → ast-grep patterns), Go scanner with ast-grep + grep, CLI command, scan API, document upload proxy, debian-slim runtime |
