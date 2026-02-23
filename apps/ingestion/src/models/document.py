from datetime import datetime
from uuid import UUID

from pydantic import BaseModel


class Document(BaseModel):
    id: UUID
    org_id: str
    name: str
    description: str
    file_name: str
    file_path: str
    file_size: int
    status: str
    created_at: datetime
