# VibeGuard — Architecture Decision Records

**Created:** 2026-02-10 | **Updated:** 2026-03-02 (post-pivot to SPG)

Each decision records the context, options considered, and rationale.

---

## ADR-001: Monorepo Tooling — Turborepo + pnpm

**Date:** 2026-02-10 | **Status:** Accepted

**Context:** VibeGuard is polyglot (Go + TypeScript). We need a monorepo tool that can orchestrate builds, caching, and task dependencies.

**Decision:** Turborepo + pnpm. Dashboard builds first, dist is embedded into Go binary via `go:embed`. Turborepo handles the dependency graph (`VGX dependsOn dashboard#build`).

---

## ADR-002: Go HTTP Router — Chi

**Date:** 2026-02-10 | **Status:** Accepted

**Decision:** Chi v5. Lightweight, idiomatic, stdlib-compatible. Used for the team sync REST API (`:8080`). The core MCP path doesn't use HTTP at all (stdio).

---

## ADR-003: Frontend — Embedded React SPA in Go Binary

**Date:** 2026-02-10 | **Status:** Accepted

**Decision:** Vite + React 19 + TypeScript builds the dashboard. Go's `embed` package bundles it into the single binary. No separate server, no CORS. Currently the dashboard is a scaffold — SPG-aware UI is Phase 2.

---

## ADR-004: Product Pivot — Compliance-as-Code → Security Property Graph

**Date:** 2026-02-23 | **Status:** Accepted

**Context:** The original VibeGuard was a compliance-as-code scanner (GDPR PDF → ast-grep rules → scan). We pivoted to a Security Property Graph that serves security context to AI coding agents via MCP.

**Why:** AI agents generate 2.74x more security vulnerabilities than humans. Existing tools (Snyk, Semgrep, Talisman) scan AFTER code is written, produce 91% false positives, and developers bypass them. The SPG approach encodes security semantics into a live graph that agents query BEFORE writing code.

**What changed:**
- Deleted: Python ingestion service, PDF parsing, compliance scanner, regulatory rules
- Added: SPG engine (parser, graph, taint, MCP server, file watcher)
- Kept: Go binary, React dashboard, PostgreSQL (team sync only), Turborepo, Cobra CLI

**Decision:** Full pivot. Old compliance code archived at `archive/compliance-v0` branch.

---

## ADR-005: Graph Persistence — bbolt (BoltDB)

**Date:** 2026-02-23 | **Status:** Accepted

**Context:** The SPG needs persistence across daemon restarts. Options: Neo4j, SQLite, bbolt, file-based JSON.

| Option | Pros | Cons |
|--------|------|------|
| **bbolt** | Pure Go, embedded, zero config, fast for small graphs | No query language, manual indexing |
| Neo4j | Full graph DB, Cypher queries | External process, Java dependency, overkill for local use |
| SQLite | Embedded, SQL queries | Not optimized for graph traversal |
| JSON files | Simplest | No concurrency, slow for large graphs |

**Decision:** bbolt. The SPG lives in-memory for fast traversal, with bbolt as a write-ahead persistence layer. No external process, no Java, no config. Graph stored at `~/.vibeguard/graph/<repo-hash>/spg.db`.

---

## ADR-006: MCP Transport — stdio

**Date:** 2026-02-23 | **Status:** Accepted

**Context:** MCP supports stdio, SSE, and HTTP transports.

**Decision:** stdio only. No network exposure, no ports, no auth needed. Code never leaves the machine. This is a deliberate security decision — VibeGuard's primary audience is developers who care about IP protection. Cursor and Claude Code both support stdio MCP servers natively.

---

## ADR-007: Parser Strategy — Regex First, Tree-sitter Later

**Date:** 2026-02-23 | **Status:** Accepted

**Context:** Need to extract security-relevant constructs (sources, sinks, sanitizers) from source files.

| Option | Pros | Cons |
|--------|------|------|
| **Regex patterns** | Fast to build, pure Go, no CGo deps | Less accurate, can miss edge cases |
| Tree-sitter | AST-level accuracy, language grammar support | CGo dependency, build complexity, slower iteration |
| go/ast (Go only) | Native Go AST | Only works for Go files |

**Decision:** Regex for Phase 1, tree-sitter for Phase 2. The parser interface (`ParseFile → ParsedFile`) is stable — swapping regex for tree-sitter is a drop-in replacement. Regex is ~85% accurate on common framework patterns, which is sufficient for proving the SPG concept.

---

## ADR-008: Taint Propagation — BFS, Not Datalog

**Date:** 2026-02-23 | **Status:** Accepted

**Context:** Need to find unsanitized source-to-sink data flow paths.

| Option | Pros | Cons |
|--------|------|------|
| **BFS traversal** | Simple, deterministic, easy to debug | Doesn't scale to millions of nodes |
| Datalog (Souffle) | Declarative, handles complex flows | External process, learning curve, harder to debug |
| Differential dataflow | Incremental, efficient at scale | Complex implementation, Rust dependency |

**Decision:** BFS with max depth 20. The SPG is typically 100-10,000 nodes — BFS is O(V+E) and completes in microseconds at this scale. Path deduplication and severity sorting are post-processing steps. Differential dataflow is a Phase 3 optimization if needed.

---

## ADR-009: Database — PostgreSQL for Team Sync Only

**Date:** 2026-02-10 | **Status:** Updated

**Context:** PostgreSQL was originally the primary store for compliance rules and scans. After the pivot, it's only used for team sync metadata.

**Decision:** PostgreSQL 17 with pgx/v5. Stores: `repositories`, `spg_nodes`, `taint_paths`, `calibration_events`, `framework_classifiers`. The local SPG is always bbolt — PostgreSQL is optional (only needed for `vgx serve --api`).

---

## ADR-010: Authentication — Clerk (Team Sync Only)

**Date:** 2026-02-10 | **Status:** Updated

**Decision:** Clerk JWT auth for the team sync REST API. The core MCP path (stdio) has no auth — it's local-only. Clerk is only activated when running `vgx serve --api` for Pro/Team tier.

---

## ADR-011: Schema Migrations — dbmate

**Date:** 2026-02-10 | **Status:** Accepted

**Decision:** dbmate. Language-agnostic CLI, up/down migrations in `apps/vgx/db/migrations/`. Simple and stays out of the way.

---

## ADR-012: Container Runtime — Podman

**Date:** 2026-02-10 | **Status:** Accepted

**Decision:** Podman 5.x. Daemonless, rootless, Docker CLI compatible. Used for local PostgreSQL and team sync API development.
