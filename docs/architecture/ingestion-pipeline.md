# VibeGuard MVP — Document Ingestion Pipeline (Updated)

## Marker + LangExtract + PageIndex

**This replaces the previous "defer PDF pipeline" recommendation.** Document ingestion is VibeGuard's core differentiator — without it, we're just another linting tool.

---

## 1. Why These Three Tools

| Tool | Role | License | What It Does for VibeGuard |
|------|------|---------|--------------------------|
| **Marker** (v1.10.2) | PDF → Structured Markdown/JSON | GPL-3.0 + cc-by-nc-sa-4.0 models (free under $5M revenue) | Parses complex regulatory PDFs (GDPR: 88pp, EU AI Act: 144pp) into clean structured text with tables, cross-references, headings preserved |
| **LangExtract** (Google) | Text → Structured Rules with Source Grounding | Apache-2.0 | Extracts compliance rules with **character-level offsets** back to source text. Every extracted entity traces to its exact location. This directly powers the citation engine. |
| **PageIndex** (VectifyAI) | Cross-Reference Traversal | MIT | Builds hierarchical tree index of regulation documents. When Art. 17 references Art. 6, PageIndex traverses the tree structure to resolve it — no vector DB, no chunking. 98.7% accuracy on FinanceBench. |

### How They Chain Together

```
Regulatory PDF (e.g., GDPR, 88 pages)
        │
        ▼
┌───────────────────────────────────────────┐
│  MARKER                                   │
│  PDF → Structured Markdown + JSON         │
│                                           │
│  Input:  regulation.pdf                   │
│  Output: Clean markdown with headings,    │
│          article numbers, tables,         │
│          page boundaries preserved        │
│                                           │
│  Handles: Multi-column layouts, footnotes,│
│  annexes, table extraction, OCR fallback  │
└───────────────────┬───────────────────────┘
                    │
                    ▼
┌───────────────────────────────────────────┐
│  PAGEINDEX                                │
│  Build hierarchical tree index            │
│                                           │
│  Input:  Marker's structured markdown     │
│  Output: Tree structure mirroring         │
│          regulation hierarchy             │
│          (Chapters → Articles → Paragraphs)│
│                                           │
│  Enables: "Art. 17 references Art. 6"     │
│           → traverse tree to Art. 6 text  │
│           → resolve all cross-references  │
└───────────────────┬───────────────────────┘
                    │
                    ▼
┌───────────────────────────────────────────┐
│  LANGEXTRACT                              │
│  Extract structured compliance rules      │
│  with character-level source grounding    │
│                                           │
│  Input:  Parsed text + tree structure     │
│  Output: Structured rules (JSON), each    │
│          with exact char offsets back to   │
│          source regulation text           │
│                                           │
│  Uses Claude/Gemini API for extraction    │
│  Source grounding = citation engine fuel  │
└───────────────────┬───────────────────────┘
                    │
                    ▼
┌───────────────────────────────────────────┐
│  CLAUDE API                               │
│  Legal → Technical translation            │
│                                           │
│  Input:  Extracted rules + source text    │
│  Output: Technical checks, ast-grep       │
│          patterns, remediation guidance   │
│                                           │
│  "Right to erasure" → "Must have DELETE   │
│   endpoint for user data with cascade"    │
└───────────────────┬───────────────────────┘
                    │
                    ▼
┌───────────────────────────────────────────┐
│  HUMAN REVIEW QUEUE                       │
│  Zero-hallucination gate                  │
│                                           │
│  Reviewer sees: source text ↔ proposed    │
│  rule ↔ character offsets ↔ ast-grep      │
│  pattern. Approve / Edit / Reject.        │
└───────────────────┬───────────────────────┘
                    │
                    ▼
          Active Compliance Rules
          (identical to pre-built rules)
```

---

## 2. Architecture Change: Python Sidecar

Marker, LangExtract, and PageIndex are all Python libraries. The Go binary can't embed them directly. Solution: a small **Python FastAPI service** that handles document ingestion only, running alongside the Go binary.

