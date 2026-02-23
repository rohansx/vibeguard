-- migrate:up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- ORGANIZATIONS
-- ============================================================
CREATE TABLE organizations (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    clerk_org_id TEXT UNIQUE NOT NULL,
    name         TEXT NOT NULL,
    slug         TEXT UNIQUE NOT NULL,
    plan         TEXT NOT NULL DEFAULT 'free',
    settings     JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    clerk_user_id TEXT UNIQUE NOT NULL,
    org_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email         TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'developer',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_org ON users(org_id);
CREATE INDEX idx_users_clerk ON users(clerk_user_id);

-- ============================================================
-- DOCUMENTS (uploaded regulatory PDFs)
-- ============================================================
CREATE TABLE documents (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    org_id          TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    file_name       TEXT NOT NULL,
    file_path       TEXT NOT NULL,
    file_size       BIGINT NOT NULL,
    page_count      INTEGER,
    status          TEXT NOT NULL DEFAULT 'uploaded',
    extraction_meta JSONB NOT NULL DEFAULT '{}',
    rule_count      INTEGER NOT NULL DEFAULT 0,
    approved_count  INTEGER NOT NULL DEFAULT 0,
    uploaded_by     TEXT REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_org ON documents(org_id);
CREATE INDEX idx_documents_status ON documents(status);

-- ============================================================
-- PROPOSED RULES (AI-extracted, pending human review)
-- ============================================================
CREATE TABLE proposed_rules (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id         TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    rule_id             TEXT NOT NULL,
    version             TEXT NOT NULL DEFAULT '1.0',
    regulation          TEXT NOT NULL,
    article             TEXT,
    paragraph           TEXT,
    title               TEXT NOT NULL,
    requirement         TEXT NOT NULL,
    source_text         TEXT NOT NULL,
    source_text_offsets JSONB NOT NULL DEFAULT '{}',
    source_page         INTEGER,
    check_type          TEXT NOT NULL,
    severity            TEXT NOT NULL,
    confidence          TEXT NOT NULL DEFAULT 'high',
    languages           TEXT[] NOT NULL DEFAULT '{}',
    technical_checks    JSONB NOT NULL DEFAULT '{}',
    cross_references    TEXT[] NOT NULL DEFAULT '{}',
    remediation         JSONB NOT NULL DEFAULT '{}',
    review_status       TEXT NOT NULL DEFAULT 'pending',
    reviewed_by         TEXT REFERENCES users(id),
    reviewed_at         TIMESTAMPTZ,
    review_notes        TEXT,
    original_ai_output  JSONB,
    edits_made          JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_proposed_rules_document ON proposed_rules(document_id);
CREATE INDEX idx_proposed_rules_status ON proposed_rules(review_status);
CREATE INDEX idx_proposed_rules_regulation ON proposed_rules(regulation);

-- ============================================================
-- COMPLIANCE RULES (active, human-verified)
-- ============================================================
CREATE TABLE compliance_rules (
    id                  TEXT PRIMARY KEY,
    version             TEXT NOT NULL DEFAULT '1.0',
    regulation          TEXT NOT NULL,
    article             TEXT,
    paragraph           TEXT,
    title               TEXT NOT NULL,
    requirement         TEXT NOT NULL,
    source_text         TEXT NOT NULL,
    source_text_offsets JSONB NOT NULL DEFAULT '{}',
    source_page         INTEGER,
    check_type          TEXT NOT NULL,
    severity            TEXT NOT NULL,
    confidence          TEXT NOT NULL DEFAULT 'deterministic',
    languages           TEXT[] NOT NULL DEFAULT '{}',
    technical_checks    JSONB NOT NULL DEFAULT '{}',
    cross_references    TEXT[] NOT NULL DEFAULT '{}',
    remediation         JSONB NOT NULL DEFAULT '{}',
    source_document_id  TEXT REFERENCES documents(id),
    approved_by         TEXT REFERENCES users(id),
    approved_at         TIMESTAMPTZ,
    status              TEXT NOT NULL DEFAULT 'active',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_compliance_rules_regulation ON compliance_rules(regulation);
CREATE INDEX idx_compliance_rules_status ON compliance_rules(status);

-- ============================================================
-- SCAN RESULTS
-- ============================================================
CREATE TABLE scan_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    initiated_by        TEXT REFERENCES users(id),
    repository_url      TEXT,
    commit_hash         TEXT,
    branch              TEXT,
    scan_type           TEXT NOT NULL DEFAULT 'full',
    status              TEXT NOT NULL DEFAULT 'running',
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    total_rules_checked INTEGER NOT NULL DEFAULT 0,
    total_findings      INTEGER NOT NULL DEFAULT 0,
    critical_findings   INTEGER NOT NULL DEFAULT 0,
    high_findings       INTEGER NOT NULL DEFAULT 0,
    medium_findings     INTEGER NOT NULL DEFAULT 0,
    low_findings        INTEGER NOT NULL DEFAULT 0,
    compliance_score    NUMERIC(5,2),
    findings            JSONB NOT NULL DEFAULT '[]',
    scan_meta           JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scan_results_org ON scan_results(org_id);
CREATE INDEX idx_scan_results_status ON scan_results(status);

-- ============================================================
-- AUDIT LOG (append-only, cryptographically chained)
-- ============================================================
CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    org_id        TEXT NOT NULL REFERENCES organizations(id),
    actor_id      TEXT REFERENCES users(id),
    actor_type    TEXT NOT NULL DEFAULT 'user',
    event_type    TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    details       JSONB NOT NULL DEFAULT '{}',
    ip_address    INET,
    previous_hash TEXT,
    entry_hash    TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_org ON audit_log(org_id);
CREATE INDEX idx_audit_log_event ON audit_log(event_type);
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at);

-- Prevent modifications to audit_log
CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % operations are not permitted', TG_OP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_modification();

CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_modification();

-- migrate:down

DROP TRIGGER IF EXISTS audit_log_no_delete ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
DROP FUNCTION IF EXISTS prevent_audit_modification();
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS scan_results;
DROP TABLE IF EXISTS compliance_rules;
DROP TABLE IF EXISTS proposed_rules;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
DROP EXTENSION IF EXISTS "pgcrypto";
