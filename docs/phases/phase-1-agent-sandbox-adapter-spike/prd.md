# Phase 1 PRD: Agent Sandbox Adapter Spike

## Mission

Establish a minimal backend boundary for Agenova runtime semantics and verify whether Kubernetes SIG Apps Agent Sandbox can satisfy that boundary.

## User Value

Application agents keep a stable Agenova contract while runtime operators can choose a backend substrate without exposing application code to upstream Kubernetes resource details.

## Acceptance Criteria

- Phase 0 lifecycle semantics remain the contract baseline.
- The in-memory runtime remains available as a reference backend.
- Phase 1 docs identify what Agent Sandbox supports, what needs verification, what is not supported, and what remains Agenova-owned.
- Harness checks fail if the Phase 1 spike docs or capability matrix disappear.
- No Tool Gateway, Model Gateway, memory, rollback, UI, or cloud control plane implementation is added in this phase.

## Evidence Required

- `go test ./...`
- `.\scripts\check.ps1 -All`
- Reviewed capability matrix at `docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md`

