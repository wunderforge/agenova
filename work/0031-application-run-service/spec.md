# Feature Specification: Compose the application run service

- Ticket: [#31](https://github.com/wunderforge/agenova/issues/31)
- PRD outcome: [Claim lifecycle](../../docs/product/prd.md#2-claim-lifecycle), [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution), and the ordered [MVP user journey](../../docs/product/prd.md#mvp-user-journey).

## Intent

Provide one application-owned execution contract that consumes an accepted, system-issued Pending claim and drives it through the backend-neutral runtime operations without confusing infrastructure readiness, worker start, work outcome, and cleanup evidence. Ticket #32 and evidence consumers need one authoritative lifecycle view rather than inferring authority from backend state.

## In Scope

- One synchronous reference run operation over an issued Pending claim, a resolved neutral launch input, a `RuntimeBackend`, an application work callback, and an application deadline.
- Application-owned phase transitions and backend identity publication.
- Deterministic ordered lifecycle/resource evidence and a defensive current-claim read view.
- Reference-path failure handling for allocation, start, application work, timeout, termination, and cleanup.

## Out of Scope

- A long-running reconciler, durable queue/store, distributed idempotency, backend-call preemption, workflow engine, transport protocol, or production worker/task-delivery mechanism.
- Re-authorizing or re-resolving a request, changing #29 issuance, changing #30 backend semantics, or enforcing gateway eligibility from #32.
- Provider-specific status mapping or claims of real-backend work-start support.

## Requirements

- Given a validated Allow-form issued state with one Pending claim and no backend identity, when allocation succeeds, then the returned identity is correlated to the same claim before the application publishes Bound.
- Given a Bound claim, when observation reports readiness, then the claim remains Bound until explicit `Start` acknowledgment succeeds.
- Given successful Start acknowledgment, when the application publishes Running, then the work callback may execute exactly once for that run.
- Given successful work, when the application publishes Succeeded, then Terminate and Cleanup happen afterward and their evidence cannot change Succeeded.
- Given work failure, when the application publishes Failed, then teardown happens afterward and its evidence cannot change Failed.
- Given allocation failure, when the service applies the Owner-approved allocation-failure transition, then it publishes Failed directly from Pending with no backend identity and makes no identity-dependent teardown call.
- Given start failure with a known identity, when the service terminates the claim, then it publishes Failed without ever publishing Running and attempts teardown.
- Given the deadline is reached while Pending, Bound, or Running, when the service observes the deadline, then it publishes Expired before teardown and ignores later attempts to publish another outcome.
- Given termination or cleanup failure, when the result is returned/read, then the original terminal phase and the failed resource operation are both observable.
- Given a caller reads current state, when it mutates the returned value, then authoritative stored state remains unchanged.
- Given an invalid transition or identity mismatch, when it is proposed, then the service rejects it without changing phase, identity, or evidence attribution.

## Negative Cases

- Ready without Start acknowledgment remains Bound and grants no application Running state.
- Start failure produces no Running event and no work callback.
- Cleanup/replacement evidence never becomes Succeeded or changes a prior outcome.
- A late success/failure after Expired is rejected.
- A backend result correlated to another claim is rejected and cannot attach its identity or evidence.
- Denied/ApprovalRequired issued states and malformed/non-Pending starting claims never allocate a backend.

## Compatibility

- Keep `runtime.RuntimeBackend` at the accepted five-operation contract from #30.
- Keep `v1alpha1.SandboxClaim`, `IssuedState`, effective authority, and decision shapes unchanged; #31 updates trusted application-owned state rather than accepting caller-managed lifecycle fields.
- Existing legacy `operator.Runtime` phase helpers and `runtime.ClaimReader` remain compatibility-only until consumers migrate; new application code must not mirror a reduced allocation into the legacy AddClaim path.
- Existing gateway behavior remains unchanged in #31. Ticket #32 adapts gateways to the new authoritative read boundary.
- Agent Sandbox `Start` and `Terminate` may remain unsupported; the service must surface that honestly instead of treating readiness as work start.

## Open Decisions

- Owner/Reviewer must approve the exact authoritative read shape for #32: preferred direction is a narrow application-owned reader returning a defensive public `v1alpha1.SandboxClaim` snapshot plus existence, without exposing backend store types.
- Owner approved the narrow `Pending -> Failed` transition for allocation failure in [PR #131](https://github.com/wunderforge/agenova/pull/131#issuecomment-5659891364). The implementation slice must update the architecture lifecycle table and test the transition plus failure evidence; `Bound` remains invalid because no backend identity exists.
- #29 is merged and closed. The run service consumes `internal/issuance.Issue` and its accepted complete Allow-form snapshot rather than defining an interim duplicate.
- Confirm whether the first slice records lifecycle evidence only in the returned run result or also appends it into `IssuedState.Evidence.RuntimeEvents`; the design prefers one updated IssuedState snapshot so later evidence consumers do not gain a second model.
