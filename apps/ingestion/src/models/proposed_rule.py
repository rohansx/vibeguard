from uuid import UUID

from pydantic import BaseModel


class ProposedRule(BaseModel):
    id: UUID
    document_id: UUID
    rule_id: str
    regulation: str
    article: str
    paragraph: str
    title: str
    requirement: str
    source_text: str
    source_text_offsets: dict
    check_type: str
    severity: str
    confidence: float
    review_status: str