```
┌───────────────────────────────────────────────────────────────┐
│                     Hetzner CX33 VPS                          │
│                                                               │
│  ┌─────────────────────────┐  ┌────────────────────────────┐ │
│  │    Go Binary (VGX)      │  │  Python Ingestion Service  │ │
│  │                         │  │  (FastAPI)                 │ │
│  │  • CLI scanner          │  │                            │ │
│  │  • REST API             │  │  • Marker (PDF parsing)    │ │
│  │  • Embedded React SPA   │  │  • PageIndex (tree index)  │ │
│  │  • ast-grep rules       │  │  • LangExtract (extraction)│ │
│  │  • Citation engine      │  │  • Claude API (translation)│ │
│  │  • Scan orchestration   │  │                            │ │
│  │                         │  │  Listens on :8001          │ │
│  │  Listens on :8080       │  │  (internal only)           │ │
│  └────────────┬────────────┘  └─────────────┬──────────────┘ │
│               │                             │                │
│               └──────────┬──────────────────┘                │
│                          │                                   │
│               ┌──────────▼──────────┐                        │
│               │   PostgreSQL 17     │                        │
│               │                     │                        │
│               │  Shared database:   │                        │
│               │  rules, scans,      │                        │
│               │  documents,         │                        │
│               │  proposed_rules     │                        │
│               └─────────────────────┘                        │
└───────────────────────────────────────────────────────────────┘
```

**Why this works on CX33 (4 vCPU, 8 GB RAM):**
- Marker's lightweight Granite model (~1 GB) runs on CPU. Processing a 100-page PDF takes ~60–120 seconds — acceptable for batch document ingestion (not real-time).
- LangExtract and PageIndex are lightweight Python libraries, minimal RAM.
- The Python service only runs during document processing. It can be idle most of the time.
- The Go binary handles all real-time traffic (scanning, API, dashboard).

**Memory budget:**
- PostgreSQL: ~2 GB (shared_buffers)
- Go binary: ~200 MB
- Python service (idle): ~300 MB
- Python service (processing): ~2–3 GB (Marker model loaded)
- Headroom: ~2.5 GB
- This fits on 8 GB. If tight, configure Marker to load models on-demand and unload after processing.

---

## 3. Python Ingestion Service

### Project Structure

```
ingestion/
├── main.py                    # FastAPI entry point
├── requirements.txt
├── config.py                  # Settings (DB URL, API keys)
│
├── pipeline/
│   ├── __init__.py
│   ├── orchestrator.py        # Full pipeline: PDF → proposed rules
│   ├── pdf_parser.py          # Marker integration
│   ├── tree_indexer.py        # PageIndex integration
│   ├── rule_extractor.py      # LangExtract integration
│   ├── technical_translator.py # Claude API: legal → code checks
│   └── cross_references.py    # Cross-reference resolution via PageIndex
│
├── models/
│   ├── __init__.py
│   ├── document.py            # Document data models
│   ├── proposed_rule.py       # ProposedRule data model
│   └── extraction_schema.py   # LangExtract schema definitions
│
└── db/
    ├── __init__.py
    └── postgres.py            # DB operations (asyncpg)
```

### requirements.txt

```
fastapi==0.115.*
uvicorn==0.34.*
marker-pdf==1.10.*
langextract>=0.5
pageindex>=0.1
anthropic>=0.45
asyncpg>=0.30
pydantic>=2.0
python-multipart>=0.0.18
```

### FastAPI Entry Point

```python
# ingestion/main.py

from fastapi import FastAPI, UploadFile, File, Form, BackgroundTasks
from contextlib import asynccontextmanager
import asyncpg
import os

from pipeline.orchestrator import ExtractionOrchestrator
from config import settings

pool = None

@asynccontextmanager
async def lifespan(app: FastAPI):
    global pool
    pool = await asyncpg.create_pool(settings.DATABASE_URL, min_size=2, max_size=5)
    yield
    await pool.close()

app = FastAPI(title="VibeGuard Ingestion Service", lifespan=lifespan)

@app.post("/ingest")
async def ingest_document(
    background_tasks: BackgroundTasks,
    file: UploadFile = File(...),
    name: str = Form(...),
    description: str = Form(""),
    org_id: str = Form(...),
    uploaded_by: str = Form(...),
):
    """Accept a regulatory PDF and queue extraction."""

    # Save file to disk
    doc_id = str(uuid4())
    upload_dir = settings.UPLOAD_DIR
    os.makedirs(upload_dir, exist_ok=True)
    file_path = os.path.join(upload_dir, f"{doc_id}.pdf")

    contents = await file.read()
    with open(file_path, "wb") as f:
        f.write(contents)

    # Create document record in DB
    async with pool.acquire() as conn:
        await conn.execute("""
            INSERT INTO documents (id, org_id, name, description, file_name,
                                   file_path, file_size, status, uploaded_by)
            VALUES ($1, $2, $3, $4, $5, $6, $7, 'processing', $8)
        """, doc_id, org_id, name, description, file.filename,
             file_path, len(contents), uploaded_by)

    # Run extraction in background
    background_tasks.add_task(run_extraction, doc_id, file_path, pool)

    return {"document_id": doc_id, "status": "processing"}


async def run_extraction(doc_id: str, file_path: str, db_pool):
    """Background task: full extraction pipeline."""
    orchestrator = ExtractionOrchestrator(db_pool=db_pool)
    try:
        result = await orchestrator.process(doc_id, file_path)
        async with db_pool.acquire() as conn:
            await conn.execute("""
                UPDATE documents
                SET status = 'review_ready',
                    rule_count = $2,
                    extraction_meta = $3,
                    updated_at = now()
                WHERE id = $1
            """, doc_id, result["rule_count"], json.dumps(result["meta"]))
    except Exception as e:
        async with db_pool.acquire() as conn:
            await conn.execute("""
                UPDATE documents
                SET status = 'extraction_failed',
                    extraction_meta = $2,
                    updated_at = now()
                WHERE id = $1
            """, doc_id, json.dumps({"error": str(e)}))
        raise


@app.get("/health")
async def health():
    return {"status": "ok", "service": "ingestion"}
```

