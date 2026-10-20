-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
-- Only hashes of application credentials are persisted. Google tokens are absent.
CREATE TABLE agenova_application_sessions (
    token_hash text PRIMARY KEY,
    session_id text NOT NULL UNIQUE,
    issuer text NOT NULL,
    subject text NOT NULL,
    authenticated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    FOREIGN KEY (issuer,subject) REFERENCES agenova_members(issuer,subject),
    CHECK (expires_at > authenticated_at)
);
CREATE INDEX agenova_application_sessions_subject ON agenova_application_sessions(issuer,subject);
