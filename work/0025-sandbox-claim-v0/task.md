# Task: Define and validate SandboxClaim v0

- Ticket: [#25](https://github.com/wunderforge/agenova/issues/25)
- Mission: Implement and validate the backend-neutral effective-authority, `SandboxClaim`, authorization-decision, and evidence shapes so downstream work has one immutable issued-state snapshot and a stable Allow/Deny/ApprovalRequired vocabulary.
- Target: `api/v1alpha1` (new file(s) for the v0 issued-state contract) and its focused tests, **plus migrating the legacy `SandboxClaim`/`Spec`/`Status` runtime-spike type to `internal/runtime.BackendClaim` and updating its consumers** — owner decision, confirmed 2026-09-01, no prerequisite ticket. See Decisions and Blockers.
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

- Public Go types for `Principal` (as carried in issued state), `Action`, `PolicyReference`, `EffectiveAuthority`, `SandboxClaim` (id, requestRef, templateRef, authorityRef, phase, backendIdentity — canonical public name), `Decision` (id, principalRef, action, typed `result`, policyRef, reason), and `Evidence` (requestRef, claimId, decisionIds, runtimeEvents, toolInvocations, modelInvocations).
- Migrating the legacy `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` prototype to `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus` (same package as the `RuntimeBackend` interface) and updating every consumer: the `RuntimeBackend` interface, `internal/operator/runtime.go`, `internal/runtime/agentsandbox/adapter.go` (+ unit test), `internal/runtime/contracttest/run.go`, `internal/sandbox/pool.go`, both gateways (+ tests), `internal/cli/cli_test.go`, and the `harness/` e2e/integration claim-lifecycle tests. `ClaimPhase`/`ObjectMeta` stay in `api/v1alpha1` (shared, not renamed); `SandboxWarmPool*` is unaffected.
- A typed `DecisionResult` vocabulary of `Allow`, `Deny`, `ApprovalRequired`; no `allowed`-style boolean in the public contract.
- Two distinct parsing entrypoints driven by trusted-caller input, not a payload field: a caller-origin path that rejects caller-supplied `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` with the shared `system-managed-field` category, and a system-issued path that accepts them. See spec.md Requirements.
- Cross-object invariant validation across one snapshot (request/principal/action/policy/authority/claim/decision/evidence reference correlation); `Allow` may carry claim + authority, `Deny`/`ApprovalRequired` must not.
- Lifecycle validation that a `Pending` claim is valid without `backendIdentity` (system allocates it only after binding).
- Explicit `EvidenceRuntimeEvent{Kind string}` DTO distinct from the existing `RuntimeEvent` fact struct (fixture items are a lighter view, e.g. `{"kind": "ClaimRunning"}` with no `ClaimID`/timestamp). Minimal placeholder DTOs for `toolInvocations`/`modelInvocations` only — their detailed item schemas are owned by #33/#37, not this ticket.
- Validation that evidence can stand alone (pre-claim denial) without a fabricated `claim` or `decision.result != Deny`/`Allow` confusion.
- Focused tests consuming the five shared `issued-state.*` fixtures from `harness/fixtures/contract/v0/manifest.json` directly (no second fixture set).

Out of scope:

- `AgentTemplate` (#23) and `ClaimRequest` (#24) type definitions — this ticket only consumes their opaque string refs (`templateRef`, `requestRef`).
- Durable persistence, analytics schemas, or storage of issued state.
- Approval interruption, approval records, or resume behavior for `ApprovalRequired` (owned by #90).
- Any backend/provider-specific shape (Kubernetes, etc.).
- Changes to the PRD or architecture contract.
- Detailed `toolInvocations`/`modelInvocations` evidence-item schemas (owned by #33/#37).
- Behavioral changes to the runtime backend itself — the `internal/runtime.BackendClaim` migration is a rename/move of the existing prototype's identifiers and consumers, not a change to its logic.

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
- [ ] Rebase `codex/e1-t4-sandboxclaim-v0` onto current `main`; update PR #100 body to add `Closes #25`.
- [ ] Migrate the legacy `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` to `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus` and update every consumer (`RuntimeBackend` interface, `internal/operator/runtime.go`, `agentsandbox` adapter, `internal/runtime/contracttest/run.go`, `internal/sandbox/pool.go`, both gateways, `internal/cli/cli_test.go`, `harness/` e2e/integration tests); confirm `go build ./...` and `go test ./...` still pass with the rename alone before adding new types.
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
- PR diff audit confirming the legacy `SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` identifiers no longer exist in `api/v1alpha1` (moved to `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus`, every consumer updated) rather than left duplicated alongside the new canonical `SandboxClaim`. `SandboxWarmPool*` untouched.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Callers cannot set claim phase, effective authority, or backend identity — this must be enforced by the parser/validator via a trusted-caller-supplied origin, never a payload field, not by convention.
- `ApprovalRequired` must never be interpreted as granted authority anywhere in code or tests.
- Reuse the shared `ValidationError`/`ValidationCategory` vocabulary from the merged PR #93 (`api/v1alpha1/agent_template.go`); do not fork a second validation-error type in the same package.
- Do not land two competing public claim types in `api/v1alpha1` — the legacy prototype must be renamed/moved, not left standing beside the new canonical `SandboxClaim`.

## Decisions and Blockers

- **Naming collision — RESOLVED (owner, confirmed via assignee 2026-09-01).** Canonical public name stays `SandboxClaim`. The legacy `api/v1alpha1.SandboxClaim`/`Spec`/`Status` prototype migrates to `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus` **inside this ticket** — no prerequisite ticket. Blast-radius (assignee finding, still accurate, now scoped as ticket work rather than a split decision): the `RuntimeBackend` interface (`internal/runtime/backend.go`) plus `internal/operator/runtime.go`, `internal/runtime/agentsandbox/adapter.go` (+ test), `internal/runtime/contracttest/run.go`, `internal/sandbox/pool.go`, both gateways (+ tests), `internal/cli/cli_test.go`, and `harness/` e2e/integration tests — ~6 production files + 7-8 test files across 6 packages. `ClaimPhase`/`ObjectMeta` do not move; `SandboxWarmPool*` is unaffected. See `spec.md` Compatibility.
- **Trusted parsing boundary, cross-object invariants, and lifecycle/evidence-view semantics** — specified in `spec.md` Requirements/Compatibility, resolved design (not open).
- Planning depth: Task + Spec, because this ticket defines authority/decision semantics that multiple downstream consumers (#26, #30, #33, #37, #44, #59, #67) must agree on; no Design doc because the shared fixtures already fix the one bounded shape and there is no competing technical approach to choose between.
- Blockers: none. All prior open decisions are resolved. Remaining before coding starts: rebase the branch onto current `main` and add `Closes #25` to PR #100's body (see Execution Todo).
