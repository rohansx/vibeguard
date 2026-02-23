FROM python:3.12-slim

WORKDIR /app

COPY apps/ingestion/pyproject.toml .
RUN pip install --no-cache-dir ".[dev]"

COPY apps/ingestion/src/ src/
COPY apps/ingestion/tests/ tests/

EXPOSE 8001
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8001"]
