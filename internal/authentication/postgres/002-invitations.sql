-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
CREATE TABLE agenova_invitations (
 id text PRIMARY KEY,
 team text NOT NULL,
 email text NOT NULL,
 actor text NOT NULL,
 project text NOT NULL,
 account_role text NOT NULL CHECK (account_role IN ('owner','admin','member')),
 project_role text NOT NULL CHECK (project_role IN ('admin','member','viewer')),
 issuer text NOT NULL,
 subject text,
 principal text,
 status text NOT NULL CHECK (status IN ('reserved','pending','sent','accepted','cancelled','uncertain')),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 accepted_at timestamptz,
 UNIQUE(team,email),
 FOREIGN KEY(team,project) REFERENCES agenova_projects(team,project)
);
CREATE UNIQUE INDEX agenova_one_owner_per_team ON agenova_members(team) WHERE account_role='owner';
