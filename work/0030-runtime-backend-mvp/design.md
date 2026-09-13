# Technical Design: Reduce RuntimeBackend to the MVP contract

- Ticket: [#30](https://github.com/wunderforge/agenova/issues/30)
- Feature spec: [spec.md](spec.md)
- Status: approach approved at `016989e` in the [Owner decision](https://github.com/wunderforge/agenova/issues/30#issuecomment-5610181744). This design records the planning baseline; exact implemented types and limits are in [the #31 handoff](handoff-0031.md). Final implementation acceptance remains pending.

## Planning Baseline and Constraints

At the planning baseline, RuntimeBackend required AddTemplate, AddWarmPool, AddClaim, BindClaim, StartClaim, SucceedClaim, FailClaim, ExpireClaim, Claim and PoolStatus. It mixes pool setup, application outcomes and infrastructure operations. The reusable suite requires pool replacement/count behavior, so changing only method names would preserve the wrong boundary.

The operator is the in-memory reference implementation. The Agent Sandbox spike also implements the interface and stores synthetic claim phases locally. Tool/Model gateways depend on the entire interface but only read Claim. The app/CLI factories and E2E tests also reference the contract. Ticket #25 already separated public issued state from internal BackendClaim; preserve that distinction.

## Decision

Propose a small operation-oriented backend contract. The names below are candidate operation names, not a frozen Go API:

| Operation | Input/output responsibility |
| --- | --- |
| Allocate/bind | Resolved runtime launch input and claim correlation in; backend identity out |
| Observe | Backend identity in; readiness/resource observations out |
| Start | Explicit start request for the bound backend; acknowledgment/error out, never readiness-as-start |
| Terminate | Stop the worker execution; return backend evidence/error without choosing the application outcome |
| Cleanup | Release backend resources; report cleanup/replacement evidence independently |

Use typed internal backend-neutral inputs/results and existing canonical runtime fields; do not introduce generic provider maps or a new public schema. Determine exact Go types while implementing the approved semantics and review their exported shape before downstream adoption. Unsupported operations fail explicitly.

Remove pool setup/status and work-outcome setters from the required backend interface. Existing pool helpers may remain on concrete implementations. Existing application phase behavior stays on its current state owner during migration; Ticket #31 will supply the full application run-service owner. A compatibility bridge must not be exposed as the new backend contract or become a second authoritative store.

Give gateways the smallest internal claim-reading dependency needed to preserve their existing Authorize behavior, implemented by the existing state owner. Reuse BackendClaim temporarily if required rather than create a competing public claim type. Ticket #31/#32 replace or adapt that view to the accepted authoritative lifecycle. Do not implement new gateway decision or evidence semantics in this migration.

## Ownership and Contract Boundaries

- internal/runtime: reduced operations and neutral backend evidence types.
- internal/operator: in-memory backend implementation plus existing reference-state compatibility where necessary.
- internal/runtime/contracttest: reusable operation cases with factories that hide implementation-specific setup.
- internal/runtime/agentsandbox: minimal adapter mapping and explicit unsupported behavior; upstream objects stay here.
- internal/app, internal/cli, gateways and E2E: necessary composition/type migration, preserving existing behavior.
- Ticket #31: resolution/issuance integration, application transitions, timeout and outcome publication before cleanup can extend authority; Ticket #32: verify gateway eligibility against that lifecycle.

## Alternatives Considered

- Delete only pool methods and leave phase/outcome authority on the backend: smaller patch, but keeps application outcome mixed with infrastructure and leaves the Ticket #31 problem unresolved.
- Rewrite the complete run service now: removes the compatibility step, but absorbs Ticket #31 and its Ticket #29 dependency, preventing a bounded Ticket #30 delivery.
- Recommended: reduce the backend contract and migrate necessary consumers in one coherent change, with an explicit narrow compatibility bridge and downstream handoff.

## Verification Strategy

Replace pool-dependent reusable assertions with operation semantics, retaining useful pool regression cases in the concrete reference tests. Use deterministic failures to verify readiness/start distinction, identity association and cleanup error separation. Run the focused contract and all affected consumer suites, then the repository gate. Provider claims require actual integration output or an explicit environment blocker.

## Risks and Compatibility

Changing the gateway constructor parameter from `runtime.RuntimeBackend` to a narrow claim-reading interface touches `internal/toolgateway/gateway.go` and `internal/modelgateway/gateway.go`, which overlap the unmerged Ticket #33 draft. The Ticket #33 draft adapts to the merged Ticket #30 result afterwards, without rewriting history.

This is a cross-package migration, not a guaranteed small method edit. Current adapter startup semantics may not support the proposed start boundary; report the gap rather than equate controller readiness with work start. New real worker execution mechanisms are not implicitly authorized. Check shared types and consumer wiring in the first implementation slice before extending behavior.

No production reconciler, durable authority store or new invocation API is introduced. Do not delete regression evidence solely because the reusable suite's scope changes. Team approval selects this approach; common authorship of later Tickets does not remove independent review.
