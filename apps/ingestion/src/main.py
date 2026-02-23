import json
import logging
import os
from contextlib import asynccontextmanager
from uuid import uuid4

import asyncpg
from fastapi import BackgroundTasks, FastAPI, File, Form, UploadFile

from src.config import settings
from src.pipeline.orchestrator import ExtractionOrchestrator

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage asyncpg connection pool across the application lifecycle."""
    logging.basicConfig(level=settings.LOG_LEVEL.upper())
    logger.info("Connecting to database: %s", settings.DATABASE_URL)

    pool = await asyncpg.create_pool(dsn=settings.DATABASE_URL)
    app.state.db_pool = pool
    logger.info("Database connection pool established")

    yield

    await pool.close()
    logger.info("Database connection pool closed")


app = FastAPI(
    title="VibeGuard Ingestion Service",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health")
async def health():
    return {"status": "ok", "service": "ingestion"}


@app.post("/ingest")
async def ingest(
    background_tasks: BackgroundTasks,
    file: UploadFile = File(...),
    name: str = Form(...),
    description: str = Form(""),
    org_id: str = Form(...),
    uploaded_by: str = Form(...),
):
    """Accept a regulatory PDF, save to disk, create a DB record, and kick off extraction."""
    document_id = str(uuid4())

    # Ensure upload directory exists
    os.makedirs(settings.UPLOAD_DIR, exist_ok=True)

    # Persist the uploaded file
    file_name = file.filename or f"{document_id}.pdf"
    file_path = os.path.join(settings.UPLOAD_DIR, f"{document_id}_{file_name}")

    contents = await file.read()
    file_size = len(contents)

    with open(file_path, "wb") as f:
        f.write(contents)

    logger.info("Saved uploaded file: %s (%d bytes)", file_path, file_size)

    # Insert document record into the database
    pool: asyncpg.Pool = app.state.db_pool
    await pool.execute(
        """
        INSERT INTO documents (id, org_id, name, description, file_name, file_path, file_size, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'processing')
        """,
        document_id,
        org_id,
        name,
        description,
        file_name,
        file_path,
        file_size,
    )

    # Run extraction pipeline in background
    background_tasks.add_task(run_extraction, document_id, file_path, pool)

    return {
        "document_id": document_id,
        "status": "processing",
        "file_name": file_name,
        "file_size": file_size,
    }


async def run_extraction(doc_id: str, file_path: str, db_pool: asyncpg.Pool) -> None:
    """Background task: run the full extraction pipeline."""
    orchestrator = ExtractionOrchestrator(db_pool=db_pool)
    try:
        result = await orchestrator.process(doc_id, file_path)
        await db_pool.execute(
            """
            UPDATE documents
            SET status = 'review_ready',
                rule_count = $2,
                extraction_meta = $3,
                updated_at = now()
            WHERE id = $1
            """,
            doc_id,
            result["rule_count"],
            json.dumps(result["meta"]),
        )
        logger.info("Extraction complete for doc %s: %d rules", doc_id, result["rule_count"])
    except Exception:
        logger.exception("Extraction failed for doc %s", doc_id)
        await db_pool.execute(
            """
            UPDATE documents
            SET status = 'extraction_failed',
                extraction_meta = $2,
                updated_at = now()
            WHERE id = $1
            """,
            doc_id,
            json.dumps({"error": "extraction_failed"}),
        )
