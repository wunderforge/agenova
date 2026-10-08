-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0

-- Apply once as a dedicated migration owner, never as the application role.
-- The operator grants schema USAGE, schema_version SELECT, and records/receipts
-- SELECT/INSERT (no other privileges) to its
-- non-owner, non-superuser, NO BYPASSRLS application role after migration.
BEGIN;
CREATE SCHEMA agenova_memory;
REVOKE ALL ON SCHEMA agenova_memory FROM PUBLIC;
CREATE TABLE agenova_memory.schema_version (
    singleton boolean PRIMARY KEY CHECK (singleton),
    version integer NOT NULL
);
INSERT INTO agenova_memory.schema_version VALUES (true, 1);

CREATE TABLE agenova_memory.records (
    team text NOT NULL CHECK (team <> ''),
    project text NOT NULL CHECK (project <> ''),
    scope text NOT NULL CHECK (scope <> ''),
    id text NOT NULL CHECK (id ~ '^memory:[0-9a-f]{32}$'),
    body text NOT NULL CHECK (octet_length(body) BETWEEN 1 AND 8192),
    source_claim text NOT NULL CHECK (source_claim <> ''),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (team, project, scope, id)
);
CREATE INDEX memory_recent ON agenova_memory.records
    (team, project, scope, created_at DESC, id DESC);
CREATE TABLE agenova_memory.receipts (
    team text NOT NULL,
    project text NOT NULL,
    scope text NOT NULL,
    invocation_id text NOT NULL CHECK (invocation_id <> ''),
    request_digest text NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    memory_id text NOT NULL,
    PRIMARY KEY (team, project, scope, invocation_id),
    FOREIGN KEY (team, project, scope, memory_id)
        REFERENCES agenova_memory.records (team, project, scope, id)
);

ALTER TABLE agenova_memory.records ENABLE ROW LEVEL SECURITY;
ALTER TABLE agenova_memory.records FORCE ROW LEVEL SECURITY;
ALTER TABLE agenova_memory.receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE agenova_memory.receipts FORCE ROW LEVEL SECURITY;

CREATE POLICY records_select ON agenova_memory.records FOR SELECT USING (
    team = current_setting('agenova.team', true)
    AND project = current_setting('agenova.project', true)
    AND scope = current_setting('agenova.scope', true)
);
CREATE POLICY records_insert ON agenova_memory.records FOR INSERT WITH CHECK (
    team = current_setting('agenova.team', true)
    AND project = current_setting('agenova.project', true)
    AND scope = current_setting('agenova.scope', true)
);
CREATE POLICY receipts_select ON agenova_memory.receipts FOR SELECT USING (
    team = current_setting('agenova.team', true)
    AND project = current_setting('agenova.project', true)
    AND scope = current_setting('agenova.scope', true)
);
CREATE POLICY receipts_insert ON agenova_memory.receipts FOR INSERT WITH CHECK (
    team = current_setting('agenova.team', true)
    AND project = current_setting('agenova.project', true)
    AND scope = current_setting('agenova.scope', true)
);
REVOKE ALL ON ALL TABLES IN SCHEMA agenova_memory FROM PUBLIC;
COMMIT;
