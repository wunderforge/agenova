# AGENTS.md

This repository uses harness-driven development. Keep the implementation small, testable, and aligned with the current phase.

## Default Context

Read these first:

1. `README.md`
2. `docs/product/purpose.md`
3. `docs/product/architecture-contract.md`
4. `docs/phases/phase-0-foundation-alpha/README.md`
5. `docs/phases/phase-0-foundation-alpha/spec.md`
6. `harness/phase-0-foundation-alpha/README.md`

Do not load `docs/product/roadmap.md` or `docs/human-design-decisions/` by default. Those files are planning and architecture rationale, not the active task contract.

## Current Boundary

Phase 0 proves local lifecycle behavior and remains the semantic baseline. Phase 1 is an Agent Sandbox Adapter Spike: define a small runtime backend boundary, keep the in-memory runtime as the reference backend, and evaluate Kubernetes SIG Apps Agent Sandbox before building any native Kubernetes sandbox lifecycle controllers.

Do not add controller-runtime, real CRDs, Kubernetes controllers, real network proxies, memory systems, rollback systems, SPIFFE, Vault, DAG orchestration, cloud control plane, or web UI unless a later phase explicitly asks for them.

## Naming Rules

Use these concepts consistently:

- `SandboxClaim`: one agent worker run / sandbox execution lease.
- `Input`: phase-local run configuration, not a tool-call payload model.
- `ToolInvocation`: a future fact/event for one concrete tool call inside a claim.
- `ModelInvocation`: a future fact/event for one concrete model call inside a claim.
- `Tool Gateway`: the component that mediates tool access.
- `Model Gateway`: the component that mediates model access.

Avoid wording that implies `SandboxClaim` is one tool call.

Do not design Control Plane and Runtime Plane as if they must always run in the same Kubernetes cluster.

Do not let application-facing Agenova APIs depend on upstream Agent Sandbox CRD shape.

Do not let `RuntimeBackend` implementation details appear in application-facing Agenova APIs. Swapping backends must be invisible to application agents.

## Evidence Rules

Every phase change needs evidence:

```powershell
go test ./...
.\scripts\check.ps1 -All
```

If a change only updates docs or harness, still run `scripts/check.ps1 -All`. Record failures and do not broaden scope to make tests pass unless the current task requires it.

## Phase 1-3 Delivery Rules

Phase 1-3 work runs on a non-`main` delivery branch until human acceptance. Use `docs/harness/phase-delivery.md` for phase scope, `docs/harness/evidence-gates.md` for quality gates, and `docs/harness/claude-worker-playbook.md` when delegating execution to Claude Code.

Hard rules:

- Evidence check quality gates are mandatory. A feature is not done until the relevant gate passes or the failure is recorded as a blocker.
- Code and docs must stay clear, readable, and small enough to review.
- Prefer lightweight, boring, industry-standard designs. Do not add broad abstractions, controllers, or platforms before a phase evidence gate requires them.
- Claude workers may implement in worktrees or worker branches, but Codex remains responsible for integration review, tests, and acceptance evidence.
