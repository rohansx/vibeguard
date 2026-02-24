# VibeGuard

**Security Property Graph (SPG) oracle for AI coding agents.** VibeGuard builds a persistent, live graph encoding the full security semantics of your codebase — taint sources, sinks, sanitizers, trust boundaries, attack paths — and exposes them to AI coding agents (Cursor, Claude Code) via MCP, so agents write secure code by construction.

No code leaves your machine. Runs locally on stdio. Sub-500ms incremental updates.

## How It Works

```
┌─────────────────────────────────────────────────────────────────────┐
│                         YOUR CODEBASE                               │
│   Python · TypeScript · JavaScript · Go                             │
└──────────────┬──────────────────────────────────────────────────────┘
               │  file save (inotify / FSEvents)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  VGX DAEMON                                                          │
│                                                                      │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────────────┐ │
│  │  Parser       │──▶│  SPG Builder  │──▶│  Taint Propagation (BFS) │ │
│  │  (per-file)   │   │  (classify    │   │  source → sink paths     │ │
│  │               │   │   nodes/edges)│   │  sanitizer detection     │ │
│  └──────────────┘   └──────────────┘   └──────────────────────────┘ │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │  MCP Server (stdio)                                              ││
│  │  8 deterministic security query tools → structured JSON          ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────┬───────────────────────────────────────────────────────┘
               │  stdio (MCP protocol)
               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  AI CODING AGENT (Cursor / Claude Code / Copilot)                    │
│  Queries SPG before writing code → writes secure code by default     │
└──────────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- [Go](https://golang.org/) 1.23+

### Install and run

```bash
# Clone
git clone git@github.com:rohansx/vibeguard.git
cd vibeguard

# Build
cd apps/vgx && go build -o vgx ./cmd/vgx/

# Initialize SPG for any repository
./vgx init /path/to/your/repo

# Start daemon (file watcher + MCP server on stdio)
./vgx serve --repo /path/to/your/repo
```

### Connect to Cursor / Claude Code

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "vibeguard": {
      "command": "/path/to/vgx",
      "args": ["serve", "--repo", "."]
    }
  }
}
```

The agent can now query your codebase's security properties in real time.

## MCP Tools

All tools return deterministic structured JSON. Same query + same code = same result. No LLM in the verification path — pure graph traversal.

| Tool | Security Question | Status |
|------|------------------|--------|
| `query_taint_paths(sink)` | Unsanitized data flows reaching this sink? | Phase 1 |
| `get_attack_surface(module)` | All untrusted entry points for this module? | Phase 1 |
| `find_missing_sanitizers()` | Where does tainted data reach a sink without sanitization? | Phase 1 |
| `trace_data_flow(node_id, symbol, file)` | Where does this variable travel through the codebase? | Phase 1 |
| `calculate_blast_radius(function)` | What security properties change if I modify this? | Phase 1 |
| `get_security_context(file)` | Full security posture of this file? | Phase 1 |
| `get_trust_boundary_violations()` | Paths crossing trust boundaries without auth? | Phase 2 |
| `check_auth_coverage(endpoint)` | Does this endpoint enforce auth? | Phase 2 |

### Example: query taint paths

```
Agent → query_taint_paths(sink: "sqli")

VibeGuard → {
  "query": "sqli",
  "path_count": 1,
  "paths": [{
    "vuln_class": "sqli",
    "severity": "high",
    "confidence": 0.95,
    "source": { "symbol": "request.args.get", "file_path": "app.py", "line": 12 },
    "sink": { "symbol": "cursor.execute", "file_path": "db.py", "line": 34 },
    "path_length": 3
  }],
  "proof": "UNSANITIZED_PATHS_FOUND: 1 path(s) detected"
}
```

## CLI Commands

```bash
vgx init [path]           # Build initial SPG for a repository
vgx serve                 # Start daemon: file watcher + MCP server (stdio)
vgx serve --api           # Also start team sync REST API on :8080
vgx query <tool> [args]   # Run a security query against the live SPG
vgx report                # Generate security posture report
vgx diff [ref]            # Show SPG changes since a git ref
vgx ci                    # CI mode: exit non-zero on new taint paths
```

## What Gets Classified

