# Task: Define and validate SandboxClaim v0

- Ticket: [#25](https://github.com/wunderforge/agenova/issues/25)
- Mission: Implement and validate the backend-neutral effective-authority, `SandboxClaim`, authorization-decision, and evidence shapes so downstream work has one immutable issued-state snapshot and a stable Allow/Deny/ApprovalRequired vocabulary.
- Target: `api/v1alpha1` (new file(s) for the v0 issued-state contract; existing runtime-spike types stay unchanged) and its focused tests.
- User value: Contributors can validate a real claim-issuance snapshot (or a pre-claim denial) against one reviewable, backend-neutral schema instead of inventing ad hoc shapes per consumer.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract — Claim Lifecycle](../../docs/product/architecture-contract.md#claim-lifecycle), [Authority and Credentials](../../docs/product/architecture-contract.md#authority-and-credentials), [Facts and Lineage](../../docs/product/architecture-contract.md#facts-and-lineage)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [E1-T1 fixture manifest and issued-state inputs](../../harness/fixtures/contract/v0/manifest.json) (`issued-state.*` cases)
- [Current shared API types](../../api/v1alpha1/types.go)
- [E1-T2 AgentTemplate v0 reference implementation](https://github.com/wunderforge/agenova/pull/93) — establishes the `ValidationError`/`ValidationCategory` pattern this ticket should reuse, not duplicate.

## Scope

In scope:

- Public Go types for `Principal` (as carried in issued state), `Action`, `PolicyReference`, `EffectiveAuthority`, `SandboxClaim` (id, requestRef, templateRef, authorityRef, phase, backendIdentity), `Decision` (id, principalRef, action, typed `result`, policyRef, reason), and `Evidence` (requestRef, claimId, decisionIds, runtimeEvents, toolInvocations, modelInvocations).
- A typed `DecisionResult` vocabulary of `Allow`, `Deny`, `ApprovalRequired`; no `allowed`-style boolean in the public contract.
- Parsing/validation that rejects caller-supplied `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` with the shared `system-managed-field` category.
- Validation that evidence can stand alone (pre-claim denial) without a fabricated `claim` or `decision.result != Deny`/`Allow` confusion.
- Focused tests consuming the six shared `issued-state.*` fixtures from `harness/fixtures/contract/v0/manifest.json` directly (no second fixture set).

Out of scope:

- `AgentTemplate` (#23) and `ClaimRequest` (#24) type definitions — this ticket only consumes their opaque string refs (`templateRef`, `requestRef`).
- Durable persistence, analytics schemas, or storage of issued state.
- Approval interruption, approval records, or resume behavior for `ApprovalRequired` (owned by #90).
- Any backend/provider-specific shape (Kubernetes, etc.) or change to the existing `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` runtime-spike types.
- Changes to the PRD or architecture contract.

## Acceptance Criteria

- Effective authority, `SandboxClaim`, decision, and evidence types parse and validate against the shared `issued-state.valid.team-a-engineer` and `issued-state.valid.team-b-denial` fixtures.
- Effective authority is a distinct field from any requested-access data; nothing in this contract lets requested access stand in for granted authority.
- Parsing rejects caller-supplied `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` (fixtures `issued-state.invalid.caller-effective-authority`, `issued-state.invalid.caller-claim-phase`, `issued-state.invalid.caller-backend-identity`) with a stable `system-managed-field` category.
- `Decision` carries principal, action, a typed `result`, policy ID/version, and reason; `result` is one of `Allow`, `Deny`, `ApprovalRequired` and is never represented as a boolean.
- `ApprovalRequired` is defined in the vocabulary but not exercised as granted authority anywhere in this ticket's validators or tests (only `Allow`/`Deny` are exercised, per fixtures).
- `Evidence` validates standalone on the Team B denial case: no `claim` field, correlated `decisionIds`, no fabricated claim ID or backend allocation.
- Tests load the six `issued-state.*` fixtures from the shared manifest directly; no duplicate fixture files are added.

## Negative Case

- Parsing `issued-state.invalid.caller-effective-authority`, `issued-state.invalid.caller-claim-phase`, and `issued-state.invalid.caller-backend-identity` must fail closed with category `system-managed-field`, not silently accept or strip the caller-supplied value.

## Execution Todo

- [ ] Scout the relevant implementation, tests, risks, and dependencies (including PR #93's `ValidationError`/`ValidationCategory` pattern).
- [ ] Confirm this packet with the Owner and Reviewer before implementation — including the open decisions below.
- [ ] Add the v0 issued-state types (effective authority, SandboxClaim, decision, evidence) with strict JSON decoding and system-managed-field rejection.
- [ ] Add focused tests consuming the six shared `issued-state.*` fixtures by case ID.
- [ ] Run the focused gate and `.\scripts\check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./api/v1alpha1 -run <FocusedTestName>` (exact name recorded once the test is written)
- `go test ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused test output naming all six shared `issued-state.*` case IDs.
- Passing `go test ./...` and repository baseline output.
- PR diff audit confirming no existing `api/v1alpha1` runtime-spike type (`SandboxClaim`, `SandboxClaimSpec`, `SandboxClaimStatus`, `SandboxWarmPool*`) was modified.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Callers cannot set claim phase, effective authority, or backend identity — this must be enforced by the parser/validator, not by convention.
- `ApprovalRequired` must never be interpreted as granted authority anywhere in code or tests.
- Reuse the shared `ValidationError`/`ValidationCategory` vocabulary from PR #93 if it has merged before this ticket implements; do not fork a second validation-error type in the same package without a recorded reason.

## Decisions and Blockers

- **Naming collision — RESOLVED: Option (a).** `api/v1alpha1` already defines a Kubernetes-facing `SandboxClaim` runtime-spike type (`Metadata`/`Spec`/`Status`, see `api/v1alpha1/types.go`) that is structurally unrelated to the backend-neutral v0 issued-state type this ticket must define (id/requestRef/templateRef/authorityRef/phase/backendIdentity). Decision: place the new type in the same package (`api/v1alpha1`) under the exported name **`SandboxClaimV0`**, pending a later rename of the legacy spike type. The legacy `SandboxClaim`/`Spec`/`Status` types are not touched by this ticket. See `spec.md` Open Decisions for full rationale. Recorded by the assignee on 2026-08-31; **still needs Owner/Reviewer confirmation on issue #25 before implementation starts** (this task packet's own execution todo requires packet confirmation, including open decisions, before coding).
- Open decision: PR #93 (E1-T2) is open, unmerged (APPROVED review, green CI, not yet merged as of 2026-08-31), and adds a `ValidationError`/`ValidationCategory` type to `api/v1alpha1` that this ticket also needs (`system-managed-field` category already defined there). Sequence numbers in the Ticket (102/103/104) imply E1-T2 and E1-T3 land before E1-T4; recommend either branching from #93 once merged or coordinating explicitly to avoid two competing `ValidationError` definitions in the same package. **Unresolved.**
- Planning depth: Task + Spec, because this ticket defines authority/decision semantics that multiple downstream consumers (#26, #30, #33, #37, #44, #59, #67) must agree on; no Design doc because the shared fixtures already fix the one bounded shape and there is no competing technical approach to choose between.
- Blockers: none on GitHub (`blocked-by #22`, merged); the PR #93 sequencing decision above should be resolved, and Owner/Reviewer should confirm the naming decision, before the Execution Todo proceeds past scouting.
