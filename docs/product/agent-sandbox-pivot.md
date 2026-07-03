# Agent Sandbox Pivot

Phase 1 pivots from "build Agenova Kubernetes sandbox controllers by default" to "define a stable runtime backend boundary and spike Kubernetes SIG Apps Agent Sandbox as the first backend substrate."

This is not a rewrite. Phase 0 remains the semantic baseline and reference implementation.

## Preserved Agenova Semantics

- `SandboxClaim` is one agent worker run / sandbox execution lease.
- `SandboxClaim` is not one tool call.
- `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` are facts under a claim.
- Warm-pool replacement is resource evidence, not a terminal claim phase.
- External system credentials stay behind gateways and do not enter sandboxes.
- Application-facing Agenova APIs must not depend on upstream Agent Sandbox CRD shape.

## New Phase 1 Direction

```text
Application Agent / Framework
  -> Agenova stable runtime contract
  -> RuntimeBackend interface
     +-- InMemoryBackend reference implementation
     +-- AgentSandboxAdapter
     |   -> Kubernetes SIG Apps Agent Sandbox
     |   -> Kubernetes / runtimeClass / node pools / stronger isolation substrate
     +-- alternative backend adapter
         -> another sandbox substrate or native implementation
```

The `RuntimeBackend` interface is the replacement boundary. Agent Sandbox is the first backend to evaluate, not the only possible backend.

## What Agenova Still Owns

- Stable claim lifecycle semantics.
- Claim-anchored authorization.
- Tool Gateway and future `ToolInvocation` facts.
- Model Gateway and future `ModelInvocation` facts.
- Runtime facts below a claim.
- Parent/child claim governance.
- Future memory and checkpoint interfaces.
- Control Plane / Runtime Plane separation.

## Phase 1 Spike Rule

Do not build competing Kubernetes Template/WarmPool/Claim/Sandbox controllers unless the spike shows Agent Sandbox cannot carry Agenova's required semantics.

The MVP runtime contract is recorded in `docs/product/runtime-backend-mvp-contract.md`. That document is the detailed source for template layering, warm pool granularity, gateway-mediated network posture, runner lifecycle, and Agent Sandbox status mapping.

Phase 1 should first prove:

- whether upstream acquisition semantics can represent an Agenova claim lease;
- whether lifecycle status can map to `Pending`, `Bound`, `Running`, `Succeeded`, `Failed`, and `Expired`;
- whether sandbox cleanup or replacement can be recorded as resource evidence without changing terminal claim outcome;
- whether runtimeClass, placement, storage, and cleanup knobs exist and are usable;
- whether upstream API churn can be isolated behind an Agenova adapter.
