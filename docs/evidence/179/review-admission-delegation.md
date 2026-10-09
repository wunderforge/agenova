# E17 Atomic Admission and Delegation Corrections

Date: 10 October 2026 (Australia/Sydney).
Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Baseline: `e25c5f7787a05c2f2d2d20ea806d9d4b937e93ca`, passed CI run 436.
Findings: [terminal/admission ordering](https://github.com/wunderforge/agenova/pull/212#discussion_r4230299888)
and [privilege delegation](https://github.com/wunderforge/agenova/pull/212#discussion_r4230299896).

## Lifecycle Publication

Memory now requires the existing lifecycle `ObserveState` boundary and publishes
its decision and attempted-call facts while holding that read boundary. The
callback uses its supplied snapshot without reentering the state owner. Terminal
transitions take the same owner's write lock. All backend IO runs after releasing
the read lock; a known write acknowledgement remains truthful after revocation,
while a revoked read withholds content. Required fact publication remains local
and bounded. There is no snapshot-only fallback or public contract change.

The [composed regression](../../../internal/connectedclient/memory_admission_test.go)
uses the actual RunService and Journal. Test-only sink/observer wrappers trigger
cancellation or the run-owned expiry signal at decision/attempt publication,
without sleeps or production hooks. The old code fails eight admission cases
and the connected reader rejects those real producer records. Four terminal-first
denial controls already pass. The corrected test additionally covers four
in-flight controls: terminal publication completes while the backend is blocked,
known writes remain Written, and read content is withheld. All 16 cases pass;
pre-dispatch interruption and terminal-first cases make zero backend calls.

The initial combined repeated run exposed delayed cancellation propagation:
`context.AfterFunc` schedules asynchronously, so a cancelled run could still
reach the backend while the call context and canonical state remained active.
A separate [delayed-callback regression](../../../internal/memory/admission_test.go)
uses a controllable context scheduler; all four read/write and decision/attempt
cases fail before the synchronous pre-dispatch run-context check. The correction
retains the zero-call assertion and closes the existing admitted invocation.
See Go's [AfterFunc contract](https://pkg.go.dev/context#AfterFunc).

Twenty final race-enabled repetitions exercise 320 lifecycle interleavings and
80 delayed-callback cases, plus existing retry handoffs. The
[16 public producer traces](../../../work/0179-scoped-memory/memory-admission-traces.json)
were captured with explicit `AGENOVA_MEMORY_ADMISSION_TRACES` through the public
serializer, checked for private sentinels, and accepted by Portal request/list
readers. Normal/repeated tests do not write artifacts. Regeneration preserves
behavior, with fresh invocation IDs/timestamps. Early fixture corrections removed
an unused import and supplied the required Failed/Expired outcome and failure
reason; the final pre-fix reproduction preserves all negative assertions.

## Database Delegation

Readiness rejects schema USAGE grant options, table/column SELECT grant options
on records, receipts and schema_version, and table/column INSERT grant options on
tenant tables. Column inquiry functions also detect table grants. The role flag
separately rejects ADMIN options held by the current role or a reachable member
role, preventing delegation through ordinary custom data-group membership.
Required ordinary privileges and non-administrative memberships remain usable.
The query still returns five booleans and performs no privilege repair.

References: PostgreSQL 18 [privilege inquiry functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE)
and [object/role grant semantics](https://www.postgresql.org/docs/18/sql-grant.html).
The stricter reachable-role check is a conservative dedicated-identity rule.

The [live SQL gate](../../../internal/memory/postgres/sql_integration_test.go)
adds 20 direct/inherited table/column grant-option cases, two schema USAGE cases
and two direct/transitive role ADMIN cases. Independent effective-privilege
oracles confirm every unsafe capability; the old readiness query accepts all
24 and fails the new assertions. Revoking the option restores safe readiness
without removing the application's required ordinary grants.

## Verification

Same macOS arm64, Go 1.27.1, Node 24.21.0, PowerShell 7.4.7, Playwright 1.63.0,
Docker 29.7.2 and official PostgreSQL 18.6 digest as the
[SQL campaign](postgres-sql.md). Repository toolchain/dependency locks unchanged.

| Gate | Command / artifact | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 -timeout=2m ./internal/memory/... ./internal/facts ./internal/app ./internal/connectedclient` | pass |
| Race | same packages with `-race -count=1` | pass |
| Repeated | `go test -race -count=20 -timeout=2m ./internal/memory/... ./internal/connectedclient -run 'TestRetry\|TestReadiness\|TestMemoryAdmission'` | pass, 320 lifecycle + 80 delayed-callback cases plus existing 2,560 retry handoffs |
| Portal | `npm --prefix ui run test -- src/memory-evidence.test.ts` | pass, 77 tests including 16 producer traces |
| Real SQL | explicit official digest plus `go test -tags memorypostgres -v -race -count=3 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` | pass, three fresh databases; 24 delegation cases and all prior controls |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` after each correction | both pass, all Go, 205 frontend tests, build, 59 browser cases |
| Publication | selected SQL vet, `check.ps1 -Docs`, `check-pr-body.ps1`, `git diff --check` | pass |

Captured logs: [admission before](../../../work/0179-scoped-memory/review-admission-before-macos.log),
[admission after](../../../work/0179-scoped-memory/review-admission-after-macos.log),
[initial focused sandbox failure](../../../work/0179-scoped-memory/review-admission-focused-macos.log),
[focused retry](../../../work/0179-scoped-memory/review-admission-focused-macos-retry.log),
[admission race](../../../work/0179-scoped-memory/review-admission-race-macos.log),
[admission repeated](../../../work/0179-scoped-memory/review-admission-repeated-macos.log),
[Portal](../../../work/0179-scoped-memory/review-admission-portal-macos.log),
[admission full gate](../../../work/0179-scoped-memory/review-admission-all-macos.log),
[delegation query before](../../../work/0179-scoped-memory/review-delegation-before-macos.log),
[delegation real SQL before](../../../work/0179-scoped-memory/review-delegation-sql-before-macos.log),
[final focused](../../../work/0179-scoped-memory/review-admission-delegation-focused-macos.log),
[final race](../../../work/0179-scoped-memory/review-admission-delegation-race-macos.log),
[final repeated](../../../work/0179-scoped-memory/review-admission-delegation-repeated-macos.log),
[repeated cancellation failure](../../../work/0179-scoped-memory/review-admission-delegation-repeated-before-macos.log),
[delayed-callback reproduction](../../../work/0179-scoped-memory/review-admission-callback-before-macos.log),
and [corrected real SQL](../../../work/0179-scoped-memory/review-delegation-sql-final-macos.log),
plus [final full gate](../../../work/0179-scoped-memory/review-admission-delegation-all-macos.log)
and [publication checks](../../../work/0179-scoped-memory/review-admission-delegation-publication-macos.log).
The initial focused test could not open httptest loopback ports under the sandbox;
the normal-environment retry passed without changing HTTP assertions.

This is real lifecycle/journal and real SQL proof. Memory backend acknowledgements
in the lifecycle controls and Portal fetch are synthetic; no new live UI proof
is claimed. Go driver/pool, accepted host credentials, installed worker Memory,
Work A/B/restart/C, network/data-use oracles, live public records and team lead/
Sonia independent acceptance remain outstanding. #155/#213, #192 and #207 remain
unmerged. Keep #212 draft and #179 open; new commit CI/re-review are separate gates.
