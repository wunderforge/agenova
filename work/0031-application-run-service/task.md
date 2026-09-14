# Task: Compose the application run service

- Ticket: [#31](https://github.com/wunderforge/agenova/issues/31)
- Mission: Compose the accepted admission/issuance path with one backend-neutral runtime lifecycle so every run has one authoritative application outcome and separate resource evidence.
- Target: `internal/app/` run-service composition, a narrow authoritative claim-state reader, focused app/runtime tests, and deterministic lifecycle harness evidence.
- User value: One authorized request can proceed from a system-issued Pending claim through allocation and work start to a reproducible terminal result without treating backend readiness or cleanup as the work outcome.
- PRD outcome: [Claim lifecycle](../../docs/product/prd.md#2-claim-lifecycle) and [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md) and [technical design](design.md)
- [Architecture contract](../../docs/product/architecture-contract.md), especially Submission and Resolution, Backend Neutrality, Claim Lifecycle, and Authority and Credentials
- [Start a GitHub Ticket and Change a Core Contract](../../docs/harness/playbooks.md)
- [Quality gates](../../docs/harness/quality-gates.md)
- [RuntimeBackend contract](../../internal/runtime/backend.go) and [ClaimReader compatibility bridge](../../internal/runtime/claim_reader.go)
- [Reference runtime](../../internal/operator/runtime.go), allocation implementation, and focused tests in that package
- [Application composition root](../../internal/app/runtime.go) and reference admission path
- [Ticket #30 handoff](../0030-runtime-backend-mvp/handoff-0031.md)
- [Ticket #29](https://github.com/wunderforge/agenova/issues/29), its accepted `internal/issuance.Issue` entry point, and merged task packet
- [Ticket #32](https://github.com/wunderforge/agenova/issues/32), the consumer of the authoritative lifecycle view

## Scope

In scope:

- Compose the accepted admission/resolution/issuance result with `RuntimeBackend` allocation, observation, explicit start, application work completion, termination, and cleanup in one application-owned path.
- Own and validate the public claim phase transitions `Pending -> Bound -> Running -> Succeeded|Failed`, plus `Expired` from the documented non-terminal phases.
- Publish backend identity before Bound, publish Running only after `RuntimeBackend.Start` succeeds, and publish the terminal work outcome before termination and cleanup.
- Record deterministic lifecycle and resource evidence for success, allocation failure, start failure, timeout, termination failure, and cleanup failure.
- Expose a narrow, backend-neutral read view over authoritative application claim state for Ticket #32 without changing gateway eligibility in this Ticket.

Out of scope:

- Production reconciliation, durable storage, distributed transactions, workflow scheduling, a new public claim schema, gateway Running-only integration (#32), or provider-specific runtime behavior.
- Claim issuance semantics owned by #29, policy/effective-authority recomputation, network isolation, and real worker task delivery.
- Claiming cancellation of an in-flight backend call when the five-operation `RuntimeBackend` has no context parameter.

## Acceptance Criteria

- A valid issued Pending claim follows one deterministic trace through Allocate, Bound, readiness observation, Start, Running, work completion, terminal publication, Terminate, and Cleanup; success returns Succeeded plus correlated backend cleanup evidence.
- Allocation failure produces Failed without fabricating backend identity, Bound, Running, termination, or cleanup success.
- Start failure after binding produces Failed, never Running, and still attempts termination and cleanup using the known identity.
- A run that reaches its application deadline from Pending, Bound, or Running publishes Expired; a late or still-existing worker cannot rewrite that terminal outcome.
- Termination or cleanup failure is retained as resource evidence and never rewrites Succeeded, Failed, or Expired.
- Invalid and late phase transitions are rejected, and the authoritative reader returns defensive snapshots that cannot be caller-mutated.
- Existing `RuntimeBackend` contract tests and backend-neutral source-boundary checks remain unchanged and pass.

## Negative Case

- Backend readiness alone cannot publish Running or Succeeded.
- A Start error cannot publish Running.
- Cleanup success or failure cannot select or replace the application work outcome.
- Once terminal, a late start/work/backend result cannot move the claim to another phase.
- Unknown or mismatched backend identity cannot be attached to another claim.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies; read #30's handoff and confirm #29 is merged and closed.
- [ ] Confirm this Task + Spec + Design with the Owner and an independent Reviewer before implementation.
- [ ] Slice 1: add the application-owned lifecycle store/state machine and focused valid/invalid transition tests, with no backend call composition yet.
- [ ] Slice 2: compose issued Pending state with Allocate/Observe/Start and deterministic success, allocation-failure, and start-failure traces.
- [ ] Slice 3: add deadline handling, terminal-before-teardown ordering, and termination/cleanup-failure evidence without outcome rewriting.
- [ ] Slice 4: expose the narrow authoritative read boundary and document the #32 handoff without changing gateway authorization.
- [ ] Add focused deterministic lifecycle evidence and update task-local decisions/blockers.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, source-of-truth changes, and #29/#32 boundary compliance.

## Quality Gates

- `go test -count=1 -v ./internal/app/...`
- `go test -count=1 -race ./internal/app/... ./internal/operator/... ./internal/runtime/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Exact commands, exit codes, revision, and deterministic ordered traces for success, allocation failure, start failure, timeout in each permitted non-terminal phase, termination failure, and cleanup failure.
- Assertions that backend readiness never publishes Running, terminal outcome precedes teardown, cleanup failure preserves the outcome, and invalid/late transitions fail closed.
- A short #32 handoff naming the authoritative reader and the phases it exposes; it is interface evidence, not proof that gateways enforce it.
- The repository baseline and PR race profile, or an explicit environment blocker that does not claim a pass.

## Constraints

- Preserve [the architecture contract](../../docs/product/architecture-contract.md); application state, not backend observation, owns claim phase and outcome.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Consume #29's accepted complete Allow-form `IssuedState`; do not fork, reconstruct, or silently redefine issuance.
- Keep provider types and lifecycle interpretations out of `api/v1alpha1` and outside runtime adapters.
- Serialize the reference in-memory backend as required by #30's handoff; do not claim it has a general concurrency contract.
- Do not modify gateway eligibility or claim-bound invocation context until #32.

## Decisions and Blockers

- Planning depth is Task + Spec + Design because the Ticket crosses application/runtime boundaries and must settle state ownership, timeout limits, teardown ordering, and compatibility before implementation.
- The user authorized starting #31 and #32 on 2026-09-14. Work proceeds in dependency order; #32 remains paused until #31's authoritative lifecycle view is accepted.
- #29 is merged through PR #129 and closed. #31 will consume the accepted pure `internal/issuance.Issue` entry point and its validated Allow-form `IssuedState` without duplicating issuance.
- Proposed lifecycle-contract decision requiring Owner approval: add `Pending -> Failed` for allocation failure only. Allocation failure has no backend identity, so publishing `Bound` would fabricate a binding; leaving the claim non-terminal would hide the application failure. The transition must carry allocation-failure evidence and must not call identity-dependent teardown. The architecture contract will be updated in the implementation slice only after this decision is approved.
- `RuntimeBackend` calls have no context parameter. The MVP deadline can govern readiness polling and results observed between calls, but this Ticket must not claim preemption of a backend call already in progress.
- Packet approval and an independent Reviewer are still required before implementation by the repository AIDLC.
