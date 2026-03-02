# VibeGuard — Moat & Competitive Positioning

**Updated:** March 2026

---

## The Market Gap

AI coding agents (Cursor, Claude Code, Copilot) are the new default for writing code. They generate 2.74x more security vulnerabilities than human-written code. The security tooling industry is worth $8B+ but is built for a world where HUMANS write code and TOOLS scan it after.

**Nobody is building security tooling designed for the agent-first world.**

---

## VibeGuard's Moat

### 1. Agent-Native Architecture (not bolt-on)

Every competitor started as a human-facing tool and is now adding MCP as an afterthought:
- Semgrep added MCP in late 2025 — it's a wrapper around `semgrep scan`
- Snyk added MCP — it's a wrapper around `snyk test`
- GitGuardian MCP — it's secrets scanning only

VibeGuard was **built for MCP from day one**. The SPG is the product. The CLI is a convenience layer. The agent is the primary consumer.

### 2. Live Graph vs. Point-in-Time Scans

Every existing tool runs a scan, produces a report, and forgets. VibeGuard maintains a **persistent, live graph** that updates in <500ms when you save a file. The agent always sees the current security state, not a stale scan from 8 minutes ago.

### 3. Proof-Based, Not Pattern-Based

| Tool | Approach | False positive rate |
|------|----------|-------------------|
| Semgrep | Pattern matching (grep-like rules) | 91% (industry average for SAST) |
| Snyk Code | Cloud-based AI analysis | High (no reachability) |
| Talisman | Entropy + keyword matching | Very high |
| **VibeGuard** | Graph traversal: prove source→sink path exists | Low (only flags what it can trace) |

VibeGuard doesn't guess — it traces the actual data flow path from source to sink. If it can't prove the path, it doesn't flag it. This eliminates the #1 reason developers ignore security tools.

### 4. Local-Only, Zero Trust Required

- **Snyk Code** sends your source code to the cloud
- **Semgrep Cloud** uploads findings to a dashboard
- **VibeGuard** runs entirely locally. Code never leaves the machine. MCP on stdio — no network, no ports, no auth.

This is a selling point for:
- Enterprises with IP-sensitive code
- Government contractors
- Finance/healthcare with data residency requirements
- Any developer who doesn't want their code on someone else's server

### 5. The Feedback Loop Gets Smarter

The calibration flywheel (Phase 2):
```
Agent writes code → SPG flags taint path → Agent says "this is a false positive"
→ SPG records calibration event → Future queries for similar patterns are adjusted
→ False positive rate decreases over time per-codebase
```

No other tool has a per-codebase learning loop integrated into the agent workflow.

---

## Competitive Landscape

### Direct Competitors (MCP security tools)

| Tool | What it does | VibeGuard advantage |
|------|-------------|-------------------|
| **Semgrep MCP** | Pattern matching rules | No live graph, no taint propagation, no cross-file analysis in free tier, no auto-fix |
| **Snyk MCP** | Dependency scanning | No code analysis, no taint flow, cloud-dependent, expensive ($50-100/dev/mo) |
| **GitGuardian MCP** | Secrets scanning only | Single feature, no code analysis, no taint, no graph |

### Indirect Competitors (traditional security tools)

| Tool | Category | VibeGuard advantage |
|------|----------|-------------------|
| **Talisman** | Pre-commit secrets | .talismanrc whack-a-mole, trivially bypassed, no context |
| **SonarQube** | SAST | Heavy server, slow, noisy, not agent-native |
| **CodeQL** | Deep SAST | GitHub-only, slow (minutes), not real-time, not MCP |
| **Checkmarx** | Enterprise SAST | Expensive, cloud, slow, not agent-native |

### Why Not Just Use CodeQL?

CodeQL is the closest in capability (it builds a database of code properties). But:
1. CodeQL takes **minutes** to build its database. VibeGuard takes **milliseconds** to update.
2. CodeQL is GitHub-specific. VibeGuard works with any agent via MCP.
3. CodeQL requires learning QL (a query language). VibeGuard's MCP tools are natural language.
4. CodeQL runs in CI. VibeGuard runs in the editor, in real-time.

---

## Why This Is Hard to Copy

1. **The graph needs to be fast.** Building an in-memory SPG with <500ms incremental updates requires careful engineering. Competitors can't just add "live graph" to their existing scan-based architecture.

2. **MCP-native design.** Bolting MCP onto a scan-based tool produces a worse experience than a tool designed for MCP. The query interface, the response format, the incremental update model — all optimized for agent consumption.

3. **Framework knowledge.** The parser's framework-specific patterns (FastAPI's `Query()`, Django's `request.POST`, Express's `req.body`, Chi's `URLParam`) represent accumulated security knowledge that's hard to replicate quickly.

4. **The flywheel.** Once calibration events accumulate, VibeGuard gets smarter per-codebase. Switching costs increase over time.

---

## Positioning Statement

> **For developers using AI coding agents** who need security without friction,
> **VibeGuard** is a Security Property Graph oracle
> that **gives agents real-time security context via MCP**
> so they **write secure code by construction**.
> **Unlike** Snyk, Semgrep, and Talisman, which scan code after it's written and overwhelm developers with false positives,
> **VibeGuard** maintains a live, local graph that agents query before writing code — zero noise, zero cloud, zero bypass.

---

## Go-to-Market

### Phase 1: Developer Tool (Now)
- Free, open-core CLI
- Target: Individual developers using Cursor/Claude Code
- Distribution: GitHub, Homebrew tap, AUR
- Growth: HN launch, Twitter/X dev community, MCP server directory

### Phase 2: Team Product
- Team sync API, shared calibration
- Target: Security-conscious startups (10-50 devs)
- Distribution: PLG from individual adoption → team license

### Phase 3: Enterprise
- On-prem, SSO, compliance mapping
- Target: Regulated industries (finance, healthcare, government)
- Distribution: Enterprise sales, partnerships with AI coding tool vendors
