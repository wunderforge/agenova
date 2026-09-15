# Technical Design: Compose the application run service

- Ticket: [#31](https://github.com/wunderforge/agenova/issues/31)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

- #29 is merged and owns creation through `internal/issuance.Issue` of a validated Allow-form `IssuedState` containing one system-managed Pending claim, immutable effective authority, and empty non-nil invocation/runtime-event lists.
- #30 reduced `runtime.RuntimeBackend` to Allocate, Observe, Start, Terminate, and Cleanup. It deliberately owns no public claim phase or work outcome.
- The reference `operator.Runtime` still contains a legacy phase state machine and implements `runtime.ClaimReader` for existing gateways. #30 explicitly forbids mirroring a reduced allocation through legacy `AddClaim`.
- The reference runtime is not generally concurrency-safe and must be serialized by its application consumer.
- Backend calls have no `context.Context`; application deadlines cannot honestly promise interruption of a call already executing.
- #32 needs an authoritative Running-only lifecycle read, while #31 must not change gateway enforcement itself.

## Decision

Add an application-owned run service under `internal/app` with a private per-service lifecycle store and a single serialized reference execution path.

The service consumes #29's accepted issued state and a neutral launch plan, validates that the starting state is Allow/Pending with no backend identity, then drives this ordered state machine:

```text
validate issued state
  -> Allocate
     -> on allocation failure, publish Failed directly from Pending (Owner-approved contract addition)
  -> attach correlated backend identity and publish Bound
  -> Observe until Ready or deadline
  -> Start
  -> publish Running
  -> execute application work once
  -> publish Succeeded or Failed (or Expired at deadline)
  -> Terminate
  -> Cleanup
  -> return the terminal issued-state snapshot plus teardown evidence
```

All phase changes pass through one transition function. It checks the architecture transition table, terminal immutability, claim/identity correlation, and evidence ordering before replacing the stored defensive snapshot. The service publishes a terminal application outcome before calling Terminate or Cleanup. Teardown records success/failure separately and cannot call the phase-transition path.

The allocation-failure edge is an Owner-approved lifecycle-contract change: permit `Pending -> Failed` only when Allocate fails before any backend identity exists. The alternatives are invalid: `Bound` would assert a resource binding that never happened, while remaining Pending would conceal a terminal application failure. The implementation slice updates the architecture contract and proves the edge plus its failure evidence.

Use small injected collaborators rather than backend-specific branches:

- the accepted #29 issuer/admission composition produces the initial `IssuedState`;
- a launch resolver produces an `app.ResolvedLaunch` bound to the granted runtime profile; its runtime-template reference may differ from the Agent Template reference, and RunService supplies the issued ClaimID when constructing `runtime.AllocateRequest`;
- `runtime.RuntimeBackend` performs the five resource operations;
- a work function represents the reference application work result;
- an injected clock/deadline signal makes readiness and work-boundary timeout tests deterministic.

The first implementation will not start backend calls in detached goroutines. Deadline checks occur before each operation, during readiness polling, and immediately after synchronous results return. If a backend call itself blocks past the deadline, the service cannot publish until the call returns; after it returns, the deadline wins and the result cannot advance the claim. This is an explicit contract limitation, not silent cancellation support.

Application work is different from a backend operation: it runs behind a buffered completion channel so the service can select deadline expiry, publish Expired, tear down, and release the serialized run path even if the callback is still executing. The callback is not forcibly cancelled; any late result is ignored and cannot rewrite the terminal claim.

The application store also retains backend-identity-to-claim ownership. An identity already seen under another claim is rejected before Bound and is not torn down by the second claim.

Expose a narrow application reader backed by the same store. It returns defensive public claim snapshots and never exposes `runtime.BackendClaim` or backend/provider status. Ticket #32 will adapt gateway eligibility to this reader and add trusted invocation-context matching.

## Ownership and Contract Boundaries

- `api/v1alpha1`: no new duplicate claim, policy, decision, or evidence model. Reuse the accepted public types; add only a lifecycle evidence vocabulary if explicitly approved as part of this packet.
- `internal/app`: owns orchestration, the authoritative reference lifecycle store, transition validation, deadline decisions, and composition with #29.
- `internal/runtime`: remains the five-operation backend boundary. No application phase, deadline policy, or outcome method is added.
- `internal/operator`: remains the in-memory backend oracle. New #31 tests may use its allocation/fault hooks but do not call the legacy claim phase helpers.
- `internal/runtime/agentsandbox`: unchanged unless compilation requires a neutral interface adaptation; unsupported Start/Terminate remains explicit.
- Gateways: unchanged in #31. A handoff documents the reader for #32.
- Fixtures/evidence: deterministic ordered traces live in focused tests and repository evidence; provider claims require separate real-backend output.

## Alternatives Considered

- Keep phase ownership in `operator.Runtime`: rejected because it makes one backend implementation the governance authority and cannot support replaceable backends.
- Add outcome/state methods back to `RuntimeBackend`: rejected because #30 deliberately separated application outcome from resource operations.
- Infer Running from `Observation.Ready`: rejected by the PRD, architecture contract, #30 handoff, and #31 negative case.
- Mirror the new allocation into legacy `AddClaim` for gateway compatibility: rejected because the same ClaimID conflicts across paths and creates two state owners. #32 performs an explicit reader migration instead.
- Launch every backend call in a goroutine to enforce hard deadlines: rejected for the MVP because the backend has no cancellation contract; it can leak operations and create late allocation/cleanup races. The limitation is documented until the backend contract gains cancellation semantics through a separate decision.
- Build a production reconciler/store: rejected by the Ticket non-goal and MVP scope.

## Verification Strategy

- Table-driven transition tests cover every accepted lifecycle edge and invalid/terminal transitions.
- Deterministic service tests use a recording backend/work double to assert exact operation/event order for success, allocation failure, not-ready-to-ready, start failure, work failure, timeout at Pending/Bound/Running, termination failure, cleanup failure, identity mismatch, and late result rejection.
- Mutation tests prove issued input and reader output cannot change stored state.
- Existing `internal/runtime` and `internal/operator` suites prove the five-operation contract remains intact.
- Focused commands:
  - `go test -count=1 -v ./internal/app/...`
  - `go test -count=1 -race ./internal/app/... ./internal/operator/... ./internal/runtime/...`
- Repository evidence: `.\scripts\check.ps1 -All`; PR CI runs the race profile.
- A #32 handoff names the reader and explicitly states that #31 evidence does not prove gateway enforcement.

## Risks and Compatibility

- The accepted #29 issuer API is pure and requires the exact admitted request, trusted principal/decision, and request-bound authority resolution. Mitigation: compose through `internal/issuance.Issue`; do not reconstruct or bypass its proof inputs.
- `IssuedState` is described as an immutable snapshot. Updating lifecycle must be implemented as successive defensive snapshots, not caller mutation; Owner/Reviewer approval must confirm this interpretation.
- Open-string `RuntimeEvent.Kind` currently lacks an accepted lifecycle vocabulary. Mitigation: approve exact event names in this packet or keep trace evidence internal until the owning evidence Ticket accepts them.
- A synchronous backend call can overrun the deadline. The service applies Expired after return and rejects state advancement, but cannot honestly claim prompt cancellation.
- A work callback may continue after Expired because its signature has no cancellation contract. Its authority is revoked by the terminal claim, its late result is ignored, and a separate change would be required to promise cooperative cancellation.
- Existing gateways read legacy backend claims. #31 leaves them untouched; #32 owns migration and must not accept caller-supplied claim IDs as trusted worker binding.
- Agent Sandbox cannot currently acknowledge work start. The reference service must return an explicit unsupported/start failure and cannot demonstrate its happy path on that backend.