### Pipeline Orchestrator

```python
# ingestion/pipeline/orchestrator.py

from pipeline.pdf_parser import MarkerParser
from pipeline.tree_indexer import PageIndexer
from pipeline.rule_extractor import LangExtractRuleExtractor
from pipeline.technical_translator import TechnicalTranslator

class ExtractionOrchestrator:
    def __init__(self, db_pool):
        self.db_pool = db_pool
        self.parser = MarkerParser()
        self.indexer = PageIndexer()
        self.extractor = LangExtractRuleExtractor()
        self.translator = TechnicalTranslator()

    async def process(self, doc_id: str, file_path: str) -> dict:
        # === Step 1: Parse PDF with Marker ===
        parsed = self.parser.parse(file_path)
        # parsed.markdown: str (full structured markdown)
        # parsed.json_output: dict (tree-like structure with blocks)
        # parsed.metadata: dict (page count, language, etc.)

        # === Step 2: Build tree index with PageIndex ===
        tree_index = self.indexer.build_index(parsed.markdown)
        # tree_index: hierarchical structure mirroring regulation
        # Chapters → Articles → Paragraphs

        # === Step 3: Extract structured rules with LangExtract ===
        raw_rules = self.extractor.extract(
            text=parsed.markdown,
            tree_index=tree_index,
        )
        # raw_rules: list of dicts with source grounding (char offsets)

        # === Step 4: Resolve cross-references via PageIndex ===
        for rule in raw_rules:
            refs = self._find_cross_references(rule, tree_index)
            rule["cross_references"] = refs

        # === Step 5: Translate legal → technical with Claude ===
        proposed_rules = []
        for rule in raw_rules:
            technical = await self.translator.translate(rule)
            proposed_rules.append({**rule, **technical})

        # === Step 6: Store proposed rules in DB ===
        async with self.db_pool.acquire() as conn:
            for rule in proposed_rules:
                await conn.execute("""
                    INSERT INTO proposed_rules
                    (document_id, rule_id, regulation, article, paragraph,
                     title, requirement, source_text, source_text_offsets,
                     source_page, check_type, severity, confidence,
                     languages, technical_checks, cross_references,
                     remediation, review_status, original_ai_output)
                    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
                            $14,$15,$16,$17,'pending',$18)
                """,
                    doc_id,
                    rule["id"],
                    rule["regulation"],
                    rule.get("article"),
                    rule.get("paragraph"),
                    rule["title"],
                    rule["requirement"],
                    rule["source_text"],
                    json.dumps(rule.get("source_offsets", {})),
                    rule.get("source_page"),
                    rule["check_type"],
                    rule["severity"],
                    rule.get("confidence", "high"),
                    rule.get("languages", []),
                    json.dumps(rule["technical_checks"]),
                    rule.get("cross_references", []),
                    json.dumps(rule.get("remediation", {})),
                    json.dumps(rule),  # raw AI output for debugging
                )

        return {
            "rule_count": len(proposed_rules),
            "meta": {
                "pages": parsed.metadata.get("page_count"),
                "articles_found": len(raw_rules),
                "languages_detected": parsed.metadata.get("languages"),
            }
        }

    def _find_cross_references(self, rule: dict, tree_index) -> list:
        """Use PageIndex to resolve cross-references in source text."""
        source = rule.get("source_text", "")

        # Look for patterns like "Article 6", "Art. 6(1)", "Recital 47"
        import re
        ref_patterns = re.findall(
            r'(?:Article|Art\.?|Recital|Section|Annex)\s+(\d+)(?:\((\d+)\))?',
            source
        )

        refs = []
        for match in ref_patterns:
            article_num = match[0]
            paragraph = match[1] if match[1] else None

            # Use PageIndex tree search to find the referenced section
            ref_node = self.indexer.search_tree(
                tree_index,
                query=f"Article {article_num}",
            )

            if ref_node:
                ref_label = f"{rule['regulation']} Art. {article_num}"
                if paragraph:
                    ref_label += f"({paragraph})"
                refs.append(ref_label)

        return refs
```

