# VibeGuard — Architecture Overview

**Version:** 2.0 | **Date:** March 2026 | **Status:** Phase 1 Complete

---

## What VibeGuard Is

VibeGuard is a **Security Property Graph (SPG)** that encodes the full security semantics of a codebase — taint sources, sinks, sanitizers, data flow edges, trust boundaries — and exposes them to AI coding agents via MCP (Model Context Protocol).

**The thesis:** AI agents (Cursor, Claude Code, Copilot) generate 2.74x more security vulnerabilities than human-written code. Current security tools (Snyk, Semgrep, Talisman) scan code AFTER it's written and shout at developers. VibeGuard inverts this — the agent queries the SPG BEFORE writing code, so it writes secure code by construction.

```
Traditional:  Write code → Scan → Block → Fix → Re-scan → Ship
VibeGuard:    Agent asks "is this safe?" → SPG answers → Agent writes safe code → Ship
```

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         YOUR CODEBASE                               │
│   Python · TypeScript · JavaScript · Go                             │
└──────────────┬──────────────────────────────────────────────────────┘
               │  file save (inotify / FSEvents)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  VGX DAEMON (single Go binary)                                       │
│                                                                      │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────────────┐ │
│  │  Parser       │──▶│  SPG Builder  │──▶│  Taint Engine (BFS)      │ │
│  │  regex-based  │   │  classify &   │   │  source → sink paths     │ │
│  │  per-file     │   │  build edges  │   │  sanitizer detection     │ │
│  └──────────────┘   └──────────────┘   └──────────────────────────┘ │
│         │                    │                     │                  │
│         ▼                    ▼                     ▼                  │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  Security Property Graph (in-memory + bbolt persistence)        │ │
│  │  Nodes: Source | Sink | Sanitizer | TrustBoundary | AuthCritical│ │
│  │  Edges: DataFlow | Call | Return | Assignment                   │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │  MCP Server (stdio transport)                                    │ │
│  │  8 deterministic query tools → structured JSON                   │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│         │                                                            │
│  ┌──────┴────────────────────────────────────────────────────────┐  │
│  │  File Watcher (fsnotify) — incremental re-parse on save       │  │
│  └───────────────────────────────────────────────────────────────┘  │
└──────────────┬───────────────────────────────────────────────────────┘
               │  stdio (MCP protocol)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  AI CODING AGENT (Cursor / Claude Code / Copilot)                    │
│  Queries SPG while writing code → writes secure code by default      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Parser (`internal/parser/`)

**Job:** Extract security-relevant constructs from source files.

- Regex-based pattern matching (tree-sitter planned for Phase 2)
- Detects: SourceNodes (HTTP params), SinkNodes (SQL, RCE, XSS, SSRF, path traversal, deserialization), SanitizerNodes
- Per-file SHA-256 hash for content-addressed caching
- Supports: Python (FastAPI, Django, Flask), TypeScript/JS (Express, Next.js), Go (Chi, Gin, net/http)

**Key type:** `ParsedFile` → `[]ParsedNode` + `[]FuncDef`

### 2. Graph Store (`internal/graph/`)

**Job:** Persistent Security Property Graph with typed nodes and edges.

- In-memory graph (`map[string]*Node` + adjacency lists) for fast traversal
- bbolt (BoltDB) persistence at `~/.vibeguard/graph/<repo-hash>/`
- File-level invalidation for incremental updates (remove all nodes for a file, rebuild)
- Thread-safe (sync.RWMutex)

**Node types:** `SourceNode`, `SinkNode`, `SanitizerNode`, `TrustBoundaryEdge`, `AuthCriticalNode`
**Edge types:** `DataFlow`, `Call`, `Return`, `Assignment`

### 3. Graph Builder (`internal/graph/builder.go`)

**Job:** Convert `ParsedFile` → SPG nodes + intra-file data-flow edges.

- Classifies parsed nodes into graph node types
- Builds data-flow edges by matching tainted variable names in sink lines
- Idempotent — calling `BuildFromFile` twice for the same file produces the same graph

### 4. Taint Engine (`internal/taint/`)

**Job:** BFS-based taint propagation from sources to sinks.

- `FindAllPaths()` — all unsanitized source→sink paths in the graph
- `FindPathsToSink(pattern)` — paths to a specific sink type (e.g., "sqli")
- `FindMissingSanitizers()` — where tainted data reaches sinks unprotected
- `TraceDataFlow(nodeID)` — full reachability from a given node
- `GetAttackSurface(module)` — all sources + reachable sinks for a module
- `CalcBlastRadius(nodeID)` — what security properties change if this node is modified
- Sanitizer-aware: paths through SanitizerNodes are excluded
- Max BFS depth: 20, path deduplication, severity sorting

### 5. MCP Server (`internal/mcp/`)

**Job:** Expose SPG queries to AI agents via Model Context Protocol (stdio).

