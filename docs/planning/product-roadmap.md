# VibeGuard — Product Roadmap

**Created:** 2026-02-10 | **Version:** 1.0

---

## Vision

VibeGuard is a compliance-as-code platform that automatically scans source code against regulatory frameworks and produces deterministic, citation-backed findings. Every finding traces from code → rule → exact character offset in the original regulation text. Zero hallucination.

---

## MVP — Weeks 1-9

**Goal:** Ship a working product that scans code against GDPR with full document ingestion pipeline.

### P0 (Must Have)

| Feature | Component | Description |
|---------|-----------|-------------|
| **GDPR rule library** | VGX + Ingestion | Pre-built ast-grep rules for GDPR Articles 5-49. Also ingestion pipeline can extract new rules from uploaded PDFs. |
| **Document ingestion** | Ingestion | Upload GDPR PDF → Marker parses → PageIndex indexes → LangExtract extracts → Claude translates → human reviews → active rules |
| **CLI scanner** | VGX | `vgx scan ./my-project` scans Python + TypeScript against active GDPR rules. Outputs JSON/SARIF/table. |
| **REST API** | VGX | Authenticated API for scans, rules, documents, dashboard data |
| **Compliance dashboard** | Dashboard | Org compliance overview, scan history, compliance score |
| **Rule review UI** | Dashboard | Side-by-side: regulation text + AI-proposed rule. Approve/reject/edit. Source text highlighting via LangExtract char offsets. |
| **Document upload UI** | Dashboard | Upload regulatory PDF, track extraction progress, view proposed rules |
| **Citation engine** | VGX | Every finding includes: regulation, article, paragraph, verbatim source text, character offsets |
| **Clerk auth** | VGX + Dashboard | JWT auth, org management, role-based access (developer, compliance officer, admin) |
| **PostgreSQL schema** | VGX | organizations, users, documents, proposed_rules, compliance_rules, scan_results, audit_log |
| **Docker deployment** | Docker | docker-compose with Go + Python + PostgreSQL on Hetzner CX33 |

### P1 (Should Have)

| Feature | Component | Description |
|---------|-----------|-------------|
| **Scan diff in CLI** | VGX | `vgx scan --diff` shows new/resolved findings vs. last scan |
| **Bulk rule approval** | Dashboard | Approve all pending rules for a document at once |
| **Rule editing** | Dashboard | Edit AI-proposed rules before approval (modify patterns, severity, remediation) |
| **Export scan results** | VGX | Export as PDF report or SARIF for CI/CD integration |
| **Remediation guidance** | Dashboard | Code examples showing how to fix each finding, per language |

### P2 (Nice to Have)

| Feature | Component | Description |
|---------|-----------|-------------|
| **Dark mode** | Dashboard | Tailwind dark mode toggle |
| **Keyboard shortcuts** | Dashboard | Navigate findings, approve/reject rules with keyboard |
| **Scan scheduling** | VGX | Periodic re-scans on a schedule |

---

## Phase 2 — Months 4-8

**Goal:** Expand framework coverage, add PII tracking, enterprise features.

### P0

| Feature | Component | Description |
|---------|-----------|-------------|
| **SOC 2 framework** | Ingestion + VGX | Add SOC 2 Type II controls (Trust Services Criteria). ~80 rules. |
| **EU AI Act framework** | Ingestion + VGX | EU AI Act requirements. High-risk AI system checks. |
| **PII taint analysis** | VGX (Joern) | Trace personal data flow through code: API input → processing → storage → transmission. Map to GDPR Art. 30 (Records of Processing). |
| **PII flow visualization** | Dashboard (React Flow) | Interactive node-based diagram showing PII data flows |
| **Audit trail** | VGX + Dashboard | Cryptographically chained append-only log. View all compliance events. Export for auditors. |
| **Go + Java language support** | VGX | Extend scanner to Go and Java codebases |

### P1

| Feature | Component | Description |
|---------|-----------|-------------|
| **Cross-reference resolution** | Ingestion (PageIndex) | Automatic traversal of regulation cross-references (e.g., "subject to Article 6(1)") |
| **OPA config checks** | VGX | Check Terraform, Kubernetes YAML, cloud configs against compliance rules |
| **Opengrep integration** | VGX | Additional pattern rules from Semgrep/Opengrep ecosystem |
| **Evidence packages** | Dashboard | Generate audit-ready evidence packages (scan results + citations + PII maps) |
| **Multi-org support** | Dashboard | Manage multiple organizations with separate compliance postures |
| **Webhook notifications** | VGX | Notify Slack/Teams/email on compliance status changes |

### P2