### Step 1: Marker PDF Parser

```python
# ingestion/pipeline/pdf_parser.py

from marker.converters.pdf import PdfConverter
from marker.models import create_model_dict
from marker.output import text_from_rendered
from dataclasses import dataclass
import json

@dataclass
class ParsedDocument:
    markdown: str
    json_output: dict
    metadata: dict

class MarkerParser:
    def __init__(self):
        # Load models once, reuse across documents
        self._model_dict = None

    def _ensure_models(self):
        if self._model_dict is None:
            self._model_dict = create_model_dict()

    def parse(self, file_path: str) -> ParsedDocument:
        """Parse a regulatory PDF into structured markdown + JSON."""
        self._ensure_models()

        converter = PdfConverter(artifact_dict=self._model_dict)
        rendered = converter(file_path)

        # Get markdown text
        text, _, images = text_from_rendered(rendered)

        # Get JSON tree structure (Marker's native block hierarchy)
        # This preserves document structure: headings, paragraphs, tables
        json_output = rendered.model_dump() if hasattr(rendered, 'model_dump') else {}

        metadata = {
            "page_count": getattr(rendered, 'page_count', None),
            "languages": getattr(rendered, 'languages', []),
            "file_path": file_path,
        }

        return ParsedDocument(
            markdown=text,
            json_output=json_output,
            metadata=metadata,
        )
```

### Step 2: PageIndex Tree Builder

```python
# ingestion/pipeline/tree_indexer.py

from pageindex import PageIndex

class PageIndexer:
    def __init__(self, model: str = "claude-sonnet-4-20250514"):
        self.model = model

    def build_index(self, markdown: str) -> dict:
        """
        Build a hierarchical tree index from regulation markdown.
        PageIndex creates a ToC-like tree structure that mirrors
        the regulation's natural hierarchy.
        """
        # PageIndex processes markdown with # headings into tree nodes
        index = PageIndex.from_markdown(
            markdown,
            model=self.model,
        )
        return index

    def search_tree(self, index, query: str) -> dict | None:
        """
        Search the tree index for a specific section.
        Uses LLM reasoning to traverse the tree, not vector similarity.
        """
        results = index.search(query, max_results=1)
        if results:
            return results[0]
        return None

    def get_section_text(self, index, article_number: str) -> str | None:
        """Get the full text of a specific article by number."""
        result = self.search_tree(index, f"Article {article_number}")
        if result:
            return result.get("text", "")
        return None
```

### Step 3: LangExtract Rule Extractor

