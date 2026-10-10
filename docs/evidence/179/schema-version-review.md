# E17 Schema-Version Privilege Review Fix

Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Finding: [P2 include the migration marker in privilege checks](https://github.com/wunderforge/agenova/pull/212#discussion_r4226493832).
Source baseline: `43a4b423f345179ad0792214ec0c944b56329ea3` plus this bounded fix.

## Correction and Proof

Readiness now includes a separate exact-schema/table aggregate for
`agenova_memory.schema_version`. It requires one matching object, SELECT and
no ownership/owner-role membership or INSERT/UPDATE/DELETE/TRUNCATE privileges.
Column-level INSERT/UPDATE grants are rejected too. The marker does not require
tenant RLS or INSERT; the existing records/receipts checks are unchanged.
The additional result participates in the same bounded, read-only startup query.
No migration, role provisioning, worker protocol, credential interface or driver
dependency is added.

[PostgreSQL's privilege inquiry functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE)
define table, column and direct/indirect role-membership checks. All SQL details
remain inside the PostgreSQL adapter.

Before the production change, the new query-shape regression failed with
`missing separate schema-version safety aggregate`. Afterward it passes and
independently requires each marker-specific predicate. Scripted driver tests
cover the safe read-only result, a false safety result, NULL/invalid responses,
stable error sanitization, all five readiness flags and one query with zero
transactions/commits/rollbacks. NULL/invalid result faults retain the existing
typed-failure mapping; they never return a backend.

Sources: [query/result regressions](../../../internal/memory/postgres/readiness_test.go),
[existing flag controls](../../../internal/memory/postgres/backend_test.go) and
[adapter](../../../internal/memory/postgres/backend.go).
These are query-structure and scripted-driver checks, not executed PostgreSQL
privilege, RLS or live readiness proof.

## Gates

Native focused tests passed. The campaign uses existing WSL Ubuntu 24.04 tools:
Go 1.27.1, Node 24.12.0, PowerShell 7.6.6, GCC 13.3 and Playwright 1.63.0.

| Gate | Exact command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Repeated readiness | `go test -count=20 ./internal/memory/postgres -run 'TestReadiness'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 189 frontend tests / build / 59 browser cases |

[Source output](../../../work/0179-scoped-memory/review-marker-linux.log)
records the campaign. No database, cluster, Secret or PVC was read or changed.
The user authorized #155, now a separate foundation draft [#213](https://github.com/wunderforge/agenova/pull/213),
but its Platform composition, live evidence and independent acceptance are
outstanding. No existing kind context is supplied. Driver selection, installed
Memory, real database/restart/network/data-use and teammate reproduction remain
blocked or incomplete. Keep #212 draft and #179 open.

Only documentation/publication edits followed the passing source campaign.
[Publication output](../../../work/0179-scoped-memory/review-marker-publication-linux.log)
records Docs, PR-body and whitespace checks. New commit CI and independent
review remain separate gates.
