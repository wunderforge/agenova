# Task: Reduce RuntimeBackend to the MVP contract

- Ticket: [#30](https://github.com/wunderforge/agenova/issues/30)
- Mission: Separate the backend operations required by one golden run from claim authority and warm-pool administration, so the application run service can consume a small backend-neutral contract.
- Target: internal/runtime/backend.go and contracttest; internal/operator reference backend; necessary agentsandbox, app, CLI, gateway and test consumers.
- User value: One reference backend passes reusable lifecycle-operation cases without forcing application callers to manage pools or infer work success from infrastructure readiness.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Specification](spec.md) and [proposed design](design.md)
- [Architecture contract](../../docs/product/architecture-contract.md), Runtime Boundary, Claim Lifecycle, Authority and Credentials
- [Core contract and runtime backend playbooks](../../docs/harness/playbooks.md)
- [Quality gates](../../docs/harness/quality-gates.md)
- [Backend contract](../../internal/runtime/backend.go) and [reusable tests](../../internal/runtime/contracttest/run.go)
- [Reference implementation](../../internal/operator/runtime.go) and runtime_test.go in that directory
- internal/runtime/agentsandbox/adapter.go, adapter_unit_test.go; [backend evidence and gaps](../../docs/backends/agent-sandbox.md)
- internal/app/runtime.go, internal/cli/cli.go, both internal/*gateway/gateway.go consumers and their tests; harness/e2e/multi_agent_reference_test.go
- [Issued-state producer Ticket #25](https://github.com/wunderforge/agenova/issues/25) and [run-service consumer Ticket #31](https://github.com/wunderforge/agenova/issues/31)

## Scope

In scope:

- Define only allocation/binding, observation/readiness, backend identity, explicit work-start boundary, termination and cleanup evidence on the common backend boundary.
- Adapt the in-memory backend and reusable contract tests to that boundary; keep backend setup and pool behavior implementation-specific.
- Migrate existing consumers sufficiently to compile and preserve their accepted behavior. Separate existing gateway claim lookup from the infrastructure interface without implementing Ticket #31's new run service.
- Adapt the existing Agent Sandbox spike to the reduced boundary where supported; document unsupported semantics and real-environment blockers explicitly.
- Preserve canonical issued-state types and separate work outcome from resource cleanup observations.

Out of scope:

- Ticket #29 request issuance, Ticket #31 run-service composition, Ticket #32 new Running-only integration coverage, Ticket #33 Invoke/fact changes.
- Warm-pool management features, placement, checkpointing, snapshots, HA, production reconciliation or durable claim storage.
- New public claim/authority schemas, provider details in shared API, or promotion of the Kubernetes spike to a supported production backend.

## Acceptance Criteria

- The shared interface contains only the backend operation categories in the Ticket; application callers do not need pool registration/status to perform them.
- The in-memory implementation passes reusable contract cases for identity, allocation/binding, readiness, explicit start, termination and cleanup, including failure cases.
- Readiness is infrastructure evidence for Bound and never independently proves work start or Succeeded; only the explicit work-start boundary can acknowledge work start.
- Cleanup/replacement observations remain distinct from the application work outcome; cleanup failure is visible and does not overwrite that outcome.
- Provider-specific objects remain in the adapter boundary. Every old interface consumer is migrated or has a documented, tested internal compatibility bridge.
- Unsupported semantics are explicit. Focused evidence, the repository gate and the existing provider boundary scan pass; real-backend claims have real output or an explicit blocker.

## Negative Case

- Unknown backend identity, unavailable/not-ready allocation, and start before readiness cannot report successful work start.
- Readiness alone cannot make a claim Succeeded or grant gateway authority.
- Termination/cleanup failure is distinguishable from successful cleanup; replacement does not create a new application work outcome.
- The same external identity cannot cause evidence to be silently attributed to another claim.
- Existing unknown/non-Running/invalid-parent gateway denial behavior survives the compatibility migration.

## Execution Todo

- [x] Record human approval of this Task + Spec + Design in Ticket #30 and recheck the merged baseline and producer types: approved by the Owner at commit 016989e ([Ticket #30 comment](https://github.com/wunderforge/agenova/issues/30#issuecomment-5610181744)); implementation baseline is 016989e merged with main b060842 (Ticket #26 PolicyBundle), which left the #30 packet and the Ticket #25 producer types unchanged.
- [ ] Record the independent Reviewer for this Ticket in Ticket #30 (not yet named in the approval comment).
- [x] Slice 1: implement the approved reduced interface and in-memory support, reusable behavioral cases, and the minimum compiling consumer/adapter migration. Keep the existing Authorize behavior. Run focused contract and consumer gates, then the repository gate before expanding.
- [x] Slice 2: complete fault-path cases, audit adapter mappings/unsupported operations, and preserve relocated pool-specific regression tests. Capture real backend output for changed provider claims or record the exact environment blocker.
- [x] Slice 3 technical work: review the full diff and validation evidence, correct the reproduced cleanup-binding issue, and document the operation/type surface in the [#31 handoff](handoff-0031.md). See [review.md](review.md).
- [ ] Slice 3 acceptance: obtain independent human review and final acceptance before merge; record the decision in Ticket #30 / PR #106.

## Quality Gates

- go test -count=1 -v ./internal/operator/... ./internal/runtime/...
- go test -count=1 -v ./internal/app/... ./internal/cli/... ./internal/toolgateway/... ./internal/modelgateway/... ./harness/e2e/...
- pwsh -File scripts/check.ps1 -All (includes the existing provider boundary check)
- CI PR profile additionally runs the race detector. For provider behavior changes: pwsh -File scripts/check.ps1 -Integration -KubeContext <explicitly-confirmed-test-context>; record a blocker if unavailable and do not represent unit doubles as cluster evidence.

## Evidence Required

- Exact revision/working-file manifest, commands, exit codes and raw output; current planning-only status is not G2 evidence.
- An old-to-new method/consumer map showing what leaves the shared boundary and where compatibility remains.
- Positive and negative reusable backend cases plus existing consumer regressions; pool-only tests moved rather than silently deleted.
- Explicit supported/translated/unsupported adapter behavior and current real-backend evidence or blocker.
- A short downstream interface handoff with accepted types, operation semantics and remaining Ticket #31 responsibilities.

## Constraints

- Preserve the PRD and architecture; resolve a discovered conflict through human review, not an expanded implementation.
- Keep the existing E4 draft, stashes and unrelated documentation work intact; use a separate implementation checkout after approval.
- Do not fork the canonical public SandboxClaim, DecisionResult or authority snapshot.
- Do not implement the request-to-run chain merely to make a backend contract test pass.
- Local proposal links must be converted to repository-relative links and checked before a planning PR is published.

## Decisions and Blockers

- Automated review of PR #106 found that the PowerShell entry point supplied a default context, bypassing the Go harness's explicit-context guard. The wrapper now rejects missing, empty and whitespace-only contexts for both `-Integration` and `-Profile Backend` before running any checks. Nine child-process regression cases exercise the actual wrapper with isolated check modules, including explicit argument forwarding and baseline use without a cluster. They run in the documentation/repository gates. See [repository evidence](../../docs/evidence/30/repository-baseline/summary.md); this proves entry-point behavior, not real-cluster execution.

- Slice 3 review found that normal adapter Cleanup also needed the binding check already used by observation/recovery. The fix rejects changed, missing or unreadable bindings before deletion, retains identity on failure, and is covered by four regression variants. The [handoff](handoff-0031.md) records exact types, operation semantics, compatibility, serial reference usage and the responsibilities retained by #31. Build, both focused groups, full race and all 12 repository checks passed after the fix. Technical review/handoff are complete; human acceptance remains pending.

- Slice 2 local evidence (2026-09-10): twelve shared contract cases now include resource-start, termination, replacement, and implicit-termination failures with retries. Build, both focused groups, full race, the repository baseline, and integration-package compilation passed. The six legacy reference pool cases are unchanged. See [contract evidence](../../docs/evidence/30/reference-contract/summary.md), [repository evidence](../../docs/evidence/30/repository-baseline/summary.md), and the [real-backend blocker](../../docs/evidence/30/agent-sandbox/summary.md). The no-context integration guard rejects execution before contacting a cluster; this guard is not real-backend evidence.
- Slice 1 local evidence (2026-09-10): `go build ./...`, both focused commands above, `go test -count=1 -race ./...`, and `pwsh -File scripts/check.ps1 -All` all exited 0. The three recovery-identity review regressions also passed unchanged with the race detector. Six pool-specific reference regressions remain in operator tests; gateway authorization behavior is unchanged. This completes the local implementation slice, not final Ticket acceptance or real-backend verification.
- Adapter recovery checks and reserves the worker identity in both normal binding and every compensation/retry path. Conflicts retain their upstream claim without deletion. Unknown workers also retain the claim so a later retry can learn its binding; a claim that disappeared before its worker was observed remains an explicit recovery blocker. Failed cleanup keeps the reservation without exposing it as a usable allocation. Formal tests cover the three conflict entries, late binding, changed binding, failed-delete retries, and concurrent binding during recovery.
- Planning approved at commit 016989e; the approval keeps backend operations separate from application lifecycle/outcome, preserves provider neutrality, requires unsupported Agent Sandbox start semantics to be reported honestly, and treats existing parent-claim gateway tests as compatibility regressions only. Created with the canonical generator using Task + Spec + Design because shared semantics, backend mapping and consumer compatibility need one concrete decision.
- Inspected main: 3365cd0e37d181146dd5b6f8e65a58e03b6fb39e; remote main matched at inspection. Ticket #25 is closed, so the declared upstream Ticket dependency is satisfied; this is not implementation approval.
- Proposed approach is specified in design.md. Approval must cover the backend/application split and compatibility strategy; exact implementation names may vary without changing the accepted semantics.
- RuntimeBackend.Claim is not retained as the future source of governance authority merely to avoid a consumer migration. Ticket #30 preserves the current internal claim view through a narrow compatibility dependency; Ticket #31 owns the authoritative run-service view and publication semantics.
- Ticket #29 remains separately owned; it is needed by Ticket #31, not a newly invented dependency for Ticket #30.
- Decision: this Ticket shrinks only the shared `runtime.RuntimeBackend` interface. `operator.Runtime` keeps its current phase state machine (`AddClaim`, `BindClaim`, `StartClaim`, `SucceedClaim`, `FailClaim`, `ExpireClaim`, `Claim`) as concrete methods and additionally implements the reduced interface; no method is deleted from the reference runtime here. Moving phase ownership into a run service is Ticket #31.
- Adapter audit: both `Start` and legacy `StartClaim` now report unsupported work-start semantics. Readiness stays Bound evidence, and the integration gate no longer asserts Running/Succeeded or replacement from readiness/deletion. Legacy outcome helpers remain source compatibility only, with their evidence limits documented in the backend note.
- Real-cluster evidence: the current local environment has no `kubectl` executable and no explicitly confirmed test context. See the [recorded environment blocker](../../docs/evidence/30/agent-sandbox/summary.md). The PR #99 / Ticket #66 owners may be able to run the updated integration gate on their kind substrate; they are also natural adapter-side Reviewer candidates, subject to team confirmation.
- No claim that there is no concurrent unpublished work: confirm assignment before implementation. Do not mark any backend environment or test gate passed until executed.