```python
# ingestion/pipeline/rule_extractor.py

import langextract as lx
from models.extraction_schema import ComplianceRuleExtraction, ExtractionExample

class LangExtractRuleExtractor:
    def __init__(self, model: str = "claude-sonnet-4-20250514"):
        self.model = model

    def extract(self, text: str, tree_index=None) -> list[dict]:
        """
        Extract structured compliance rules from regulation text.
        Uses LangExtract for source-grounded extraction.

        Returns rules with character-level offsets back to source text.
        """

        prompt_description = """
        Extract every technical compliance requirement from this regulatory text.

        For each requirement, identify:
        - The article and paragraph number
        - The exact requirement stated in the regulation
        - Whether it can be verified by scanning source code, configuration
          files, or infrastructure settings
        - The severity (critical if violation = direct legal penalty,
          high if significant compliance gap, medium if best practice,
          low if informational)
        - The verbatim source text that states the requirement

        Only extract requirements that have technical implications for
        software systems. Skip purely organizational or procedural
        requirements that cannot be verified in code.
        """

        # Few-shot examples teach LangExtract the output format
        examples = self._build_examples()

        result = lx.extract(
            text_or_documents=text,
            prompt_description=prompt_description,
            examples=examples,
            language_model_type=lx.inference.AnthropicLanguageModel,
            model_id=self.model,
            # Source grounding: every extraction includes char offsets
            # This is LangExtract's killer feature for VibeGuard
        )

        # Convert LangExtract output to VibeGuard rule format
        rules = []
        for extraction in result.extractions:
            rule = {
                "id": self._generate_rule_id(extraction),
                "regulation": extraction.get("regulation", ""),
                "article": extraction.get("article", ""),
                "paragraph": extraction.get("paragraph", ""),
                "title": extraction.get("title", ""),
                "requirement": extraction.get("requirement", ""),
                "source_text": extraction.get("source_text", ""),
                "source_offsets": {
                    "start": extraction.source_start_offset,
                    "end": extraction.source_end_offset,
                },
                "source_page": extraction.get("source_page"),
                "check_type": extraction.get("check_type", "manual"),
                "severity": extraction.get("severity", "medium"),
                "confidence": "high",  # AI-extracted, needs human review
            }
            rules.append(rule)

        return rules

    def _build_examples(self) -> list:
        """Few-shot examples for LangExtract."""
        return [
            lx.data.ExampleData(
                text="""Article 17 - Right to erasure ('right to be forgotten')
                1. The data subject shall have the right to obtain from the
                controller the erasure of personal data concerning him or her
                without undue delay and the controller shall have the obligation
                to erase personal data without undue delay where one of the
                following grounds applies...""",
                extractions=[
                    lx.data.Extraction(
                        regulation="GDPR",
                        article="17",
                        paragraph="1",
                        title="Right to Erasure",
                        requirement="Controller must delete personal data upon request without undue delay",
                        source_text="The data subject shall have the right to obtain from the controller the erasure of personal data concerning him or her without undue delay",
                        check_type="ast_pattern",
                        severity="critical",
                    )
                ]
            ),
        ]

    def _generate_rule_id(self, extraction) -> str:
        """Generate a deterministic rule ID from extraction."""
        reg = extraction.get("regulation", "unknown").lower().replace(" ", "-")
        art = extraction.get("article", "0")
        para = extraction.get("paragraph", "0")
        title = extraction.get("title", "check")
        short = title.lower().replace(" ", "-")[:30]
        return f"{reg}-{art}-{para}-{short}"
```

### Step 4: Claude Technical Translator

```python
# ingestion/pipeline/technical_translator.py

import anthropic
import json
from config import settings

class TechnicalTranslator:
    def __init__(self):
        self.client = anthropic.Anthropic(api_key=settings.ANTHROPIC_API_KEY)

    async def translate(self, rule: dict) -> dict:
        """
        Translate a legal requirement into technical code checks.

        Input: rule with legal language
        Output: technical_checks, ast-grep patterns, remediation
        """

        prompt = f"""You are translating a regulatory compliance requirement into
concrete software verification checks.

REGULATION: {rule['regulation']}
ARTICLE: {rule.get('article', 'N/A')}
REQUIREMENT: {rule['requirement']}
SOURCE TEXT: {rule['source_text']}

Generate the following as valid JSON:

1. "check_type": One of "ast_pattern" (can check with code pattern matching),
   "config_check" (check config/infra files), "composite" (needs multiple checks),
   or "manual" (cannot be automated).

2. "languages": Array of programming languages this check applies to
   (from: python, typescript, go, java, rust).

3. "technical_checks": Object with:
   - "description": What to look for in code
   - "positive_patterns": Code patterns that indicate compliance (what SHOULD exist)
   - "negative_patterns": Code patterns that indicate violation (what should NOT exist)
   - "ast_grep_rules": For each language, a valid ast-grep YAML rule object with
     "rule" containing pattern/any/all/has/inside operators

4. "remediation": Object with:
   - "description": What the developer needs to do
   - "code_examples": Object mapping language to example fix code

5. "severity": "critical" | "high" | "medium" | "low"

Respond ONLY with valid JSON. No markdown fences, no commentary."""

        response = self.client.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=4096,
            messages=[{"role": "user", "content": prompt}],
        )

        text = response.content[0].text.strip()

        # Parse response, handle potential JSON issues
        try:
            result = json.loads(text)
        except json.JSONDecodeError:
            # Try to extract JSON from response
            import re
            json_match = re.search(r'\{.*\}', text, re.DOTALL)
            if json_match:
                result = json.loads(json_match.group())
            else:
                result = {
                    "check_type": "manual",
                    "languages": [],
                    "technical_checks": {
                        "description": "Auto-translation failed. Manual rule writing required.",
                    },
                    "remediation": {"description": rule["requirement"]},
                    "severity": rule.get("severity", "medium"),
                }

        return result
```

---

## 4. Updated Database Schema

Add these tables to the existing schema from the MVP Blueprint:

