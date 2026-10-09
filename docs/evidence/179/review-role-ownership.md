# E17 Administrative Role and Ownership Corrections

Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Source baseline: `a2332a62dd6f69dec9b00e16368d9042c1db8970` plus these fixes.
Findings: [replication-capable role](https://github.com/wunderforge/agenova/pull/212#discussion_r4229813161)
and [schema ownership](https://github.com/wunderforge/agenova/pull/212#discussion_r4229813173).

## Dedicated Role Contract

Readiness now examines the current application role and its member roles,
rejecting SUPERUSER, BYPASSRLS, CREATEROLE, CREATEDB and REPLICATION attributes.
It also rejects memberships in reserved PostgreSQL predefined server roles,
including transitive memberships. This reference adapter requires a dedicated
data-only role; monitoring or server administration uses a separate identity.
Ordinary custom data groups remain compatible when their effective privileges
meet the existing tenant/marker contract.

Schema safety now examines the exact namespace owner and rejects owner-role
membership as well as CREATE. It separately rejects current-database ownership,
owner-role membership and database CREATE. An owner retains management authority
even after its ordinary CREATE permission is revoked. These checks produce the
same five booleans through one bounded read-only query; no privileges are repaired,
schema migrated or administrative authority added.

References: [role attributes](https://www.postgresql.org/docs/18/view-pg-roles.html),
[predefined server roles](https://www.postgresql.org/docs/18/predefined-roles.html)
and [ownership semantics](https://www.postgresql.org/docs/18/sql-grant.html).
The stricter dedicated-role requirement is the adapter's reference configuration
rule; it is not a claim that ordinary custom role membership implies administration.

## Real Regressions

The [live SQL gate](../../../internal/memory/postgres/sql_integration_test.go)
adds ten direct/inherited attribute cases, six transitive predefined-role cases
and four direct/inherited schema/database ownership cases. The old production
query fails 16 of these 20 cases; four previously rejected direct attributes
remain negative controls. For schema ownership, an independent oracle proves
CREATE is false while owner-role membership is true. No SQL execution errors
remain in the selected pre-fix reproduction.

The corrected query rejects all 20 unsafe configurations and restores five true
flags after each operator reset. Three database CREATE cases separately prove
direct, inherited and PUBLIC grants are rejected. Three fresh race-enabled full
campaigns pass these cases alongside the prior table/column privilege, RLS,
transaction reset, literal matching, receipt and database-container restart
controls. The unchanged scripted flag tests ensure false readiness cannot return
a backend from `New`.

Early fixture failures used the wrong SUPERUSER catalog-column name and lost
the application's original schema USAGE during ownership transfer cleanup.
The corrected fixture uses `rolsuper` and restores that initial USAGE grant;
it preserves all safety assertions and reruns the pre-fix and corrected proofs.

## Gates and Limits

Same macOS/Go/Node/PowerShell/Docker/PostgreSQL versions, explicit official image
digest and isolated cleanup as the [SQL campaign](postgres-sql.md).

| Gate | Command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Repeated readiness | `go test -race -count=20 ./internal/memory/postgres -run TestReadiness` | pass |
| Real SQL | explicit digest plus `go test -tags memorypostgres -v -race -count=3 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` | pass, three fresh databases |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go, 189 frontend tests, build, 59 browser cases |
| Publication | selected SQL vet, `check.ps1 -Docs`, `check-pr-body.ps1`, `git diff --check` | pass |

Captured [before-fix query shape](../../../work/0179-scoped-memory/review-role-ownership-before-macos.log),
[before-fix real SQL](../../../work/0179-scoped-memory/review-role-ownership-sql-before-macos.log),
[focused](../../../work/0179-scoped-memory/review-role-ownership-focused-macos.log),
[race](../../../work/0179-scoped-memory/review-role-ownership-race-macos.log),
[repeated](../../../work/0179-scoped-memory/review-role-ownership-repeated-macos.log),
[corrected real SQL](../../../work/0179-scoped-memory/review-role-ownership-sql-final-macos.log)
and [full gate](../../../work/0179-scoped-memory/review-role-ownership-all-macos.log),
plus [publication checks](../../../work/0179-scoped-memory/review-role-ownership-publication-macos.log),
remain sanitized. No actual credentials, existing database, cluster, Secret or
PVC was read or changed.

Go driver/pool, accepted #155 credentials, Platform/worker Memory execution,
independent A/B/restart/C, network/data-use oracles, live CLI/API/Portal records
and the team lead/Sonia's independent review/reproduction remain outstanding.
Keep #212 draft and #179 open. New commit CI/re-review are separate gates.
