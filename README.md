# Agenova

Agenova gives each AI agent job a clear, temporary, and auditable operating boundary.

For example, a reusable `engineer` agent role may be asked to fix one bug in one repository. Agenova records which role and runnable artifact are being used, what this run is asked to do, what tools/models/memory it may access, when that authority ends, what actually happened, and where the worker ran.

Technically, Agenova is a backend-neutral governance runtime for agent work. It turns one agent worker run into a scoped assignment:

```text
AgentTemplate + ClaimRequest -> effective authority -> governed claim -> facts -> backend evidence
```

Kubernetes, Agent Sandbox, E2B, Daytona, Docker, or another runtime may execute the process. Agenova owns the stable contract around that execution: why the run exists, what it may access, what it actually did, and which backend carried it.

The canonical application input is a declarative `ClaimRequest` in YAML or equivalent API JSON. The CLI submits that same schema with `agenova run -f <file>`; it does not define a separate flag-based authority model.

## Start Here

1. [Project design](docs/project-design.md) — the product model, flows, diagrams, and end-state vision.
2. [MVP PRD](docs/product/prd.md) — the current shared delivery target and acceptance criteria.
3. [Current status](docs/project-status.md) — what is implemented, what is only a spike, and what comes next.
4. [Contributing](CONTRIBUTING.md) — how to select, implement, verify, and review a task.
5. [AIDLC workflow](docs/development/AIDLC.md) — source-of-truth ownership and the ten-person AI-assisted delivery loop.
6. [Architecture contract](docs/product/architecture-contract.md) — stable rules that changes must preserve.

To reproduce the current reference installation and a real kind + Ollama Work from the CLI, then inspect the same record in a local React Portal, follow the [kind + Ollama guide](docs/reference-cli-kind-ollama.md). It separates substrate preparation from Agenova configuration, registration, execution and optional UI connection, and states the reference-only limits.

Feature planning is adaptive: every Ticket gets a compact Agent task packet under [`work/`](work/README.md); spec and design files are added only when ambiguity or shared contracts justify them.

## Current Baseline

The repository contains a working Go reference implementation for:

- `SandboxClaim` lifecycle semantics;
- an in-memory `RuntimeBackend` and reusable contract tests;
- claim-scoped Tool and Model Gateway authorization against exact system-issued effective authority;
- append-only in-memory runtime, tool, and model facts;
- experimental parent/child claim governance and a multi-agent reference scenario retained outside the committed MVP;
- a Kubernetes Agent Sandbox adapter spike with documented semantic gaps;
- a declarative CLI path for Platform validate/plan/apply/status, narrow Policy and AgentTemplate registration, and Work submission against an installed reference service;
- a real kind + Agent Sandbox worker and Ollama model path for the documented reference configuration.

It does **not** yet contain durable Work/evidence, production identity or controllers, Helm packaging, a Memory Interface, or OpenTelemetry integration. The CLI can query current-session Work after submission; a separately started local React Portal can use an explicit loopback connection to the same installed private API. The UI is not installed by `platform apply`, and the local tunnel is not production authentication. See [Current status](docs/project-status.md) for the exact boundary.

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

CLI composition-root smoke:

```powershell
go run ./cmd/agenova --help
go run ./cmd/agenova version
go run ./cmd/agenova run --backend memory -f harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml
```

Unknown commands, authority flags, and invalid ClaimRequest documents exit non-zero. `--backend memory` explicitly selects the in-memory reference path; the default connected `run -f` uses the installed service. The [kind + Ollama guide](docs/reference-cli-kind-ollama.md) contains the reproducible connected flow and governance checks.

## Repository Map

- `api/v1alpha1/`: current product-type sketches.
- `cmd/agenova/`: executable entrypoint that wires command behavior to the composition root.
- `internal/cli/`: backend-neutral command behavior (`--help`, version, `run -f`, usage errors).
- `internal/app/`: composition root for reference admission, authority resolution, claim issuance, and run lifecycle.
- `internal/runtime/`: backend-neutral runtime contract and adapters.
- `internal/operator/`: in-memory reference backend.
- `internal/toolgateway/`, `internal/modelgateway/`: in-process governance reference paths.
- `internal/facts/`, `internal/governance/`: fact storage and experimental claim-lineage behavior.
- `harness/`: executable reference and integration scenarios.
- `docs/`: product authorities, implementation evidence, backend notes, and delivery rules.
- `work/`: Ticket-derived Agent task packets; GitHub remains the team tracker.

## Product Boundary

Agenova is not an agent framework, prompt orchestration layer, workflow DAG engine, or replacement for sandbox/runtime providers. Agent code and its framework own reasoning and task logic. Runtime backends own process execution. Agenova owns the claim-scoped governance contract between them.

## License

Agenova is licensed under the [Apache License 2.0](LICENSE). See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for upstream attribution.
