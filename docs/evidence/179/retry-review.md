# E17 Uncertain-Write Retry Review Fix

Task: [#179](https://github.com/wunderforge/agenova/issues/179),
[packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Review: [P1 preserve invocation IDs on uncertain-write retry](https://github.com/wunderforge/agenova/pull/212#discussion_r4216500678).
The user authorized this bounded review correction on 2026-10-09.

## Correction

Previously every `Session.Invoke` minted a new ID, so a caller could not recover
an uncertain write through its original PostgreSQL receipt. The backend's
direct replay unit test did not prove a usable governed retry path.

An uncertain result now carries an opaque host-only `WriteRetry` continuation.
The trusted caller retains it and explicitly calls `Session.RetryWrite`; it is
not serialized into the worker reply and cannot be reconstructed from an ID.
The continuation binds the originating Session and immutable request. It
preserves the original logical write/receipt ID while each admitted retry gets
a fresh journal invocation ID and its own decision/attempt/outcome correlation.
It repeats lifecycle, issued authority, worker, deadline and ownership checks.

Copied continuations share serialized disposition. Concurrent copies cannot
dispatch another recovery after known completion. Caller/run cancellation while
waiting returns before admission, with no new invocation or backend call.
Transient failures do not erase the original uncertainty; known completion or
an evidence error disables all copies. No write is automatically retried.

Calling `Invoke` again still represents independent intent, not retrying by
body similarity. Future installed worker composition must retain/redeem these
host continuations through its trusted handler; no worker retry protocol or
receipt-ID field is introduced by this fix. Continuations cannot survive host
restart or claim rebinding. This does not restore claims or durable Work history.

## Behavioral Evidence

- Committed-but-unacknowledged and not-committed cases both resolve with the
  same receipt and immutable original body/scope/claim, producing one record.
- Every retry audit attempt is unique and correctly attributed; body/continuation
  state is absent from fact and worker JSON exports.
- Forged/decoded, foreign/rebound-session and completed continuation use has
  zero additional backend calls. Authority/worker/lifecycle/deadline/ownership
  negatives are rechecked before dispatch.
- Transient outage and repeated uncertainty retain the original receipt without
  automatic retries. Evidence loss before/after dispatch disables every copy.
- Concurrent copies resolve only once; cancelled queue waiters return without
  starting an audit invocation or another backend call.
- A composed Memory service plus real PostgreSQL adapter SQL-driver spy executes
  record/receipt insert and lost COMMIT acknowledgement, then checks that the
  governed retry queries the same receipt and performs no second INSERT/COMMIT.

Sources: [boundary tests](../../../internal/memory/retry_test.go) and
[composed SQL-driver test](../../../internal/memory/postgres/retry_test.go).
The composed test uses the real adapter code but a scripted driver. It does not
execute PostgreSQL, establish real RLS or demonstrate crash/network behavior.

## Gates

Environment: WSL Ubuntu 24.04 with the workspace-local tools documented in
[adapter/privacy evidence](adapter-privacy.md). No dependency lock, cluster,
database, Secret or PVC was changed.

| Gate | Command | Result |
| --- | --- | --- |
| Focused boundary / adapter / journal | `go test -v -count=1 ./internal/memory/... ./internal/facts` | pass |
| Race | `go test -race -count=1 ./internal/memory/... ./internal/facts` | pass |
| Repeat retry/cancellation/concurrency | `go test -count=20 ./internal/memory -run 'TestRetry\|TestWaitingRetry\|TestConcurrentRetry\|TestUncertainWriteRetry'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 126 frontend tests / build / 54 browser cases |

The initial focused/race campaign passed before a self-review added cancellable
queue waiting. The selected [final log](../../../work/0179-scoped-memory/review-retry-final-linux.log)
covers both that change and the strengthened already-waiting cancellation test;
an earlier baseline is not substituted. Only documentation/publication changes
followed this final passing source campaign.
Installed database/worker/network acceptance and independent review remain
outstanding. E17 and #212 remain incomplete/draft.
