-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
-- Display-only metadata. Tenant identity and existing foreign keys are unchanged.
CREATE TABLE agenova_organization_metadata (
 team text PRIMARY KEY CHECK (team <> ''),
 display_name text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
 description text NOT NULL DEFAULT '' CHECK (length(description) <= 1024)
);
