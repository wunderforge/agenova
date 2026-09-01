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
- Any Kubernetes/provider-specific shape.
- Durable persistence or analytics schemas (explicit Ticket non-goal).
- `ApprovalRequired` interruption/resume/approval-record behavior (owned by #90) — this ticket only reserves the vocabulary value.

**Note (owner review on #100):** the legacy `api/v1alpha1.SandboxClaim`/`Spec`/`Status` prototype is no longer treated as out-of-scope/protected — see Open Decisions.

## Requirements

- Given the shared `issued-state.valid.team-a-engineer` fixture, when it is parsed, then `EffectiveAuthority`, `SandboxClaim`, `Decision`, and `Evidence` all decode and validate, and `claim.authorityRef` correlates to `effectiveAuthority.id`.
- Given the shared `issued-state.valid.team-b-denial` fixture, when it is parsed, then `Decision.result` is `Deny`, no `claim` or `effectiveAuthority` is present, and `Evidence` still validates with correlated `decisionIds` and no fabricated `claimId`.
- Given a decision, when its `result` is inspected, then the value is one of the typed constants `Allow`, `Deny`, `ApprovalRequired`; no boolean `allowed`-style field exists anywhere in the public contract.
- Given `issued-state.invalid.caller-effective-authority`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `effectiveAuthority`.
- Given `issued-state.invalid.caller-claim-phase`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `claim.phase`.
- Given `issued-state.invalid.caller-backend-identity`, when parsed, then validation fails with category `system-managed-field` and field path pointing at `claim.backendIdentity`.
- Given any parsed `Decision` with `result: ApprovalRequired`, then no validator or test in this ticket treats it as granted authority or as equivalent to `Allow`.
- Given a document from a caller-facing entrypoint, when it is parsed via `ParseCallerIssuedState`, then `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` are rejected with category `system-managed-field` if present, regardless of any `source`-style field inside the payload.
- Given a document from a trusted system-issued entrypoint, when it is parsed via `ParseSystemIssuedState`, then `effectiveAuthority`, `claim.phase`, and `claim.backendIdentity` are accepted — the two functions are the only way to select this behavior; there is no payload field that can switch between them.
- Given a parsed snapshot, when cross-object invariants are checked, then: `claim.requestRef == evidence.requestRef`, `claim.authorityRef == effectiveAuthority.id`, `decision.principalRef`/`decision.action` correlate to `principal`/`action`, `decision.policyRef` matches the issued `policyRef`, every id in `evidence.decisionIds` corresponds to a decision present in the snapshot, and `evidence.claimId` is absent unless `decision.result == Allow`.
- Given `decision.result == Allow`, then `claim` and `effectiveAuthority` must both be present; given `decision.result` is `Deny` or `ApprovalRequired`, then `claim` and `effectiveAuthority` must both be absent.
- Given `claim.phase == Pending`, then `claim.backendIdentity` may be absent and the claim still validates; `backendIdentity` is only valid from `Bound` phase onward (system-allocated after binding).
- Given `evidence.runtimeEvents`/`toolInvocations`/`modelInvocations`, then each decodes into an explicit evidence-view DTO (not the existing `RuntimeEvent`/`ToolInvocation`/`ModelInvocation` fact structs) — see Compatibility for the proposed shape.

## Negative Cases

- Caller-supplied `effectiveAuthority` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-effective-authority`).
- Caller-supplied `claim.phase` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-claim-phase`).
- Caller-supplied `claim.backendIdentity` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-backend-identity`).
- A `Decision.result` value outside `Allow`/`Deny`/`ApprovalRequired` must be rejected at decode time, not silently accepted as a string.
- A document carrying a spoofed `"source": "system"`-style field must still be rejected by `ParseCallerIssuedState` — origin is a function choice made by the calling code, never a payload value.
- A snapshot with `decision.result == Allow` but no `claim`/`effectiveAuthority` (or vice versa: `Deny`/`ApprovalRequired` with a `claim` present) must be rejected by the cross-object invariant check.

## Compatibility

- **Superseded by owner review on PR #100:** the existing `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` types are an incomplete reference-runtime prototype, not a Kubernetes/provider type and not a compatibility authority. They do **not** get to keep the name `SandboxClaim`. They must be moved/renamed into an internal runtime boundary (e.g. an `internal/` package or a distinct internal name) and their consumers migrated, either inside this ticket or a recorded prerequisite ticket — see Open Decisions. `SandboxWarmPool*` types are unaffected (no name collision).
- `ClaimPhase` already exists as a typed string enum in `api/v1alpha1/types.go` (`Pending`, `Bound`, `Running`, `Succeeded`, `Failed`, `Expired`) and matches the architecture contract's claim lifecycle; the v0 issued-state `SandboxClaim.phase` reuses this existing type rather than redefining a parallel enum. `backendIdentity` is valid only from `Bound` onward — `Pending` claims validate without it.
- `ToolInvocation`, `RuntimeEvent`, and `ModelInvocation` already exist in `api/v1alpha1/types.go` shaped by `ClaimID`/kind/timestamp; the fixtures' embedded evidence items are a lighter view (e.g. `{"kind": "ClaimRunning"}` with no `ClaimID`/timestamp; `toolInvocations`/`modelInvocations` are empty in both shared fixtures, so their per-item shape is not fixture-exercised). Per owner review, this ticket defines explicit evidence-view DTOs rather than deciding this ad hoc during implementation. Proposed shape (open to refinement during implementation, but the DTO-vs-fact-struct split itself is decided): `EvidenceRuntimeEvent{Kind string}`, `EvidenceToolInvocation{ToolName string, Timestamp time.Time}`, `EvidenceModelInvocation{ModelName string, Timestamp time.Time}` — no `ClaimID` field, since the parent `Evidence.claimId` already carries that correlation.
- PR #93 (E1-T2) merged 2026-08-31T22:46 UTC. `ValidationError`/`ValidationCategory` (including `ValidationCategorySystemManagedField`) live in `api/v1alpha1/agent_template.go`. This ticket branches from current `main` and imports that vocabulary directly rather than defining a second one.
- Trusted parsing boundary: expose `ParseCallerIssuedState(data []byte) (*IssuedState, *ValidationError)` and `ParseSystemIssuedState(data []byte) (*IssuedState, *ValidationError)` (naming mirrors `ParseAgentTemplateYAML` from PR #93). Origin is selected by which function the caller invokes, never by a field inside `data`. Only `ParseCallerIssuedState` performs the `system-managed-field` rejection.

## Open Decisions

- **Naming collision on `SandboxClaim` — CHANGES REQUESTED, corrected direction recorded, one sub-decision still open.** Owner review on PR #100 (2026-08-31) rejected Option (a) (`SandboxClaimV0` alongside an untouched legacy type): the legacy `api/v1alpha1.SandboxClaim` is not a compatibility authority, so the canonical public name `SandboxClaim` belongs to this ticket's backend-neutral issued-state type. Corrected direction: keep the public name `SandboxClaim`; move/rename the legacy prototype (`SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus`) into an internal runtime boundary and migrate its consumers. **Open:** whether that legacy-type migration is done inside this ticket (larger diff, one PR) or split into an explicit prerequisite ticket that lands first (keeps this ticket's original "S" sizing, but adds sequencing). Owner allows either; landing two competing public claim types is the only ruled-out outcome. Needs an Owner/assignee call before implementation starts.
  - **Blast-radius finding (assignee, 2026-09-01):** the legacy type is consumed well beyond `api/v1alpha1/types.go`. It is typed directly into the `RuntimeBackend` **interface** (`internal/runtime/backend.go`: `AddWarmPool`, `AddClaim`, `Claim`, `PoolStatus`), and threaded through `internal/operator/runtime.go` (the in-memory reference runtime's claim-phase state machine), `internal/runtime/agentsandbox/adapter.go` (the interface's implementation), `internal/runtime/contracttest/run.go` (shared black-box suite exercising `RuntimeBackend` implementations), `internal/sandbox/pool.go`, `internal/modelgateway/gateway.go`, `internal/toolgateway/gateway.go`, `internal/cli/cli_test.go`, and e2e/integration tests under `harness/`. Roughly 6 production files plus 7-8 test files across 6 packages, including one exported interface signature.
  - **Assignee recommendation: split into a prerequisite ticket.** The rename is mechanical and compiler-checked (no logic changes — `ClaimPhase` itself doesn't move), so risk of a silently-missed call site is low, but folding it into this ticket would make the PR 3-5x larger than the actual contract-type work (2 new files) and mix "define the authority/decision contract" with "refactor the runtime backend interface" in one review, working against the Ticket's own "S" sizing and one-bounded-outcome intent. A prerequisite ticket also isolates rollback risk. This is a recommendation, not a decision — posted to the Owner on PR #100 for a call.
- **Sequencing with #93 — RESOLVED.** PR #93 merged 2026-08-31T22:46 UTC; this ticket branches from current `main` and imports `ValidationError`/`ValidationCategory` from `api/v1alpha1/agent_template.go` directly.
