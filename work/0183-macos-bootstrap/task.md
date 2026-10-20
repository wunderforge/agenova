# Task: One-command macOS local reference bootstrap

- Ticket: [#183](https://github.com/wunderforge/agenova/issues/183)
- Mission: Provide one safe macOS command that prepares the existing local reference substrate and installs Agenova without hiding or replacing the declarative Platform path.
- Target: `harness/local/`, `scripts/`, `README.md`, `docs/reference-cli-kind-ollama.md`, and focused harness tests
- User value: A macOS contributor no longer has to manually build/load multiple images and remember the ordering of kind, Agent Sandbox, Platform, registration, and verification steps.
- PRD outcome: [`Demonstrable contributor path`](../../docs/product/prd.md#8-demonstrable-contributor-path) and [`Reference installation and initial policy bootstrap`](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md)
- [`docs/product/architecture-contract.md`](../../docs/product/architecture-contract.md)
- [`docs/reference-cli-kind-ollama.md`](../../docs/reference-cli-kind-ollama.md)
- [`harness/spike/agent-sandbox-substrate/RUNBOOK.md`](../../harness/spike/agent-sandbox-substrate/RUNBOOK.md)
- [`harness/spike/agent-sandbox-substrate/reproduce.sh`](../../harness/spike/agent-sandbox-substrate/reproduce.sh)
- [`deploy/reference/platform.kind.yaml`](../../deploy/reference/platform.kind.yaml)
- [`deploy/reference/Dockerfile`](../../deploy/reference/Dockerfile)
- [`harness/integration/agentsandbox/testworker/Dockerfile`](../../harness/integration/agentsandbox/testworker/Dockerfile)
- [`scripts/evidence.ps1`](../../scripts/evidence.ps1)

## Scope

In scope:

- One macOS-oriented bootstrap entry point with `doctor`, dry-run, setup, and cleanup-safe behavior.
- Automatic orchestration of the existing pinned substrate, required image builds/loads, CLI build, declarative Platform apply, and reference registration.
- Optional reference Work execution after installation.
- Command-double coverage for sequencing, fail-fast behavior, non-mutating modes, and repeat runs.
- macOS Intel/Apple Silicon documentation and repair of directly encountered PowerShell portability defects.

Out of scope:

- Production cluster lifecycle, package-manager installation, cloud bootstrap, Helm, multi-cluster support, or a second Platform/install contract.
- Changes to claim, authority, gateway, or backend-neutral runtime contracts.
- Treating the local reference bootstrap as production deployment support.

## Acceptance Criteria

- On a supported macOS host, one documented command prepares the named kind cluster and pinned Agent Sandbox substrate, builds and loads architecture-correct worker/control-plane images, applies the existing Platform YAML, and registers the reference Policy and AgentTemplate.
- Repeating the command is safe and does not duplicate owned resources.
- `doctor` and dry-run report prerequisites and planned stages without mutating Docker or Kubernetes.
- Failures identify the blocked stage and a concrete recovery action.
- The README leads macOS contributors through the one-command path while retaining a manual expert path.

## Negative Case

- Missing/unreachable Docker or a missing required executable fails before partial Agenova application; doctor/dry-run tests prove no mutating Docker, kind, or kubectl calls occur.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Self-review this packet against the Ticket and PRD before implementation.
- [x] Add command-double tests that define prerequisite checks, stage ordering, dry-run non-mutation, and rerun behavior.
- [x] Implement the thin macOS bootstrap orchestration over existing substrate and Platform entry points.
- [x] Repair the `powershell`/`pwsh` evidence portability defect and add focused coverage.
- [x] Update README and the kind + Ollama reference guide with macOS commands, created resources, troubleshooting, and cleanup.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All` (repository gate reached one unrelated frontend smoke blocker recorded below).
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `bash harness/local/test-macos-bootstrap.sh`
- `pwsh ./scripts/check.ps1 -Docs`
- `pwsh ./scripts/check.ps1 -All`

## Evidence Required

- Command-double output covering stage order, fail-fast behavior, dry-run non-mutation, and repeat-run behavior.
- macOS output for doctor, first setup, identical rerun, Platform status, and one optional reference Work; record an explicit blocker when Docker/kind/Ollama access is unavailable.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- The bootstrap must call the existing pinned substrate harness and declarative `platform validate/plan/apply` path; it must not become a second installer or silently grant authority.
- Do not automatically install or start Docker Desktop, Homebrew, Ollama, or other host-level packages.
- Destructive cleanup must remain explicit and limited to named resources owned by the local reference environment.

## Decisions and Blockers

- Self-review found no PRD or architecture conflict. Default `setup` stops after Platform, Policy, and AgentTemplate readiness; `verify` is the explicit real-Ollama Work path.
- Command-double focused gate passes. Real macOS/arm64 `doctor`, first `setup`, and identical rerun pass; the rerun reports Platform `changes: 0`, `changed: false`, and both registrations as `already registered`.
- Real `verify` stops before setup mutation because `llama3.1:latest` is absent and prints the exact remediation `ollama pull llama3.1:latest`; this is the explicit external-model blocker rather than a false installation failure.
- Full repository gate passes documentation, Go, contracts, types, components, and production build; its Playwright phase is blocked at 51/52 by the unchanged `console keyboard, focus, semantic structure and source announcements` test under unsupported local Node 25.9.0 (the repo requires Node 24). No UI files are changed by this Ticket.
