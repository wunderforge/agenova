# Feature Specification: Define and validate SandboxClaim v0

- Ticket: [#25](https://github.com/wunderforge/agenova/issues/25)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

Give every downstream consumer of an issued claim (gateways, evidence surfaces, adapters) one immutable, backend-neutral snapshot of what was actually granted and decided — distinct from what was requested — plus a decision/evidence vocabulary that cannot be mistaken for a boolean "allowed" flag or a self-issued claim.

## In Scope

- `EffectiveAuthority`: id, tools, resourceScopes, modelProfile, memoryScopes, runtime (profileRef, timeout). System-issued only.
- `SandboxClaim` (v0 issued-state shape): id, requestRef, templateRef, authorityRef, phase, backendIdentity (backend, workerId). System-issued only.
- `Decision`: id, principalRef, action, `result` (typed `Allow` | `Deny` | `ApprovalRequired`), policyRef (id, version), reason.
- `Evidence`: requestRef, claimId (optional — absent for pre-claim denial), decisionIds, runtimeEvents, toolInvocations, modelInvocations.
- `Principal` and `Action` as carried inside issued state (subject/team/authenticationContext; name/project/templateRef) — the issued-state representation only, not the `ClaimRequest`-side definitions owned by #24.
- Strict parsing that rejects a caller-supplied `effectiveAuthority`, `claim.phase`, or `claim.backendIdentity` with a stable `system-managed-field` category.

## Out of Scope

- `AgentTemplate` (#23) and `ClaimRequest` (#24) type definitions themselves.
- Any Kubernetes/provider-specific shape, including the existing `api/v1alpha1.SandboxClaim` runtime-spike type's `Spec`/`Status` fields.
- Durable persistence or analytics schemas (explicit Ticket non-goal).
- `ApprovalRequired` interruption/resume/approval-record behavior (owned by #90) — this ticket only reserves the vocabulary value.

## Requirements

- Given the shared `issued-state.valid.team-a-engineer` fixture, when it is parsed, then `EffectiveAuthority`, `SandboxClaim`, `Decision`, and `Evidence` all decode and validate, and `claim.authorityRef` correlates to `effectiveAuthority.id`.
- Given the shared `issued-state.valid.team-b-denial` fixture, when it is parsed, then `Decision.result` is `Deny`, no `claim` or `effectiveAuthority` is present, and `Evidence` still validates with correlated `decisionIds` and no fabricated `claimId`.
- Given a decision, when its `result` is inspected, then the value is one of the typed constants `Allow`, `Deny`, `ApprovalRequired`; no boolean `allowed`-style field exists anywhere in the public contract.
- Given `issued-state.invalid.caller-effective-authority`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `effectiveAuthority`.
- Given `issued-state.invalid.caller-claim-phase`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `claim.phase`.
- Given `issued-state.invalid.caller-backend-identity`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `claim.backendIdentity`.
- Given any parsed `Decision` with `result: ApprovalRequired`, then no validator or test in this ticket treats it as granted authority or as equivalent to `Allow`.

## Negative Cases

- Caller-supplied `effectiveAuthority` on an issued-state document (`issued-state.invalid.caller-effective-authority`).
- Caller-supplied `claim.phase` on an issued-state document (`issued-state.invalid.caller-claim-phase`).
- Caller-supplied `claim.backendIdentity` on an issued-state document (`issued-state.invalid.caller-backend-identity`).
- A `Decision.result` value outside `Allow`/`Deny`/`ApprovalRequired` must be rejected at decode time, not silently accepted as a string.

## Compatibility

- The existing `api/v1alpha1.SandboxClaim`, `SandboxClaimSpec`, `SandboxClaimStatus`, and `SandboxWarmPool*` runtime-spike types (`api/v1alpha1/types.go`) are Kubernetes-facing and predate this contract; they must remain unchanged by this ticket.
- `ClaimPhase` already exists as a typed string enum in `api/v1alpha1/types.go` (`Pending`, `Bound`, `Running`, `Succeeded`, `Failed`, `Expired`) and matches the architecture contract's claim lifecycle; the v0 issued-state `SandboxClaim.phase` should reuse this existing type rather than redefining a parallel enum, once the naming collision below is resolved.
- `ToolInvocation`, `RuntimeEvent`, and `ModelInvocation` already exist in `api/v1alpha1/types.go` shaped by `ClaimID`/kind/timestamp; `Evidence.toolInvocations`/`runtimeEvents`/`modelInvocations` in the fixtures are currently loosely shaped (e.g. `{"kind": "ClaimRunning"}` with no `ClaimID`) — reconciling the fixture shape with the existing fact types (or documenting why `Evidence` embeds a lighter-weight view) is part of this ticket's implementation, not a re-opened fixture decision.
- PR #93 (E1-T2, open) introduces `ValidationError`/`ValidationCategory` (including `system-managed-field`) in `api/v1alpha1`. This ticket must reuse that vocabulary rather than defining a second one in the same package.

## Open Decisions

- **Naming collision on `SandboxClaim` — RESOLVED: Option (a).** The v0 backend-neutral issued-state type is exported as **`SandboxClaimV0`** in the same `api/v1alpha1` package. The existing Kubernetes-facing `SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` runtime-spike types are left untouched, per this ticket's compatibility constraint. Rationale: matches this ticket's own "v0" vocabulary (task title, fixture family), keeps the type in-package so it can reuse `ClaimPhase` and the PR #93 `ValidationError`/`ValidationCategory` vocabulary without a cross-package import, and avoids widening scope by touching the legacy spike type (ruled out by the Compatibility section above). A future ticket is expected to retire/rename the legacy spike type and mechanically rename `SandboxClaimV0` → `SandboxClaim`; that rename is explicitly out of scope here. **Status: recorded by the assignee; still needs Owner/Reviewer confirmation on issue #25 before implementation starts, per the task packet's own execution todo.**
- **Sequencing with #93.** Recommend confirming whether this ticket branches after PR #93 merges (to import its `ValidationError` type directly) or whether the Owner wants both landed independently with a follow-up consolidation. Either is workable; the Ticket's own sequence numbers (102/103/104) suggest E1-T2 lands first. **Status: unresolved — PR #93 is APPROVED with green CI but not yet merged as of 2026-08-31.**
