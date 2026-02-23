# VibeGuard — Architecture Decision Records

**Created:** 2026-02-10

Each decision records the context, options considered, and rationale.

---

## ADR-001: Monorepo Tooling — Turborepo + pnpm

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** VibeGuard is polyglot (Go + Python + TypeScript). We need a monorepo tool that can orchestrate builds, caching, and task dependencies across all three.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| **Turborepo + pnpm** | Fast builds, great caching, pnpm workspaces for JS/TS, Go/Python via thin package.json wrappers | Requires wrapper pattern for non-JS apps |
| Nx | More feature-rich, native Go/Python task support | Heavier, steeper learning curve, overkill for small team |
| Just pnpm workspaces | Lightweight, no orchestrator overhead | Manual task coordination, no caching, no dependency graph |
| Bazel | Hermetic builds, excellent polyglot support | Steep learning curve, massive overkill for early MVP |
| Bun | Fastest installs, integrated runtime | Less mature Turborepo integration, Bun's bundler/runtime overlaps with Vite |

**Decision:** Turborepo + pnpm. Battle-tested integration, stays out of the way in a polyglot monorepo. Go and Python apps get thin `package.json` wrappers where scripts delegate to native commands.

---

## ADR-002: Go HTTP Router — Chi

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** The Go binary needs an HTTP router for the REST API.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| **Chi** | Lightweight, idiomatic, stdlib-compatible, excellent middleware ecosystem | Not as fast as fasthttp-based routers |
| net/http (stdlib) | Zero dependencies, Go 1.22+ has path params | More verbose, less middleware ecosystem |
| Fiber | Fastest benchmarks, Express-like API | Uses fasthttp (not stdlib compatible), less idiomatic Go |
| Echo | Feature-rich, good middleware | Slightly heavier than Chi |

**Decision:** Chi. Idiomatic Go, stdlib-compatible (important for `go:embed` and standard middleware), and the spec's code examples already use it. Clerk SDK provides Chi-compatible middleware.

---

## ADR-003: Frontend Architecture — Embedded React SPA in Go Binary

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** The dashboard needs to be served alongside the API. Two approaches: separate Next.js app or embedded SPA.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| **Embedded React SPA** | Single binary deployment, no CORS, same origin for API+UI, simpler infra | No SSR, build dependency between dashboard and Go |
| Next.js 15 standalone | SSR, API routes, more powerful | Two servers to deploy, CORS config needed, more complex |
| Vite SPA (served separately) | Lightweight SPA with Vite | Still needs separate serving, CORS issues |

**Decision:** Embedded React SPA. Vite builds the dashboard, Go's `embed` package bundles it into the binary. Single binary serves everything. During development, Vite's proxy sends `/api` calls to the Go service. This is simpler to deploy and eliminates CORS entirely.

---

## ADR-004: MVP Scope — GDPR Only

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** The tech spec mentions GDPR, SOC 2, and EU AI Act as initial targets.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| **GDPR only** | Fastest to MVP, nail one framework completely | Narrower market appeal initially |
| GDPR + SOC 2 | Broader appeal, covers EU + US | More rules to write, longer timeline |
| GDPR + SOC 2 + EU AI Act | Full coverage from day one | Significantly more work, diluted quality |

**Decision:** GDPR only. Ship one framework done exceptionally well rather than three done partially. The ingestion pipeline makes adding new frameworks straightforward once the foundation is solid. SOC 2 is first expansion target.

---

## ADR-005: Database — PostgreSQL 17

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Need a database for compliance rules, scan results, documents, audit logs.

**Decision:** PostgreSQL 17. JSONB for flexible rule storage, pgcrypto for audit log integrity, row-level security for multi-tenancy. Both Go (pgx) and Python (asyncpg) have excellent PostgreSQL drivers. No need for a separate vector DB — pgvector can be added later for rule search.

---

## ADR-006: Authentication — Clerk

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Need auth with SSO support for enterprise customers.

**Decision:** Clerk. Provides JWT-based auth, organization management, RBAC, and SSO (SAML/OIDC) out of the box. Go SDK (`clerk-sdk-go/v2`) provides middleware. React SDK (`@clerk/clerk-react`) handles the frontend. Avoids building auth from scratch.

---

## ADR-007: Python Sidecar Architecture

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Marker, LangExtract, and PageIndex are Python libraries. The Go binary can't embed them directly.

**Decision:** Separate Python FastAPI service running alongside Go. Internal-only (port 8001, Docker `expose` not `ports`). Go proxies document uploads to it. The Python service handles all document ingestion and is idle most of the time. This keeps the Go binary focused on real-time traffic.

---

## ADR-008: Deployment Target — Hetzner CX33

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Need affordable hosting that can run all three services + PostgreSQL.

**Decision:** Hetzner CX33 (4 vCPU, 8 GB RAM, ~EUR 7/month). Memory budget: PostgreSQL 2 GB, Go 200 MB, Python idle 300 MB, Python processing 2-3 GB. Fits with headroom. If tight, upgrade to CX42 (16 GB, EUR 16.40/month). Docker Compose for orchestration.

---

## ADR-009: Schema Migrations — dbmate

**Date:** 2026-02-10 | **Status:** Accepted (updated)

**Context:** Both Go and Python services access the same PostgreSQL database. Need a migration tool that's language-agnostic and doesn't couple schema management to any single service.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| Custom Go migration runner | Self-contained, no external deps | Custom code to maintain, no rollback support, tied to Go binary |
| **dbmate** | Language-agnostic, simple CLI, up/down migrations, schema dump, Docker-friendly | External dependency |
| golang-migrate | Popular Go library | Still Go-specific, overkill for our use |
| Flyway | Enterprise-grade, Java ecosystem | Heavy, Java dependency |

**Decision:** dbmate. Runs from project root via `.dbmate.toml`, migrations in `apps/vgx/db/migrations/`. Language-agnostic means both Go and Python teams can create migrations. Supports rollbacks with `-- migrate:down` markers. The Python service reads/writes but never modifies schema.

---

## ADR-010: Audit Log — Append-Only with DB Triggers

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Compliance products need tamper-proof audit trails.

**Decision:** PostgreSQL triggers that raise exceptions on UPDATE or DELETE attempts on the `audit_log` table. Cryptographic chaining (each entry includes SHA-256 hash of previous entry) for tamper detection. This is enforced at the database level, not the application level — even application bugs can't corrupt the audit trail.

---

## ADR-011: Container Runtime — Podman

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** Need a container runtime for local development and production deployment.

**Options considered:**
| Option | Pros | Cons |
|--------|------|------|
| **Podman** | Daemonless, rootless by default, OCI-compliant, Docker CLI compatible | Slightly less ecosystem support for niche tools |
| Docker | Industry standard, largest ecosystem | Requires daemon, Docker Desktop licensing for enterprises |

**Decision:** Podman 5.x. Daemonless architecture (no root daemon required), `docker` aliased to `podman` for compatibility. Compose files use standard OCI format — work with both runtimes. All docs reference `podman compose` as the primary command.
