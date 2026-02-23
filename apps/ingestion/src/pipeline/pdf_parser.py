import logging
from dataclasses import dataclass, field

import httpx

from src.config import settings

logger = logging.getLogger(__name__)

MARKER_API_URL = "https://www.datalab.to/api/v1/marker"


@dataclass
class ParsedDocument:
    markdown: str
    metadata: dict = field(default_factory=dict)


class MarkerParser:
    """Parse regulatory PDFs into structured markdown using Marker cloud API."""

    def __init__(self) -> None:
        self.api_key = settings.MARKER_API_KEY

    async def parse(self, file_path: str) -> ParsedDocument:
        """Upload a PDF to Marker API and return structured markdown."""
        if not self.api_key:
            raise ValueError("MARKER_API_KEY is not configured")

        logger.info("Parsing PDF via Marker API: %s", file_path)

        async with httpx.AsyncClient(timeout=300.0) as client:
            with open(file_path, "rb") as f:
                response = await client.post(
                    MARKER_API_URL,
                    headers={"X-Api-Key": self.api_key},
                    files={"file": (file_path.split("/")[-1], f, "application/pdf")},
                    data={"output_format": "markdown"},
                )

            response.raise_for_status()
            result = response.json()

        if not result.get("success", False):
            raise RuntimeError(f"Marker API failed: {result.get('error', 'unknown error')}")

        markdown = result.get("markdown", "")
        metadata = result.get("metadata", {})

        logger.info(
            "Marker API returned %d chars of markdown, metadata keys: %s",
            len(markdown),
            list(metadata.keys()),
        )

        return ParsedDocument(markdown=markdown, metadata=metadata)
