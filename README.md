# VibeGuard

Compliance-as-code scanner that verifies source code against regulatory frameworks (GDPR, PCI DSS, SOC 2). Every finding traces from code to the exact regulation text that requires it.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        VIBEGUARD STACK                           │
│                                                                  │
│  ┌────────────────────┐  ┌─────────────────────────────────────┐│
│  │  Ingestion Service  │  │  VGX (Go binary)                   ││
│  │  (Python/FastAPI)   │  │                                     ││
│  │                     │  │  • CLI scanner (vgx scan)           ││
│  │  • Marker API (PDF) │  │  • REST API (:8080)                ││
│  │  • OpenAI (extract) │  │  • Embedded React dashboard        ││
│  │  • OpenAI (translate)│  │  • ast-grep + pattern matching     ││
│  │                     │  │                                     ││
│  │  :8001 (internal)   │  │  :8080 (public)                    ││
│  └─────────┬───────────┘  └──────────────┬──────────────────────┘│
│            │                             │                       │
│            └──────────┬──────────────────┘                       │
│                       │                                          │
│            ┌──────────▼──────────┐                               │
│            │   PostgreSQL 17     │                               │
│            │   :5432             │                               │
│            └─────────────────────┘                               │
└──────────────────────────────────────────────────────────────────┘
```

| Service | Language | Port | Role |
|---------|----------|------|------|
| **VGX** | Go | :8080 | CLI scanner + REST API + embedded React dashboard |
| **Ingestion** | Python | :8001 | Regulatory PDF parsing + rule extraction (internal only) |
| **Dashboard** | React/TS | embedded | Compliance dashboard (built into VGX binary) |
| **PostgreSQL** | — | :5432 | Shared database for all services |

## Quick Start

### Prerequisites

- [Podman](https://podman.io/) 5.x + Podman Compose (or Docker)
- [Node.js](https://nodejs.org/) 20+
- [pnpm](https://pnpm.io/) 9+
- [Go](https://golang.org/) 1.25+
- [Python](https://python.org/) 3.12+
- [dbmate](https://github.com/amacneil/dbmate) (migration tool)

### 1. Clone and configure

```bash
git clone <repo-url>
cd vibeguard-io
cp docker/.env.example docker/.env
```

Edit `docker/.env` with your API keys:

```env
DB_PASSWORD=change-me-in-production
OPENAI_API_KEY=sk-...          # Required for ingestion pipeline
MARKER_API_KEY=...             # Required for PDF parsing (datalab.to)
CLERK_SECRET_KEY=sk_test_...   # Optional (auth disabled in dev)
```

### 2. Start all services

```bash
make up
```

This builds all three containers and starts them with PostgreSQL. Migrations run automatically on startup.

- **Dashboard:** http://localhost:8080
- **API health:** http://localhost:8080/api/v1/health
- **PostgreSQL:** localhost:5432

### 3. Verify

```bash
# Health check
curl http://localhost:8080/api/v1/health

# List compliance rules
curl http://localhost:8080/api/v1/rules

# List scans
curl http://localhost:8080/api/v1/scans
```

### Other Make targets

```bash
make setup       # First-time: build + start db + run migrations
make dev         # Start with live rebuild (foreground)
make down        # Stop all services
make logs        # Tail logs from all services
make clean       # Stop services and remove volumes
make db-migrate  # Run pending migrations
make db-status   # Show migration status
make db-new name=add_feature  # Create new migration
```

## CLI Scanner

The `vgx scan` command scans a local codebase for compliance violations against all active rules in the database.

### Usage

```bash
# Inside the container
podman exec vibeguard-api vgx scan /path/to/project

# With options
podman exec vibeguard-api vgx scan /path/to/project \
  --branch main \
  --org-id org_demo_001 \
  --repo-url https://github.com/org/repo \
  --output json
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--branch` | `main` | Branch name (metadata only) |
| `--org-id` | `org_demo_001` | Organization ID for scan record |
| `--repo-url` | (empty) | Repository URL (metadata only) |
| `-o, --output` | `table` | Output format: `table` or `json` |

### Example output

```
  VibeGuard Compliance Scan
  ========================

  Target:     /tmp/test-project
  Branch:     main
  Rules:      11 checked
  Score:      54.5%

  Findings (7 total):
  SEVERITY   RULE                          MESSAGE                              LOCATION
  --------   ----                          -------                              --------
  high       pci-log-001-py-print-debug    PCI DSS Req. 10: Using print()...   src/db.py:5
  high       gdpr-enc-001-ts-plaintext-... GDPR Art. 32: Storing data in...    src/app.ts:3
  critical   PCI-ENC-001                   Encrypt cardholder data in transit.. .

  Summary: 3 critical, 4 high, 0 medium, 0 low
```

### How it works

1. Loads all active compliance rules from PostgreSQL
2. Rules with `ast_grep_rules` in `technical_checks` → writes temporary YAML rule files, runs `sg scan` (ast-grep) as a subprocess for AST-level pattern matching
3. Rules with `checks` list → runs `grep`-based pattern matching for keyword/config detection
4. Aggregates findings by severity, computes weighted compliance score
5. Stores scan result in `scan_results` table

## REST API

All endpoints are under `/api/v1/`. Responses are JSON.

### Scans

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/scans` | List all scans (most recent first) |
| `POST` | `/scans` | Trigger a new scan against a repository |
| `GET` | `/scans/{scanID}` | Get scan details with findings |

