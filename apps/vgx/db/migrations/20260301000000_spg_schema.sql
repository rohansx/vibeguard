-- migrate:up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- repositories: SPG-indexed repos registered with vgx
CREATE TABLE repositories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID,
    path            TEXT        NOT NULL,   -- local fs path or remote URL
    languages       TEXT[]      NOT NULL DEFAULT '{}',
    framework       TEXT,                   -- primary framework detected
    last_indexed_at TIMESTAMPTZ,
    graph_version   TEXT,                   -- content hash of current on-disk SPG
    settings        JSONB       NOT NULL DEFAULT '{}',  -- per-module sensitivity modes
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- spg_nodes: security-classified nodes synced from local graph (team sync use)
CREATE TABLE spg_nodes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id     UUID        NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    node_type   TEXT        NOT NULL CHECK (node_type IN (
                    'SourceNode', 'SinkNode', 'SanitizerNode',
                    'TrustBoundaryEdge', 'AuthCriticalNode', 'PropagationEdge'
                )),
    file_path   TEXT        NOT NULL,
    line_start  INT,
    line_end    INT,
    symbol      TEXT,                   -- function/variable/method name
    framework   TEXT,                   -- django|fastapi|express|gin|spring|etc
    vuln_class  TEXT,                   -- sqli|ssrf|xss|path_traversal|etc (for sinks)
    confidence  FLOAT       NOT NULL DEFAULT 1.0 CHECK (confidence BETWEEN 0 AND 1),
    metadata    JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_spg_nodes_repo   ON spg_nodes(repo_id);
CREATE INDEX idx_spg_nodes_type   ON spg_nodes(node_type);
CREATE INDEX idx_spg_nodes_file   ON spg_nodes(repo_id, file_path);

-- taint_paths: live source-to-sink paths without sanitizers
CREATE TABLE taint_paths (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id         UUID        NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    source_node_id  UUID        NOT NULL REFERENCES spg_nodes(id) ON DELETE CASCADE,
    sink_node_id    UUID        NOT NULL REFERENCES spg_nodes(id) ON DELETE CASCADE,
    path_nodes      UUID[]      NOT NULL DEFAULT '{}',   -- ordered intermediate node IDs
    vuln_class      TEXT        NOT NULL,                -- sqli|ssrf|xss|path_traversal|deserialization|etc
    severity        TEXT        NOT NULL CHECK (severity IN ('critical', 'high', 'medium', 'low')),
    is_active       BOOL        NOT NULL DEFAULT TRUE,
    introduced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_taint_paths_repo    ON taint_paths(repo_id);
CREATE INDEX idx_taint_paths_active  ON taint_paths(repo_id, is_active);
CREATE INDEX idx_taint_paths_class   ON taint_paths(vuln_class);

-- calibration_events: agent accept/reject signals — the data flywheel
-- Append-only: UPDATE and DELETE are blocked by trigger
CREATE TABLE calibration_events (
    id              BIGSERIAL   PRIMARY KEY,
    repo_id         UUID        NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    taint_path_id   UUID        REFERENCES taint_paths(id) ON DELETE SET NULL,
    action          TEXT        NOT NULL CHECK (action IN ('accepted', 'rejected', 'ignored')),
    agent_type      TEXT,                               -- cursor|claude-code|copilot|windsurf
    session_id      TEXT,
    file_path       TEXT,
    vuln_class      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_calibration_repo     ON calibration_events(repo_id);
CREATE INDEX idx_calibration_path     ON calibration_events(taint_path_id);
CREATE INDEX idx_calibration_action   ON calibration_events(action);

-- Prevent mutation of calibration events (append-only guarantee)
CREATE OR REPLACE FUNCTION prevent_calibration_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'calibration_events is append-only: % on row % is not allowed',
        TG_OP, OLD.id;
END;
$$;

CREATE TRIGGER calibration_no_update
    BEFORE UPDATE ON calibration_events
    FOR EACH ROW EXECUTE FUNCTION prevent_calibration_mutation();

CREATE TRIGGER calibration_no_delete
    BEFORE DELETE ON calibration_events
    FOR EACH ROW EXECUTE FUNCTION prevent_calibration_mutation();

-- migrate:down
DROP TRIGGER IF EXISTS calibration_no_delete ON calibration_events;
DROP TRIGGER IF EXISTS calibration_no_update ON calibration_events;
DROP FUNCTION IF EXISTS prevent_calibration_mutation();
DROP TABLE IF EXISTS calibration_events;
DROP TABLE IF EXISTS taint_paths;
DROP TABLE IF EXISTS spg_nodes;
DROP TABLE IF EXISTS repositories;
