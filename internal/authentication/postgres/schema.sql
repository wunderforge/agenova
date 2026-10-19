-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
-- Run as the migration owner, not as the application runtime role.
CREATE TABLE IF NOT EXISTS agenova_members (
    issuer text NOT NULL,
    subject text NOT NULL,
    principal text NOT NULL UNIQUE,
    team text NOT NULL,
    active boolean NOT NULL DEFAULT false,
    PRIMARY KEY (issuer, subject),
    CHECK (issuer <> '' AND subject <> '' AND principal <> '' AND team <> '')
);
CREATE TABLE IF NOT EXISTS agenova_revocations (
    issuer text NOT NULL,
    subject text NOT NULL,
    session_id text NOT NULL,
    revoked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issuer, subject, session_id)
);
CREATE TABLE IF NOT EXISTS agenova_identity_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor text NOT NULL,
    operation text NOT NULL,
    principal text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE agenova_members ADD COLUMN IF NOT EXISTS account_role text NOT NULL DEFAULT 'member'
    CHECK (account_role IN ('owner','admin','member'));
CREATE TABLE IF NOT EXISTS agenova_projects (
    team text NOT NULL,
    project text NOT NULL,
    PRIMARY KEY(team,project)
);
CREATE TABLE IF NOT EXISTS agenova_project_grants (
    principal text NOT NULL REFERENCES agenova_members(principal),
    team text NOT NULL,
    project text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin','member','viewer')),
    PRIMARY KEY(principal,project),
    FOREIGN KEY(team,project) REFERENCES agenova_projects(team,project)
);
CREATE TABLE IF NOT EXISTS agenova_browser_sessions (
    id_hash text PRIMARY KEY,
    payload bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    version bigint NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS agenova_login_attempts (
    id_hash text PRIMARY KEY,
    payload bytea NOT NULL,
    expires_at timestamptz NOT NULL
);

ALTER TABLE agenova_members ADD COLUMN IF NOT EXISTS login_after timestamptz NOT NULL DEFAULT 'epoch';
