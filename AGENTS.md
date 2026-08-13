# Agenova Agent Contract

Keep changes small, backend-neutral, and supported by mechanically checkable evidence.

## Read First

1. `README.md`
2. `docs/product/prd.md`
3. `docs/product/architecture-contract.md`
4. `docs/project-status.md`
5. Task-relevant code and harness files only

Use `docs/project-design.md` when product or architecture context is needed. Use `docs/harness/` for workflow, gates, and known traps.

## Stable Rules

- `SandboxClaim` represents one agent worker run / scoped assignment, not one tool call.
- `ClaimRequest` is the canonical application input; YAML, API JSON, and CLI `-f` must share one backend-neutral schema.
- Requested access is intent, not authority. Resolve it against template and policy limits before creating the system-managed claim.
- Claim-scoped governance is the product; Kubernetes is one possible execution substrate.
- Application-facing APIs must not expose backend CRDs, SDK types, or provider status shapes.
- Upstream Agent Sandbox knowledge stays inside `internal/runtime/agentsandbox`.
- External system credentials stay behind gateways; do not place long-lived upstream credentials in sandbox configuration.
- `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` are facts under a claim.
- Parent/child claims express governance scope, not workflow scheduling.
- Do not add controllers, CRDs, memory, UI, cloud control plane, or new platforms unless the active task and PRD require them.

## Standard Loop

1. Read the task contract and inspect the affected boundary.
2. State observable acceptance criteria and the strongest relevant gate.
3. Implement the smallest change that satisfies them.
4. Run focused tests, then `./scripts/check.ps1 -All` before completion.
5. If a gate fails, stop expansion, classify the failure, fix the smallest responsible issue, and rerun.
6. Record reusable failures in `docs/harness/learnings.md`; promote repeated failures into a gotcha or mechanical check.

## Evidence

A task is done only when its acceptance criteria have reproducible evidence. A prose report is not evidence by itself.

- Core behavior: focused unit/contract test plus `./scripts/check.ps1 -All`.
- Backend behavior: adapter test and real backend evidence, or an explicit blocker.
- User-facing flow: executable E2E/smoke output; add rendered evidence when UI exists.
- Docs/harness only: `./scripts/check.ps1 -Docs` and link/path review.

Use `tasks/task-template.md` for new work and `docs/harness/quality-gates.md` for gate selection.
