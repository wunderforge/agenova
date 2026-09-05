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
- Migrating the legacy `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` prototype to `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus` and updating every consumer (see Compatibility) so the canonical public `SandboxClaim` name is unambiguous.

## Out of Scope

- `AgentTemplate` (#23) and `ClaimRequest` (#24) type definitions themselves.
- Any Kubernetes/provider-specific shape.
- Durable persistence or analytics schemas (explicit Ticket non-goal).
- `ApprovalRequired` interruption/resume/approval-record behavior (owned by #90) — this ticket only reserves the vocabulary value.

**Now in scope (owner decision, 2026-09-01):** migrating the legacy `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` prototype into an internal runtime representation and updating its consumers — see Compatibility.

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
- Given `evidence.runtimeEvents`/`toolInvocations`/`modelInvocations`, then each decodes into an explicit evidence-view DTO (not the existing `RuntimeEvent`/`ToolInvocation`/`ModelInvocation` fact structs) — see Compatibility for the proposed shape. Only the `Evidence` envelope and its `runtimeEvents` view are fully specified here; detailed `toolInvocations`/`modelInvocations` item schemas are owned by #33/#37 and stay intentionally minimal in this ticket.
- Given the legacy `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` prototype, when this ticket's diff is reviewed, then those identifiers no longer exist in `api/v1alpha1` — they are renamed to an internal runtime representation and every consumer (the `RuntimeBackend` interface and its implementers/tests) compiles against the renamed type. See Compatibility.

## Negative Cases

- Caller-supplied `effectiveAuthority` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-effective-authority`).
- Caller-supplied `claim.phase` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-claim-phase`).
- Caller-supplied `claim.backendIdentity` on an issued-state document, parsed via `ParseCallerIssuedState` (`issued-state.invalid.caller-backend-identity`).
- A `Decision.result` value outside `Allow`/`Deny`/`ApprovalRequired` must be rejected at decode time, not silently accepted as a string.
- A document carrying a spoofed `"source": "system"`-style field must still be rejected by `ParseCallerIssuedState` — origin is a function choice made by the calling code, never a payload value.
- A snapshot with `decision.result == Allow` but no `claim`/`effectiveAuthority` (or vice versa: `Deny`/`ApprovalRequired` with a `claim` present) must be rejected by the cross-object invariant check.

## Compatibility

- **`SandboxClaim` remains Agenova's single canonical, backend-neutral claim type.** The existing `api/v1alpha1.SandboxClaim`/`SandboxClaimSpec`/`SandboxClaimStatus` is a prototype runtime shape, not a separate public contract — it does not keep the name `SandboxClaim`. **Decision (owner, confirmed via assignee 2026-09-01): this ticket migrates that prototype shape and its consumers itself; no prerequisite ticket.** Runtime-only fields move to an internal runtime representation, `internal/runtime.BackendClaim`/`BackendClaimSpec`/`BackendClaimStatus` (same package as the `RuntimeBackend` interface in `internal/runtime/backend.go`, so the interface references the local type with no cross-package import). Consumers to migrate: the `RuntimeBackend` interface itself, `internal/operator/runtime.go`, `internal/runtime/agentsandbox/adapter.go` (+ its unit test), `internal/runtime/contracttest/run.go`, `internal/sandbox/pool.go`, `internal/modelgateway/gateway.go` (+ test), `internal/toolgateway/gateway.go` (+ test), `internal/cli/cli_test.go`, and the `harness/e2e`/`harness/integration/agentsandbox` claim-lifecycle tests. `ClaimPhase` and `ObjectMeta` are shared building blocks and do **not** move. `SandboxWarmPool*` types are unaffected (no name collision) and stay in `api/v1alpha1`.
- `ClaimPhase` already exists as a typed string enum in `api/v1alpha1/types.go` (`Pending`, `Bound`, `Running`, `Succeeded`, `Failed`, `Expired`) and matches the architecture contract's claim lifecycle; the v0 issued-state `SandboxClaim.phase` reuses this existing type rather than redefining a parallel enum. `backendIdentity` is valid only from `Bound` onward — `Pending` claims validate without it.
- `ToolInvocation`, `RuntimeEvent`, and `ModelInvocation` already exist in `api/v1alpha1/types.go` shaped by `ClaimID`/kind/timestamp; the fixtures' embedded evidence items are a lighter view (e.g. `{"kind": "ClaimRunning"}` with no `ClaimID`/timestamp; `toolInvocations`/`modelInvocations` are empty in both shared fixtures). This ticket defines the `Evidence` envelope and an explicit `EvidenceRuntimeEvent{Kind string}` DTO (the only evidence-item shape actually fixture-exercised) rather than exposing the in-memory fact structs directly. `toolInvocations`/`modelInvocations` get a minimal placeholder DTO only (e.g. an empty-safe slice type) — their detailed item schemas are owned by #33/#37, not this ticket.
- PR #93 (E1-T2) merged 2026-08-31T22:46 UTC. `ValidationError`/`ValidationCategory` (including `ValidationCategorySystemManagedField`) live in `api/v1alpha1/agent_template.go`. This ticket branches from current `main` and imports that vocabulary directly rather than defining a second one.
- Trusted parsing boundary: expose `ParseCallerIssuedState(data []byte) (*IssuedState, *ValidationError)` and `ParseSystemIssuedState(data []byte) (*IssuedState, *ValidationError)` (naming mirrors `ParseAgentTemplateYAML` from PR #93). Origin is selected by which function the caller invokes, never by a field inside `data`. Only `ParseCallerIssuedState` performs the `system-managed-field` rejection.
- Cross-object invariants use exact field correlations per owner review: `claim.requestRef`/`evidence.requestRef` both match the top-level `requestRef`; `claim.authorityRef == effectiveAuthority.id`; `decision.principalRef == principal.subject`; `decision.action`/`decision.policyRef` match the top-level `action`/`policyRef`; `evidence.decisionIds` contains the emitted decision ID; `evidence.claimId`, when present, matches `claim.id`; `Deny`/`ApprovalRequired` must not carry a `claim` or `effectiveAuthority`.

## Open Decisions

None. The canonical public name remains `SandboxClaim`; the legacy-prototype migration happens inside this ticket, per owner decision confirmed 2026-09-01 (see Compatibility).
