# E17 Atomic Read Completion

Date: 10 October 2026 (Australia/Sydney).
Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Baseline: `b4bf8f38ab395e4c0d4144cdc377cb8d4d4b1362`, passed
[CI run 438](https://github.com/wunderforge/agenova/actions/runs/37941178902);
[automatic review](https://github.com/wunderforge/agenova/pull/212#issuecomment-6082587651)
reported no major issues. This additional defect was found by local self-review.

## Behavior and Reproduction

The final read eligibility check previously took a separate state snapshot before
publishing ProviderOutcome. Cancellation/expiry could publish terminal Runtime
between those steps: the read returned Found with body data, while both connected
readers rejected that late read-success record. Admission was already atomic;
this is the separate completion boundary.

The actual RunService/Journal regression now adds two controlled read-completion
cases. Its test-only reader lets terminal publication overtake the old snapshot
after a backend search returns. Both cases fail against b4bf8f3: one entry is
delivered, success follows terminal authority, and the connected validator
rejects the evidence. No sleep or production hook is involved.

Read eligibility and outcome publication now share the existing lifecycle
ObserveState boundary. The callback uses its supplied current snapshot without
reentering the reader; backend IO and reply validation remain outside the lock.
If terminal wins first, entries are withheld and the admitted call closes with
Cancelled/Timeout metadata. A known write acknowledgement stays truthful after
revocation; it is not converted to rollback. Public schemas, credentials, adapter
SQL and installed contracts are unchanged.

The [18 captured public producer traces](../../../work/0179-scoped-memory/memory-admission-traces.json)
pass both connected readers, including the two revoked completions. Twenty final
race-enabled repetitions cover 360 lifecycle cases, 80 delayed-callback cases and
the existing 2,560 retry handoffs. Read-completion cases make one prior authorized
backend call and deliver zero entries; pre-admission/dispatch denial controls
retain zero backend calls. Four in-flight controls show terminal publication can
proceed during backend IO and retain committed-write truth/read withholding.

The original 16-trace [admission/delegation campaign](review-admission-delegation.md)
is retained at [its exact published capture](https://github.com/wunderforge/agenova/blob/b4bf8f38ab395e4c0d4144cdc377cb8d4d4b1362/work/0179-scoped-memory/memory-admission-traces.json).
The current fixture extends that campaign to 18 traces; fresh IDs/timestamps are
expected when explicitly regenerating it.

## Verification and Limits

Same tool versions and environment as the preceding campaign. No driver,
toolchain requirement or dependency lock changed.

| Gate | Command | Result |
| --- | --- | --- |
| Pre-fix | `go test -v -count=1 -timeout=30s ./internal/connectedclient -run '^TestMemoryAdmissionCorrelatesWithTerminalTransitions/(cancel\|expire)/read-completion'` | fail, both named cases |
| Focused | `go test -v -count=1 -timeout=2m ./internal/memory/... ./internal/facts ./internal/app ./internal/connectedclient` | pass, with explicit public trace capture |
| Race | same packages with `-race -count=1` | pass |
| Repeated | `go test -race -count=20 -timeout=2m ./internal/memory/... ./internal/connectedclient -run 'TestRetry\|TestReadiness\|TestMemoryAdmission'` | pass |
| Portal | `npm --prefix ui run test -- src/memory-evidence.test.ts` | pass, 79 tests |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go, 207 frontend tests, build, 59 browser cases |

Captured [pre-fix](../../../work/0179-scoped-memory/review-read-completion-before-macos.log),
[focused](../../../work/0179-scoped-memory/review-read-completion-focused-macos.log),
[race](../../../work/0179-scoped-memory/review-read-completion-race-macos.log),
[repeated](../../../work/0179-scoped-memory/review-read-completion-repeated-macos.log),
[Portal](../../../work/0179-scoped-memory/review-read-completion-portal-macos.log)
and [full gate](../../../work/0179-scoped-memory/review-read-completion-all-macos.log)
record exact output; [publication checks](../../../work/0179-scoped-memory/review-read-completion-publication-macos.log)
cover documentation, PR body, selected SQL vet and whole-change whitespace.
The PostgreSQL adapter SQL is identical to b4bf8f3, whose
three fresh real SQL campaigns remain applicable; focused scripted-driver tests
pass with the updated service composition. No redundant database campaign is
claimed for this service-only correction.

These are real lifecycle/journal records with synthetic Memory acknowledgements
and Portal fetch, separate from installed worker/backend acceptance. Accepted
#155/#213, #192/#207 producer contracts, Go-driver/host composition, installed
Work A/B/restart/C, network/data-use oracles, live UI/CLI/API records and team
lead/Sonia independent acceptance remain outstanding. Keep #212 draft and #179
open. New commit CI/re-review are separate gates; automatic review is not human
acceptance.
