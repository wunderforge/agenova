# RuntimeBackend handoff to Ticket #31

- Producer: [Ticket #30](https://github.com/wunderforge/agenova/issues/30), [task](task.md), [PR #106](https://github.com/wunderforge/agenova/pull/106).
- Consumer: [Ticket #31: application run service](https://github.com/wunderforge/agenova/issues/31), which also depends on #29 issuance.
- Status: implementation surface for independent review. The operation boundary was approved in [the Owner's planning decision](https://github.com/wunderforge/agenova/issues/30#issuecomment-5610181744); final implementation acceptance remains pending.
- Sources: [Go contract](../../internal/runtime/backend.go), [ClaimReader bridge](../../internal/runtime/claim_reader.go), [review and evidence](review.md), [backend note](../../docs/backends/agent-sandbox.md).

## Operation and type surface

Use the existing `v1alpha1.SandboxClaimBackendIdentity{Backend, WorkerID}` unchanged. It identifies a resource known to the selected backend instance; it is neither authority nor a durable lookup service across adapter restarts.

| Operation | Input and result | Required interpretation |
| --- | --- | --- |
| Allocate | `AllocateRequest{ClaimID, TemplateRef, Input map[string]string}` → `Allocation{ClaimID, Identity}` | Correlates an already issued application claim with a worker. Creates no public claim and grants no authority. Do not use the result when an error is returned. |
| Observe | identity → `Observation{ClaimID, Identity, Ready, Released, Replaced, Detail}` | Readiness supports Bound only. There is no application phase, outcome, or started field. Detail is diagnostic text, not a field to parse for policy. |
| Start | identity → error | A nil result acknowledges explicit reference work start. Only the application owner may publish Running after that acknowledgment. Unsupported or failed start cannot do so. |
| Terminate | identity → error | Stops reference work or cancels an unstarted allocation. Selects no application outcome. Repetition succeeds until release; after release it returns ErrReleased. |
| Cleanup | identity → `CleanupResult{Identity, Released, Replaced}` and error | Keeps resource evidence separate from outcome. On failure, preserve the known identity and do not claim release. After successful cleanup, repeated cleanup returns the same result and Observe still resolves the identity. |

`ClaimID` and `TemplateRef` must be non-empty. Both current implementations require exactly one registered pool for the runtime template; no match or multiple matches is an error. Pool setup remains concrete/backend-specific. Register setup before processing requests, and do not replace active pools.

`TemplateRef` is the resolved runtime-template reference, not proof that a raw ClaimRequest AgentTemplate name is directly usable. #31 must define the resolution mapping. `Input` is only the existing flat launch vocabulary; neither current backend delivers it to a running agent. Structured task input, delivery and timeout policy require consumer/runner design rather than silently flattening or discarding ClaimRequest data.

A successful allocation reserves the ClaimID for the lifetime of this in-memory backend instance, including after release. A duplicate is rejected. The adapter may reuse a failed attempt's ClaimID only after confirmed compensation; unresolved attempts remain reserved and may require manual recovery. There is no backend-neutral retryability classification for Allocate errors.

## Errors and backend differences

Match shared sentinels with `errors.Is`, never with error strings:

| Error | Caller interpretation |
| --- | --- |
| ErrUnknownIdentity | The complete Backend/WorkerID pair is not a usable allocation in this backend instance. |
| ErrNotReady | Reference Start was attempted before readiness. |
| ErrAlreadyStarted | Reference Start already succeeded; do not acknowledge a second start. |
| ErrTerminated | Reference Start followed successful termination. |
| ErrReleased | Start or Terminate followed confirmed Cleanup. |
| ErrUnsupported | The backend cannot establish this operation's semantics. This is not successful execution. |

Memory implements all five operations. Its work-start acknowledgment and replacement are reference state transitions, not evidence of a real subprocess. Failed resource operations remain retryable as proved by the shared tests. Termination uses the pool's private Failed marker and never calls the application FailClaim setter.

Agent Sandbox implements allocation, identity-matched readiness and confirmed release; Start and Terminate return ErrUnsupported, including when infrastructure is Ready. Unknown/released identities are checked first. Replacement remains unobserved (`Replaced=false`). Observe and Cleanup reject a changed or missing recorded binding; a failed binding query cannot authorize deletion. Cleanup can confirm an already missing claim and worker on retry.

The supported adapter behavior has unit-double evidence only for this revision. [Real-cluster evidence remains blocked](../../docs/evidence/30/agent-sandbox/summary.md). The existing interface has no caller context parameter; individual kubectl commands have timeouts, but #31's application deadline cannot directly cancel an in-flight backend call. Do not report the current adapter as a complete runnable-agent backend merely because its pod is Ready.

## Compatibility migration

| Previous shared methods/consumers | Current location and migration rule |
| --- | --- |
| AddTemplate, AddWarmPool, PoolStatus | Concrete implementation setup/status; no application dependency through RuntimeBackend. |
| AddClaim, BindClaim | Concrete legacy phase path retained for existing reference scenarios. New application code uses its issued claim plus Allocate. |
| StartClaim | Concrete reference compatibility method; the adapter's legacy method now returns unsupported. New code uses Start. |
| SucceedClaim, FailClaim, ExpireClaim | Concrete compatibility only. #31 owns terminal outcomes; these are not new run-service operations. |
| Claim | Narrow ClaimReader used by existing Tool/Model gateways. It does not become a backend operation again. |
| App and CLI factories | Still return RuntimeBackend; no new claim issuance or run orchestration is wired into the CLI by #30. |

Allocate does not populate the legacy BackendClaim map. Calling Start on a reduced allocation therefore does not make existing gateways authorize that claim. #31 must provide the authoritative read/publication boundary, and #32 binds gateway eligibility to it. Do not call legacy AddClaim to mirror a new allocation; the same ClaimID is rejected across paths. Keep legacy scenarios and new orchestration separate, including their adapter instances.

The reference Runtime and its pools have no synchronization. Serialize calls to each reference instance, including setup and read/control hooks. The adapter has focused concurrent recovery protection; that is not a general concurrency contract for mixing its legacy and reduced paths.

## Responsibilities retained by #31

1. Resolve authorized request/template input and consume #29's issued public claim. No backend operation performs authorization or issuance.
2. Maintain authoritative claim state and correlate successful allocation identity before publishing Bound. Readiness or successful polling must never publish Running by itself.
3. Publish Running only after explicit start acknowledgment. Define allocation/start failure and timeout outcomes; reject invalid transitions and late start results after a claim has become terminal.
4. Publish terminal outcome and revoke governed eligibility before waiting for termination/cleanup. Preserve the original outcome when either operation fails; record their errors/evidence separately. Unknown identity after a failed allocation is not permission to invent a worker or report cleanup success.
5. Define how late backend results are handled after application timeout. Deliver deterministic success, allocation failure, start failure, timeout and cleanup-failure traces as required by #31. This handoff does not claim those application traces already exist.

The twelve shared backend cases, six legacy pool regressions, consumer denial/E2E tests and adapter failure cases remain reproducible from the [review evidence](review.md). The shared fixture requires readiness and resource-failure hooks; failure injection sits below RuntimeBackend so the real backend state handling is tested. Do not skip those cases or replace the backend with a wrapper that merely returns the expected errors.
