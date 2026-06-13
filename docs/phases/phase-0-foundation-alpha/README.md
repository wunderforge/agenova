# Phase 0: Foundation Alpha

Phase 0 establishes the smallest useful foundation for Agenova. It is intentionally local and in-memory.

## Goal

Prove that an agent worker run can acquire a warm sandbox lease, execute through a minimal lifecycle, and leave auditable terminal state without modeling each internal tool call as a Kubernetes object.

## Implemented Documents

- `prd.md`: product requirements for Phase 0.
- `spec.md`: local runtime and API specification.
- `acceptance.md`: acceptance matrix and evidence expectations.
- `progress.md`: implementation status.
- `feedback.md`: review notes and open follow-up items.

## Key Rule

`SandboxClaim` represents one agent run / sandbox execution lease. It is not a per-tool-call abstraction. Future `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` facts live under a claim.
