# E17 SET Reachability and Session Identity

Date: 10 October 2026 (Australia/Sydney).
Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Baseline: `e00c27f8db0e3e86a6b84808d0dfcccb78f381d8`, passed CI runs 439/440.
Finding: [SET-only membership](https://github.com/wunderforge/agenova/pull/212#discussion_r4231303531).

## Configuration Rule

The reference adapter now rejects every SET-reachable role whose privileges are
not also immediately inherited (SET=true, USAGE=false). This conservative rule
keeps the existing effective ACL checks complete without introducing a second
role-specific privilege evaluator. Ordinary inherited data groups remain
supported; a non-inherited group with SET=false remains unreachable and passes
when the other existing safety conditions hold.

This campaign required current_user to equal session_user, rejecting SET ROLE
masking. Subsequent review showed that equality does not prove the original
authenticated identity: SET SESSION AUTHORIZATION can change both values.
The [authentication correction](review-authentication.md) supersedes that claim
and the operator-session positive fixtures with actual SCRAM data-role logins.
These are adapter reference configuration restrictions, not new public authority
or credential contracts.
One bounded read-only query still returns five booleans, with no grant repair.

References: PostgreSQL 18 [role membership](https://www.postgresql.org/docs/18/role-membership.html)
and [privilege inquiry functions](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-ACCESS-TABLE).

## Real Negative and Positive Controls

The [live SQL gate](../../../internal/memory/postgres/sql_integration_test.go)
proves direct and transitive INHERIT=false/SET=true paths. Independent oracles
show the application has no immediate TRUNCATE permission, can SET the hidden
role and does not inherit it. This historical campaign changed session
authorization to memory_app before switching roles and TRUNCATEing both tables.
That establishes the SET reachability control but not a direct data-role login;
the later authentication campaign repeats it with actual SCRAM memory_app login.
ROLLBACK preserves the test database.

The old exact e00c27f readiness query accepts both dangerous memberships and a
third configuration where a privileged session masks itself using SET ROLE.
All three new regressions fail before correction. The old query was temporarily
restored for that selected reproduction, then the corrected source restored
before focused/final gates. No fixture safety assertion was relaxed.

The corrected role flag rejects all three cases. Revocation restores five true
flags. Positive controls prove unreachable non-inherited membership and inherited
benign data membership remain usable. Three fresh race-enabled complete SQL
campaigns pass these and all earlier privilege/delegation/RLS/context/receipt/
database-container restart cases.

## Gates and Limits

Same versions, official PostgreSQL 18.6 digest, local socket transport, network
isolation and scoped cleanup as [the SQL campaign](postgres-sql.md). The existing
application-role fixture used operator session authorization and did not establish
original data-role login, TCP/password authentication or Go-driver/pool integration.
The authentication correction replaces these positive fixtures; this page retains
the historical campaign and its original logs.

| Gate | Command / artifact | Result |
| --- | --- | --- |
| Old query | selected `TestReadinessSQLRejectsHiddenSetRolePrivileges` and `TestPostgresSQLIntegration/(non_inheriting_SET_roles\|masked_privileged_session)` | fail: query shape plus all three live cases |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts ./internal/app ./internal/connectedclient` | pass |
| Race | same packages with `-race -count=1` | pass |
| Repeated | `go test -race -count=20 -timeout=2m ./internal/memory/... ./internal/connectedclient -run 'TestRetry\|TestReadiness\|TestMemoryAdmission'` | pass, includes current lifecycle/retry controls |
| Real SQL | explicit official digest plus `go test -tags memorypostgres -v -race -count=3 -timeout=3m ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'` | pass, three fresh databases |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go, 207 frontend tests, build, 59 browser cases |

Captured [query before](../../../work/0179-scoped-memory/review-set-role-before-macos.log),
[SQL before](../../../work/0179-scoped-memory/review-set-role-sql-before-macos.log),
[focused](../../../work/0179-scoped-memory/review-set-role-focused-macos.log),
[race](../../../work/0179-scoped-memory/review-set-role-race-macos.log),
[repeated](../../../work/0179-scoped-memory/review-set-role-repeated-macos.log),
[corrected SQL](../../../work/0179-scoped-memory/review-set-role-sql-final-macos.log)
and [full gate](../../../work/0179-scoped-memory/review-set-role-all-macos.log)
record exact results; [publication checks](../../../work/0179-scoped-memory/review-set-role-publication-macos.log)
cover selected SQL vet, documentation, PR body and whole-change whitespace.
Lifecycle/reader behavior and the 18 public producer traces
are unchanged from the [read-completion campaign](review-read-completion.md).

No existing cluster, database, Secret or PVC changed. This is executed SQL and
reference lifecycle proof, separate from accepted driver/host credentials,
installed Memory, independent A/B/restart/C, network/data-use oracles, live
CLI/API/Portal and team lead/Sonia acceptance. #155/#213, #192 and #207 remain
unmerged. Keep #212 draft and #179 open; new commit CI/re-review are separate gates.
