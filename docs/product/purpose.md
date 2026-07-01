# Product Purpose

Agenova provides an agent-native operating model for reusable agent workforces. Its core deployable product is `Agenova Runtime`.

It separates long-lived agent roles from short-lived scoped assignments. A role represents a reusable workforce capability, such as a BA agent, engineer agent, reviewer agent, or researcher agent. Each concrete assignment is represented by a `SandboxClaim` and can carry context, authority, tool/model access, runtime facts, lineage, and outcome.

It exists for teams that need to run agent workers safely near internal systems without forcing operators to stitch together compute, IAM, gateways, traces, logs, and workflow records for every use case.

## Focus

Agenova focuses on:

- scoped assignments for reusable agent roles;
- sandbox lifecycle management for agent worker runs;
- warm-pool scheduling to reduce cold-start and idle-cost pressure;
- claim-based execution leases with auditable state transitions;
- Tool Gateway and Model Gateway boundaries for credential and access control;
- append-only runtime facts that can later support audit, replay, memory, and rollback.

## Product Shapes

- `Agenova Runtime`: core runtime in customer-managed infrastructure.
- `Agenova Cloud BYOC`: Agenova-managed control plane with runtime plane in the customer's cloud account, VPC, or cluster.
- `Agenova Cloud Fully Managed`: Agenova-managed control plane and runtime plane behind standard Agenova APIs.

## Non-Goals

Agenova is not:

- an agent framework;
- a prompt orchestration layer;
- a workflow DAG engine;
- a general-purpose serverless platform;
- a replacement for Kubernetes security primitives.

Application agents still decide prompts, tools, memory usage, reasoning, and task semantics. Agenova governs the runtime substrate around that work.
