# E17 Retry Handoff and Unexpected Privilege Corrections

Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Source baseline: `ce9b38de47554da332b1e63e54ca5afe5538b9b3` plus these fixes.
Findings: [TRIGGER readiness](https://github.com/wunderforge/agenova/pull/212#discussion_r4229292476)
and [retry cancellation handoff](https://github.com/wunderforge/agenova/pull/212#discussion_r4229292485).

## Retry Cancellation

When gate release and cancellation are simultaneously ready, Go's select can
choose gate acquisition. `RetryWrite` now rechecks both caller and run errors
immediately after acquisition and before using the continuation or invoking
admission. Cancellation returns its context error without allocating an ID,
appending facts or dispatching. The gate is released and the uncertainty
continuation's disposition is untouched.

The [behavioral regression](../../../internal/memory/retry_test.go) first
executes an uncertain write and a transient recovery attempt. A context wrapper
cancels and releases an occupied gate during select-operand evaluation,
making both cases ready without scheduler sleeps or a production test hook.
The old code failed caller and run cases at trials 3 and 1. The corrected code
passes all 64 handoffs for each context and preserves zero additional IDs,
facts and backend calls, with no retained gate token. Twenty race-enabled
repetitions cover 2,560 handoffs alongside the existing retry tests.

## Database Privileges

The existing operator contract permits SELECT/INSERT on tenant tables and
SELECT on the migration marker. Readiness now also rejects effective TRIGGER,
REFERENCES, column-level REFERENCES and MAINTAIN for all three tables. This
completes the PostgreSQL 18 table/column privilege checks rather than granting
new schema-management or maintenance rights. The startup query stays bounded,
read-only and five-result; it neither repairs privileges nor migrates.

[PostgreSQL's trigger requirements](https://www.postgresql.org/docs/18/sql-createtrigger.html#SQL-CREATETRIGGER-NOTES)
and [privilege definitions](https://www.postgresql.org/docs/18/ddl-priv.html)
explain why these permissions exceed the role contract. Adapter SQL remains
inside its PostgreSQL boundary.

The [real SQL gate](../../../internal/memory/postgres/sql_integration_test.go)
adds 27 table-privilege cases (three privileges, three tables and direct,
inherited/PUBLIC grants) and three column REFERENCES cases. Independent
server oracles confirm each effective privilege. The pre-fix query fails all
30 cases; the corrected query rejects them, and revocation restores readiness.
Three fresh race-enabled databases pass the expanded campaign and the existing
column UPDATE, marker mutation, RLS, transaction-local reset, literal matching,
receipt constraint and database-container restart controls.

## Gates and Limits

macOS arm64, Go 1.27.1, Node 24.21.0, PowerShell 7.4.7, Playwright 1.63.0,
Docker 29.7.2 and PostgreSQL 18.6; image and cleanup are unchanged from the
[SQL campaign](postgres-sql.md).

| Gate | Exact command | Result |
| --- | --- | --- |
| Before production fixes | `go test -v -count=1 ./internal/memory/... -run 'TestRetryCancellationAtGateHandoff\|TestReadinessSQLRejectsUnexpected'` | expected fail, both findings reproduced |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Repeated | `go test -race -count=20 ./internal/memory/... -run 'TestRetry\|TestReadiness'` | pass |
| Real SQL | explicit digest plus `go test -tags memorypostgres -v -race -count=3 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` | pass, three fresh databases |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go, 189 frontend tests, build, 59 browser cases |

Captured [before-fix unit output](../../../work/0179-scoped-memory/review-handoff-privileges-before-macos.log),
[before-fix real SQL](../../../work/0179-scoped-memory/review-trigger-sql-before-macos.log),
[focused](../../../work/0179-scoped-memory/review-handoff-privileges-focused-macos.log),
[race](../../../work/0179-scoped-memory/review-handoff-privileges-race-macos.log),
[repeated](../../../work/0179-scoped-memory/review-handoff-privileges-repeated-macos.log),
[real corrected SQL](../../../work/0179-scoped-memory/review-trigger-sql-final-macos.log)
and [full gate](../../../work/0179-scoped-memory/review-handoff-privileges-all-macos.log)
output is sanitized. No actual credentials, existing database, cluster, Secret
or PVC was read or changed.

This remains psql/schema and reference-session evidence. Driver/host-credential
composition, installed Platform/worker wiring, independent Work A/B/restart/C,
worker network/data-use oracles, live CLI/API/Portal records and the nominated
team lead/Sonia's independent review/reproduction are outstanding. Keep #212
draft and #179 open. New commit CI and re-review remain separate gates.
