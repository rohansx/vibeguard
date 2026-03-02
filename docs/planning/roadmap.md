# VibeGuard — Product Roadmap

**Updated:** March 2026

---

## Vision

VibeGuard is a **Security Property Graph oracle** that gives AI coding agents real-time security context via MCP. Agents write secure code by construction — not scan for vulnerabilities after the fact.

---

## What's Built (Phase 1 — Complete)

### Core Engine
| Component | Status | Lines | Description |
|-----------|--------|-------|-------------|
| Parser | Done | ~660 | Regex-based extraction for Python, TypeScript/JS, Go. Detects sources, sinks, sanitizers across 8+ frameworks |
| Graph Store | Done | ~310 | In-memory SPG with bbolt persistence. File-level invalidation for incremental updates |
| Graph Builder | Done | ~140 | ParsedFile → SPG nodes + intra-file data-flow edges |
| Taint Engine | Done | ~355 | BFS taint propagation. FindAllPaths, FindPathsToSink, FindMissingSanitizers, TraceDataFlow, GetAttackSurface, CalcBlastRadius |
| MCP Server | Done | ~480 | 8 registered tools via mark3labs/mcp-go. Stdio transport. Deterministic JSON output |
| File Watcher | Done | ~125 | fsnotify with 150ms debounce. Incremental re-parse on save |
| Daemon | Done | ~250 | Init (full repo scan) + Serve (watcher + MCP) orchestrator |

### CLI
| Command | Status | Description |
|---------|--------|-------------|
| `vgx init` | Done | Build initial SPG (32 files, 13 nodes, 126ms on VibeGuard repo) |
| `vgx serve` | Done | Daemon: file watcher + MCP stdio server |
| `vgx report` | Done | Security posture report with A-F risk score. JSON/table output |
| `vgx query` | Done | CLI access to 5 MCP tools. JSON output |
| `vgx ci` | Done | CI gate: exit non-zero on taint paths above severity threshold |
| `vgx diff` | Done | SPG delta between git refs (new/resolved taint paths) |

### Tests
| Package | Tests | Status |
|---------|-------|--------|
| parser | 10 | Passing |
| graph | 11 | Passing |
| taint | 10 | Passing (incl. sanitizer blocking, severity ordering) |

### Infrastructure
- Turborepo + pnpm monorepo
- PostgreSQL schema for team sync (migrations done)
- Docker Compose config
- React dashboard scaffold (needs SPG-aware UI)
- CLAUDE.md, README, .gitignore

---

## What's Next

### Phase 1.5 — Make It Actually Useful (Next)

These are the features that turn the demo into something developers would use on real codebases.

| # | Feature | Impact | Effort |
|---|---------|--------|--------|
| 1 | **Inter-procedural taint analysis** | Currently only intra-file edges. Cross-function/cross-file flows needed to find real vulnerabilities. Without this, engine finds 0 paths on most codebases | High — 2-3 days |
| 2 | **Auto-fix suggestions in MCP responses** | When a taint path is found, return the exact code fix (e.g., "use parameterized query"). #1 differentiator vs every other tool | Medium — 1-2 days |
| 3 | **Secrets detection in parser** | Regex + entropy analysis for hardcoded API keys, passwords, JWT secrets. Replaces Talisman entirely | Medium — 1 day |
| 4 | **TS/JS SQL sink detection** | `pool.query`, `knex`, `prisma.$queryRaw` not covered. Common real-world sinks | Low — half day |
| 5 | **`vgx setup cursor/claude`** | One-liner to write `.mcp.json` config. Zero-friction onboarding | Low — half day |
| 6 | **`vgx hook install`** | Smart pre-commit hook that only flags NEW unsanitized paths. Replaces Talisman | Medium — 1 day |

### Phase 2 — Product Features

| # | Feature | Description |
|---|---------|-------------|
| 1 | **VS Code extension** | D3.js attack path visualization. Click a finding → see full source→sink flow |
| 2 | **Dashboard rewrite** | Replace compliance UI with SPG-aware: taint path explorer, attack surface map, risk trends |
| 3 | **GitHub Action** | `uses: vibeguard/action@v1` with SARIF upload to GitHub Security tab |
| 4 | **Dependency CVE checking** | Parse lock files, check OSV database (free). Reachability analysis — only flag CVEs your code actually calls |
| 5 | **Business logic patterns** | IDOR, mass assignment, broken access control, rate limiting gaps |
| 6 | **Tree-sitter parser upgrade** | Replace regex with AST-level parsing. More accurate, supports any language |
| 7 | **Trust boundary detection** | Automatically infer auth/authz boundaries from middleware patterns |
| 8 | **Calibration flywheel** | Agent feedback loop: "this finding was wrong" → SPG learns, reduces false positives |

### Phase 3 — Enterprise & Scale

| # | Feature | Description |
|---|---------|-------------|
| 1 | **Team sync API** | Share SPG metadata across repos via REST API (handlers are stubbed) |
| 2 | **Multi-repo federation** | Query security properties across microservice boundaries |
| 3 | **Java + Rust support** | Extend parser for JVM and systems languages |
| 4 | **Claude-powered hybrid classification** | Use Anthropic API for ambiguous sanitizer/source classification |
| 5 | **Enterprise dashboard** | Org-wide security posture, compliance mapping, SBOM |
| 6 | **On-prem deployment** | Self-hosted team sync server for regulated industries |

---

## Pricing Strategy (Planned)

| Tier | Price | Features |
|------|-------|----------|
| **Free (Core)** | $0 | CLI, MCP server, local SPG, all 8 query tools, CI gate |
| **Pro** | ~$15/dev/mo | VS Code extension, GitHub Action, priority support |
| **Team** | ~$30/dev/mo | Team sync API, multi-repo, shared calibration, dashboard |
| **Enterprise** | Custom | On-prem, SSO, audit logs, compliance mapping, SLA |

---

## Key Metrics to Track

| Metric | Target | Why |
|--------|--------|-----|
| Init time (1000-file repo) | < 5 seconds | Developers won't wait |
| Incremental update | < 500ms per file | Must feel instant |
| False positive rate | < 10% | The #1 reason developers abandon security tools |
| Taint paths found (real vuln) | > 80% of OWASP Top 10 | Must catch what matters |
| Time-to-first-query | < 2 minutes | `npm install && vgx init && vgx serve` |