```sql
-- Uploaded regulatory documents
CREATE TABLE documents (
    id              TEXT PRIMARY KEY,        -- UUID as text
    org_id          TEXT NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,            -- "DORA", "Internal Security Policy"
    description     TEXT,
    file_name       TEXT NOT NULL,
    file_path       TEXT NOT NULL,            -- filesystem path to PDF
    file_size       BIGINT NOT NULL,
    page_count      INTEGER,
    status          TEXT DEFAULT 'uploaded',  -- uploaded | processing | review_ready
                                             -- | active | extraction_failed
    extraction_meta JSONB DEFAULT '{}',       -- articles found, processing stats
    rule_count      INTEGER DEFAULT 0,        -- total proposed rules extracted
    approved_count  INTEGER DEFAULT 0,        -- rules approved so far
    uploaded_by     TEXT REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_documents_org ON documents(org_id);
CREATE INDEX idx_documents_status ON documents(status);

-- Proposed rules (AI-extracted, pending human review)
-- Once approved, row is copied to compliance_rules with status "active"
CREATE TABLE proposed_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id     TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    rule_id         TEXT NOT NULL,            -- e.g. "dora-11-1-ict-risk-framework"
    version         TEXT DEFAULT '1.0',

    -- Same core fields as compliance_rules
    regulation      TEXT NOT NULL,
    article         TEXT,
    paragraph       TEXT,
    title           TEXT NOT NULL,
    requirement     TEXT NOT NULL,
    source_text     TEXT NOT NULL,            -- verbatim regulation text
    source_text_offsets JSONB DEFAULT '{}',   -- {start: int, end: int} char offsets
                                             -- from LangExtract source grounding
    source_page     INTEGER,
    check_type      TEXT NOT NULL,            -- ast_pattern | config_check | composite | manual
    severity        TEXT NOT NULL,
    confidence      TEXT DEFAULT 'high',
    languages       TEXT[] DEFAULT '{}',
    technical_checks JSONB NOT NULL,          -- ast-grep patterns, config checks
    cross_references TEXT[] DEFAULT '{}',
    remediation     JSONB DEFAULT '{}',

    -- Review fields
    review_status   TEXT DEFAULT 'pending',   -- pending | approved | rejected | edited
    reviewed_by     TEXT REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    review_notes    TEXT,
    original_ai_output JSONB,                -- full raw pipeline output for debugging
    edits_made      JSONB DEFAULT '{}',       -- diff: what reviewer changed

    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_proposed_rules_document ON proposed_rules(document_id);
CREATE INDEX idx_proposed_rules_status ON proposed_rules(review_status);
```

---

## 5. Go API Additions (Proxy to Python Service)

The Go binary proxies document upload to the Python ingestion service and manages the review workflow directly.

```go
// internal/api/handlers/documents.go

func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
    // Forward the multipart upload to the Python ingestion service
    // Add org_id and uploaded_by from Clerk auth context

    orgID := auth.GetOrgID(r.Context())
    userID := auth.GetUserID(r.Context())

    // Read the uploaded file
    file, header, err := r.FormFile("file")
    if err != nil {
        respondError(w, http.StatusBadRequest, "No file provided")
        return
    }
    defer file.Close()

    // Forward to Python ingestion service
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, _ := writer.CreateFormFile("file", header.Filename)
    io.Copy(part, file)
    writer.WriteField("name", r.FormValue("name"))
    writer.WriteField("description", r.FormValue("description"))
    writer.WriteField("org_id", orgID)
    writer.WriteField("uploaded_by", userID)
    writer.Close()

    resp, err := http.Post(
        h.ingestionURL+"/ingest",
        writer.FormDataContentType(),
        body,
    )
    if err != nil {
        respondError(w, http.StatusBadGateway, "Ingestion service unavailable")
        return
    }
    defer resp.Body.Close()

    // Forward response
    var result map[string]any
    json.NewDecoder(resp.Body).Decode(&result)
    respondJSON(w, http.StatusAccepted, result)
}

func (h *DocumentHandler) ApproveRule(w http.ResponseWriter, r *http.Request) {
    ruleUUID := chi.URLParam(r, "ruleId")
    userID := auth.GetUserID(r.Context())

    // Fetch proposed rule
    proposed, err := h.db.GetProposedRule(r.Context(), ruleUUID)
    if err != nil {
        respondError(w, http.StatusNotFound, "Rule not found")
        return
    }

    // Copy to active compliance_rules table
    err = h.db.CreateComplianceRule(r.Context(), db.CreateComplianceRuleParams{
        ID:              proposed.RuleID,
        Version:         proposed.Version,
        Regulation:      proposed.Regulation,
        Article:         proposed.Article,
        Paragraph:       proposed.Paragraph,
        Title:           proposed.Title,
        Requirement:     proposed.Requirement,
        SourceText:      proposed.SourceText,
        SourcePage:      proposed.SourcePage,
        CheckType:       proposed.CheckType,
        Severity:        proposed.Severity,
        Confidence:      proposed.Confidence,
        Languages:       proposed.Languages,
        TechnicalChecks: proposed.TechnicalChecks,
        CrossReferences: proposed.CrossReferences,
    })
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to activate rule")
        return
    }

    // Update proposed rule status
    h.db.UpdateProposedRuleStatus(r.Context(), ruleUUID, "approved", userID)

    // Update document approved count
    h.db.IncrementDocumentApprovedCount(r.Context(), proposed.DocumentID)

    // Audit log
    h.audit.Log(r.Context(), audit.Event{
        Type:         "rule.approved",
        ActorID:      userID,
        ResourceType: "compliance_rule",
        ResourceID:   proposed.RuleID,
    })

    respondJSON(w, http.StatusOK, map[string]string{
        "status":  "approved",
        "rule_id": proposed.RuleID,
    })
}
```

