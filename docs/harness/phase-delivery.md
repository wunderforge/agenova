# Phase 1-3 Delivery Harness

This harness controls the path from the current backend-boundary work to a Phase 3 alpha. It favors small verified slices over broad platform work.

## Roles

- Human owner: accepts phase outcomes and approves any merge to `main`.
- Codex controller: plans work, creates task packets, reviews worker output, runs evidence gates, and integrates approved changes.
- Claude Code worker: implements bounded tasks in worker branches or worktrees and reports evidence. A worker does not approve its own work.

## Branch Model

- `main` stays stable.
- `codex/phase3-delivery` is the integration branch for Phase 1-3 delivery.
- Worker branches use `worker/<phase>-<topic>` or worktrees created from the delivery branch.
- Merge worker output into the delivery branch only after Codex review and evidence capture.

## Phase Outcomes

### Phase 1: Backend Boundary

Outcome: Agenova has a small `RuntimeBackend` contract, an in-memory reference backend, backend-neutral contract tests, and an Agent Sandbox adapter spike scaffold.

Required evidence:

- `go test ./...`
- `./scripts/check.ps1 -All`
- Contract tests run against the in-memory backend.
- Capability matrix marks each item as supported, needs verification, not supported, or Agenova-owned with evidence.

### Phase 2: Deployable Runtime

Outcome: Agenova can run a real claim lifecycle against a Kubernetes-backed runtime in a local cluster.

Required evidence:

- Local cluster bootstrap output, preferably kind or minikube.
- Installed upstream Agent Sandbox path or a documented blocker.
- End-to-end test for claim create, sandbox acquire, worker run, event emission, finish, cancel, and failure.
- `kubectl` evidence for resources, pods, events, and logs.
- Backend-neutral API check proving upstream CRD shape is not application-facing.
- `./scripts/check.ps1 -Phase2Evidence`

### Phase 3: Governance Runtime

Outcome: Agenova provides the smallest useful governance layer on top of runtime execution.

Required evidence:

- Tool Gateway MVP integration tests.
- Model Gateway MVP integration tests.
- Claim-anchored authorization negative tests.
- Parent/child claim lineage test.
- `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` facts attached to claims.
- Multi-agent scenario that runs through the same API on the reference backend and the Kubernetes backend where supported.
- `./scripts/check.ps1 -Phase3Evidence`

## Design Guardrails

- Keep `SandboxClaim` as one agent worker run / sandbox execution lease.
- Keep `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` as facts under a claim.
- Keep external credentials behind gateways; do not inject them into sandbox runtime config.
- Keep application-facing APIs independent of Agent Sandbox CRD shape.
- Do not build a competing Kubernetes sandbox controller unless Phase 2 evidence proves Agent Sandbox cannot carry a required Agenova semantic.
- Prefer simple interfaces, explicit tests, and narrow adapters over framework-level abstractions.
- Upstream Agent Sandbox packages may only appear inside the dedicated adapter package `internal/runtime/agentsandbox`; application-facing and shared runtime packages must stay backend-neutral.

## Parallel Worker Protocol

- Assign each worker a non-overlapping target by package, doc area, or evidence gate.
- If two workers need the same package, sequence them instead of parallelizing.
- Integrate one worker branch at a time into `codex/phase3-delivery`.
- After each integration, run the relevant contract or evidence gate before starting the next merge.
- If a Kubernetes-backed scenario is skipped as "not supported", it must be represented by a passing `multi-agent-kubernetes-or-blocker` evidence gate with blocker details.