- Uses `mark3labs/mcp-go v0.44.0`
- 8 registered tools with JSON Schema parameter definitions
- All outputs: deterministic structured JSON (same query + same code = same result)
- No LLM in the verification path — pure graph traversal

### 6. File Watcher (`internal/watcher/`)

**Job:** Detect file changes and trigger incremental SPG updates.

- fsnotify-based with 150ms debounce
- Ignores: `node_modules`, `.git`, `__pycache__`, `vendor`, `dist`, etc.
- On change: re-parse → remove stale nodes → rebuild → re-propagate taint

### 7. Daemon (`internal/daemon/`)

**Job:** Orchestrate the full lifecycle.

- `Init()` — full repo walk, parse all files, build SPG, persist to disk
- `Serve()` — load graph, start file watcher, start MCP stdio server
- `DefaultStorePath()` — `~/.vibeguard/graph/<fnv-hash>/`

### 8. CLI (`cmd/vgx/`)

| Command | Status | What it does |
|---------|--------|-------------|
| `vgx init [path]` | Done | Build initial SPG for a repository |
| `vgx serve` | Done | Start daemon: file watcher + MCP server (stdio) |
| `vgx report` | Done | Security posture report with risk score (A-F) |
| `vgx query <tool>` | Done | CLI access to MCP tools (JSON output) |
| `vgx ci` | Done | CI gate: exit non-zero on unsanitized taint paths |
| `vgx diff [ref]` | Done | SPG delta between git refs |

---

## Data Flow: How a Security Query Works

```
1. Agent calls: query_taint_paths(sink: "sqli")

2. MCP server receives request via stdio

3. Taint engine runs BFS:
   - Get all SourceNodes from graph
   - For each source, BFS forward through DataFlow edges
   - If path reaches a SinkNode with vuln_class="sqli" AND no SanitizerNode in path:
     → Record as unsanitized taint path

4. Return structured JSON:
   {
     "query": "sqli",
     "path_count": 1,
     "paths": [{
       "severity": "high",
       "vuln_class": "sqli",
       "source": "request.args.get",
       "source_file": "app.py", "source_line": 12,
       "sink": "cursor.execute",
       "sink_file": "db.py", "sink_line": 34,
       "path_length": 3,
       "confidence": 0.85
     }]
   }
```

---

## Technology Stack

| Component | Technology | Why |
|-----------|-----------|-----|
| Core binary | Go 1.23 | Single binary, fast, no runtime deps |
| CLI framework | Cobra | Industry standard Go CLI |
| HTTP router | Chi v5 | Lightweight, stdlib-compatible |
| Graph persistence | bbolt (BoltDB) | Embedded, pure Go, no external process |
| MCP protocol | mark3labs/mcp-go | Stdio transport for AI agent integration |
| File watching | fsnotify | Cross-platform inotify/FSEvents wrapper |
| Database (team sync) | PostgreSQL 17 + pgx/v5 | Team metadata only — local SPG is bbolt |
| Dashboard | React 19 + Vite + Tailwind | Embedded into Go binary via go:embed |
| Auth (team sync) | Clerk | JWT auth for Pro/Team tier API |
| Monorepo | Turborepo + pnpm | Orchestrates Go + React builds |

---

## What's NOT in VibeGuard

- **No LLM in the verification path.** Security queries are pure graph traversal. Deterministic.
- **No cloud dependency.** SPG lives locally. Code never leaves the machine.
- **No network exposure.** MCP runs on stdio. No HTTP server unless `--api` flag (team sync).
- **No scan-then-block model.** VibeGuard informs; it doesn't gate (unless you want it to via `vgx ci`).

---

## File Structure

```
apps/vgx/
├── cmd/vgx/                  # CLI entrypoints (Cobra)
│   ├── main.go               # Root command + version
│   ├── init.go               # vgx init — build SPG
│   ├── serve.go              # vgx serve — daemon + MCP
│   ├── query.go              # vgx query — security queries
│   ├── report.go             # vgx report — posture report
│   ├── diff.go               # vgx diff — SPG delta
│   └── ci.go                 # vgx ci — CI/CD gate
├── internal/
│   ├── parser/               # Per-file parsing + security classification
│   ├── graph/                # SPG types, store (bbolt), builder
│   ├── taint/                # BFS taint propagation engine
│   ├── mcp/                  # MCP server + 8 tool handlers
│   ├── watcher/              # fsnotify file watcher
│   ├── daemon/               # Lifecycle orchestrator
│   ├── api/                  # Team sync REST API (Chi)
│   ├── config/               # Environment config
│   ├── db/                   # pgx/v5 pool
│   └── embed/                # Embedded SPA filesystem
└── db/migrations/            # SQL migrations (dbmate)

apps/dashboard/               # React SPA (Phase 2: SPG visualization)
docs/                         # Architecture, planning, research
```