### Updated API Routes

```
# Document Management
POST   /api/v1/documents                    Upload regulatory PDF → proxies to Python service
GET    /api/v1/documents                    List documents (org-scoped)
GET    /api/v1/documents/:id                Document details + extraction progress
DELETE /api/v1/documents/:id                Delete document + associated rules

# Rule Review (managed by Go API directly)
GET    /api/v1/documents/:id/rules          List proposed rules for a document
GET    /api/v1/documents/:id/rules/:ruleId  Get single proposed rule with citation
PATCH  /api/v1/documents/:id/rules/:ruleId  Edit rule fields (source text, patterns, severity)
POST   /api/v1/documents/:id/rules/:ruleId/approve   Approve → copies to active rules
POST   /api/v1/documents/:id/rules/:ruleId/reject    Reject rule
POST   /api/v1/documents/:id/approve-all    Bulk approve all pending rules
```

---

## 6. Updated Docker Compose

```yaml
# docker-compose.yml
version: "3.8"

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    command: ["vgx", "serve", "--port", "8080"]
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: "postgres://vibeguard:${DB_PASSWORD}@db:5432/vibeguard?sslmode=disable"
      CLERK_SECRET_KEY: "${CLERK_SECRET_KEY}"
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
      INGESTION_SERVICE_URL: "http://ingestion:8001"
      SENTRY_DSN: "${SENTRY_DSN}"
    depends_on:
      db:
        condition: service_healthy
      ingestion:
        condition: service_started
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 1G

  ingestion:
    build:
      context: ./ingestion
      dockerfile: Dockerfile
    command: ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]
    # NOT exposed to public — internal only
    expose:
      - "8001"
    environment:
      DATABASE_URL: "postgres://vibeguard:${DB_PASSWORD}@db:5432/vibeguard?sslmode=disable"
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
      UPLOAD_DIR: "/data/uploads"
      TORCH_DEVICE: "cpu"
    volumes:
      - uploads:/data/uploads
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 4G       # Marker needs ~2-3GB when processing
        reservations:
          memory: 1G

  db:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: vibeguard
      POSTGRES_USER: vibeguard
      POSTGRES_PASSWORD: "${DB_PASSWORD}"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U vibeguard"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
    command:
      - "postgres"
      - "-c"
      - "shared_buffers=2GB"
      - "-c"
      - "effective_cache_size=4GB"
      - "-c"
      - "work_mem=64MB"

volumes:
  pgdata:
  uploads:
```

### Python Service Dockerfile

```dockerfile
# ingestion/Dockerfile
FROM python:3.12-slim

WORKDIR /app

# System dependencies for Marker
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libgl1-mesa-glx \
    libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Pre-download Marker models (so container startup is fast)
RUN python -c "from marker.models import create_model_dict; create_model_dict()"

COPY . .

EXPOSE 8001
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]
```

---

## 7. Source Grounding → Citation Engine

The key advantage of using LangExtract over raw Claude API calls is **character-level source grounding**. Here's how it flows through the system:

```
LangExtract extracts rule:
  source_text: "The data subject shall have the right..."
  source_offsets: { start: 14523, end: 14612 }
                          │
                          ▼
  Stored in proposed_rules.source_text_offsets as JSONB
                          │
                          ▼
  Human reviewer sees exact highlighted text in original document
  (frontend highlights chars 14523–14612 in the parsed markdown)
                          │
                          ▼
  Approved rule → compliance_rules table
                          │
                          ▼
  Scanning engine produces finding → citation includes:
    - regulation: "GDPR"
    - article: "17"
    - paragraph: "1"
    - source_text: "The data subject shall have the right..."
    - source_offsets: { start: 14523, end: 14612 }
    - confidence: "deterministic" (for ast-grep checks)
                          │
                          ▼
  Developer sees in CLI/IDE:
    "GDPR Art. 17(1) — Right to Erasure
     No DELETE endpoint found for user data.
     Source: 'The data subject shall have the right to obtain
     from the controller the erasure of personal data...'"
```

