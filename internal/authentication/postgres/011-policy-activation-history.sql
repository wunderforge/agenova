-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
-- Structured scope keys avoid ambiguous slash-delimited audit targets.
CREATE TABLE agenova_policy_activations (
 event_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 team text NOT NULL, project text NOT NULL, id text NOT NULL, version text NOT NULL,
 actor text NOT NULL, activated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(team,project,id,version) REFERENCES agenova_project_policies(team,project,id,version)
);
CREATE INDEX agenova_policy_activations_scope ON agenova_policy_activations(team,project,event_id DESC);
