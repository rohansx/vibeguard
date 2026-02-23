# VibeGuard — Complete Architecture & Technology Specifications

## Detailed Technical Blueprint with Best-in-Class Tool Recommendations

**Version:** 1.0 | **Date:** February 2026 | **Status:** Pre-Seed Architecture Planning

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Layer 1: Regulatory Document Parsing Pipeline](#2-layer-1-regulatory-document-parsing-pipeline)
3. [Layer 2: Code Analysis & Scanning Engine](#3-layer-2-code-analysis--scanning-engine)
4. [Layer 3: Gap Detection & AI-Assisted Analysis](#4-layer-3-gap-detection--ai-assisted-analysis)
5. [Regulatory Feed & Versioning System](#5-regulatory-feed--versioning-system)
6. [Dashboard & Web Platform](#6-dashboard--web-platform)
7. [Developer Tooling (CLI, IDE, CI/CD)](#7-developer-tooling-cli-ide-cicd)
8. [Data Layer & Infrastructure](#8-data-layer--infrastructure)
9. [RAG & Knowledge System](#9-rag--knowledge-system)
10. [Complete Technology Stack Summary](#10-complete-technology-stack-summary)
11. [Build vs. Buy Decisions](#11-build-vs-buy-decisions)
12. [Phase-wise Tool Adoption Roadmap](#12-phase-wise-tool-adoption-roadmap)

---

## 1. Architecture Overview

VibeGuard's architecture is organized into three processing layers, supported by infrastructure services. Every component is selected for determinism, auditability, and zero-hallucination guarantees.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        VIBEGUARD SYSTEM ARCHITECTURE                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─── INGESTION ──────┐  ┌─── ANALYSIS ──────┐  ┌─── REPORTING ─────┐ │
│  │                     │  │                    │  │                    │ │
│  │  Regulatory PDFs    │  │  AST Parsing       │  │  Dashboard         │ │
│  │  ↓                  │  │  Taint Analysis    │  │  Audit Reports     │ │
│  │  Document Parsing   │  │  Pattern Matching  │  │  Evidence Packages │ │
│  │  ↓                  │  │  Config Checks     │  │  Gap Analysis      │ │
│  │  Structured Rules   │  │  ↓                 │  │  PII Flow Maps     │ │
│  │  ↓                  │  │  Deterministic     │  │                    │ │
│  │  Human Review       │  │  Pass/Fail         │  │                    │ │
│  │                     │  │                    │  │                    │ │
│  └─────────────────────┘  └────────────────────┘  └────────────────────┘ │
│                                                                         │
│  ┌─── DEVELOPER TOOLS ────────────────────────────────────────────────┐ │
│  │  CLI (VGX)  ·  VS Code Extension  ·  GitHub Action  ·  Pre-commit │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌─── INFRASTRUCTURE ─────────────────────────────────────────────────┐ │
│  │  PostgreSQL · Kafka/SQS · S3 · Redis · Append-only Audit Log      │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Layer 1: Regulatory Document Parsing Pipeline

This is where regulatory PDFs are transformed into structured, machine-readable compliance rules. This layer is the ONLY place where AI/LLMs are used in the compliance pipeline, and human review gates prevent any hallucination from reaching customers.

### 2.1 PDF Extraction & Parsing

The pipeline needs to handle complex regulatory documents (GDPR: 88 pages, EU AI Act: 144 pages) with cross-references, footnotes, annexes, and multi-column layouts.

**Recommended Primary Tool: MinerU (by OpenDataLab)**

- **Why:** Highest completion and accuracy among open-source PDF parsers. Actively maintained (v2.7.4 as of Jan 2026). Handles complex layouts, tables, footnotes, headers/footers, and outputs structured Markdown/JSON. Supports 84-language OCR.
- **Strengths for VibeGuard:** Preserves document structure (headings, paragraphs, lists), which is critical for mapping articles/sections. Removes headers/footers/page numbers for clean text. Outputs in both Markdown and JSON formats.
- **GitHub:** github.com/opendatalab/MinerU (61k+ stars)
- **License:** AGPL-3.0 (evaluate for commercial use; may need their commercial license or use API)
- **Alternative:** Docling (IBM) — better for downstream NLP/LLM pipeline integration, modular design, supports PDF/DOCX/PPTX. MIT licensed. Use as a secondary parser or for integration with LangChain/LlamaIndex pipelines.

**For Vision-Based Parsing (backup for scanned/image PDFs):**

- **Marker** by VikParuchuri — fast, multi-format support (PDF, EPUB, DOCX), good structure fidelity. Use for batch preprocessing.
- **Gemini 2.5 Pro** (via API) — for the most complex edge cases, Gemini consistently delivers the most reliable results for document parsing. Use as a fallback for documents MinerU struggles with.

### 2.2 Structured Information Extraction from Legal Text

Once the PDF is parsed into clean text, the next step is extracting structured compliance rules (article, requirement, technical check, source text, cross-references).

**Recommended Primary Tool: Google LangExtract**

- **Why:** This is exactly the tool you found — and it's a perfect fit. LangExtract extracts structured data from unstructured text with precise source grounding (character-level offsets). Every extracted entity maps back to its exact location in the source document.
- **Key capabilities for VibeGuard:**
  - Define custom extraction schemas via few-shot examples (e.g., "extract article number, requirement text, technical implication, cross-references")
  - Source grounding ensures every extracted rule has a verifiable citation pointer back to the original regulation text — this directly powers VibeGuard's citation engine
  - Handles 100+ page documents with chunking and multi-pass extraction
  - Works with Claude/Gemini/OpenAI or local models via Ollama
  - Outputs structured JSONL with character offsets
- **License:** Apache 2.0 (fully open source, free to use commercially)
- **GitHub:** github.com/google/langextract
- **Usage pattern:** Define extraction schemas for each regulation type (GDPR articles, SOC 2 controls, EU AI Act requirements), run multi-pass extraction, output structured rules with source grounding

**Complementary: Claude API with Structured Outputs**

- Use Claude (Sonnet or Opus) for the "understanding" layer — mapping abstract legal language to concrete technical requirements
- Claude's structured output mode ensures consistent JSON schema
- LangExtract handles extraction + grounding; Claude handles the legal-to-technical translation
- Both outputs feed into the human review queue

### 2.3 RAG for Regulatory Cross-References

Regulations reference each other extensively. The system needs to understand and traverse these cross-references.

**Recommended Tool: PageIndex (by VectifyAI)**

- **Why:** This is the second tool you found — and it solves a critical problem. PageIndex builds hierarchical tree indexes from documents (like a smart table of contents) and uses LLM reasoning for retrieval instead of vector similarity search.
- **Key advantages for VibeGuard:**
  - No vector DB needed — uses document structure and reasoning
  - 98.7% accuracy on FinanceBench (state-of-the-art for professional document analysis)
  - Builds tree structures that mirror how regulations are actually organized (chapters → articles → paragraphs)
  - Reasoning-based retrieval understands cross-references ("Article 17 references Article 6") rather than just finding similar text
  - Better explainability and traceability — retrieval is traceable with page and section references
- **GitHub:** github.com/VectifyAI/PageIndex (11.6k stars)
- **License:** MIT (fully open source)
- **Usage pattern:** Build PageIndex trees for each regulation document. When extracting rules for Article 17, use PageIndex to automatically traverse to referenced Article 6 and Recital 65. Store the cross-reference graph.

**Why NOT traditional vector RAG here:**

Traditional vector-based RAG (e.g., Pinecone + embeddings) finds text that is semantically *similar* but not necessarily *relevant*. When Article 17 says "subject to Article 6(1)", vector search might return random paragraphs mentioning "articles" or "subjects." PageIndex's reasoning-based approach understands this is a structural cross-reference and navigates directly to the correct section. For legal documents where precision is non-negotiable, this is a significant advantage.

### 2.4 Human Review Interface

The final gate before any AI-extracted rule becomes active. This is a custom-built React interface (part of the dashboard).

- Show side-by-side: original regulation text ↔ AI-proposed structured rule
- Highlight source grounding (character offsets from LangExtract)
- Show cross-reference impact graph
- Actions: Approve / Edit & Approve / Reject / Defer
- All review actions logged immutably for audit trail

---

## 3. Layer 2: Code Analysis & Scanning Engine

This layer is entirely deterministic — NO AI/LLM involved. It operates like a specialized linter that checks regulatory compliance instead of code style.

### 3.1 AST Parsing (Multi-Language)

The scanning engine needs to parse code into ASTs for structural analysis across Python, TypeScript, Go, Java, and Rust.

**Recommended Primary Tool: Tree-sitter**

- **Why:** Industry standard for multi-language incremental parsing. Used by GitHub, Neovim, every major code editor, and tools like Semgrep. Supports 100+ languages via community grammars. Written in C with bindings for Go, Rust, Python, JS.
- **For the Go-based CLI (VGX):** Use `go-tree-sitter` (github.com/smacker/go-tree-sitter) — provides Go bindings via cgo. Benchmarks show 36x speedup over traditional parsers.
- **Language grammars needed:** tree-sitter-python, tree-sitter-typescript, tree-sitter-go, tree-sitter-java, tree-sitter-rust (all official/community maintained)

**Complementary Tool: ast-grep**

- **Why:** ast-grep is built on top of Tree-sitter but adds a powerful pattern-matching and linting layer that maps extremely well to VibeGuard's compliance rule engine.
- **Key capabilities for VibeGuard:**
  - Write compliance rules as YAML that look like actual code patterns — easier for the compliance team to understand and review
  - Pattern-based search: `pattern: 'app.delete($PATH, $HANDLER)'` to find deletion endpoints
  - Relational rules: find patterns *inside* other patterns (e.g., find `await` inside `for` loops)
  - Multi-language support out of the box
  - SARIF output for CI/CD integration
  - Rust-based, extremely fast, multi-core
  - Node.js API for programmatic access
- **GitHub:** github.com/ast-grep/ast-grep (14k+ stars)
- **License:** MIT
- **Usage pattern:** Compliance rules (from Layer 1) get translated into ast-grep YAML rules. These run deterministically — same code, same rules, same results every time.

**Example: GDPR Article 17 compliance check as an ast-grep rule:**

```yaml
id: gdpr-17-1-delete-endpoint
language: Python
rule:
  any:
    - pattern: "@app.route($PATH, methods=['DELETE'])"
    - pattern: "@app.delete($PATH)"
    - pattern: "router.delete($PATH, $HANDLER)"
metadata:
  regulation: GDPR
  article: "17"
  paragraph: "1"
  requirement: "Application must have data deletion API endpoint"
  severity: critical
  source_text: "The data subject shall have the right to obtain..."
message: "GDPR Art. 17(1) requires a data deletion endpoint. No DELETE route found for user data."
severity: error
```

### 3.2 Taint Analysis & Data Flow (PII Tracking)

For GDPR Article 30 (Records of Processing Activities) and PII flow mapping, VibeGuard needs to trace how personal data flows through the system.

**Recommended Primary Tool: Joern**

- **Why:** Open-source code analysis platform that generates Code Property Graphs (CPGs) — a unified representation combining ASTs, control flow graphs, and data dependency graphs. Provides inter-procedural taint analysis across multiple languages.
- **Key capabilities for VibeGuard:**
  - Language-agnostic taint analysis (C, C++, Java, JavaScript, Python, Kotlin, PHP)
  - Define sources (e.g., API inputs, form data) and sinks (e.g., database writes, log statements, third-party API calls)
  - Trace data flow from API input → processing → storage → transmission → logging
  - Scala-based query language for writing custom taint queries
  - Recent research shows Joern is scalable for partial-program analysis of modern programs
  - Can annotate external library behavior for more precise analysis
- **GitHub:** github.com/joernio/joern (2.2k+ stars)
- **License:** Apache 2.0
- **Usage pattern:** Run Joern on customer codebases to build CPGs. Execute taint queries that trace PII-tagged variables through the codebase. Output: PII flow map showing entry points → processing paths → storage locations → transmission points.

**Complementary Tool: Opengrep (Semgrep fork)**

- **Why:** After Semgrep changed its licensing, the community forked it as Opengrep with previously restricted features now open. Offers intra-file cross-function taint analysis (detected 7/9 multi-hop cases vs Semgrep's 4/9 in benchmarks).
- **Key advantages:** YAML-based rules (easy to author), fast scanning, SARIF output, large community rule library
- **GitHub:** github.com/opengrep/opengrep
- **License:** LGPL-2.1
- **Usage pattern:** Use for pattern-matching compliance checks where full taint analysis isn't needed. Complements ast-grep for cases where Semgrep/Opengrep's existing rule ecosystem has relevant patterns.

**For Deep Taint Analysis: CodeQL**

- **Why:** GitHub's CodeQL provides the highest precision for taint analysis (88% accuracy, 5% false positive rate in benchmarks). Creates queryable databases from code.
- **Limitation:** Not free for private/commercial code. Requires buildable environment.
- **Usage pattern:** Offer as a premium/enterprise integration option for customers already using GitHub Advanced Security. VibeGuard can import CodeQL SARIF results and correlate with compliance rules.

### 3.3 Configuration & Infrastructure Checks

Many compliance requirements map to infrastructure configurations, not application code.

**Recommended Approach: Custom JSON/YAML rule engine + existing tools**

- **OPA (Open Policy Agent)** — for policy-as-code evaluation against configuration files (Terraform, Kubernetes manifests, cloud configs). Written in Go, deterministic evaluation.
- **Checkov** (by Bridgecrew) — open-source static analysis for IaC (Terraform, CloudFormation, Kubernetes). Has 1000+ built-in policies.
- **Custom config_check rule type:** For database encryption settings, cloud region configs, TLS settings — parse config files (YAML, JSON, TOML, HCL) and run deterministic checks.

---

## 4. Layer 3: Gap Detection & AI-Assisted Analysis

For requirements that can't be fully expressed as deterministic checks (e.g., "adequate organizational measures"), AI assists but with hard confidence thresholds.

### 4.1 Confidence Scoring Engine

**Approach: Claude API (Sonnet) with structured output + confidence calibration**

- Each ambiguous finding gets a confidence score (0-100)
- Above 80: Flag as "likely non-compliant" with citation, marked "requires human review"
- Below 80: Hard stop — "Cannot determine compliance. Manual review required."
- Every AI assessment includes: the regulation text, the code context, the reasoning, and explicit uncertainty markers
- Results are NEVER presented as deterministic — always clearly labeled as AI-assisted

### 4.2 Remediation Suggestions

**Tool: Claude API with regulation-specific context**

- When a deterministic check fails, generate remediation guidance
- Include: code patterns that would satisfy the requirement, links to regulation text, examples from the curated rule library
- Clearly labeled as suggestions, not authoritative compliance advice

---

## 5. Regulatory Feed & Versioning System

(As detailed in the existing Versioning Architecture document, with the following tool selections)

### 5.1 Regulatory Source Monitoring

**API Integrations (free, government-provided):**
- **EUR-Lex CELLAR/SPARQL API** — for all EU regulations (GDPR, AI Act, DORA, NIS2). Free, no API key.
- **eCFR API** — for US regulations (HIPAA). Free, rate-limited.
- **California Legislative Info API** — for CCPA/CPRA. Free.

**Web Scraping (for non-API sources):**
- **Crawlee** (by Apify) — TypeScript web scraping framework, handles JavaScript-rendered pages, built-in anti-blocking, proxy rotation. MIT licensed.
- **Alternative:** Playwright + custom scrapers for PCI SSC, AICPA, ISO metadata pages.

### 5.2 Change Detection & Diffing

**Document Fingerprinting:** SHA-256 of full text for quick change detection.

**Section-Level Diffing:**
- **diff-match-patch** (by Google) — for text-level diffs of regulation sections
- **Custom section parser** — break regulations into articles/sections using PageIndex tree structure, then diff each section independently
- **LangExtract** — re-extract structured data from changed sections, compare against previous extraction

### 5.3 Event Queue & Processing

**Recommended: Apache Kafka (or AWS SQS for simpler setup)**

- Kafka for regulatory change events that need to cascade to multiple consumers (re-scan pipeline, notification service, dashboard updates)
- For V1 (lower scale): AWS SQS + SNS is simpler and sufficient
- For V2+: Migrate to Kafka for more complex event routing and replay capability

---

## 6. Dashboard & Web Platform

### 6.1 Frontend Stack

**Recommended: Next.js 15 + TypeScript + Tailwind CSS + shadcn/ui**

- **Next.js 15:** App Router, Server Components, built-in API routes, excellent DX
- **shadcn/ui:** Pre-built, accessible components (tables, charts, forms, dialogs). Not a dependency — components are copied into the project, fully customizable.
- **Recharts or Tremor:** For compliance status charts, risk score visualizations, trend graphs
- **React Flow:** For PII data flow visualization (interactive node-based diagrams)
- **Monaco Editor:** For inline code viewing with compliance annotations (same editor as VS Code)

### 6.2 Authentication & Authorization

**Recommended: Clerk or Auth0**

- SSO support (SAML, OIDC) required for enterprise customers
- Role-based access: Developer, Compliance Officer, Auditor, Admin
- Team management, organization hierarchy

### 6.3 Real-time Updates

**Recommended: Pusher or Ably (or self-hosted with Socket.io)**

- Push scan results, compliance status changes, regulatory update notifications
- Dashboard live updates without polling

---

## 7. Developer Tooling (CLI, IDE, CI/CD)

### 7.1 CLI Scanner (VGX)

**Language: Go**

- Single binary distribution, cross-platform (Linux, macOS, Windows)
- Built on: Tree-sitter (via go-tree-sitter) + ast-grep rules (via subprocess or NAPI bridge) + custom rule engine
- Local execution — code never leaves the developer's machine
- Output formats: JSON, SARIF, human-readable table
- Fast enough for pre-commit hooks (target: <5s for typical project)

### 7.2 VS Code Extension

**Built with: VS Code Extension API (TypeScript)**

- Language Server Protocol (LSP) for inline diagnostics
- Inline compliance warnings (red squiggly lines + hover for regulation citation)
- Quick-fix suggestions linking to remediation guidance
- Status bar showing compliance score
- Links to regulation source text

### 7.3 GitHub Action

**Built with: Docker-based GitHub Action**

- Runs VGX in CI/CD pipeline
- SARIF output → GitHub Security tab integration
- Configurable severity thresholds (block on critical, warn on moderate)
- Compliance status badges for repositories
- PR comments with compliance diff (new findings, resolved findings)

---

## 8. Data Layer & Infrastructure

### 8.1 Primary Database

**Recommended: PostgreSQL 16+**

- JSONB columns for flexible compliance rule storage (technical_checks, cross_references)
- Excellent indexing for the query patterns VibeGuard needs
- Row-level security for multi-tenant data isolation
- pg_crypto for audit log integrity
- Full-text search for regulation text search

### 8.2 Audit Log (Append-Only)

**Recommended: Custom append-only table in PostgreSQL + S3 archival**

- Every scan, finding, remediation, rule change logged immutably
- Cryptographic chaining (each entry includes hash of previous entry) for tamper detection
- Timestamp + commit hash + developer identity + scan results
- S3 archival with lifecycle policies for long-term retention
- For enterprise: Consider Amazon QLDB or Immuta for stronger immutability guarantees

### 8.3 Object Storage

**Recommended: AWS S3 (or MinIO for self-hosted/on-prem)**

- Store regulation PDFs, scan results, evidence packages, audit reports
- Versioned buckets for regulatory document versions
- Pre-signed URLs for secure report downloads

### 8.4 Cache Layer

**Recommended: Redis**

- Cache scan results for frequently-scanned repositories
- Rate limiting for API endpoints
- Session storage for dashboard
- Pub/sub for real-time notifications

### 8.5 Search (for regulation text search)

**Recommended: PostgreSQL Full-Text Search (V1) → Typesense or Meilisearch (V2+)**

- V1: PostgreSQL's built-in full-text search is sufficient for searching across regulation text, compliance rules, and scan findings
- V2+: If search volume/complexity grows, Typesense (MIT licensed, Rust-based, fast) for a dedicated search engine

---

## 9. RAG & Knowledge System

### 9.1 Architecture: Hybrid Approach (No Traditional Vector RAG)

VibeGuard's regulatory knowledge system deliberately avoids traditional vector-based RAG to prevent the "similarity ≠ relevance" problem.

**The Knowledge Pipeline:**

```
Regulatory PDF
    ↓
MinerU (PDF → structured text)
    ↓
PageIndex (build hierarchical tree index)
    ↓
LangExtract (extract structured rules with source grounding)
    ↓
Claude API (legal → technical translation)
    ↓
Human Review Queue
    ↓
Verified Rule Library (JSON in PostgreSQL)
    ↓
Deterministic Scanning Engine (ast-grep + Joern + custom checks)
```

**Why this works better than vector RAG for VibeGuard:**

| Aspect | Vector RAG | VibeGuard's Approach |
|--------|------------|---------------------|
| Retrieval basis | Semantic similarity | Structural reasoning + exact citation |
| Determinism | Different results each time | Same rules → same results always |
| Traceability | Opaque embedding distances | Character-level source grounding |
| Cross-references | May miss structural links | PageIndex traverses document hierarchy |
| Auditability | "The AI said so" | Every finding cites exact article, paragraph, and source text |

### 9.2 When Vector Embeddings ARE Used

There is one place where embeddings add value: **semantic search across the rule library itself.**

**Recommended: PostgreSQL pgvector extension**

- When a compliance officer types "where do we check for data retention policies?" → search across 5,000 compliance rules using semantic similarity
- Use OpenAI `text-embedding-3-small` or `nomic-embed-text` (open source) for embedding rule descriptions
- pgvector keeps everything in PostgreSQL — no separate vector DB to manage
- This is a search/discovery feature, NOT part of the compliance verification pipeline

---

## 10. Complete Technology Stack Summary

### Core Processing

| Component | Tool | License | Why |
|-----------|------|---------|-----|
| **PDF Parsing** | MinerU | AGPL-3.0 | Best accuracy for complex legal PDFs, structured output, active development |
| **PDF Parsing (backup)** | Docling (IBM) | MIT | Better LLM pipeline integration, modular |
| **Structured Extraction** | LangExtract (Google) | Apache 2.0 | Source-grounded extraction, character-level citations, multi-model support |
| **Document RAG** | PageIndex (VectifyAI) | MIT | Reasoning-based retrieval, tree-structured indexes, 98.7% accuracy on benchmarks |
| **AST Parsing** | Tree-sitter | MIT | Industry standard, 100+ languages, incremental, fast |
| **Pattern Matching/Linting** | ast-grep | MIT | YAML rules that look like code, built on Tree-sitter, blazing fast (Rust) |
| **Taint Analysis** | Joern | Apache 2.0 | Code Property Graphs, inter-procedural taint, multi-language |
| **Pattern Rules (complement)** | Opengrep (Semgrep fork) | LGPL-2.1 | Large rule ecosystem, cross-function taint, SARIF output |
| **IaC Policy Checks** | OPA (Open Policy Agent) | Apache 2.0 | Deterministic policy evaluation, Rego language |
| **AI Translation** | Claude API (Anthropic) | Commercial | Legal text → technical requirements, structured output |
| **AI Extraction (backup)** | Gemini API (Google) | Commercial | Controlled generation, schema enforcement |

### Developer Tools

| Component | Tool | Language | Notes |
|-----------|------|----------|-------|
| **CLI (VGX)** | Custom | Go | Single binary, Tree-sitter + ast-grep rules |
| **VS Code Extension** | Custom | TypeScript | LSP-based, inline diagnostics |
| **GitHub Action** | Custom | Docker | SARIF output, GitHub Security tab integration |
| **Pre-commit Hook** | Custom | Shell + Go | Lightweight subset of VGX checks |

### Web Platform

| Component | Tool | Notes |
|-----------|------|-------|
| **Frontend** | Next.js 15 + TypeScript | App Router, Server Components |
| **UI Components** | shadcn/ui + Tailwind CSS | Accessible, customizable |
| **Charts** | Recharts / Tremor | Compliance visualizations |
| **Data Flow Viz** | React Flow | PII flow mapping diagrams |
| **Code Viewer** | Monaco Editor | Inline compliance annotations |
| **Auth** | Clerk or Auth0 | SSO, RBAC, team management |

### Infrastructure

| Component | Tool | Notes |
|-----------|------|-------|
| **Database** | PostgreSQL 16+ | JSONB, pgvector, row-level security |
| **Vector Search** | pgvector (PostgreSQL ext) | Rule library semantic search only |
| **Cache** | Redis | Scan results, rate limiting, sessions |
| **Event Queue** | AWS SQS (V1) → Kafka (V2+) | Regulatory change events, re-scan pipeline |
| **Object Storage** | AWS S3 / MinIO | PDFs, reports, evidence packages |
| **Search** | PostgreSQL FTS (V1) → Typesense (V2+) | Regulation and rule search |
| **Hosting** | AWS (ECS/EKS) or Fly.io | Container-based deployment |
| **Monitoring** | Grafana + Prometheus | System health, scan metrics |
| **Error Tracking** | Sentry | Application error monitoring |
| **CI/CD** | GitHub Actions | Build, test, deploy pipeline |

### Regulatory Feed

| Component | Tool | Notes |
|-----------|------|-------|
| **EU Regulations** | EUR-Lex SPARQL API | Free, covers GDPR, AI Act, DORA, NIS2 |
| **US Regulations** | eCFR API | Free, covers HIPAA |
| **California** | CA Legislative Info API | Free, covers CCPA/CPRA |
| **Web Scraping** | Crawlee (Apify) | TypeScript, anti-blocking, for non-API sources |
| **Diffing** | diff-match-patch + custom | Section-level regulatory change detection |

---

## 11. Build vs. Buy Decisions

### Build In-House (Core IP)

- **Compliance Rule Engine:** The mapping from legal text to code checks is VibeGuard's core IP. This must be built and owned entirely.
- **Citation Engine:** The zero-hallucination guarantee and citation architecture. Non-negotiable to own.
- **Rule Data Model:** The JSON specification format and versioning system.
- **CLI Scanner (VGX):** The Go-based scanning tool that orchestrates Tree-sitter, ast-grep, Joern, and custom checks.
- **Human Review UI:** The interface where compliance experts validate AI-proposed rules.

### Use Open-Source Tools (Integrate)

- **Tree-sitter, ast-grep, Joern, Opengrep:** These are the "engines" that VGX orchestrates. Integrate, don't rebuild.
- **MinerU, LangExtract, PageIndex:** Document processing pipeline. Integrate and customize.
- **OPA:** Policy evaluation. Embed the Go library.

### Buy/Subscribe (Infrastructure)

- **Cloud hosting (AWS/GCP):** Not worth self-managing.
- **Auth (Clerk/Auth0):** SSO, SAML, enterprise auth is complex to build.
- **LLM APIs (Claude, Gemini):** For Layer 1 parsing and Layer 3 gap detection.
- **Error tracking (Sentry):** Standard SaaS.

### Partner Decision: Regulatory Data Feeds

- **V1 (Months 1-8):** Use free government APIs (EUR-Lex, eCFR). Manual upload for others.
- **V2 (Months 9-18):** Evaluate Thomson Reuters Regulatory Intelligence, LexisNexis, or Ascent RegTech ($50K-200K/yr) when expanding past 5-6 frameworks. The ROI of not maintaining 20+ scrapers across jurisdictions will be clear by then.

---

## 12. Phase-wise Tool Adoption Roadmap

### Phase 1: Foundation (Months 1–3)

**Priority: Get the scanning engine and basic rule library working.**

| Tool | Purpose | Priority |
|------|---------|----------|
| Tree-sitter (Go bindings) | AST parsing for VGX CLI | P0 |
| ast-grep | Compliance rule execution | P0 |
| PostgreSQL + pgvector | Rule storage, scan results | P0 |
| MinerU | PDF parsing for initial rule extraction | P0 |
| LangExtract | Structured extraction from GDPR/SOC 2/EU AI Act | P0 |
| Claude API | Legal → technical translation | P0 |
| Next.js + shadcn/ui | Basic dashboard | P1 |
| VS Code Extension | Inline diagnostics | P1 |
| GitHub Actions (CI/CD) | GitHub Action for PR checks | P1 |

**Deliverables:** VGX CLI scanning Python + TypeScript against GDPR/SOC 2/EU AI Act rules. VS Code extension. Basic dashboard.

### Phase 2: Platform (Months 4–8)

**Priority: Document ingestion, audit trail, PII mapping.**

| Tool | Purpose | Priority |
|------|---------|----------|
| PageIndex | Cross-reference resolution for regulation documents | P0 |
| Joern | PII taint analysis / data flow mapping | P0 |
| React Flow | PII flow visualization in dashboard | P0 |
| AWS SQS | Event queue for re-scan pipeline | P1 |
| Opengrep | Additional pattern rules, expanded coverage | P1 |
| OPA | IaC and config policy checks | P1 |
| Redis | Caching, rate limiting | P1 |
| Clerk/Auth0 | Enterprise SSO | P2 |

**Deliverables:** Document ingestion engine. PII data flow mapping. Audit trail. Policy-as-code. 10-20 paying customers.

### Phase 3: Scale (Months 9–18)

**Priority: Automation, integrations, enterprise features.**

| Tool | Purpose | Priority |
|------|---------|----------|
| Kafka | Event streaming for regulatory change cascade | P1 |
| Crawlee | Web scraping for non-API regulatory sources | P1 |
| EUR-Lex SPARQL API | Automated EU regulation monitoring | P1 |
| eCFR API | Automated US regulation monitoring | P1 |
| Typesense | Dedicated search engine | P2 |
| Grafana + Prometheus | Production monitoring | P1 |
| Regulatory data partner | Long-tail framework coverage | P2 (evaluate) |

**Deliverables:** 10+ frameworks. MCP governance. License compliance. Risk scoring. Vanta/Drata integrations. 50-100 paying customers.

---

## Key Architecture Decisions Summary

1. **LangExtract over custom RAG extraction** — Source grounding (character offsets) directly powers the citation engine. No need to build custom extraction pipeline.

2. **PageIndex over vector RAG** — Reasoning-based retrieval with tree indexes is superior to vector similarity for structured legal documents with cross-references.

3. **ast-grep over raw Tree-sitter rules** — YAML rules that look like code are easier for the compliance team to review and maintain. Built on Tree-sitter anyway.

4. **Joern over CodeQL for taint analysis** — Open source (Apache 2.0), language-agnostic, no build environment required. CodeQL offered as enterprise add-on.

5. **MinerU over cloud-only parsers** — Runs locally, best accuracy for complex legal documents, no data leaves the system.

6. **PostgreSQL + pgvector over separate vector DB** — Keeps everything in one database. Vector search is only for rule discovery, not compliance verification.

7. **No AI in the verification loop** — Layer 2 is entirely deterministic. AI assists in Layer 1 (extraction) and Layer 3 (gap detection), always gated by human review or hard confidence thresholds.

---

*This architecture is designed to evolve. Start with the simplest version that works (Phase 1), validate with real customers, then add complexity as the product-market fit demands it.*
