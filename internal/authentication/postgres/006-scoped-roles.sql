-- Copyright 2026 Agenova contributors.
-- SPDX-License-Identifier: Apache-2.0
-- Additive grants; existing project membership is deliberately unchanged.
CREATE TABLE agenova_scoped_roles (
 principal text NOT NULL REFERENCES agenova_members(principal),
 team text NOT NULL,
 role text NOT NULL CHECK (role IN ('governance-admin','platform-operator')),
 scope_kind text NOT NULL CHECK (scope_kind IN ('account','project','installation')),
 scope_ref text NOT NULL CHECK (scope_ref <> ''),
 PRIMARY KEY(principal,team,role,scope_kind,scope_ref),
 CHECK ((role='governance-admin' AND scope_kind IN ('account','project')) OR
        (role='platform-operator' AND scope_kind='installation'))
);
