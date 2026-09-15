# Task: Freeze the supported Agent Sandbox mapping and gaps

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- Mission: Freeze the evidence-backed Agent Sandbox v0.4.6 support classification for every RuntimeBackend v0 operation and the minimum filesystem boundary, while preserving unsupported behavior as explicit gaps for downstream #49 and #51.
- Target: `docs/backends/agent-sandbox.md`, `internal/runtime/agentsandbox/doc.go`, the existing reduced-contract support-matrix test, `docs/project-status.md`, and task-local evidence under `docs/evidence/48/agent-sandbox-mapping/`.
- User value: Runtime, evidence, and kind-path contributors share one reviewed statement of what the Agent Sandbox adapter supports, what it translates, and what must still fail explicitly.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Ticket #48](https://github.com/wunderforge/agenova/issues/48)
- [Architecture contract: Backend Neutrality](../../docs/product/architecture-contract.md#backend-neutrality)
- [Add or Change a Runtime Backend playbook](../../docs/harness/playbooks.md#add-or-change-a-runtime-backend)
- [Feature specification](spec.md) and [technical design](design.md)
- [RuntimeBackend v0](../../internal/runtime/backend.go)
- [Agent Sandbox adapter note](../../docs/backends/agent-sandbox.md)
- [Provisional v0.4.6 mapping from #66](../../docs/backends/agent-sandbox-v0.4.6-mapping.md)
- [#66 retained real-cluster evidence](../../docs/evidence/E8-S1/agent-sandbox-mapping/summary.md)
- [#89 filesystem handoff](../0089-filesystem-boundary/handoff-0048-0051.md)
- [Agent Sandbox reduced-contract adapter and tests](../../internal/runtime/agentsandbox/allocation.go)

## Scope

In scope:

- Freeze `Allocate`, `Observe`, `Start`, `Terminate`, and `Cleanup` as exactly one of `native`, `translated`, `adapter-held`, or `unsupported`, with a direct source/test/real-cluster evidence link.
- Freeze backend identity, restart durability, `FilesystemBoundary`, and general isolation as supporting semantics without presenting an unverified capability as parity.
- Reconcile the formal adapter note and package documentation with merged #30, #66, #89, #124, and the retained September 14 kind evidence.
- Update the existing support-matrix test output so it reports the current evidence state without changing adapter behavior.
- Update the merged implementation-status snapshot and add reproducible focused, documentation, and repository-gate evidence.

Out of scope:

- Implementing a new Start channel, worker-stop protocol, adapter restart recovery, durable allocation store, worker filesystem layout, mount profile, credential delivery, or production adapter.
- Running #51's worker-identity filesystem, descendant-stop, cross-claim, or hostile-isolation probes.
- Changing shared API types, RuntimeBackend, FilesystemBoundary, claim lifecycle, authority, facts, gateway contracts, or the upstream v0.4.6 pin.
- Treating warm-pool replenishment as `Replaced`, deletion as termination, readiness as work start, or PodSpec configuration as isolation evidence.

## Acceptance Criteria

- The formal adapter note classifies `Allocate=translated`, `Observe=translated`, `Start=unsupported`, `Terminate=unsupported`, and `Cleanup=translated`, with backend identity translated through `Backend/WorkerID`.
- Durability/restart and `FilesystemBoundary` are explicitly unsupported. General filesystem/process/network isolation remains unknown until #51 produces real-worker evidence.
- Filesystem documentation covers the candidate worker-visible directory, writable/read-only mount layout, HOME/temp/cache placement, retention, host-path exposure, and credential exposure while making clear that the current adapter returns only `EvidenceLevel=Unsupported` and empty/zero remaining fields.
- The note distinguishes verified #66 behavior from #51-owned evidence and links every supported claim to retained source, focused tests, or real-cluster output.
- Package documentation, support-matrix test output, backend note, and project status agree; no public or shared contract changes.
- Focused adapter tests, documentation checks, and the full repository gate pass with complete captured output.

## Negative Case

- A Ready worker still leaves `Start` unsupported; claim deletion or confirmed resource absence still cannot be reported as independent `Terminate` evidence.
- An unverified directory or PodSpec mount must not populate `WorkingDirectory`, `OutsideBoundary`, `Ephemeral`, or `BackendVerified`.
- A missing/conflicting worker, query failure, incomplete deletion, or process restart must not be described as successful allocation recovery, release, replacement, or durability.

## Execution Todo

- [x] Scout the accepted RuntimeBackend, #66 mapping/evidence, #89 filesystem handoff, adapter implementation/tests, and current status note.
- [x] Record Owner approval from Frank Yang and independent Reviewer approval from `wunderforge` on Ticket #48 before implementation.
- [x] Replace the stale formal adapter summary with the frozen operation and filesystem mapping from the approved specification.
- [x] Align package documentation and the existing support-matrix test output without changing RuntimeBackend behavior.
- [x] Update `docs/project-status.md` and add task-local evidence with retained-source hashes and exact commands.
- [x] Run the focused adapter gate, `./scripts/check.ps1 -Docs`, and `./scripts/check.ps1 -All`.
- [x] Review the diff for provider leakage, unsupported parity claims, stale evidence, and changes outside #48.
- [x] Update the PR with exact evidence, request Codex and independent review, and hand the frozen matrix to #49/#51.

## Quality Gates

- `go test -count=1 -v ./internal/runtime/agentsandbox/...`
- `./scripts/check.ps1 -Docs`
- `./scripts/check.ps1 -All`

## Evidence Required

- Complete focused adapter output showing the five-operation support matrix and negative behavior.
- Complete `-Docs` and `-All` output with source commit/tree, UTC timestamp, command, and exit code.
- SHA256 values for RuntimeBackend, adapter implementation/test, formal backend note, #66 mapping, and #89 filesystem handoff.
- Links to the retained #66 kind transcript for the verified allocation/readiness/unsupported Start/Terminate/cleanup slice; no new real-backend claim from #48.

## Constraints

- Preserve [the architecture contract](../../docs/product/architecture-contract.md), especially backend neutrality and readiness/lifecycle separation.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Upstream API group strings and provider shapes remain confined to `internal/runtime/agentsandbox` or the existing task-scoped spike harness.
- If review requests executable adapter behavior, revise and reapprove this packet before implementation and add fresh real-backend evidence proportionate to the new claim.

## Decisions and Blockers

- Owner: Frank Yang (`yanyang15037755`). Independent Reviewer: `wunderforge`.
- Agent Sandbox stays a verified spike/partial backend; #48 does not rename `SpikeAdapter` or claim production readiness.
- Existing retained #66 real-cluster evidence is reused because #48 changes classification documentation and evidence wording only. #51 owns new G5 worker/isolation proof.
- Owner approval and independent Reviewer approval are recorded on Ticket #48. Implementation completed without changing the shared contract or ordinary adapter behavior.
- #134's opt-in controlled-worker protocol is #51 evidence. It does not change the ordinary adapter classifications frozen by this Ticket.
