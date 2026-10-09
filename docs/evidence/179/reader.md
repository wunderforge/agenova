# E17 Memory Evidence Reader Slice

Task: [#179](https://github.com/wunderforge/agenova/issues/179),
[packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Source baseline: `8dd85ea98ef1eca55b575e4d6bbd28cb7df53094` plus this reader slice.
The user requested continued E17 progress on 2026-10-09.

## Scope

Enable the coordinated read-only CLI/Portal consumer path for existing Memory
decision/attempt/outcome facts. The installed service still rejects executable
Memory until host credentials, driver, Platform and worker wiring are delivered.
There is no new execution endpoint, content reveal, automatic retry, mock
persistence fallback or restored authority. The PRD and architecture contract,
shared fact/schema types, generated bindings and dependency locks are unchanged.

Both readers require the explicit public projection, trusted claim/policy
attribution, no operation/scope expansion on Allow, one ordered correlated
decision/attempt/outcome and bounded typed metadata. Deny has no attempt/outcome;
`memory.invalid` is Deny-only. Stable reason codes are accepted, but free-text
Memory reasons, unknown codes, unrelated payloads and raw body/query/credential
fields are rejected. Explicit empty model/tool observation arrays are required
for Memory-only records; null/missing histories remain invalid.

In-progress allowed calls may be incomplete. Terminal Work must close them all.
An already admitted write may report Written/WriteUncertain after cancellation
or expiry without fabricating rollback; late reads are content-free
Cancelled/Timeout. A post-terminal denial is inspectable before RunOutcome, but
no new Allow can start. Facts after final RunOutcome remain invalid.

## Behavioral Evidence

- [Shared vectors](../../../work/0179-scoped-memory/memory-reader-vectors.json)
  provide one public Running Work with write/read/denial facts and 35 corruptions
  consumed by both Go and TypeScript readers. Cases include missing projection,
  hidden input, incomplete histories, expanded operation/scope, foreign claim,
  request/policy/target, orphan/reused invocation, duplicated/reordered facts,
  missing/foreign metadata, bad statuses/counts/durations/references and raw
  body/query/credential/error payloads.
- A real Memory service and Journal run Write/Search plus a foreign-target
  denial against a fake backend. The backend sees exactly two calls. Public
  serialization contains no private sentinels and the connected CLI reader
  accepts it through a local HTTP test server. This is a service/transport/reader
  composition test, not an installed backend or database campaign.
- All result classes remain distinguishable, including Empty and WriteUncertain.
  Cancellation tests preserve known write truth, withhold late Found results,
  accept denial without a new call, and reject new post-terminal Allow.
- Cross-Work invocation/worker identity reuse and unfinished terminal calls are
  rejected. Existing generic CLI list integrity checks remain in force.
- CLI text/JSON and Portal show logical operation/scope, typed result, count,
  duration, truncation and audit ID; JSON and Portal include validated opaque
  references. Portal shows
  issued/requested Memory operations, correlated original permission and a
  fixed content-withheld label. Memory never counts as model inference or as an
  installed backend health check; denied checks do not become executing calls.

Sources: [Go reader tests](../../../internal/connectedclient/memory_test.go),
[CLI tests](../../../internal/cli/memory_test.go),
[Portal tests](../../../ui/src/memory-evidence.test.ts),
[presentation tests](../../../ui/src/record-presentation.test.ts),
[worker activity tests](../../../ui/src/WorkerActivity.test.tsx), and
[browser cases](../../../ui/smoke/memory.spec.ts).

## Gates

Environment: existing WSL Ubuntu 24.04 workspace tools (Go 1.27.1, Node 24.12.0,
PowerShell 7.6.6, GCC 13.3, Playwright 1.63.0/Chromium 153). No database, cluster,
Secret or PVC was installed or changed. Native Windows focused Go tests passed;
the Windows frontend invocation could not locate the Linux dependency executables
and was rerun using the prepared WSL toolchain.

Early focused failures were fixture composition omissions (resolution reason
code and the test client's explicit context) and nullable observation arrays.
The attempted shape-validator relaxation failed the existing empty-versus-missing
history regressions; it was removed. The fixtures now record explicit empty
arrays and the Memory Go reader applies the same requirement. Existing shape
validation and tests were not weakened. Screenshot review also corrected a
misleading execution-call label and added correlated permission in details.

| Gate | Command | Result |
| --- | --- | --- |
| Focused service/reader/privacy | `go test -v -count=1 ./internal/connectedclient ./internal/cli ./internal/evidence ./internal/memory/... ./internal/facts` | pass |
| Race | same packages with `go test -race -count=1` | pass |
| Repeated reader/lifecycle/composition | `go test -count=20 ./internal/connectedclient -run 'TestMemory'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 179 frontend tests / build / 57 browser cases |

The [final campaign log](../../../work/0179-scoped-memory/slice4-reader-linux.log)
owns final results, not an earlier focused pass. Rendered fixture screenshots
are captured by the three Memory reader browser cases at 1100px and 390px.
They are not live Memory database/installed-record evidence. Credential/driver,
installed integration, A/B/restart/C, network/data-use and independent
reproduction/review remain outstanding; E17 is incomplete and #212 stays draft.

## Rendered Proof

The final campaign's four screenshots were visually inspected for correct
Memory category/result/permission, withheld content, readable references and no
desktop/mobile overflow. They use synthetic public records, not live data.

- [Desktop Work](reader/desktop-work.png) and [record details](reader/desktop-record.png)
- [Mobile Work](reader/mobile-work.png) and [record details](reader/mobile-record.png)

Only documentation/publication edits followed the passing source campaign.
The [publication log](../../../work/0179-scoped-memory/slice4-reader-publication-linux.log)
records final documentation, PR-body and whitespace checks.
