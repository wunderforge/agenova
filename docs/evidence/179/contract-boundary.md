# E17 Contract and Governed Boundary Evidence

- Ticket: [#179](https://github.com/wunderforge/agenova/issues/179)
- Execution packet: [scoped Memory](../../../work/0179-scoped-memory/task.md)
- Planning baseline: `ebb4c04f7b8cc76d63036c820e04c62af68fad3d`
- Verification date: 2026-10-08
- Source state at verification: working-tree changes on `codex/0179-scoped-memory`, not merged or published at the time of the campaign. The draft PR publishes this verified foundation; it does not establish E17 completion.

## Proven Surface

This campaign verifies only the operation-grant contracts and an independently
tested, in-process governed boundary. It does not prove persistent storage,
installed Memory availability, authenticated hostile-worker identity, or
network isolation.

- Strict request/template/issued-state parsing accepts only explicit `read` and `write` operations. Missing operations remain non-executable, including legacy scopes.
- Authority resolution intersects request and template limits. Issuance proof, provenance and defensive copies include the new operation dimension.
- A trusted session captures issued principal/action/authority and allocated worker identity. Each call checks current Running state, target consistency, operation/scope grants and operator-owned Team/Project routing before dispatch.
- Fake-adapter counters assert zero storage calls for foreign targets/scopes/owners, unknown or non-Running state, missing grants, read-only Write, expired deadlines and malformed input. Facts remain attributed to the bound claim, never the nominated victim.
- Call deadlines respect the earlier of the run deadline, caller deadline and five seconds. Late/cancelled Search replies withhold body data. A fake adapter reporting a known commit remains Written; an uncertain acknowledgement remains WriteUncertain without an automatic retry.
- Reply validation rejects namespace misrouting, malformed references and invalid records. Encoded replies stay within 32768 bytes by omitting complete records and reporting truncation, including a record whose JSON escaping alone exceeds the cap.
- Memory facts require one decision, one allowed attempt and one correlated typed outcome. Body/query/raw-error sentinels are absent from journal serialization. Evidence failure prevents dispatch or returns a stable error after a reported commit, without a replay.
- Concurrent calls retain distinct host-issued invocation IDs and immutable correlated facts. Race checks pass for Memory, journal and application lifecycle packages.
- CLI and Portal fail closed on not-yet-supported Memory facts/metadata instead of accepting foreign annotations or rendering them as model calls. Regression cases preserve ordinary non-Memory records.

## Commands and Results

Run at the repository root with Go, Node, PowerShell, GCC and the repository's
Playwright browser prerequisites available:

```sh
go test -count=1 ./api/v1alpha1 ./internal/authority ./internal/issuance ./internal/memory ./internal/facts ./internal/app ./internal/connectedclient
go test -race -count=1 ./internal/memory ./internal/facts ./internal/app
go run ./ui/contractgen generate .
pwsh -NoProfile -File ./scripts/check.ps1 -All
git diff --check
```

All commands passed in the final campaign. The full gate includes all Go tests,
integration-package compilation, generated contract checks, 125 frontend tests,
the production build and 52 Playwright smoke cases on desktop/mobile. These
browser cases validate the existing UI, not rendered Memory records.

Environment: Windows host, WSL Ubuntu 24.04, Linux Go 1.27.1, Node 24.12.0,
PowerShell 7.6.6, GCC 13.3, Playwright 1.63.0 and Chromium 153.0.8010.12.
The repository's Go requirement and package locks were not changed. Portable
tool archives were checksum-verified; browser libraries were extracted into
ignored workspace storage. GCC/C development prerequisites were installed in
WSL after the race detector identified the missing compiler.

Captured output:

- [Contract full-gate retry](../../../work/0179-scoped-memory/slice1-contracts-linux-retry.log)
- [Boundary focused, race and full-gate retry](../../../work/0179-scoped-memory/slice1-boundary-linux-retry.log)
- [Final focused, race and full gate after reader safeguards](../../../work/0179-scoped-memory/slice1-final-linux.log)

Earlier failures were environmental and were not hidden: the unchanged Windows
operator symlink test lacked creation privilege; the first Linux browser launch
lacked three libraries; the first race run lacked a compiler. Final retries ran
the actual gates successfully without weakening their assertions.

## Remaining Acceptance

- No PostgreSQL driver, schema, migrations, forced RLS, durable receipts, pool-isolation or restart tests have been delivered yet.
- No Platform Memory configuration, host-credential integration, controlled-worker Memory action or installed live capability has been enabled. A nil adapter is Unsupported; there is no fake persistence fallback.
- The journal producer exists, but Memory-aware connected-reader validation, public request/outcome redaction and UI presentation remain a later coordinated slice. Do not wire content-bearing Memory into the installed service before that privacy slice passes.
- #155 remains open/unassigned and owns the credential resolver consumed by E17. User authorization to implement that separate producer Ticket is pending; no competing secret resolver was introduced.
- Docker/kind/kubectl and an existing explicit cluster context remain unavailable/unconfirmed. No cluster, database, PVC or Secret was created or changed.
- The A/B/restart/C campaign, independent backend-call/network/data-use oracles, rendered live Memory proof, teammate reproduction and independent review remain outstanding. E17 is not complete.
