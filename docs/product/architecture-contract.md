# Architecture Contract

This file is default agent context. Keep it short. It records stable product constraints, not the full long-term design archive.

## Product Boundary

- Agenova is not an agent framework, prompt orchestration layer, or workflow DAG engine.
- Application agents own reasoning, prompt assembly, tool choice, memory strategy, and task semantics.
- Agenova owns the runtime substrate around agent work: sandbox leases, access boundaries, identity, audit facts, and later state continuity.

## Runtime Vocabulary

- `SandboxClaim` is one agent worker run / sandbox execution lease.
- `SandboxClaim` is not one tool call.
- `ToolInvocation` is a future fact/event for one concrete tool call inside a claim.
- `ModelInvocation` is a future fact/event for one concrete model call inside a claim.
- `RuntimeEvent` is a future append-only runtime fact under a claim.

## Identity and Credentials

- External system credentials must stay outside sandboxes and behind gateways.
- Sandbox identity credentials may enter sandboxes when they are needed to authenticate to Agenova components.
- Warm idle pods must not hold standing authority; authorization should be anchored to a claim, not to an unclaimed sandbox.

## Product Shapes

- `Agenova Runtime`: the core deployable runtime that can run in customer-managed infrastructure.
- `Agenova Cloud BYOC`: Agenova-managed control plane with runtime plane in the customer's cloud account, VPC, or cluster.
- `Agenova Cloud Fully Managed`: Agenova-managed control plane and runtime plane, consumed through standard Agenova APIs.

## Plane Separation

Control Plane and Runtime Plane must not be designed as if they always run in the same Kubernetes cluster.

Current phases may implement a local or single-cluster path first, but API and spec language should preserve future separation between:

- Control Plane: tenants, projects, policy, audit, usage, billing, and management APIs.
- Runtime Plane: templates, pools, sandboxes, gateways, worker execution, and runtime facts.

## Scope Discipline

Current implementation must follow the current phase spec. Do not treat long-term product shapes as permission to add cloud control plane, tenancy, billing, UI, real gateways, memory, rollback, or distributed execution before a phase explicitly scopes them.
