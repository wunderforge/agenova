# Feature Specification: Reduce RuntimeBackend to the MVP contract

- Ticket: [#30](https://github.com/wunderforge/agenova/issues/30)
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution)
- Status: planning boundary approved at `016989e` in the [Owner decision](https://github.com/wunderforge/agenova/issues/30#issuecomment-5610181744). The implemented surface is documented in [the #31 handoff](handoff-0031.md); final implementation acceptance remains pending.

## Intent

Application lifecycle code requests backend operations and interprets their evidence. Infrastructure readiness and cleanup never become independent sources of application authority or work outcome.

## In Scope

- One backend allocation identity and the minimum operations needed by the reference golden run.
- Backend-neutral input/output data and a reusable behavioral suite for the reference backend.
- Compatibility of current consumers and explicit adapter limitations.

## Out of Scope

- Request authorization/issuance, application run orchestration, governed invocation policy, pool administration, production durability and new provider features.

## Requirements

- Allocation/binding accepts resolved runtime input and associates one backend identity with the application claim identity. It does not issue a new public claim or grant requested authority.
- Observation reports backend identity and readiness separately from application phase. Readiness may support Bound; it does not establish Running or Succeeded.
- An explicit start operation/boundary is distinguishable from observation. A backend that cannot establish actual start must report that limitation rather than infer it from readiness.
- Termination concerns worker/backend execution. It does not choose Succeeded, Failed or Expired for the application.
- Cleanup reports resource cleanup or replacement evidence and any failure independently of application work outcome. Preserve identity for correlation even after resource removal.
- Pool setup and pool counters are not requirements of a replaceable backend. An implementation may use a pool internally.
- Required operation failures are explicit; unknown handles, unavailable readiness and unsupported operations must not fabricate success.
- Shared request/result types contain no Kubernetes objects, provider condition structures or credentials; use the existing canonical identity/runtime vocabulary where applicable.

## Negative Cases

| Case | Required observation |
| --- | --- |
| Unknown handle | Lookup/operation fails explicitly, no success evidence attributed to another claim |
| Allocation failure | No successful binding/readiness result fabricated |
| Observe ready without start | No inferred successful work or authority grant |
| Start before readiness or start failure | No successful start acknowledgment |
| Cleanup failure following an established outcome | Failure visible separately; outcome not rewritten |
| Replacement | Resource evidence changes without creating another work outcome |
| Unsupported adapter operation | Explicit unsupported result/error, not a silent pass |

## Compatibility

- Preserve existing Authorize signatures and fail-closed claim/parent behavior via a narrow internal claim-reading dependency on the existing state owner; do not add a second state registry.
- Existing reference scenario and CLI composition tests remain executable. Pool-specific assertions remain in implementation-specific tests rather than defining every backend's contract.
- The application lifecycle and its real timeout/cleanup sequencing are delivered by Ticket #31. This Ticket establishes the backend seam without claiming the full run is implemented.

## Open Decisions

- The operation boundary and compatibility approach were approved. Independent implementation review must still verify the final Go surface and compatibility evidence before downstream adoption.
- Ticket #31 must select its authoritative claim read/publication boundary. The current compatibility view is not a promise that backend observation supplies future authority.
- Real Agent Sandbox support depends on the adapter's actual capabilities and environment evidence. Unsupported start/termination semantics are documented gaps, not invented platform behavior.
