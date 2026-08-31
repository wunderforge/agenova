# Task: Add the backend-neutral CLI composition root

- Ticket: [#40](https://github.com/wunderforge/agenova/issues/40)
- Mission: Add an `agenova` executable that hosts application services through a replaceable RuntimeBackend without importing Kubernetes or provider types into command behavior.
- Target: `cmd/agenova/`, `internal/cli/`, `internal/app/`, and the architecture-boundary scan.
- User value: A teammate can build and invoke `agenova --help` and `agenova version` locally; later `run` and `install` commands have a backend-neutral place to attach.
- PRD outcome: [MVP Deliverables](../../docs/product/prd.md#mvp-deliverables) — one runnable local golden demo exposed through CLI or an equivalent executable entrypoint, starting with the composition root rather than the full golden path.

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md)
- [Current implementation snapshot](../../docs/project-status.md)
- [Add a User-Facing Demo Slice playbook](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice)
- [RuntimeBackend contract](../../internal/runtime/backend.go)
- [In-memory reference backend](../../internal/operator/runtime.go)
- [E6-T2 follow-on](https://github.com/wunderforge/agenova/issues/41)

## Scope

In scope:

- A thin `cmd/agenova` main that wires command behavior to a composition root.
- `--help` and version output, including the hosted runtime backend name.
- Actionable non-zero exits for unknown commands, unknown flags, and invalid `--backend` configuration.
- Dependency construction for the in-memory reference backend and injected test doubles.
- Architecture-boundary evidence that command packages do not import provider/Kubernetes types.

Out of scope:

- `agenova run -f` submission (E6-T2).
- Trusted local principal boundary (E6-T3).
- `agenova install` (E7-T2).
- A production API server or a broad command suite.
- Wiring the Kubernetes Agent Sandbox adapter into the CLI.

## Acceptance Criteria

- `agenova --help` and `agenova version` exit 0 with usage or version-plus-hosted-backend text.
- Invalid command or configuration exits non-zero with actionable stderr.
- The default composition root hosts `internal/operator.Runtime`.
- Tests can inject a RuntimeBackend double without changing command code.
- Command behavior packages do not import Kubernetes/provider adapter types.

## Negative Case

- `agenova not-a-command` and `agenova --backend=kubernetes version` exit 2 with stderr that names the problem and points at `--help`.
- Authority flags such as `--repo` are rejected rather than treated as granted access.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner before implementation.
- [x] Add `internal/cli` command behavior and `internal/app` composition root.
- [x] Add the `cmd/agenova` executable and smoke/boundary tests.
- [x] Extend the architecture-boundary scan to CLI packages.
- [x] Run the focused gate and `.\scripts\check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 ./internal/cli ./internal/app ./cmd/agenova`
- `.\scripts\check.ps1 -All`

## Evidence Required

- CLI smoke output for help, version, unknown command, and invalid backend.
- Architecture-boundary scan result covering `cmd/`, `internal/cli`, and `internal/app`.
- Passing repository baseline output.
- Exact commands recorded in the PR; prose-only confirmation is not evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Possessing the CLI grants no authority.
- YAML/`run -f` is the future submission contract; do not add `--repo`, `--tools`, or `--model` authority shortcuts.
- E1-T3 (`ClaimRequest` v0) remains open; this ticket does not consume or redefine that schema.

## Decisions and Blockers

- Planning depth: Task only. Help/version/wiring is one ownership boundary with no new public product schema.
- Decision: keep command behavior in `internal/cli` and backend construction in `internal/app`. The executable glues them; tests inject a RuntimeFactory double.
- Decision: `--backend` accepts only `memory` in this root. Provider names fail in the composition root rather than importing an adapter.
- Decision: `version` constructs the hosted backend so the executable proves it hosts application services without adding a broad command suite.
- Owner authorization: the assigned Owner directed implementation of E6-T1 in this session.
- Blockers: none for this composition root. `agenova run -f` remains owned by E6-T2 and still depends on ClaimRequest v0.

## Verification Evidence

- `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova` passed.
- Built `./.tmp/agenova`: `--help` and `version` exit 0; `not-a-command` and `--backend kubernetes version` exit 2 with actionable stderr.
- `./scripts/check.ps1 -Docs` passed, including `CLI composition root stays backend-neutral`.
- `./scripts/check.ps1 -All` passed.
- Captured artifacts: `docs/evidence/40/cli-smoke/`, `docs/evidence/40/architecture-boundary/`, `docs/evidence/40/repository-baseline/`.
