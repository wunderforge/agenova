# Task: Package one minimal engineer Agent Artifact

- Ticket: [#53](https://github.com/wunderforge/agenova/issues/53)
- Mission: Build a reproducibly buildable engineer worker binary that reads canonical task input through a thin mock tool client, so the contract can be taught and demonstrated without a live gateway or Kubernetes backend.
- Target: `examples/engineer/` (new package: main binary, tool client interface, mock implementation, and focused tests)
- User value: A teammate can run the canonical Team A engineer scenario locally against fixture inputs and observe governed behavior — tool access behind an explicit interface, no live credentials — before E4-T2 and E6-T2 are available.
- PRD outcome: [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path), [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md)
- [Add a User-Facing Demo Slice playbook](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice)
- [E1-T1 contract fixtures](../../harness/fixtures/contract/v0/manifest.json) — canonical task inputs consumed by this artifact
- [E9-T2 reviewer child assignment](https://github.com/wunderforge/agenova/issues/54) — future/non-blocking (P2 Future Backlog per #109); not an MVP dependency of this artifact

## Scope

In scope:

- `examples/engineer/main.go` — CLI entry point: reads a ClaimRequest YAML or JSON file path from args, parses task input, runs the mock tool sequence, and exits cleanly.
- `examples/engineer/toolclient/client.go` — a minimal Go interface (`ToolClient`) with one `Invoke(tool, scope string) error` method; the interface is the only boundary between the worker and tool access.
- `examples/engineer/toolclient/mock.go` — a mock implementation that records invocations and returns configurable results; labeled clearly as fixture/mock, not live enforcement.
- Focused tests in `examples/engineer/` covering two fixture-driven runs (YAML and JSON canonical inputs) and one invalid-input exit.
- A `FIXTURE_MODE=true` build label or startup log line that makes the mock boundary explicit at runtime.

Out of scope:

- Live Tool Gateway integration (owned by E4-T2).
- `agenova run -f` CLI submission (owned by E6-T2).
- Kubernetes execution or any real backend allocation.
- A general coding-agent framework, reasoning loop, or prompt engineering.
- Changes to shared API types in `api/v1alpha1/`, the PRD, or the architecture contract.
- E9-T2 reviewer child assignment (separate ticket; this artifact must not pre-implement it).

## Acceptance Criteria

- `go build ./examples/engineer/` produces a reproducible binary with a fixed entrypoint; the build command and resulting binary identity are recorded in the PR.
- The binary accepts a `--task` flag pointing to a ClaimRequest YAML or JSON file and runs to completion using the mock tool client.
- Two distinct fixture-driven runs are demonstrated using `claim-request.valid.team-a-engineer-yaml` and `claim-request.valid.team-a-engineer-json` from E1-T1 without modifying or rebuilding the binary between runs.
- The `ToolClient` interface is the only code path through which the worker accesses tools; no direct provider SDK imports exist in the binary.
- The binary receives no long-lived external secret values; the mock client uses no real credentials.
- Mock execution is visibly labeled (e.g., startup log: `[FIXTURE MODE] tool calls are mocked — not live Agenova governance`) so the boundary is unambiguous.
- Supplying a malformed or schema-invalid ClaimRequest file exits non-zero with an actionable error message naming the file and the validation failure; it does not panic.
- Focused tests pass: two valid fixture runs and one invalid-input rejection are machine-verifiable.

## Negative Case

- Supplying an invalid ClaimRequest input (e.g., `claim-request.invalid.missing-task.json` from E1-T1) must cause the binary to exit non-zero with a human-readable error identifying the missing field; the mock tool sequence must not execute.
- Focused tests must fail if the `ToolClient` interface is bypassed by a direct tool call, if the mock label is absent from output, or if the binary panics on invalid input instead of returning a structured error.

## Execution Todo

- [ ] Scout the existing `examples/` layout (if any), the E1-T1 fixture paths, and the `api/v1alpha1` types that the ClaimRequest parser may reference.
- [x] Confirm this packet with the Owner and record independent review as a PR gate before implementation. (Approved in [#53 comments](https://github.com/wunderforge/agenova/issues/53); compliance recorded in [issue comment](https://github.com/wunderforge/agenova/issues/53#issuecomment-5660330963).)
- [ ] Add `examples/engineer/toolclient/client.go` with the `ToolClient` interface and `examples/engineer/toolclient/mock.go` with the labeled mock.
- [ ] Add `examples/engineer/main.go` with arg parsing, YAML/JSON ClaimRequest loading, mock tool invocation sequence, and invalid-input exit handling.
- [ ] Add focused tests: two valid fixture-driven runs and one invalid-input exit check.
- [ ] Run the focused gate and `.\scripts\check.ps1 -All`.
- [ ] Review the diff for scope, regressions, provider imports, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./examples/engineer/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Reproducible build output: exact `go build` command and binary identity (size, checksum, or `go version -m` output).
- Two fixture-driven run outputs: one from the YAML canonical input and one from the JSON canonical input, each showing the mock label and tool invocations.
- One invalid-input run output: non-zero exit code and the actionable error message.
- Passing focused test output naming the `examples/engineer` package.
- Passing repository baseline output.
- Prose-only confirmation is not sufficient; exact command output is required.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- The `ToolClient` interface must remain the sole tool-access boundary; no direct provider or gateway SDK calls in the binary.
- The binary must not hold, log, or pass through any long-lived external secret values.
- Fixture/mock execution must be explicitly labeled so no consumer mistakes this for live Agenova governance enforcement.
- E1-T1 fixture files are read-only inputs; do not copy, fork, or modify them.
- This artifact does not implement or simulate E9-T2 reviewer child assignment behavior.

## Decisions and Blockers

- Planning depth: Task only. The mock `ToolClient` interface is internal to this artifact and will be replaced by the real E4-T2 gateway; no shared cross-component contract is being defined here. Task-only planning approved by the Owner in [#53 comments](https://github.com/wunderforge/agenova/issues/53).
- Decision (Owner scope clarification on #53): #22 provides the same canonical Team A engineer request in YAML and JSON, not two different business payloads. The acceptance criterion is satisfied by two canonical input representations consumed without rebuilding; no second shared fixture may be invented or modified.
- Decision (per #109): #54 (E9-T2 reviewer child assignment) is reclassified to P2 Future Backlog and is not a downstream MVP dependency. Its reference in this packet is future/non-blocking; no reviewer-child behavior may be added to this implementation.
- Decision: Task input is read from a file path supplied via `--task` flag, matching the `agenova run -f` intent without depending on the E6-T2 CLI implementation.
- Decision: the two fixture-driven runs use the YAML and JSON canonical forms of the same Team A engineer request (case IDs `claim-request.valid.team-a-engineer-yaml` and `claim-request.valid.team-a-engineer-json`), demonstrating format-agnostic input without requiring a second distinct scenario.
- Integration note: downstream integration with E1-T2 (typed AgentTemplate), E4-T2 (real Tool Gateway), and E6-T2 (CLI submission) is deferred; this ticket closes at fixture/mock stage and records these as explicit integration blockers for later work.
- Decision (2026-09-14, dependency check after pulling `b060842..10b0fbe`): no scope or acceptance change. The E1-T1 valid fixtures now carry an optional `projectRef: payments` field (`ClaimRequest` schema gained optional `projectRef`, merged with #129). The change is additive: manifest case IDs are unchanged and parsing through the shared `api/v1alpha1` loader is unaffected; run-output evidence will show the field if the artifact echoes the request. The E9 epic rename per #110 ("Example Agent and Bounded Adversarial Cases") confirms this artifact's single-engineer, no-reviewer-child scope. Recorded in [#53 comment](https://github.com/wunderforge/agenova/issues/53#issuecomment-5661333509).
- Blockers: none. Hard dependency #22 (E1-T1) is closed; E1-T2, E4-T2, and E6-T2 are not required for fixture/mock stage.
