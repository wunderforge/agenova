-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
CREATE TABLE agenova_project_policies (
 team text NOT NULL, project text NOT NULL, id text NOT NULL, version text NOT NULL,
 document jsonb NOT NULL, PRIMARY KEY(team,project,id,version),
 FOREIGN KEY(team,project) REFERENCES agenova_projects(team,project)
);
CREATE TABLE agenova_project_policy_active (
 team text NOT NULL, project text NOT NULL, id text NOT NULL, version text NOT NULL,
 PRIMARY KEY(team,project),
 FOREIGN KEY(team,project,id,version) REFERENCES agenova_project_policies(team,project,id,version)
);
CREATE TABLE agenova_project_templates (
 team text NOT NULL, project text NOT NULL, name text NOT NULL, document jsonb NOT NULL,
 PRIMARY KEY(team,project,name), FOREIGN KEY(team,project) REFERENCES agenova_projects(team,project)
);
