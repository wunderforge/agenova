# Phase 1 Harness

The Phase 1 harness keeps the Agent Sandbox adapter spike evidence small and decision-focused.

## Scenarios

- `smoke-backend-capability-matrix`: the spike has a capability matrix that separates supported, needs-verification, not-supported, and Agenova-owned capabilities.
- `smoke-backend-neutrality`: the spike protects `RuntimeBackend` as a pluggable boundary so Agent Sandbox can be replaced without application-facing API changes.

## Run

```powershell
.\scripts\check.ps1 -All
```

## Evidence Rule

Do not mark an Agent Sandbox capability as verified until a later spike records upstream docs, installation output, or behavior-level test evidence.
