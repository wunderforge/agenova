# E17 Installed RLS Policy Definitions

Date: 10 October 2026 (Australia/Sydney).
Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Baseline: `6eeff5bbf289c6b6dccd0c61d659459d1ca81c4d`, passed CI run 444.
Finding: [validate installed RLS definitions](https://github.com/wunderforge/agenova/pull/212#discussion_r4235519000).

## Configuration Rule

Enabled/forced RLS and a version marker alone cannot prove namespace isolation.
Applicable permissive policies combine by OR, so an additional or altered TRUE
rule exposes another namespace. PostgreSQL documents the
[policy combination](https://www.postgresql.org/docs/18/sql-createpolicy.html)
and the [policy catalog](https://www.postgresql.org/docs/18/catalog-pg-policy.html).

The tenant-table readiness flag now requires exactly two policies per table:
the version-one SELECT and INSERT names, correct commands, permissive mode,
PUBLIC-only roles, the complete canonical ownership filter in its appropriate
USING/WITH CHECK field, and NULL in the unused field. Each policy predicate
coalesces NULL to false before aggregation. Missing/extra/restrictive/renamed,
wrong-role/command/setting and TRUE/FALSE/absent-filter definitions fail readiness.
Whitespace is normalized by PostgreSQL itself; semantic equivalence or manual
text rewriting does not approve schema drift.

Canonical expression comparison requires the three namespace columns to remain
text with the default collation and pg_catalog to lead the effective search path.
The namespace guard uses an explicitly qualified builtin function, type and
equality operator outside the aggregate. An unsupported path cannot shadow the
aggregate and turn its failed guard into success. This conservative PostgreSQL 18
reference rule is a read-only verification of the existing migration; the
migration/version and shared authority/credential contracts do not change.
The same one bounded query retains five result flags and repairs no schema/grants.

## Real Reproductions and Controls

The [production SQL fixture](../../../internal/memory/postgres/sql_integration_test.go)
uses actual SCRAM application logins. Independent transactions demonstrate cross-
namespace SELECT/INSERT after permissive policy drift, rolling writes back.
The [final exact-baseline run](../../../work/0179-scoped-memory/review-policy-baseline-final-macos.log)
fails all 48 negatives before restoring corrected source: 44 policy mutations
across both tables, two namespace-collation changes and two parser/aggregate
controls. Every mutation restores its prior definition and five true flags.
Fixture-only rows, functions and aggregates are removed within the owned container.

Supplementary read-only agent review found an initial namespace guard placed
inside the unqualified aggregate. A custom public.bool_and plus a permissive rule
reproduces the bypass in [intermediate output](../../../work/0179-scoped-memory/review-policy-shadow-before-macos.log).
The guard now sits outside it; the final focused real gate passes all 48 cases.
This agent review does not replace team lead/Sonia independent acceptance.

An [initial full SQL campaign](../../../work/0179-scoped-memory/review-policy-postgres-macos.log)
failed only the mixed-PUBLIC role fixtures: PostgreSQL normalized the list to
PUBLIC, leaving no installed drift to reject. The corrected case uses memory_app
and memory_owner, independently checking two non-PUBLIC role OIDs. No production
safety condition was weakened to make those tests pass.

## Gates and Residual Limits

Same macOS arm64 tool versions and official PostgreSQL 18.6 digest as the
[authentication campaign](review-authentication.md), with network=none and
container-local socket transport. Synthetic passwords remain private to each
owned disposable fixture and do not enter arguments or published output.

| Gate | Exact command / artifact | Result |
| --- | --- | --- |
| Exact old query | selected RLS policy-definition cases, baseline source temporarily restored | fails all 48; corrected source restored before final gates |
| Focused real SQL | `go test -race -tags memorypostgres -v -count=1 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration/RLS_policy_definitions'` with explicit image | pass, 16.421s |
| Focused Go | `go test -count=1 ./internal/memory ./internal/memory/postgres ./internal/facts ./internal/app ./internal/connectedclient` | pass |
| Race | same packages with `-race -count=1` | pass |
| Repeated/vet | `go test -race -count=20 ./internal/memory/postgres -run Readiness`; `go vet -tags memorypostgres ./internal/memory/postgres` | pass |
| Full real SQL | explicit official digest plus `go test -race -tags memorypostgres -v -count=3 -timeout=5m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` | pass, three fresh databases, 133.247s |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass |
| Publication | Docs, PR body and staged whitespace | pass |

Captured [focused Go](../../../work/0179-scoped-memory/review-policy-focused-final-macos.log),
[focused SQL](../../../work/0179-scoped-memory/review-policy-focused-postgres-macos.log),
[final full SQL](../../../work/0179-scoped-memory/review-policy-postgres-final-macos.log),
[repository](../../../work/0179-scoped-memory/review-policy-all-macos.log) and
[publication](../../../work/0179-scoped-memory/review-policy-publication-macos.log)
records distinguish successful final runs from the documented failed attempts.

This is startup SQL/schema verification through psql, not continuous protection
against later administrator DDL, a Go-driver/pool or accepted credential consumer.
No existing database, cluster, Secret or PVC changed. #155/#213, #192 and #207
remain unmerged. Installed Memory A/B/restart/C, backend/network/data-use oracles,
live CLI/API/Portal and team lead/Sonia review/reproduction remain outstanding.
Keep #212 draft and #179 open; new commit CI/re-review remain separate gates.
