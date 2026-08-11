# Roadmap

The roadmap is phase-based. Each phase must preserve a small harness with explicit evidence checks.

Product through-line: Agenova turns reusable agent roles into scoped, auditable assignments. Runtime, gateway, facts, lineage, and backend work should strengthen claim-scoped governance rather than become isolated infrastructure features.

Cross-phase architecture constraint: Control Plane and Runtime Plane must not be specified as if they always run in the same Kubernetes cluster.

## Phase 0: Foundation Alpha

Status: implemented locally.

Purpose:

- define the minimal API vocabulary for a scoped agent assignment;
- prove claim lifecycle semantics locally;
- establish harness discipline;
- prevent concept drift before real backend integration.

## Phase 1: Runtime Backend Boundary

Status: pivoted to Agent Sandbox Adapter Spike.

Purpose:

- define a minimal runtime backend boundary;
- keep the Phase 0 in-memory runtime as a reference backend;
- evaluate Kubernetes SIG Apps Agent Sandbox as the first backend adapter;
- preserve the Phase 0 assignment lifecycle contract;
- produce behavior-level evidence before choosing any default substrate.

Out of scope until scoped explicitly: self-built competing sandbox lifecycle controllers, real gateway policy, memory, rollback, web UI, cloud control plane.

## Phase 2: Tool Gateway Boundary

Purpose:

- introduce claim-scoped tool authority for reusable agent roles;
- introduce a real Tool Gateway for controlled external tool access;
- keep external system credentials out of sandboxes;
- record `ToolInvocation` facts below a claim.

## Phase 3: Model Gateway Boundary

Purpose:

- introduce claim-scoped model access and usage accounting;
- connect facts and lineage into a multi-agent assignment reference path;
- record `ModelInvocation` facts;
- enable audit and cost attribution without taking over prompt logic.

## Later Phases

Later phases may add role registry, policy authoring, memory interfaces, state checkpoints, rollback, SPIFFE-based identity hardening, UI, richer cloud shapes, and additional backend adapters such as kagent/Substrate, E2B, Daytona, ECS/Fargate, or Firecracker. They should only be implemented after the earlier runtime and gateway contracts have behavior-level evidence.

## Contributor Expansion Gate

Broader contributor work should begin after the MVP contracts are stable enough that tests and adapters can be added without reshaping the core semantics.

This gate is not production readiness. It means:

- core vocabulary is stable: agent role, `SandboxClaim`, `ToolInvocation`, `ModelInvocation`, `RuntimeEvent`, and `RuntimeBackend`;
- claim lifecycle and terminal semantics are stable;
- gateway request, authorization, and fact contracts are stable enough for adapter work;
- reference in-memory paths and contract tests pass;
- contributor tasks can be scoped as tests, adapters, docs, or install smoke tests without redesigning core objects.

## Product Deployment Track

Agenova should support three product shapes:

- `Agenova Runtime`
- `Agenova Cloud BYOC`
- `Agenova Cloud Fully Managed`

Cloud work stays out of Phase 0. Later specs should preserve standard Agenova APIs so managed users do not need to operate clusters directly.

Deployment packaging is described in `docs/product/deployment-model.md`; implementation remains phase-scoped and evidence-backed.
