# Agenova

Agenova is a Kubernetes-native runtime control plane for agent work. It is not a prompt orchestration framework, agent builder, or general serverless platform.

Agenova focuses on the infrastructure boundary below application-level agent frameworks and above Kubernetes:

- isolated sandbox execution for agent runs;
- warm sandbox pools to reduce idle resource cost;
- claim-based lifecycle management for one agent worker run;
- Tool Gateway and Model Gateway boundaries so external credentials do not enter sandboxes;
- structured runtime facts for debugging, audit, and future work evidence.

The core product shape is `Agenova Runtime`, with future `Agenova Cloud BYOC` and `Agenova Cloud Fully Managed` deployment paths kept open by separating Control Plane and Runtime Plane concerns.

## Current Phase

The repository has completed Phase 0: Foundation Alpha and is preparing Phase 1: Agent Sandbox Adapter Spike. Phase 0 remains the semantic baseline and reference implementation. Phase 1 intentionally starts with docs, harness, and backend contract evidence before introducing real Kubernetes integration.

Implemented Phase 0 scope:

- Go API types for `AgentSandboxTemplate`, `SandboxWarmPool`, `SandboxClaim`, and `SandboxClaimStatus`.
- In-memory warm-pool runtime for local validation.
- Claim lifecycle: `Pending -> Bound -> Running -> Succeeded`, `Pending -> Bound -> Running -> Failed`, `Pending -> Bound -> Failed`, and `Pending -> Expired`.
- Sandbox replacement evidence stored on claim status without overwriting terminal claim phase.
- Static harness scenarios and check script for smoke validation.

Phase 1 direction:

- define a minimal `RuntimeBackend` boundary around Phase 0 behavior;
- keep the in-memory runtime as the reference backend and contract test target;
- evaluate Kubernetes SIG Apps Agent Sandbox through an adapter;
- avoid exposing application-facing Agenova APIs to upstream Agent Sandbox CRD shape;
- avoid building competing Kubernetes sandbox controllers unless the spike proves they are required.

## Repository Map

- `api/v1alpha1/`: Phase 0 API type sketches.
- `internal/operator/`: local in-memory runtime used by tests and harness checks.
- `internal/sandbox/`: warm-pool model.
- `internal/toolgateway/`: placeholder boundary for future gateway work.
- `cmd/`: command entry points kept as placeholders until the relevant phase.
- `docs/product/`: product purpose, architecture contract, and roadmap.
- `docs/phases/phase-0-foundation-alpha/`: Phase 0 PRD, spec, acceptance, and progress.
- `docs/phases/phase-1-agent-sandbox-adapter-spike/`: Phase 1 spike scope and capability matrix.
- `harness/phase-0-foundation-alpha/`: executable evidence shape for Phase 0.
- `harness/phase-1-agent-sandbox-adapter-spike/`: Phase 1 spike evidence scaffold.
- `docs/human-design-decisions/`: human-only design archive, excluded from agent auto-context.

## Validate

```powershell
go test ./...
.\scripts\check.ps1 -All
```

On Linux or CI, install PowerShell 7+ and run:

```bash
pwsh ./scripts/check.ps1 -All
```

## Context Policy

Agent-facing context should stay small and executable. Load `AGENTS.md`, the current phase docs, and the harness for the current phase first. The human design archive is tracked for public reasoning, but agents should only read it when a task explicitly asks for product-positioning or architecture rationale.
