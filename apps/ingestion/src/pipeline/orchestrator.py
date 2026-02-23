import json
import logging
from uuid import uuid4

import asyncpg

from src.pipeline.cross_references import CrossReferenceResolver
from src.pipeline.pdf_parser import MarkerParser
from src.pipeline.rule_extractor import RuleExtractor
from src.pipeline.technical_translator import TechnicalTranslator
from src.pipeline.tree_indexer import PageIndexer

logger = logging.getLogger(__name__)


class ExtractionOrchestrator:
    """Coordinates the full extraction pipeline: parse -> index -> extract -> translate."""

    def __init__(self, db_pool: asyncpg.Pool) -> None:
        self.db_pool = db_pool
        self.parser = MarkerParser()
        self.indexer = PageIndexer()
        self.extractor = RuleExtractor()
        self.translator = TechnicalTranslator()
        self.xref_resolver = CrossReferenceResolver(self.indexer)

    async def process(self, doc_id: str, file_path: str) -> dict:
        """Run the complete extraction pipeline for a single document.

        Returns a summary dict with rule_count and metadata.
        """
        # Step 1: Parse PDF via Marker API
        logger.info("[%s] Step 1: Parsing PDF with Marker API", doc_id)
        parsed = await self.parser.parse(file_path)

        # Step 2: Build tree index from markdown headings
        logger.info("[%s] Step 2: Building tree index", doc_id)
        tree_index = self.indexer.build_index(parsed.markdown)

        # Detect regulation name from document metadata or first heading
        regulation_name = self._detect_regulation(parsed.markdown)

        # Step 3: Extract structured rules via OpenAI
        logger.info("[%s] Step 3: Extracting rules via OpenAI", doc_id)
        raw_rules = await self.extractor.extract(parsed.markdown, regulation_name)

        # Step 4: Resolve cross-references for each rule
        logger.info("[%s] Step 4: Resolving cross-references", doc_id)
        for rule in raw_rules:
            source = rule.get("source_text", "")
            reg = rule.get("regulation", regulation_name)
            refs = self.xref_resolver.resolve(source, reg, tree_index)
            rule["cross_references"] = refs

        # Step 5: Translate each rule into technical checks via OpenAI
        logger.info("[%s] Step 5: Translating %d rules to technical checks", doc_id, len(raw_rules))
        proposed_rules: list[dict] = []
        for i, rule in enumerate(raw_rules):
            logger.info("[%s] Translating rule %d/%d: %s", doc_id, i + 1, len(raw_rules), rule.get("rule_id", "?"))
            technical = await self.translator.translate(rule)
            proposed_rules.append({**rule, **technical})

        # Step 6: Store proposed rules in DB
        logger.info("[%s] Step 6: Storing %d proposed rules in DB", doc_id, len(proposed_rules))
        async with self.db_pool.acquire() as conn:
            for rule in proposed_rules:
                await conn.execute(
                    """
                    INSERT INTO proposed_rules
                    (id, document_id, rule_id, regulation, article, paragraph,
                     title, requirement, source_text, source_text_offsets,
                     source_page, check_type, severity, confidence,
                     languages, technical_checks, cross_references,
                     remediation, review_status, original_ai_output)
                    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
                            $11, $12, $13, $14, $15, $16, $17, $18, 'pending', $19)
                    """,
                    str(uuid4()),
                    doc_id,
                    rule.get("rule_id", f"auto-{uuid4().hex[:8]}"),
                    rule.get("regulation", regulation_name),
                    rule.get("article"),
                    rule.get("paragraph"),
                    rule.get("title", "Untitled Rule"),
                    rule.get("requirement", ""),
                    rule.get("source_text", ""),
                    json.dumps(rule.get("source_offsets", {})),
                    rule.get("source_page"),
                    rule.get("check_type", "manual"),
                    rule.get("severity", "medium"),
                    "high",
                    rule.get("languages", []),
                    json.dumps(rule.get("technical_checks", {})),
                    rule.get("cross_references", []),
                    json.dumps(rule.get("remediation", {})),
                    json.dumps(rule),
                )

        meta = {
            "pages": parsed.metadata.get("page_count") or parsed.metadata.get("pages"),
            "articles_found": len(raw_rules),
            "rules_proposed": len(proposed_rules),
        }

        logger.info("[%s] Pipeline complete: %d rules proposed", doc_id, len(proposed_rules))
        return {"rule_count": len(proposed_rules), "meta": meta}

    def _detect_regulation(self, markdown: str) -> str:
        """Try to detect the regulation name from markdown content."""
        lower = markdown[:2000].lower()
        if "gdpr" in lower or "general data protection" in lower:
            return "GDPR"
        if "pci" in lower or "payment card" in lower:
            return "PCI DSS"
        if "soc 2" in lower or "trust services" in lower:
            return "SOC 2"
        if "hipaa" in lower or "health insurance portability" in lower:
            return "HIPAA"
        if "ai act" in lower or "artificial intelligence act" in lower:
            return "EU AI Act"
        return "Unknown"
