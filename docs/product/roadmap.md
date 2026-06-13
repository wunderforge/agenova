# Roadmap

The roadmap is phase-based. Each phase must preserve a small harness with explicit evidence checks.

Cross-phase architecture constraint: Control Plane and Runtime Plane must not be specified as if they always run in the same Kubernetes cluster.

## Phase 0: Foundation Alpha

Status: implemented locally.

Purpose:

- define the minimal API vocabulary;
- prove warm-pool claim lifecycle locally;
- establish harness discipline;
- prevent concept drift before real Kubernetes integration.

## Phase 1: Kubernetes Runtime Plane

Purpose:

- turn the local runtime model into Kubernetes-native resources;
- add controller reconciliation for templates, pools, claims, and sandboxes;
- preserve the Phase 0 lifecycle contract;
- add behavior-level harness evidence.

Out of scope until scoped explicitly: real gateway policy, memory, rollback, web UI.

## Phase 2: Tool Gateway Boundary

Purpose:

- introduce a real Tool Gateway for controlled external tool access;
- keep external system credentials out of sandboxes;
- record `ToolInvocation` facts below a claim.

## Phase 3: Model Gateway Boundary

Purpose:

- introduce a model access boundary;
- record `ModelInvocation` facts;
- enable audit and cost attribution without taking over prompt logic.

## Later Phases

Later phases may add memory interfaces, state checkpoints, rollback, SPIFFE-based identity hardening, and UI. They should only be implemented after the earlier runtime and gateway contracts have behavior-level evidence.

## Product Deployment Track

Agenova should support three product shapes:

- `Agenova Runtime`
- `Agenova Cloud BYOC`
- `Agenova Cloud Fully Managed`

Cloud work stays out of Phase 0. Later specs should preserve standard Agenova APIs so managed users do not need to operate clusters directly.