**POST /scans** — trigger a scan:

```bash
curl -X POST http://localhost:8080/api/v1/scans \
  -H 'Content-Type: application/json' \
  -d '{
    "repository_url": "https://github.com/org/repo",
    "branch": "main",
    "org_id": "org_demo_001"
  }'
```

Returns `202 Accepted` with `{"scan_id": "...", "status": "running"}`. The scan runs in the background — poll `GET /scans/{scanID}` for results.

### Documents

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/documents` | List all documents |
| `POST` | `/documents` | Upload a regulatory PDF for extraction |

**POST /documents** — upload a regulatory PDF:

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -F "file=@gdpr-regulation.pdf" \
  -F "name=GDPR Full Text" \
  -F "org_id=org_demo_001" \
  -F "uploaded_by=usr_demo_admin"
```

The Go API proxies the upload to the Python ingestion service, which:
1. Parses the PDF via Marker cloud API
2. Builds a heading-based tree index
3. Extracts rules via OpenAI (gpt-4o)
4. Resolves cross-references between articles
5. Translates legal requirements into technical checks + ast-grep patterns
6. Stores proposed rules in `proposed_rules` table

Poll `GET /documents` to track status progression: `processing` → `review_ready` (or `extraction_failed`).

### Rules

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/rules` | List all active compliance rules |

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Service health check |

## Ingestion Pipeline

The Python ingestion service (`apps/ingestion/`) processes regulatory PDFs into structured compliance rules. All pipeline stages are in `apps/ingestion/src/pipeline/`:

| Stage | File | Tool | Description |
|-------|------|------|-------------|
| 1. Parse PDF | `pdf_parser.py` | Marker cloud API | Converts PDF to structured markdown |
| 2. Index | `tree_indexer.py` | Custom parser | Builds heading tree (Chapters → Articles → Paragraphs) |
| 3. Extract | `rule_extractor.py` | OpenAI gpt-4o | Extracts compliance rules with structured JSON output |
| 4. Cross-refs | `cross_references.py` | Regex + tree | Resolves Article/Recital/Section references |
| 5. Translate | `technical_translator.py` | OpenAI gpt-4o | Converts legal rules → technical checks + ast-grep patterns |
| 6. Orchestrate | `orchestrator.py` | — | Chains all stages, stores results in DB |

### Required API keys

| Key | Service | Purpose |
|-----|---------|---------|
| `OPENAI_API_KEY` | OpenAI | Rule extraction + technical translation |
| `MARKER_API_KEY` | Datalab (datalab.to) | PDF → markdown conversion |

## Database

PostgreSQL 17 with 7 tables:

| Table | Purpose |
|-------|---------|
| `organizations` | Multi-tenant orgs (Clerk integration) |
| `users` | User accounts linked to Clerk |
| `documents` | Uploaded regulatory PDFs + extraction status |
| `proposed_rules` | AI-extracted rules pending human review |
| `compliance_rules` | Active rules the scanner checks against |
| `scan_results` | Scan history with findings (JSONB) |
| `audit_log` | Append-only audit trail (triggers prevent UPDATE/DELETE) |

Migrations managed by [dbmate](https://github.com/amacneil/dbmate) in `apps/vgx/db/migrations/`.

## Project Structure

```
vibeguard-io/
├── apps/
│   ├── vgx/                    # Go binary (CLI + API + embedded SPA)
│   │   ├── cmd/vgx/            # CLI entrypoints (serve, scan)
│   │   ├── internal/
│   │   │   ├── api/            # REST API handlers + router
│   │   │   ├── config/         # Configuration
│   │   │   ├── embed/          # Embedded SPA filesystem
│   │   │   └── scanner/        # Compliance scanner (ast-grep + grep)
│   │   └── db/migrations/      # SQL migration files (dbmate)
│   ├── ingestion/              # Python FastAPI sidecar
│   │   └── src/pipeline/       # PDF parsing + extraction pipeline
│   └── dashboard/              # React SPA (Vite + Tailwind)
│       └── src/pages/          # Dashboard, Rules, Documents, Scans
├── docs/                       # Architecture and planning docs
├── docker/                     # Podman/Docker Compose configs
├── Makefile                    # Build/dev commands
└── CLAUDE.md                   # AI assistant conventions
```

## Development (without containers)

For local development without Podman:

```bash
pnpm install

# Start just PostgreSQL
podman compose -f docker/docker-compose.yml up vibeguard-db -d

# Run migrations
export DATABASE_URL="postgres://vibeguard:vibeguard@localhost:5432/vibeguard?sslmode=disable"
dbmate -d ./apps/vgx/db/migrations up

# Start all services in dev mode
turbo run dev
```

- Dashboard dev server: http://localhost:5173
- Go API: http://localhost:8080
- Ingestion: http://localhost:8001

## Documentation

- [Architecture Overview](docs/architecture/overview.md)
- [Ingestion Pipeline Design](docs/architecture/ingestion-pipeline.md)
- [MVP Implementation Roadmap](docs/planning/mvp-implementation-roadmap.md)
- [Tech Decisions](docs/planning/tech-decisions.md)
- [Product Roadmap](docs/planning/product-roadmap.md)
- [Build Progress](docs/planning/progress.md)

## License

Proprietary. All rights reserved.
