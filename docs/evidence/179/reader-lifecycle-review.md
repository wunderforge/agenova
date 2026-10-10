# E17 Memory Denial Lifecycle Review

Task: [#179](https://github.com/wunderforge/agenova/issues/179),
[packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Source baseline: `b002f17d7e0636012c1a7c74993340da96edaae1` plus this correction.
The user requested continued progress on 2026-10-09. Baseline CI passed, but
the [P2 review](https://github.com/wunderforge/agenova/pull/212#discussion_r4226381503)
identified acceptance of Memory denials before the claim had reached Running.

## Correction

The connected CLI and Portal now require an earlier, ordered Running fact for
every Memory decision, including Deny. This matches the unchanged
`Session.Bind` requirement: only a Running claim can establish a Memory
session. A bound worker or infrastructure readiness alone is insufficient.
Current claim phase, or a Running fact occurring after the denial, cannot
replace that prior evidence.

The terminal exception applies only to a session that previously reached
Running: a later denial remains inspectable before final RunOutcome, without
a ProviderAttempt or backend dispatch. New post-terminal Allow is still
rejected. Existing in-flight write truth, read withholding, explicit content
projection, metadata limits and cross-Work correlation are unchanged. No
execution, authority, credential, database, worker-protocol or shared schema
contract changed.

## Behavioral Evidence

New tests preceded the production correction. The Go reader wrongly accepted
all five invalid traces; the Portal wrongly accepted four (its existing Bound
requirement already rejected the earliest case). Five legitimate controls
passed in both readers. The first sandboxed native Go attempt was blocked by
cache permissions; the normal-environment rerun reproduced the behavioral
failures. Native focused tests passed after the correction.

Ten cases in the [shared vectors](../../../work/0179-scoped-memory/memory-reader-vectors.json)
are consumed by the [Go reader](../../../internal/connectedclient/memory_test.go)
and [Portal tests](../../../ui/src/memory-evidence.test.ts):

- Reject denial before Bound, after Bound, and after BackendReady without Running.
- Reject denial following startup failure without Running.
- Reject denial ordered before a later Running fact, even with current phase Running.
- Accept denial under active Running and after Failed, Cancelled, Expired or
  Succeeded when Running was already recorded.

The Go cases exercise public serialization and strict decode. The Portal cases
verify valid schema shapes and request/list transport acceptance or rejection
(502), rather than weakening existing shape checks. The original 35 corruption
vectors, late write/read lifecycle cases, post-terminal Allow rejection,
terminal invocation closure and zero-database-call regressions also pass.

Two new [browser cases](../../../ui/smoke/memory.spec.ts) prove that a pre-Running
denial produces the incomplete-record error with no Work title or worker
activity, while a valid cancellation-after-Running denial remains visible as
one blocked access check, not an active execution call. The mobile case also
checks horizontal overflow. These use synthetic public records, not an
installed Memory backend.

## Gates

Environment: the existing WSL Ubuntu 24.04 tools (Go 1.27.1, Node 24.12.0,
PowerShell 7.6.6, GCC 13.3 and Playwright 1.63.0/Chromium 153). Toolchain
requirements, dependency locks, generated bindings and product authorities
are unchanged. No database, cluster, Secret or PVC was installed or changed.

| Gate | Exact command | Result |
| --- | --- | --- |
| Focused | `go test -v -count=1 ./internal/connectedclient ./internal/cli ./internal/evidence ./internal/memory/... ./internal/facts` | pass |
| Race | same packages with `go test -race -count=1` | pass |
| Repeated reader/lifecycle | `go test -count=20 ./internal/connectedclient -run 'TestMemory'` | pass |
| Repository | `pwsh -NoProfile -File ./scripts/check.ps1 -All` | pass, all Go / 189 frontend tests / build / 59 browser cases |

The [final source campaign](../../../work/0179-scoped-memory/review-running-linux.log)
includes both new browser cases. Only documentation, generated screenshot
copying and publication edits followed the passing source run. The
[publication log](../../../work/0179-scoped-memory/review-running-publication-linux.log)
records documentation, PR-body and whitespace checks.

## Rendered Proof and Gaps

The [desktop rejection](reader-lifecycle/denial-rejected.png) and
[mobile late denial](reader-lifecycle/late-denial.png) were visually inspected
for readable errors/checks, no fabricated execution, withheld input and no
overlap or overflow. These are rendered fixture regressions, not live backend
acceptance.

#155 remains open/unassigned. Driver/credential composition, installed
Platform/worker integration, real database RLS/pool/restart/A-B-C/network/data-use
evidence, live rendered records and independent reproduction/review remain
outstanding. Keep #179 open and #212 draft. New commit CI and reviewer
acceptance are separate from the captured local passes.
