# Architecture Contract

This file is default agent context. Keep it short. It records stable product constraints, not the full long-term design archive.

## Product Boundary

- Agenova models reusable agent workforces: long-lived agent roles are invoked through short-lived scoped assignments.
- Agenova is not an agent framework, prompt orchestration layer, or Phase 1 workflow DAG engine.
- Claim graph semantics must still preserve a future path for optional multi-agent orchestration at the claim level.
- Application agents own reasoning, prompt assembly, tool choice, memory strategy, and task semantics.
- Agenova owns the runtime substrate around agent work: sandbox leases, access boundaries, identity, audit facts, and later state continuity.

## Runtime Vocabulary

- A reusable agent role is long-lived; a `SandboxClaim` is one scoped invocation or assignment of that role.
- `SandboxClaim` is one agent worker run / sandbox execution lease.
- `SandboxClaim` is not one tool call.
- `ToolInvocation` is a future fact/event for one concrete tool call inside a claim.
- `ModelInvocation` is a future fact/event for one concrete model call inside a claim.
- `RuntimeEvent` is a future append-only runtime fact under a claim.

## Identity and Credentials

- External system credentials must stay outside sandboxes and behind gateways.
- Sandbox identity credentials may enter sandboxes when they are needed to authenticate to Agenova components.
- Warm idle pods must not hold standing authority; authorization should be anchored to a claim, not to an unclaimed sandbox.

## Runtime Isolation

- Ordinary Pod isolation is not a hard boundary for mutually hostile agents.
- Sandbox isolation requirements must remain explicit in runtime design; future runtime specs may map them to Kubernetes placement, runtime classes, dedicated node pools, or stronger backends.
- Gateway authorization must not rely only on network location or sandbox self-reporting.

## Runtime Backend Abstraction

`RuntimeBackend` is the isolation boundary between Agenova's stable runtime contract and any concrete sandbox substrate.

```text
Application Agent / Framework
  -> Agenova stable runtime contract
  -> RuntimeBackend interface
     +-- InMemoryBackend reference implementation
     +-- AgentSandboxAdapter
     +-- future backend adapters
  -> selected sandbox substrate
```

- Application-facing Agenova APIs must not change when the selected backend changes.
- The in-memory runtime is the reference backend and contract test oracle.
- Agent Sandbox is the first backend adapter to evaluate, not a hard dependency of the product contract.
- If Agent Sandbox cannot carry required Agenova semantics, another backend adapter or a native backend may satisfy the same `RuntimeBackend` interface.
- Agent Sandbox and Kubernetes readiness are substrate evidence; Agenova owns claim execution phases and runtime facts.

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
