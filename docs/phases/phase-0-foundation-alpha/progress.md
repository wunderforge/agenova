# Phase 0 Progress

## Status

Phase 0 is implemented as a local foundation alpha.

## Completed

- API type sketches for templates, pools, claims, phases, and conditions.
- In-memory warm-pool runtime.
- Claim lifecycle tests for success, failure, expiry, and entrypoint validation.
- Harness scenarios for warm-pool claim behavior and static gateway boundary evidence.
- Check script that runs tests and validates Phase 0 fixtures.
- Public-facing English documentation and context rules.

## Validated Behaviors

- A pool creates the requested number of idle sandboxes.
- A claim binds to an idle sandbox and records `SandboxID`.
- A claim can move from `Pending` to `Bound` to `Running` to `Succeeded`.
- A failed claim preserves `Failed` as the terminal phase.
- An expired claim preserves `Expired` as the terminal phase.
- Sandbox recycle/replacement evidence is recorded as a condition, not as claim phase.

## Next Phase Candidate

Phase 1 should introduce Kubernetes-native reconciliation while preserving the Phase 0 lifecycle contract.
