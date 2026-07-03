# Phase 1 PRD: Agent Sandbox Adapter Spike

## Mission

Establish a minimal backend boundary for Agenova runtime semantics and verify whether Kubernetes SIG Apps Agent Sandbox can satisfy that boundary.

## User Value

Application agents keep a stable Agenova contract while runtime operators can choose a backend substrate without exposing application code to upstream Kubernetes resource details.

## Acceptance Criteria

- Phase 0 lifecycle semantics remain the contract baseline.
- The in-memory runtime remains available as a reference backend.
- Phase 1 docs identify what Agent Sandbox supports, what needs verification, what is not supported, and what remains Agenova-owned.
- Agent Sandbox is treated as a backend substrate, not as Agenova's product API.
- MVP sandbox external access is gateway-mediated: tools go through Tool Gateway and models go through Model Gateway.
- MVP scope does not require every agent to have a dedicated warm pool.
- MVP supports both warm and cold acquisition decisions by runtime/template capacity.
- Harness checks fail if the Phase 1 spike docs or capability matrix disappear.
- No Tool Gateway, Model Gateway, memory, rollback, UI, or cloud control plane implementation is added in this phase.
- No shared runtime image plus dynamic agent artifact loading is required in this phase.
- No isolation tier implementation is required in this phase.

## Evidence Required

- `go test ./...`
- `.\scripts\check.ps1 -All`
- Reviewed capability matrix at `docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md`
- Product/runtime contract review at `docs/product/runtime-backend-mvp-contract.md`
