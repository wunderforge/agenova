# E17 Tenant Column-Privilege Review Correction

Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Finding: [P2 reject tenant column-level UPDATE](https://github.com/wunderforge/agenova/pull/212#discussion_r4226857918).
Source baseline: `a249a21c31b94372f61783b0dfc9195805e97cf2` plus this correction.

## Behavior

The existing tenant-table readiness aggregate now rejects any column UPDATE
privilege on both `records` and `receipts`, alongside its table-level checks.
This includes direct grants, inherited-role grants and PUBLIC grants.
Required SELECT/INSERT, forced RLS, role/schema safety and the separate marker
aggregate remain intact. Startup still makes one bounded read-only query;
it does not repair privileges, migrate or begin a mutating transaction.

[PostgreSQL's privilege inquiry functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE)
distinguish table privileges from grants on individual columns. A table-only
UPDATE check can report false while a column UPDATE check reports true.

The new [query-shape regression](../../../internal/memory/postgres/readiness_test.go)
failed before the correction with `tenant-table safety permits column-level UPDATE`.
The [real SQL regression](../../../internal/memory/postgres/sql_integration_test.go)
also failed with the old query in all four cases: record body, receipt digest,
inherited record-column grant and PUBLIC receipt-column grant.
The independent server oracle reports table UPDATE false / column UPDATE true;
the corrected production query reports the tenant safety flag false.
Revoking each offending grant restores all five readiness flags to true.
The existing scripted result tests prove `New` refuses a false safety flag.

## Verification

On macOS arm64: Go 1.27.1, Node 24.21.0, PowerShell 7.4.7,
Playwright 1.63.0, Docker 29.7.2 and PostgreSQL 18.6.

| Gate | Command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts ./internal/app` | pass |
| Real regression, old query | `go test -tags memorypostgres -v -count=1 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration/column_privileges_reject_readiness$'` with the explicit image | expected fail, four unsafe grants reproduce |
| Real SQL, corrected query | `go test -tags memorypostgres -v -race -count=3 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` with the explicit image | pass, three fresh databases |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go, 189 frontend tests, build, 59 browser cases |

Captured [focused](../../../work/0179-scoped-memory/review-column-focused-macos.log),
[race](../../../work/0179-scoped-memory/review-column-race-macos.log),
[old-query reproduction](../../../work/0179-scoped-memory/postgres-column-before-macos.log),
[real final SQL](../../../work/0179-scoped-memory/postgres-sql-final-macos.log) and
[full corrected gate](../../../work/0179-scoped-memory/review-column-all-macos-final.log)
output is content-free. The broader SQL proof and its limits are in the
[SQL campaign](postgres-sql.md).

The first full attempt hit sandbox dependency DNS; a retry then exposed an
existing macOS native-select shortcut assumption. The browser smoke retains
keyboard navigation, focus, semantic and live-announcement assertions, using
Playwright option selection for the platform-dependent native control. It does
not claim native-select keyboard-shortcut coverage.

No existing cluster, Secret, PVC or database was changed. Driver/credential
composition, installed Memory, worker network/data-use proof, live public
records and independent review/reproduction remain outstanding. #155/#213,
#192 and #207 remain unmerged. Keep #212 draft and #179 open.
