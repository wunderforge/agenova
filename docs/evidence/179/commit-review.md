# E17 COMMIT Uncertainty Review Fix

Task: [#179](https://github.com/wunderforge/agenova/issues/179),
[packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Review: [P1 handle SQLSTATE 40003 as uncertain](https://github.com/wunderforge/agenova/pull/212#discussion_r4225007987).
The user authorized continued review fixes on 2026-10-09.

Source baseline: `0d2a158491042b790e493d20abbb284566f52434` on
`codex/0179-scoped-memory`, with this bounded adapter correction applied.

## Correction

The adapter previously treated every class-40 COMMIT error as definite failure.
[PostgreSQL's error table](https://www.postgresql.org/docs/current/errcodes-appendix.html)
defines `40003` as `statement_completion_unknown`; it cannot establish rollback.
The adapter now excludes that code from its definite-failure branch and returns
the existing `ErrWriteUncertain`, including when a driver wraps its SQLSTATE error.

The shared service can consequently return `WriteUncertain` and its host-only
`WriteRetry` continuation. Explicit governed recovery reuses the original receipt
while repeating admission and minting a fresh audit invocation ID. It performs no
automatic retry or second write. Known rollback/serialization/deadlock and the
existing constraint/failed-transaction classifications are unchanged. SQLSTATE
handling stays inside the PostgreSQL adapter; no shared request schema, credential
interface, driver dependency or worker-supplied receipt field was added.

## Behavioral Evidence

Before changing production code, the new tests failed on both direct and wrapped
`40003`: the adapter returned definite failure and the governed service withheld
the retry continuation. After the correction, the same tests pass.

- Direct COMMIT cases cover success, raw lost acknowledgement, `40003`, wrapped
  `40003`, `08007`, cancellation and timeout; uncertainty never returns a record.
- Negative classification controls cover `40000`, `40001`, `40002`, `40P01`,
  `23514`, `25P02` and wrapped known rollback; these remain definite failures.
- The composed service/adapter test covers raw lost acknowledgement, direct
  `40003` and wrapped `40003`. The first write is uncertain; governed recovery
  queries the original receipt and returns Written, with one INSERT sequence,
  one COMMIT total and a recovery transaction that makes no writes and is rolled
  back.
- Both attempts retain their own decision/attempt/outcome and unique audit IDs;
  outcome metadata records WriteUncertain then Written. Driver text, body and
  host-only retry state are absent from result/fact JSON exports.

Sources: [adapter cases](../../../internal/memory/postgres/backend_test.go) and
[governed receipt recovery](../../../internal/memory/postgres/retry_test.go).
These run real service/adapter code against a scripted SQL driver. They do not
execute PostgreSQL, prove real COMMIT crash behavior, RLS or persistence.

## Gates

Native Windows focused tests passed. The Linux campaign uses the existing WSL
Ubuntu 24.04 workspace tools: Go 1.27.1, Node 24.12.0, PowerShell 7.6.6,
GCC 13.3 and Playwright 1.63.0. No dependency lock, database, cluster, Secret
or PVC was changed.

| Gate | Command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts` | pass |
| Repeated COMMIT/recovery and retry regressions | `go test -count=20 ./internal/memory/... -run 'TestWriteCommitsRow\|TestGovernedRetry\|TestRetry\|TestWaitingRetry\|TestConcurrentRetry\|TestUncertainWriteRetry'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 126 frontend tests / build / 54 browser cases |

The [source campaign log](../../../work/0179-scoped-memory/review-commit-linux.log)
records these results. Installed acceptance and independent review remain
outstanding; E17 is incomplete and PR #212 stays draft.

Only documentation/publication edits followed the passing source campaign.
Documentation, PR-body and whitespace checks are recorded separately in the
[publication log](../../../work/0179-scoped-memory/review-commit-publication-linux.log).
