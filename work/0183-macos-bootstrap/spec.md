# Feature Specification: One-command macOS local reference bootstrap

- Ticket: [#183](https://github.com/wunderforge/agenova/issues/183)
- PRD outcome: [`Demonstrable contributor path`](../../docs/product/prd.md#8-demonstrable-contributor-path) and [`Reference installation and initial policy bootstrap`](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Intent

Define the local macOS contributor experience for preparing the existing reference substrate and installing Agenova. The entry point is orchestration around existing source-of-truth commands: it makes their ordering, prerequisites, outputs, and failure boundaries reproducible without introducing a second product installation model.

## In Scope

- A repository-owned command with doctor, dry-run, setup, optional verification, and explicit cleanup behavior.
- Support for both Intel and Apple Silicon Macs through Docker/kind-selected image architecture.
- Reuse of the pinned Agent Sandbox substrate harness and existing Platform YAML and CLI services.
- Deterministic, inspectable stages and command-double tests.

## Out of Scope

- Installing host package managers or applications.
- Production installation, upgrades, rollback, high availability, or cluster administration.
- New Platform schema, hidden configuration state, or Kubernetes-specific authority flags in the Agenova CLI.
- Making Ollama or the demo Work mandatory for base installation readiness.

## Requirements

- Given all required host tools are installed and Docker/Ollama are reachable, when the operator runs setup, then the named reference cluster/substrate is prepared, both repository images are built and loaded, the existing Platform manifest is validated/planned/applied, and the reference Policy and AgentTemplate are registered.
- Given an already prepared environment, when setup is repeated, then owned resources converge without duplication and the command reports reused versus changed stages.
- Given doctor or dry-run, when the command completes, then it reports host architecture, executable/service prerequisites, target resources, and planned stages without issuing mutating Docker, kind, kubectl, or Agenova apply commands.
- Given base installation succeeds and verification is requested, when the reference Work runs, then its result is reported separately from installation readiness.

## Negative Cases

- An unsupported host or architecture is rejected before mutation.
- A missing executable or unreachable Docker/Ollama endpoint reports the exact prerequisite and remediation; Ollama may be optional when Work verification is not requested.
- An existing same-name cluster that the harness cannot safely identify as owned is not overwritten or deleted.
- Failure in an early stage prevents later Platform or registration stages from running.

## Compatibility

- `ClaimRequest`, authority resolution, runtime adapters, and evidence contracts remain unchanged.
- `deploy/reference/platform.kind.yaml` remains the reviewed Platform input, and `platform validate/plan/apply` remains the installation authority.
- Existing manual substrate and reference-demo commands remain available for debugging and expert use.

## Open Decisions

- Resolved by assignee self-review: default `setup` stops after Policy/AgentTemplate registration; `verify` explicitly opts into Ollama/model checks and the reference Work so installation readiness remains distinct from execution health.
