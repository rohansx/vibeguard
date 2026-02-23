import json
import logging
import re

from openai import AsyncOpenAI

from src.config import settings

logger = logging.getLogger(__name__)


class TechnicalTranslator:
    """Translate extracted regulatory rules into actionable technical requirements using OpenAI."""

    def __init__(self) -> None:
        self.client = AsyncOpenAI(api_key=settings.OPENAI_API_KEY)

    async def translate(self, rule: dict) -> dict:
        """Translate a single legal rule into technical code checks.

        Returns a dict with: check_type, languages, technical_checks,
        ast_grep_rules (YAML strings per language), remediation.
        """
        if not settings.OPENAI_API_KEY:
            raise ValueError("OPENAI_API_KEY is not configured")

        prompt = f"""You are translating a regulatory compliance requirement into
concrete software verification checks.

REGULATION: {rule.get('regulation', 'Unknown')}
ARTICLE: {rule.get('article', 'N/A')}
PARAGRAPH: {rule.get('paragraph', 'N/A')}
REQUIREMENT: {rule.get('requirement', '')}
SOURCE TEXT: {rule.get('source_text', '')}

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

4. "ast_grep_rules": Array of objects, each with:
   - "id": rule ID string
   - "language": one of Python, TypeScript, Go, Java, Rust
   - "rule": object with ast-grep pattern operators (pattern, any, all, has, inside)
   - "message": human-readable violation message
   - "severity": "error" or "warning"

5. "remediation": Object with:
   - "description": What the developer needs to do
   - "steps": Array of remediation steps

Respond ONLY with valid JSON. No markdown fences."""

        response = await self.client.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"},
            temperature=0.1,
        )

        text = response.choices[0].message.content or "{}"

        try:
            result = json.loads(text)
        except json.JSONDecodeError:
            json_match = re.search(r"\{.*\}", text, re.DOTALL)
            if json_match:
                result = json.loads(json_match.group())
            else:
                logger.warning("Failed to parse translation response for rule %s", rule.get("rule_id"))
                result = {
                    "check_type": "manual",
                    "languages": [],
                    "technical_checks": {
                        "description": "Auto-translation failed. Manual rule writing required.",
                    },
                    "ast_grep_rules": [],
                    "remediation": {
                        "description": rule.get("requirement", ""),
                        "steps": [],
                    },
                }

        return result