| Feature | Component | Description |
|---------|-----------|-------------|
| **Redis caching** | VGX | Cache scan results, rate limiting, session storage |
| **pgvector semantic search** | VGX | Search across rule library by natural language query |
| **Compliance scoring model** | VGX | Weighted compliance score based on rule severity and coverage |

---

## Phase 3 — Months 9-18

**Goal:** Scale to 10+ frameworks, regulatory feeds, CI/CD integrations, enterprise.

### P0

| Feature | Component | Description |
|---------|-----------|-------------|
| **Regulatory feed monitoring** | New service | EUR-Lex SPARQL API (EU regulations), eCFR API (US), CA Legislative API (CCPA). Auto-detect regulation updates. |
| **VS Code extension** | New package | LSP-based inline diagnostics. Red squiggly lines + hover for regulation citation. Quick-fix suggestions. |
| **GitHub Action** | New package | Run VGX in CI/CD. SARIF → GitHub Security tab. PR comments with compliance diff. Severity-based blocking. |
| **HIPAA framework** | Ingestion | US healthcare compliance |
| **DORA framework** | Ingestion | Digital Operational Resilience Act (financial sector) |
| **NIS2 framework** | Ingestion | EU cybersecurity directive |

### P1

| Feature | Component | Description |
|---------|-----------|-------------|
| **Kafka event streaming** | Infrastructure | Replace simple queues with Kafka for regulatory change cascade, re-scan pipeline, notification routing |
| **Crawlee web scraping** | Regulatory feed | Scrape non-API regulatory sources (PCI SSC, AICPA) |
| **Section-level diffing** | Regulatory feed | Detect changes at article/section level, auto-trigger re-extraction for changed sections |
| **Vanta/Drata integration** | New integration | Import/export compliance data to/from Vanta, Drata |
| **Rust language support** | VGX | Extend scanner to Rust codebases |
| **Typesense search** | Infrastructure | Dedicated search engine for regulation text and rule library |

### P2

| Feature | Component | Description |
|---------|-----------|-------------|
| **MCP governance** | New feature | Model Context Protocol support for AI-assisted compliance queries |
| **License compliance** | VGX | Scan dependencies for license compliance (GPL, AGPL, etc.) |
| **Risk scoring dashboard** | Dashboard | Organization-level risk scoring across all frameworks |
| **Regulatory data partner** | Business | Evaluate Thomson Reuters / LexisNexis for long-tail framework coverage |
| **On-prem deployment** | Infrastructure | Self-hosted option for enterprises that can't use cloud |
| **SOC 2 certification** | Business | Get VibeGuard itself SOC 2 certified |

---

## Revenue Milestones

| Phase | Target | Revenue |
|-------|--------|---------|
| MVP (Month 3) | 5-10 design partners (free/discounted) | $0 |
| Phase 2 (Month 6) | 10-20 paying customers | $5K-15K MRR |
| Phase 2 (Month 8) | 20-50 paying customers | $15K-40K MRR |
| Phase 3 (Month 12) | 50-100 paying customers | $40K-100K MRR |
| Phase 3 (Month 18) | 100-200 paying customers | $100K-250K MRR |

### Pricing Tiers (Planned)

| Tier | Price | Includes |
|------|-------|----------|
| **Free** | $0/month | 1 framework, 1 repo, CLI only |
| **Team** | $500/month | 5 frameworks, unlimited repos, dashboard, API, CI/CD |
| **Enterprise** | Custom | All frameworks, SSO, audit trail, PII mapping, on-prem option |

---

## Cost Per Document Extraction

| Component | Cost Per 100-Page Document |
|-----------|---------------------------|
| Marker (local) | $0 (runs on VPS) |
| PageIndex (local) | $0 (runs on VPS) |
| LangExtract + Claude API | ~$0.50-1.50 |
| Claude technical translation | ~$0.50-1.00 |
| **Total** | **~$1.00-2.50 per document** |

At Team tier ($500/month), even 50 documents/month costs $50-125 in API fees. Excellent margins.

---

## Feature Backlog (Unscheduled)

These are ideas that don't fit neatly into a phase yet:

- [ ] Compliance chatbot (ask questions about your compliance status)
- [ ] Auto-remediation PRs (generate fix PRs for failing checks)
- [ ] Compliance-as-code SDK (let customers write custom rules)
- [ ] Multi-language scan in single run (detect language per file)
- [ ] Historical compliance trending (how has compliance changed over time)
- [ ] Regulation comparison (diff two versions of the same regulation)
- [ ] Compliance templates (starter configs for common tech stacks)
- [ ] Partner program (compliance consultancies can white-label or resell)
