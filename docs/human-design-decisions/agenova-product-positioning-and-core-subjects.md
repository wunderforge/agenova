# Agenova Product Positioning and Core Subjects

This document is a human-readable design archive. It explains product positioning and architecture intent, but it is not default agent context. Agents should load it only when a task explicitly asks for design rationale.

## 1. Positioning

Agenova is a Kubernetes-native runtime control plane for agent-triggered work in private infrastructure.

It sits below agent frameworks and above Kubernetes. Application agents still own reasoning, prompt assembly, tool choice, and memory strategy. Agenova governs the runtime substrate around that work: sandbox leases, access boundaries, identity, audit facts, and later state versioning.

The first focused product scenario is controlled tool execution near internal systems. That can start as customer-managed runtime infrastructure and later support managed cloud deployment shapes.

## 2. What Agenova Does Not Own

Agenova does not become the agent brain.

It does not:

- choose prompts;
- summarize context;
- decide which memory enters a prompt;
- choose tools on behalf of an agent;
- implement application-level workflows;
- run an internal LLM just to reinterpret agent intent.

Those decisions belong to application agents and their frameworks.

## 3. Core Subject Tiers

Agenova subjects fall into two tiers.

Architecture commitments are required for the product to make sense:

- `AgentSandboxTemplate`
- `SandboxWarmPool`
- `SandboxClaim`
- sandbox identity and binding facts
- Tool Gateway
- Model Gateway
- `RuntimeEvent`
- `ToolInvocation`
- `ModelInvocation`
- session-level fact grouping, with the concrete `Session` resource shape reserved for a later phase

Interface reservations are important, but can be implemented later:

- `MemoryWriter`
- `MemoryReader`
- `MemoryStore`
- `SessionState`
- `Checkpoint`
- rollback/head movement

This distinction prevents early phases from expanding into every long-term idea.

## 4. Runtime and Sandbox Management

### AgentSandboxTemplate

A template describes the worker image and runtime shape. It is a capability/workstation description, not the semantic meaning of one task.

### SandboxWarmPool

A warm pool keeps a bounded number of ready sandboxes. The goal is to reduce cold-start cost without keeping one sandbox per potential agent task.

### SandboxClaim

A claim is one agent run / sandbox execution lease. It is not one tool call.

A single claim may produce many future facts:

- `ToolInvocation`
- `ModelInvocation`
- `RuntimeEvent`

Fine-grained tool calls should not create one Kubernetes claim object each.

### Claim State

Terminal claim state should preserve business outcome:

- `Succeeded`
- `Failed`
- `Expired`

Sandbox cleanup or replacement is a resource condition/fact, not a terminal claim phase.

## 5. Access Boundaries

### Tool Gateway

The Tool Gateway mediates access from sandboxed workers to external tools and internal systems.

It owns upstream credentials and policy evaluation. External system credentials should not enter sandboxes.

`ToolInvocation` is the fact that records one concrete tool call, including policy decision and evidence metadata. It is not the same thing as a `SandboxClaim`.

### Model Gateway

The Model Gateway mediates model access and records `ModelInvocation` facts. It enables audit, cost attribution, and boundary enforcement without taking over prompt assembly.

## 6. Identity and Credentials

There are two credential classes:

- External system credentials: upstream tokens, API keys, cloud credentials. These must remain outside sandboxes and behind gateways.
- Sandbox identity credentials: short-lived credentials that prove the sandbox identity to platform components. These may enter the sandbox because they are the basis for authentication.

The intended identity structure has three layers:

1. infrastructure authentication, such as a Kubernetes projected service account token;
2. operator binding facts that map a sandbox to a claim;
3. business authorization anchored to the claim identity.

Permissions should not be attached directly to warm pods because warm pods exist before a claim and idle sandboxes should not hold standing authority.

SPIFFE can later reduce bearer-token replay risk by replacing bearer-style identity with workload identity and mTLS semantics.

## 7. Memory Interfaces

The platform provides memory interfaces, schemas, metadata, and append-only storage rules. It does not perform semantic memory extraction or decide prompt contents.

### MemoryWriter

`MemoryWriter` accepts records from agents, validates schema, attaches metadata, and writes append-only records through `MemoryStore`.

It does not decide what is worth remembering.

### MemoryReader

`MemoryReader` returns candidate memory records by session, agent, label, time, and retrieval policy.

It does not summarize, compress, rank for prompt budget, or decide what enters the prompt.

## 8. Checkpoints

Because session state should be append-only, a checkpoint can be represented as a set of high-water marks over facts and records. Rollback can move a head pointer instead of copying or mutating state.

## 9. Deployment Shapes

Agenova has three intended product shapes:

- `Agenova Runtime`: the core deployable runtime in customer-managed infrastructure.
- `Agenova Cloud BYOC`: Agenova-managed control plane with runtime plane in the customer's cloud account, VPC, or cluster.
- `Agenova Cloud Fully Managed`: Agenova-managed control plane and runtime plane behind standard Agenova APIs.

Control Plane and Runtime Plane must not be designed as if they always run in the same Kubernetes cluster. Early phases may implement a local or single-cluster path first, but specs should preserve this separation.

## 10. Phase 0 Boundary

Phase 0 implements only a local foundation alpha:

- minimal API type sketches;
- in-memory runtime lifecycle;
- warm-pool and claim behavior tests;
- static harness fixtures;
- English public documentation.

It does not implement gateways, Kubernetes reconciliation, memory, rollback, SPIFFE, Vault, DAG orchestration, or UI.
