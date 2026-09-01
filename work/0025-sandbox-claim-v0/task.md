# Task: Define and validate SandboxClaim v0

- Ticket: [#25](https://github.com/wunderforge/agenova/issues/25)
- Mission: Implement and validate the backend-neutral effective-authority, `SandboxClaim`, authorization-decision, and evidence shapes so downstream work has one immutable issued-state snapshot and a stable Allow/Deny/ApprovalRequired vocabulary.
- Target: `api/v1alpha1` (new file(s) for the v0 issued-state contract) and its focused tests. **Owner review on #100 rejects the "leave the legacy runtime-spike types unchanged" premise** — see Decisions and Blockers below; the legacy `SandboxClaim`/`Spec`/`Status` prototype must be moved/renamed either as part of this ticket or a recorded prerequisite ticket, pending an explicit scope call.
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
- [E1-T2 AgentTemplate v0 reference implementation](https://github.com/wunderforge/agenova/pull/93) (merged 2026-08-31) — `ValidationError`/`ValidationCategory` (including `ValidationCategorySystemManagedField`) now live in `api/v1alpha1/agent_template.go`; import and reuse directly, do not duplicate.

## Scope

In scope:

- Public Go types for `Principal` (as carried in issued state), `Action`, `PolicyReference`, `EffectiveAuthority`, `SandboxClaim` (id, requestRef, templateRef, authorityRef, phase, backendIdentity — canonical public name; see Decisions and Blockers on the legacy-type migration this now requires), `Decision` (id, principalRef, action, typed `result`, policyRef, reason), and `Evidence` (requestRef, claimId, decisionIds, runtimeEvents, toolInvocations, modelInvocations).
- A typed `DecisionResult` vocabulary of `Allow`, `Deny`, `ApprovalRequired`; no `allowed`-style boolean in the public contract.
- Two distinct parsing entrypoints driven by trusted-caller input, not a payload field: a caller-origin path that rejects caller-supplied `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` with the shared `system-managed-field` category, and a system-issued path that accepts them. See spec.md Requirements.
- Cross-object invariant validation across one snapshot (request/principal/action/policy/authority/claim/decision/evidence reference correlation); `Allow` may carry claim + authority, `Deny`/`ApprovalRequired` must not.
- Lifecycle validation that a `Pending` claim is valid without `backendIdentity` (system allocates it only after binding).
- Explicit evidence-view DTOs for `runtimeEvents`/`toolInvocations`/`modelInvocations` distinct from the existing `RuntimeEvent`/`ToolInvocation`/`ModelInvocation` fact structs (fixture items are a lighter view, e.g. `{"kind": "ClaimRunning"}` with no `ClaimID`/timestamp).
- Validation that evidence can stand alone (pre-claim denial) without a fabricated `claim` or `decision.result != Deny`/`Allow` confusion.
- Focused tests consuming the five shared `issued-state.*` fixtures from `harness/fixtures/contract/v0/manifest.json` directly (no second fixture set).

Out of scope:

- `AgentTemplate` (#23) and `ClaimRequest` (#24) type definitions — this ticket only consumes their opaque string refs (`templateRef`, `requestRef`).
- Durable persistence, analytics schemas, or storage of issued state.
- Approval interruption, approval records, or resume behavior for `ApprovalRequired` (owned by #90).
- Any backend/provider-specific shape (Kubernetes, etc.).
- Changes to the PRD or architecture contract.

**Superseded by owner review on #100:** the previous line here ("no change to the existing `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` runtime-spike types") is no longer accurate — the owner determined that legacy type is an incomplete reference-runtime prototype, not a compatibility authority, and must be moved/renamed to an internal boundary as part of establishing the canonical public `SandboxClaim`. Whether that migration happens inside this ticket or a recorded prerequisite ticket is an open decision — see Decisions and Blockers.

## Acceptance Criteria

- Effective authority, `SandboxClaim`, decision, and evidence types parse and validate against the shared `issued-state.valid.team-a-engineer` and `issued-state.valid.team-b-denial` fixtures via the system-issued parsing path.
- Effective authority is a distinct field from any requested-access data; nothing in this contract lets requested access stand in for granted authority.
- The caller-origin parsing path rejects caller-supplied `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` (fixtures `issued-state.invalid.caller-effective-authority`, `issued-state.invalid.caller-claim-phase`, `issued-state.invalid.caller-backend-identity`) with a stable `system-managed-field` category; origin is supplied by the calling code, never read from the payload itself.
- `Decision` carries principal, action, a typed `result`, policy ID/version, and reason; `result` is one of `Allow`, `Deny`, `ApprovalRequired` and is never represented as a boolean.
- `ApprovalRequired` is defined in the vocabulary but not exercised as granted authority anywhere in this ticket's validators or tests (only `Allow`/`Deny` are exercised, per fixtures).
- Cross-object invariants hold on a parsed snapshot: `claim`/`evidence` request refs match, `claim.authorityRef` correlates to `effectiveAuthority.id`, `decision` principal/action/policy refs correlate, `evidence.decisionIds` correlate to real decisions, and `evidence.claimId` is absent unless `decision.result == Allow`. `Decision.result == Allow` requires `claim` and `effectiveAuthority` to be present; `Deny`/`ApprovalRequired` require both absent.
- A `Pending`-phase claim validates without `backendIdentity`; `backendIdentity` is only valid once allocated (Bound phase onward).
- `Evidence` validates standalone on the Team B denial case: no `claim` field, correlated `decisionIds`, no fabricated claim ID or backend allocation.
- Tests load the five `issued-state.*` fixtures from the shared manifest directly; no duplicate fixture files are added.

## Negative Case

- Parsing `issued-state.invalid.caller-effective-authority`, `issued-state.invalid.caller-claim-phase`, and `issued-state.invalid.caller-backend-identity` through the caller-origin path must fail closed with category `system-managed-field`, not silently accept or strip the caller-supplied value.
- A payload-embedded `"source"`-style field must never be trusted to establish origin; origin must come from the calling code path (separate function/entrypoint per origin).

## Execution Todo

- [ ] Scout the relevant implementation, tests, risks, and dependencies (including the merged PR #93 `ValidationError`/`ValidationCategory` pattern in `api/v1alpha1/agent_template.go`).
- [ ] Resolve the legacy-`SandboxClaim` migration scope decision (in-ticket vs. recorded prerequisite ticket) with the Owner.
- [ ] Confirm this packet with the Owner and Reviewer before implementation — including the open decisions below.
- [ ] Add the v0 issued-state types (effective authority, SandboxClaim, decision, evidence) with strict JSON decoding, the caller/system-issued parsing split, cross-object invariants, and system-managed-field rejection.
- [ ] Add focused tests consuming the five shared `issued-state.*` fixtures by case ID, covering both parsing paths and the invariant/lifecycle rules.
- [ ] Run the focused gate and `.\scripts\check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./api/v1alpha1 -run <FocusedTestName>` (exact name recorded once the test is written)
- `go test ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused test output naming all five shared `issued-state.*` case IDs.
- Passing `go test ./...` and repository baseline output.
- PR diff audit confirming the legacy `SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus`/`SandboxWarmPool*` prototype was migrated (renamed/moved to an internal boundary, consumers updated) rather than left duplicated alongside the new canonical `SandboxClaim`, per the resolved migration-scope decision below.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Callers cannot set claim phase, effective authority, or backend identity — this must be enforced by the parser/validator via a trusted-caller-supplied origin, never a payload field, not by convention.
- `ApprovalRequired` must never be interpreted as granted authority anywhere in code or tests.
- Reuse the shared `ValidationError`/`ValidationCategory` vocabulary from the merged PR #93 (`api/v1alpha1/agent_template.go`); do not fork a second validation-error type in the same package.
- Do not land two competing public claim types in `api/v1alpha1` — the legacy prototype must be renamed/moved, not left standing beside the new canonical `SandboxClaim`.

## Decisions and Blockers

- **Naming collision — CHANGES REQUESTED by Owner review on PR #100 (2026-08-31).** The Option (a) approach (`SandboxClaimV0` alongside the untouched legacy type) is **rejected**. Owner's ruling: the existing `api/v1alpha1.SandboxClaim` is an incomplete reference-runtime prototype, not a Kubernetes/provider type and not a compatibility authority — the canonical public name `SandboxClaim` is reserved for this ticket's backend-neutral issued-state type per the architecture contract and PRD. Corrected direction: keep the public name `SandboxClaim`; move/rename the legacy prototype into an internal runtime boundary and migrate its consumers. **Open decision needing an explicit call: does that legacy-type migration happen inside this ticket, or does it get split into a recorded prerequisite ticket?** Owner explicitly allows either, but forbids landing two competing public claim types. See `spec.md` Open Decisions.
  - **Blast-radius finding (assignee, 2026-09-01):** the legacy type is wired into the `RuntimeBackend` interface (`internal/runtime/backend.go`) and threaded through `internal/operator/runtime.go`, `internal/runtime/agentsandbox/adapter.go`, `internal/runtime/contracttest/run.go`, `internal/sandbox/pool.go`, `internal/modelgateway/gateway.go`, `internal/toolgateway/gateway.go`, `internal/cli/cli_test.go`, and `harness/` e2e/integration tests — roughly 6 production files + 7-8 test files across 6 packages, including one exported interface signature. **Assignee recommends splitting into a prerequisite ticket**: the rename is mechanical/compiler-checked (low risk), but bundling it here would make the PR 3-5x larger than the actual contract work and mix an interface refactor into a contract-definition ticket. Posted to Owner on PR #100 for the final call.
- **Trusted parsing boundary, cross-object invariants, and lifecycle/evidence-view semantics — new design requirements from the same review**, now specified in `spec.md` Requirements/Compatibility. Not blocking a decision, but must be implemented as scoped there (not left to ad hoc implementation choices).
- ~~Open decision: PR #93 sequencing~~ — **Resolved.** PR #93 merged 2026-08-31T22:46 UTC; `ValidationError`/`ValidationCategory` live in `api/v1alpha1/agent_template.go`. This ticket branches from current `main` and reuses that vocabulary directly.
- Planning depth: Task + Spec, because this ticket defines authority/decision semantics that multiple downstream consumers (#26, #30, #33, #37, #44, #59, #67) must agree on; no Design doc because the shared fixtures already fix the one bounded shape and there is no competing technical approach to choose between.
- Blockers: none on GitHub (`blocked-by #22`, merged). Before implementation proceeds past scouting: (1) Owner/Reviewer must confirm the legacy-type migration scope (in-ticket vs. prerequisite ticket), (2) the packet (this file + spec.md) must be re-reviewed and approved on PR #100 with `Closes #25` added to the PR body.
