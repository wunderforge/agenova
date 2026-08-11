# Runtime Backend Positioning

Agenova should not be positioned as Kubernetes for agents.

Kubernetes Agent Sandbox, kagent/Substrate, E2B, Daytona, ECS/Fargate, Firecracker, Docker, and future substrates can all provide pieces of agent execution: process isolation, sandbox lifecycle, warm workers, snapshots, filesystems, network controls, and runtime placement.

Agenova's product center is not any one substrate. It is the claim-scoped governance contract above them.

## Core Position

```text
one agent run
  -> one claim
  -> scoped authority
  -> tool/model/runtime facts
  -> backend evidence
  -> queryable accountability
```

The claim is stable. The execution backend is replaceable.

## What Backends Provide

| Capability | Example providers | Agenova stance |
|---|---|---|
| Process isolation | Kubernetes Agent Sandbox, gVisor, Firecracker, E2B, Daytona, ECS/Fargate | Consume through `RuntimeBackend`. |
| Sandbox lifecycle | Kubernetes controllers, kagent/Substrate, hosted sandbox APIs | Adapt; do not expose upstream shape to application APIs. |
| Warm workers and checkpointing | kagent/Substrate, provider-specific snapshot APIs | Treat as backend capability and runtime evidence. |
| Filesystem and network controls | Kubernetes NetworkPolicy, sandbox provider policy, cloud VPC controls | Use as enforcement layers below Agenova policy. |
| Runtime identity | ServiceAccount, workload identity, cloud task role | Bind to claims, do not make it the product API. |

## What Agenova Provides

- reusable agent role to scoped claim semantics;
- claim-anchored authorization decisions;
- Tool Gateway and Model Gateway access boundaries;
- `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` facts below a claim;
- parent/child claim governance for multi-agent work;
- backend-neutral `RuntimeBackend` contract tests;
- backend evidence that proves where and how a claim ran.

## Enforcement Layers

Agenova policy is not enough by itself.

For a real tool request:

```text
agent sandbox -> Tool Gateway -> external system
```

- Tool Policy decides what this claim is allowed to request.
- Tool Gateway enforces the policy and records invocation facts.
- NetworkPolicy or provider network controls prevent direct bypass around the gateway.
- Kubernetes RBAC, service accounts, or cloud IAM bind runtime identity and platform permissions.

## Product Rule

If a capability can be provided by an execution substrate, Agenova should not rebuild it by default. Agenova should adapt it, record evidence from it, and keep the application-facing contract stable.