VibeGuard classifies security-relevant nodes across 4 languages and 8+ frameworks:

| Language | Frameworks | Sources | Sinks | Sanitizers |
|----------|-----------|---------|-------|------------|
| Python | FastAPI, Django, Flask | `request.args`, `request.form`, `Body()`, `Query()` | `cursor.execute`, `subprocess.run`, `eval`, `open` | `html.escape`, `markupsafe.escape`, `bleach.clean` |
| TypeScript/JS | Express, Next.js | `req.body`, `req.params`, `req.query`, `searchParams` | `pool.query`, `exec`, `innerHTML`, `eval` | `DOMPurify.sanitize`, `escape`, `encodeURI` |
| Go | Chi, Gin, net/http | `r.URL.Query()`, `r.FormValue`, `c.Param` | `db.Query`, `exec.Command`, `template.HTML` | `html.EscapeString`, `filepath.Clean`, `url.QueryEscape` |

### Vulnerability Classes

| Class | Severity | Description |
|-------|----------|-------------|
| `rce` | Critical | Remote code execution via eval/exec |
| `deserialization` | Critical | Unsafe deserialization of untrusted data |
| `sqli` | High | SQL injection via string interpolation |
| `ssrf` | High | Server-side request forgery |
| `xss` | Medium | Cross-site scripting via unsanitized output |
| `path_traversal` | Medium | Directory traversal via user-controlled paths |

## Architecture

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
│   ├── parser/               # Per-file parsing + security node extraction
│   │   └── parser.go         # Python, TypeScript, Go regex-based classifier
│   ├── graph/                # Security Property Graph
│   │   ├── nodes.go          # Node/Edge/TaintPath type definitions
│   │   ├── store.go          # In-memory graph + bbolt persistence
│   │   └── builder.go        # ParsedFile → SPG (classify + build edges)
│   ├── taint/                # Taint propagation engine
│   │   └── engine.go         # BFS source→sink, sanitizer-aware
│   ├── mcp/                  # MCP server (mark3labs/mcp-go)
│   │   ├── server.go         # Tool registration + stdio transport
│   │   └── tools.go          # 8 query tool handlers
│   ├── watcher/              # File watcher (fsnotify)
│   │   └── watcher.go        # Debounced inotify/FSEvents
│   ├── daemon/               # Orchestrator
│   │   └── daemon.go         # Init, Serve, incremental updates
│   ├── api/                  # Team sync REST API (Chi)
│   │   ├── router.go
│   │   └── handlers/
│   ├── config/               # Environment config
│   └── db/                   # pgx/v5 pool (team sync only)
└── db/migrations/            # SQL migrations (dbmate)
```

## Incremental Updates

When you save a file, VibeGuard:

1. Detects the change via `fsnotify` (~5ms)
2. Re-parses only the changed file
3. Removes stale nodes/edges for that file
4. Rebuilds security nodes + data-flow edges
5. Re-propagates taint paths through the graph

Total latency: typically < 500ms per file change.

## Team Sync (Optional)

For teams sharing security metadata across repos:

```bash
# Start with team sync API
podman compose -f docker/docker-compose.yml up
vgx serve --api --port 8080
```

Requires PostgreSQL for shared metadata storage. The local SPG always lives on-disk at `~/.vibeguard/graph/` — PostgreSQL only stores team sync metadata and calibration events.

## Environment Variables

| Variable | Required | Purpose |
|----------|----------|---------|
| `SPG_STORE_PATH` | No | Override graph store path (default: `~/.vibeguard/graph/`) |
| `DATABASE_URL` | For team sync | PostgreSQL connection string |
| `ANTHROPIC_API_KEY` | Phase 2 | Claude Sonnet for hybrid sanitizer classification |
| `CLERK_SECRET_KEY` | For team sync | Authentication for Pro/Team tier |

## Roadmap

- **Phase 1 (current):** Core SPG daemon + 6 MCP tools + CLI + Python/TS/Go support
- **Phase 2:** VS Code extension (D3.js attack path viz), trust boundary detection, GitHub Action, team tier
- **Phase 3:** Java/Rust support, calibration flywheel, enterprise, multi-repo federation

## License

Proprietary. All rights reserved.
