-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
CREATE TABLE agenova_work (
 ref text PRIMARY KEY,
 team text NOT NULL,
 owner text NOT NULL,
 project text NOT NULL,
 input_name text NOT NULL,
 input_digest text NOT NULL,
 view jsonb NOT NULL,
 staged_result jsonb,
 complete boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(team,owner,input_name),
 FOREIGN KEY(owner) REFERENCES agenova_members(principal),
 FOREIGN KEY(team,project) REFERENCES agenova_projects(team,project)
);
CREATE TABLE agenova_work_facts (
 sequence bigint PRIMARY KEY CHECK (sequence>0),
 id text NOT NULL UNIQUE,
 request_ref text NOT NULL REFERENCES agenova_work(ref),
 payload jsonb NOT NULL
);
CREATE INDEX agenova_work_facts_request ON agenova_work_facts(request_ref,sequence);
CREATE INDEX agenova_work_owner ON agenova_work(team,owner,created_at DESC);
