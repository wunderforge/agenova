# Product Purpose

Agenova provides the runtime control plane for agent-triggered work. Its core deployable product is `Agenova Runtime`.

It exists for teams that need to run agent workers safely near internal systems without giving every worker direct access to production credentials, networks, and tools.

## Focus

Agenova focuses on:

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
