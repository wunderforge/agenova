# Phase 3: Governance Runtime Harness

This harness verifies Phase 3 governance behavior on the in-memory reference backend.

## Scope

- Tool Gateway MVP: authorize by active claim, record ToolInvocation facts.
- Model Gateway MVP: authorize by active claim, record ModelInvocation facts.
- Claim lineage: parent/child registration and scope enforcement.
- Multi-agent reference scenario: parent and child claims under one fact store.
- Negative authorization: unbound, terminal, unknown, and out-of-scope claims are denied.

## Running the Reference Test

```powershell
go test -v ./harness/phase-3-governance-runtime/e2e/
```

No cluster required. The reference test runs entirely against the in-memory backend.

## Scenarios

`scenarios/smoke-multi-agent-lineage/` documents the multi-agent governance reference flow.

## Evidence

Evidence is captured under `docs/evidence/phase-3/` by `scripts/evidence.ps1`.

```powershell
.\scripts\evidence.ps1 -Phase phase-3 -Gate multi-agent-reference `
  -Command "go test -v ./harness/phase-3-governance-runtime/e2e/"
```
