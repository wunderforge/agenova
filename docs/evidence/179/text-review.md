# E17 Shared Text Validation Review Fix

Task: [#179](https://github.com/wunderforge/agenova/issues/179),
[packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Review: [P2 align NUL handling with the shared text contract](https://github.com/wunderforge/agenova/pull/212#discussion_r4224890515).
The user authorized shared validation and zero-database-call regressions on
2026-10-09.

Source baseline: `60ac89c67c0996a7bbd6bc70aa56424cc492505c` on
`codex/0179-scoped-memory`, with this bounded review correction applied.

## Correction

The shared service previously admitted nonblank UTF-8 body/query text containing
NUL, recorded Allow and a provider attempt, then received a failure from the
PostgreSQL adapter's stricter validator. The shared text validator now rejects
U+0000 before dispatch with `Denied` / `memory-invalid-input`. The adapter retains
its matching defense-in-depth check. Returned bodies use the same shared text
rule and invalid backend replies are withheld.

This is a backend-neutral text constraint, not a provider shape or new authority.
Newlines, tabs, Unicode, SQL-like text and literal backslash sequences remain
accepted within existing limits. No text normalization or new secret path is
introduced. The task spec and design explicitly state the rule.

## Behavioral Evidence

- Decoded JSON NUL at the start, middle, end and as the only character is denied
  for both Write and Search, with zero fake-adapter calls.
- Denials have one correlated MemoryDecision/Deny, no Allow/ProviderAttempt or
  ProviderOutcome, and no body/query echo in result or facts.
- A composed service and PostgreSQL adapter test independently counts driver
  connections, SQL statements, transaction starts, commits and rollbacks. All
  remain zero for NUL, blank, invalid UTF-8, oversized, foreign target/scope,
  removed grants, inactive claim and changed owner cases on read and write.
- Direct adapter NUL rejection also leaves every database counter at zero.
- Positive body/query/reply controls preserve newline/tab/Unicode and literal
  backslash data. Existing adapter positives exercise the same scripted driver
  through actual transaction and SQL paths.
- A backend reply containing NUL becomes a content-free Failed result; private
  reply text does not enter journal facts.

Sources: [shared tests](../../../internal/memory/text_test.go),
[composed/direct adapter tests](../../../internal/memory/postgres/text_test.go),
and [counting SQL-driver harness](../../../internal/memory/postgres/backend_test.go).
These execute the real service/adapter code with a scripted SQL driver, not a
live PostgreSQL instance. They prove no database dispatch in these paths, not
real RLS, network isolation, persistence or restart acceptance.

## Gates

Native Windows focused tests passed. The first test run correctly withheld an
invalid backend reply but a new assertion expected an internal reason instead
of the existing exported `memory-failed` reason; only that assertion was fixed.
No production outcome mapping was changed to satisfy the test.

The Linux campaign uses WSL Ubuntu 24.04 and the existing workspace-local Go
1.27.1, Node 24.12.0, PowerShell 7.6.6, GCC 13.3 and Playwright 1.63.0 tools.
No dependency lock, driver, installed database, cluster, Secret or PVC was changed.

| Gate | Command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/memory/... ./internal/facts` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts` | pass |
| Repeated text and zero-call tests | `go test -count=20 ./internal/memory/... -run 'TestNUL\|TestTextContract\|TestGovernedDenialsNeverCallDatabase\|TestAdapterNUL'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 126 frontend tests / build / 54 browser cases |

The [source campaign log](../../../work/0179-scoped-memory/review-text-linux.log)
records these gates. Installed Memory acceptance and independent review remain
outstanding; E17 is not complete and PR #212 stays draft.

Only documentation/publication edits followed the passing source campaign.
Documentation, PR-body and whitespace checks are recorded separately in the
[publication log](../../../work/0179-scoped-memory/review-text-publication-linux.log).
