# Agenova

Agenova gives each AI agent job a clear, temporary, and auditable operating boundary.

For example, a reusable `engineer` agent role may be asked to fix one bug in one repository. Agenova records which role and runnable artifact are being used, what this run is asked to do, what tools/models/memory it may access, when that authority ends, what actually happened, and where the worker ran.

Technically, Agenova is a backend-neutral governance runtime for agent work. It turns one agent worker run into a scoped assignment:

```text
AgentTemplate + ClaimRequest -> effective authority -> governed claim -> facts -> backend evidence
```

Kubernetes, Agent Sandbox, E2B, Daytona, Docker, or another runtime may execute the process. Agenova owns the stable contract around that execution: why the run exists, what it may access, what it actually did, and which backend carried it.

The canonical application input is a declarative `ClaimRequest` in YAML or equivalent API JSON. A future CLI will submit that same schema with `agenova run -f <file>`; it will not define a separate flag-based authority model.

## Start Here

1. [Project design](docs/project-design.md) — the product model, flows, diagrams, and end-state vision.
2. [MVP PRD](docs/product/prd.md) — the current shared delivery target and acceptance criteria.
3. [Current status](docs/project-status.md) — what is implemented, what is only a spike, and what comes next.
4. [Contributing](CONTRIBUTING.md) — how to select, implement, verify, and review a task.
5. [Architecture contract](docs/product/architecture-contract.md) — stable rules that changes must preserve.

## Current Baseline

The repository contains a working Go reference implementation for:

- `SandboxClaim` lifecycle semantics;
- an in-memory `RuntimeBackend` and reusable contract tests;
- claim-scoped Tool and Model Gateway authorization;
- append-only in-memory runtime, tool, and model facts;
- parent/child claim governance;
- a multi-agent reference scenario;
- a Kubernetes Agent Sandbox adapter spike with documented semantic gaps.

It does **not** yet contain a usable `agenova run` CLI, networked gateways, durable facts or claims, production controllers, Helm packaging, a Memory Interface, OpenTelemetry integration, or a React demo UI. See [Current status](docs/project-status.md) for the exact boundary.

## Validate

```powershell
.\scripts\check.ps1 -All
```

Focused checks:

```powershell
.\scripts\check.ps1 -Docs
.\scripts\check.ps1 -Unit
```

The Agent Sandbox integration test requires a prepared Kubernetes cluster and is not part of the default gate:

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

## Repository Map

- `api/v1alpha1/`: current product-type sketches.
- `internal/runtime/`: backend-neutral runtime contract and adapters.
- `internal/operator/`: in-memory reference backend.
- `internal/toolgateway/`, `internal/modelgateway/`: in-process governance reference paths.
- `internal/facts/`, `internal/governance/`: fact storage and claim lineage.
- `harness/`: executable reference and integration scenarios.
- `docs/`: design, PRD, status, backend notes, and harness rules.
- `tasks/`: task contract template.

## Product Boundary

Agenova is not an agent framework, prompt orchestration layer, workflow DAG engine, or replacement for sandbox/runtime providers. Agent code and its framework own reasoning and task logic. Runtime backends own process execution. Agenova owns the claim-scoped governance contract between them.

## License

Agenova is licensed under the [Apache License 2.0](LICENSE). See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for upstream attribution.
