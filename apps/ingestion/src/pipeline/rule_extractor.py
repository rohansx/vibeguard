import json
import logging
import re
import textwrap

from openai import AsyncOpenAI

from src.config import settings

logger = logging.getLogger(__name__)

MAX_CHUNK_CHARS = 12000


class RuleExtractor:
    """Extract regulatory rules from document text using OpenAI structured outputs."""

    def __init__(self) -> None:
        self.client = AsyncOpenAI(api_key=settings.OPENAI_API_KEY)

    async def extract(self, text: str, regulation_name: str = "") -> list[dict]:
        """Extract structured compliance rules from regulation text.

        Splits text into chunks and sends each to OpenAI for extraction.
        Returns a list of rule dicts.
        """
        if not settings.OPENAI_API_KEY:
            raise ValueError("OPENAI_API_KEY is not configured")

        chunks = self._split_into_chunks(text)
        logger.info("Extracting rules from %d text chunks", len(chunks))

        all_rules: list[dict] = []
        for i, chunk in enumerate(chunks):
            logger.info("Processing chunk %d/%d (%d chars)", i + 1, len(chunks), len(chunk))
            rules = await self._extract_from_chunk(chunk, regulation_name)
            all_rules.extend(rules)

        logger.info("Extracted %d total rules", len(all_rules))

        # Deduplicate by rule_id
        seen: set[str] = set()
        unique_rules: list[dict] = []
        for rule in all_rules:
            rid = rule.get("rule_id", "")
            if rid and rid not in seen:
                seen.add(rid)
                unique_rules.append(rule)
            elif not rid:
                unique_rules.append(rule)

        return unique_rules

    async def _extract_from_chunk(self, chunk: str, regulation_name: str) -> list[dict]:
        """Send a text chunk to OpenAI for rule extraction."""
        system_prompt = textwrap.dedent("""\
            You are a compliance rule extraction system. Extract every technical
            compliance requirement from the given regulatory text.

            For each requirement, provide a JSON object with these fields:
            - rule_id: a short unique ID like "GDPR-17-1-erasure" (regulation-article-paragraph-keyword)
            - regulation: the regulation name (e.g., "GDPR", "PCI DSS", "SOC 2")
            - article: the article/requirement number (e.g., "17", "Req. 4")
            - paragraph: the paragraph/sub-section (e.g., "1", "1(a)")
            - title: short descriptive title
            - requirement: what the regulated entity must do (1-2 sentences)
            - source_text: the verbatim text from the regulation that states this requirement
            - check_type: one of "ast_pattern", "config_check", "composite", "manual"
            - severity: one of "critical", "high", "medium", "low"

            Only extract requirements with technical implications for software systems.
            Skip purely organizational or procedural requirements.

            Respond with a JSON object: {"rules": [...]}""")

        user_msg = f"Regulation: {regulation_name}\n\n---\n\n{chunk}"

        response = await self.client.chat.completions.create(
            model="gpt-4o",
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_msg},
            ],
            response_format={"type": "json_object"},
            temperature=0.1,
        )

        text = response.choices[0].message.content or "{}"

        try:
            data = json.loads(text)
        except json.JSONDecodeError:
            json_match = re.search(r"\{.*\}", text, re.DOTALL)
            if json_match:
                data = json.loads(json_match.group())
            else:
                logger.warning("Failed to parse OpenAI response as JSON")
                return []

        return data.get("rules", [])

    def _split_into_chunks(self, text: str) -> list[str]:
        """Split text into chunks at heading boundaries."""
        sections = re.split(r"(?=^#{1,3}\s)", text, flags=re.MULTILINE)
        chunks: list[str] = []
        current_chunk = ""

        for section in sections:
            if len(current_chunk) + len(section) > MAX_CHUNK_CHARS and current_chunk:
                chunks.append(current_chunk)
                current_chunk = section
            else:
                current_chunk += section

        if current_chunk.strip():
            chunks.append(current_chunk)

        return chunks if chunks else [text]
