# Task: Bind governed authority to Running only

- Ticket: [#32](https://github.com/wunderforge/agenova/issues/32)
- Mission: Make Tool and Model gateway eligibility read the authoritative application lifecycle so governed interfaces are usable only while the claim is Running.
- Target: the #31 application claim reader, `internal/toolgateway/`, `internal/modelgateway/`, and focused lifecycle-phase tests.
- User value: A worker cannot exercise governed authority before application work starts or after any terminal outcome, even if backend resources still exist.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [Claim lifecycle](../../docs/product/prd.md#2-claim-lifecycle).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract](../../docs/product/architecture-contract.md), especially Claim Lifecycle and Authority and Credentials
- [Quality gates](../../docs/harness/quality-gates.md)
- [Ticket #31 packet](../0031-application-run-service/task.md) and its accepted authoritative reader
- [Compatibility ClaimReader](../../internal/runtime/claim_reader.go)
- [Tool gateway](../../internal/toolgateway/gateway.go) and [Model gateway](../../internal/modelgateway/gateway.go)

## Scope

In scope:

- Replace gateway lifecycle eligibility reads from the backend-owned compatibility reader with #31's application-owned authoritative claim reader.
- Fail closed for missing claims and every phase except Running: Pending, Bound, Succeeded, Failed, and Expired.
- Preserve the existing child-lineage rule that both child and parent claims must be Running.
- Prove terminal denial while a backend worker may still exist.

Out of scope:

- Tool/model allow-list evaluation, credential delivery, invocation transport, backend cleanup, network isolation proof, or a second lifecycle store.
- Changing public claim phases or allowing backend readiness to imply Running.

## Acceptance Criteria

- Tool and Model gateways allow lifecycle eligibility only when the authoritative application claim snapshot is Running.
- Pending and Bound claims are denied before any capability-specific checks can grant access.
- Succeeded, Failed, and Expired claims remain denied even when backend resource evidence says a worker still exists.
- Missing claims, blank claim IDs, and malformed reader results fail closed. The accepted #31 reader returns snapshot plus existence and has no separate error channel.
- Child claims remain eligible only while both child and parent authoritative snapshots are Running.

## Negative Case

- A backend worker that remains observable after Succeeded, Failed, or Expired cannot keep Tool or Model gateway authority alive.

## Execution Todo

- [x] Scout both gateways, the compatibility reader, #31 dependency, and existing lifecycle tests.
- [x] Confirm this Task + Spec with the Owner and an independent Reviewer before implementation.
- [x] Rebind both gateways to the accepted #31 authoritative reader without introducing another state model.
- [x] Add table-driven G2 evidence for every lifecycle phase, missing/malformed cases, and terminal-with-worker-exists cases.
- [x] Run the focused gates and `./scripts/check.ps1 -All`.
- [x] Review the diff for fail-closed behavior, lineage regressions, and #31 boundary compliance.

## Quality Gates

- `go test -count=1 -v ./internal/toolgateway/... ./internal/modelgateway/...`
- `go test -count=1 -race ./internal/app/... ./internal/toolgateway/... ./internal/modelgateway/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Named table rows for Pending, Bound, Running, Succeeded, Failed, Expired, missing claim, and reader failure in both gateways.
- A deterministic negative test showing a terminal authoritative claim is denied even when the backend worker still exists.
- Exact commands, exit codes, revision, and repository baseline or an explicit environment blocker.

## Constraints

- Preserve [the architecture contract](../../docs/product/architecture-contract.md); only application Running grants lifecycle eligibility.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Consume #31's accepted reader and snapshots; do not infer lifecycle from backend observation or caller-supplied fields.
- Keep capability/authority checks after lifecycle eligibility and default deny on lookup ambiguity.

## Decisions and Blockers

- Planning depth is Task + Spec because this is a small consumer migration once #31's reader is accepted.
- Delivery uses a stacked PR based on `sonia/0031-application-run-service` so #31 and #32 remain separately reviewable while work proceeds in dependency order.
- The Owner accepted the #31/#32 packets and authorized takeover implementation on 2026-09-15. #31 now exposes `app.ClaimReader` through `RunService.Claim`; #32 consumes that one boundary.
- `app.RequireRunningClaim` centralizes snapshot validation and Running-only eligibility so Tool and Model gateways cannot drift.
- The accepted reader has no error return. Unavailable reader, not found, mismatched ID, incomplete required references, incomplete Running identity, unknown phase, and every non-Running phase all fail closed.
- Existing parent/child checks remain only as an experimental compatibility regression. A test-only adapter keeps the old multi-agent harness compiling without making `runtime.ClaimReader` a production gateway dependency or restoring multi-agent work to MVP scope.
- Focused evidence passed: `go test -count=1 -v ./internal/app/... ./internal/toolgateway/... ./internal/modelgateway/... ./harness/e2e/...`. Both gateways have named cases for all six phases, missing and malformed snapshots, terminal-with-worker-exists, dependencies, and the retained experimental parent rule.
- Repository evidence passed: `./scripts/check.ps1 -All` with Go, contracts, frontend types/tests/build, and seven Playwright smoke cases. Process-local Git `safe.directory` entries were required by the Codex sandbox.
- The local race profile remains environment-blocked by `CGO_ENABLED=0`; PR CI is the authoritative race result. No local race pass is claimed.