**This is the zero-hallucination guarantee in action.** Every finding traces from code → rule → character offset in original regulation text. If the offset doesn't exist, the finding doesn't appear.

---

## 8. Marker License: What You Need to Know

Marker is GPL-3.0 with cc-by-nc-sa-4.0 model weights. The practical implications:

- **Under $5M revenue (current):** Free to use, including model weights. The $5M threshold was recently raised from $2M.
- **GPL-3.0 engine:** Since the ingestion service runs server-side and is never distributed to users, GPL's copyleft triggers are limited. However, the safest approach: keep the Python ingestion service in a **separate container** (not compiled into the Go binary). This maintains clear separation.
- **At scale:** Contact Datalab (Marker's company) for a commercial license. They offer hosted API and on-prem solutions.
- **If license becomes an issue:** Swap Marker for **Docling** (MIT, IBM) — nearly identical accuracy, slightly different API. The swap is a single file change (`pdf_parser.py`).

---

## 9. VPS Memory Considerations

Running Marker on the CX33 (8 GB RAM) is tight. Here's the plan:

**Normal operation (no document processing):**
- PostgreSQL: 2 GB
- Go binary: 200 MB
- Python service (idle, models unloaded): 300 MB
- Free: ~5.5 GB

**During document processing:**
- PostgreSQL: 2 GB
- Go binary: 200 MB
- Python service (Marker models loaded): 2.5–3 GB
- Free: ~2.5–3 GB

**Mitigation strategies:**
1. Load Marker models on-demand, unload after processing (add `del self._model_dict; gc.collect()` after each document).
2. Process one document at a time (queue via PGMQ, no parallel ingestion).
3. If memory becomes an issue, upgrade to Hetzner **CX42** (8 vCPU, 16 GB, €16.40/month) — still dirt cheap.
4. Alternative: Use Marker's hosted API instead of running locally. They offer free credits to start. This eliminates the model memory entirely.

---

## 10. Updated Build Schedule Impact

Adding the Python ingestion service adds ~1.5 weeks. Adjusted timeline:

| Week | Focus | Ingestion-Related Work |
|------|-------|----------------------|
| 1–2 | Core engine (Go) | — |
| 3 | Database + Go API | Add documents + proposed_rules tables, review endpoints |
| 4 | **Python ingestion service** | Marker + PageIndex + LangExtract + Claude translator, FastAPI, Docker setup |
| 5–6 | Frontend | Add document upload page, **rule review page** |
| 7 | Integration testing | End-to-end: upload GDPR PDF → extract rules → review → scan code |
| 8–9 | Ship | Deploy, backups, GitHub Action, docs, launch |

**Total: 9 weeks** (was 8). The extra week is worth it — document ingestion is the product.

---

## 11. Cost Per Document Extraction

| Component | Cost Per 100-Page Document |
|-----------|--------------------------|
| Marker (local) | $0 (runs on VPS) |
| PageIndex (self-hosted) | $0 (runs on VPS) |
| LangExtract + Claude API | ~$0.50–1.50 (depending on article count) |
| Claude technical translation | ~$0.50–1.00 (per article batch) |
| **Total** | **~$1.00–2.50 per document** |

At the Team tier ($500/month), even if a customer processes 50 documents/month, the API cost is $50–125. Margins are excellent.

---

## Summary of Changes to MVP Blueprint

| Component | Before (Deferred) | Now (MVP) |
|-----------|-------------------|-----------|
| PDF parsing | ❌ Deferred | ✅ Marker (Python sidecar) |
| Structured extraction | ❌ Deferred | ✅ LangExtract with source grounding |
| Cross-reference resolution | ❌ Deferred | ✅ PageIndex tree index |
| Legal → technical translation | ❌ Deferred | ✅ Claude API |
| Human review UI | ❌ Deferred | ✅ React page in dashboard |
| Architecture | Go binary only | Go binary + Python sidecar |
| Docker services | 2 (app + db) | 3 (app + ingestion + db) |
| Memory requirement | ~3 GB | ~6–8 GB (fits CX33) |
| Build time | 8 weeks | 9 weeks |
| Monthly infra cost | ~$7 | ~$7 + Claude API usage |
