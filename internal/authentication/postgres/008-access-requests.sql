-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
CREATE TABLE agenova_access_requests (
 id text PRIMARY KEY,
 issuer text NOT NULL,
 subject text NOT NULL,
 email text NOT NULL,
 team text NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
 principal text REFERENCES agenova_members(principal),
 created_at timestamptz NOT NULL DEFAULT now(),
 reviewed_at timestamptz,
 UNIQUE(issuer,subject)
);
CREATE INDEX agenova_access_requests_team_status ON agenova_access_requests(team,status);
