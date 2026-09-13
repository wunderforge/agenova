# Provisional RuntimeBackend Mapping to Agent Sandbox v0.4.6

Updated: 2026-09-13. Agenova experiment baseline: `3ce542bf951a2cbab1533cda3befabbcf922415b` (merged #30, #89, #99, and filesystem lifecycle follow-up #124).

Status: provisional E8-S1 mapping ([#66](https://github.com/wunderforge/agenova/issues/66)) with one bounded real-cluster negative reproduced on the merged E8-T3 substrate ([#99](https://github.com/wunderforge/agenova/pull/99)).

This report maps the current Agenova `RuntimeBackend` semantics to the pinned Kubernetes SIGs Agent Sandbox `v0.4.6` API. It is advisory input to E8-T1 ([#48](https://github.com/wunderforge/agenova/issues/48)), not a final adapter contract or a claim of production support.

## Pinned Baseline

- Upstream release: [`v0.4.6`](https://github.com/kubernetes-sigs/agent-sandbox/releases/tag/v0.4.6), annotated tag object `9d2acac65b7903ab2c8d4a7e55ad2d8c06568d26`, resolving to commit [`d0c124d4a1fded4ed4aecb92753696b4dd8de17b`](https://github.com/kubernetes-sigs/agent-sandbox/commit/d0c124d4a1fded4ed4aecb92753696b4dd8de17b).
- Extension API: [`extensions.agents.x-k8s.io/v1alpha1`](https://github.com/kubernetes-sigs/agent-sandbox/tree/v0.4.6/extensions/api/v1alpha1).
- Core API: [`agents.x-k8s.io/v1alpha1`](https://github.com/kubernetes-sigs/agent-sandbox/tree/v0.4.6/api/v1alpha1).
- Agenova boundary: [`internal/runtime/backend.go`](../../internal/runtime/backend.go), the five operations `Allocate`, `Observe`, `Start`, `Terminate`, and `Cleanup`, interpreted under [Backend Neutrality](../product/architecture-contract.md#backend-neutrality) and [Claim Lifecycle](../product/architecture-contract.md#claim-lifecycle).
- Existing spike adapter: [`internal/runtime/agentsandbox`](../../internal/runtime/agentsandbox), used here as evidence of current translation choices rather than evidence of upstream behavior.

The pinned source lookup is reproducible without a Kubernetes cluster:

```text
gh api repos/kubernetes-sigs/agent-sandbox/git/ref/tags/v0.4.6
gh api repos/kubernetes-sigs/agent-sandbox/git/tags/9d2acac65b7903ab2c8d4a7e55ad2d8c06568d26
gh api -H "Accept: application/vnd.github.raw+json" "repos/kubernetes-sigs/agent-sandbox/contents/extensions/api/v1alpha1/sandboxclaim_types.go?ref=v0.4.6"
gh api -H "Accept: application/vnd.github.raw+json" "repos/kubernetes-sigs/agent-sandbox/contents/extensions/api/v1alpha1/sandboxwarmpool_types.go?ref=v0.4.6"
gh api -H "Accept: application/vnd.github.raw+json" "repos/kubernetes-sigs/agent-sandbox/contents/extensions/controllers/sandboxclaim_controller.go?ref=v0.4.6"
gh api -H "Accept: application/vnd.github.raw+json" "repos/kubernetes-sigs/agent-sandbox/contents/extensions/controllers/sandboxwarmpool_controller.go?ref=v0.4.6"
```

The classifications below use:

- `native`: the pinned upstream API or controller directly exposes the required semantic.
- `translated`: authoritative upstream state exists, but the adapter must translate its representation into the Agenova contract.
- `adapter-held`: the exact Agenova semantic is maintained by adapter/control-plane state rather than upstream.
- `unsupported`: neither pinned upstream state nor the current adapter preserves the required semantic across its required lifetime.
- `unknown`: source inspection is insufficient; the result requires reproducible cluster evidence.

## Provisional Mapping

| Agenova semantic | Classification | Pinned upstream evidence | Current Agenova interpretation | Validation state |
| --- | --- | --- | --- | --- |
| Allocation | `translated` | Upstream claim `spec.sandboxTemplateRef`, optional `spec.warmpool`, and assigned `status.sandbox.name`; see the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go) and [controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | `Allocate` translates ClaimID/TemplateRef into an upstream acquisition and returns a neutral identity. It reserves the worker locally and fails closed on unknown/conflicting bindings. Pool selection is adapter setup, outside RuntimeBackend. | Source, simulated-controller tests, and one real-cluster allocation. Unknown-worker recovery remains an explicit gap. |
| Readiness | `translated` | Upstream claim mirrors the backing Sandbox `Ready` condition; see the [Sandbox API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/api/v1alpha1/sandbox_types.go) and [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | `Observe` accepts readiness only for the recorded worker. Missing/different binding or query failure cannot become readiness evidence. Ready supports Bound only, never Running or Succeeded. | One real-cluster run observed Ready for the recorded worker; broader timing and restart stability remain open. |
| Backend identity | `translated` | `SandboxClaim.status.sandbox.name` identifies the assigned sandbox; `podIPs` are addresses, not the neutral identity. See the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go). | `Allocate` constructs Backend/WorkerID and `Observe` preserves ClaimID correlation. Both local maps and identity-matched upstream observations are required. After release, resolution is local and process-lifetime only. | One real-cluster run preserved ClaimID and assigned WorkerID correlation; restart reconstruction and immutable resource identity are not proved. |
| Start | `unsupported` | Upstream readiness and controller-started pods provide no separate acknowledgement that Agenova claim work started; see the [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | For a valid unreleased identity, `Start` returns `ErrUnsupported`. Legacy `StartClaim` also rejects readiness-only start. No local Running flag supplies the missing worker-start channel. | [Real-cluster negative reproduced](../evidence/E8-S1/agent-sandbox-mapping/summary.md): a valid Ready worker still returned `ErrUnsupported`. |
| Termination | `unsupported` | Upstream `Finished` reasons and expiry/deletion describe substrate conditions; see the [Sandbox API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/api/v1alpha1/sandbox_types.go) and [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | `Terminate` returns `ErrUnsupported` for a valid unreleased identity: no worker-stop evidence channel exists independently of deletion. Succeeded/Failed/Expired belong to the application lifecycle owner, outside this operation. | [Real-cluster negative reproduced](../evidence/E8-S1/agent-sandbox-mapping/summary.md); deletion still does not prove stopped work or descendants. |
| Cleanup | `translated` | Claim shutdown policies and controller deletion provide a resource-release mechanism; warm-pool reconciliation is separate. See the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go) and [warm-pool controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxwarmpool_controller.go). | `Cleanup` checks binding before deleting, then confirms both upstream claim and recorded sandbox absent. Only then is Released true; Replaced stays false. Query failure, mismatch, timeout or remaining sandbox cannot fabricate release. Repeated successful cleanup preserves identity and result. | One real-cluster run confirmed claim/Sandbox absence, `Released=true`, and `Replaced=false`; atomic identity protection remains open. It does not establish full termination/filesystem parity. |
| Durability | `unsupported` | Kubernetes stores retained upstream resources; it does not store the adapter's allocation/recovery maps. See the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go). | The reduced path keeps allocation, worker reservation and release correlation in process memory. `entryFor` depends on those maps, with no reconstruction path. Application outcomes and durable claims/facts are separate control-plane responsibilities; legacy phase helpers do not fulfill them. | Source-verified absence of recovery persistence; adapter restart demonstration pending. An upstream controller restart is a different experiment. |
| Isolation | `unknown` | A template embeds PodSpec configuration; configuration capability alone cannot prove filesystem, process or network enforcement. See the [template API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxtemplate_types.go). | Allocation and observation explicitly report Filesystem EvidenceLevel `Unsupported`; no directory, outside rule or ephemeral guarantee is asserted. General runtime/network isolation remains unknown. | #89 proves reference-model semantics only. Real worker layout and enforcement remain for #48/#51; no promotion to BackendVerified. |

All current adapter interpretations above are traceable to [allocation.go](../../internal/runtime/agentsandbox/allocation.go), [its simulated-controller tests](../../internal/runtime/agentsandbox/allocation_test.go), and the [shared contract](../../internal/runtime/backend.go). The classification applies to the full named Agenova semantic, not merely the presence of an upstream field. Thus allocation, identity and cleanup are translated even where the underlying resource mechanism is native. `adapter-held` remains available as a category, but an incomplete or non-durable local surrogate is not support for Start, Terminate or durability.

## Verified Facts

These findings come from the pinned upstream source and the merged Agenova code. Existing [#30 test evidence](../evidence/30/reference-contract/summary.md) uses a simulated controller; it does not verify the reduced operations on a real cluster.

1. RuntimeBackend has five resource operations. `BackendClaim`, `Claim` and pool administration remain compatibility/setup surfaces outside that interface.
2. Upstream uses conditions, including `Finished` and its PodSucceeded/PodFailed reasons, rather than Agenova work phases. Pod completion does not select an application outcome.
3. Ready is Bound-level evidence. Both reduced Start and legacy StartClaim reject readiness-only work start.
4. Terminate is explicitly unsupported. Cleanup confirms resource absence separately and never reports replacement from a deletion request or a pool count.
5. Allocation recovery retains unknown workers and conflicting bindings instead of deleting blindly or reusing the ClaimID. A never-observed worker cannot be confirmed released if its claim has disappeared.
6. Binding read and deletion are separate requests. The current guard is not an atomic resource-identity precondition; concurrent replacement remains a promotion gap.
7. Reduced allocation/release correlation and legacy claim/pool bookkeeping are process-local. Legacy Claim/PoolStatus are not an authoritative durable evidence API.
8. #89 adds filesystem evidence to Allocation and Observation. Agent Sandbox returns Unsupported; the reference reports Simulated, not BackendVerified.

## Provisional Gap Report

| Gap | Consequence | Follow-up boundary |
| --- | --- | --- |
| No explicit work-start channel | Ready cannot establish Running; Start is unsupported. | #48 records the supported subset; later runner/adapter work supplies real acknowledgement. |
| No distinct worker-stop evidence | Terminate is unsupported; resource deletion cannot establish application outcome or descendant termination. | #48 records the gap; #51 must verify any future supported termination path. |
| Unknown/conflicting allocation and non-atomic binding checks | Recovery can remain pending or require intervention; no blind delete or ClaimID reuse is justified. | #66 records uncertainty; #48/later adapter work needs ownership and immutable-identity evidence. |
| Release has no replacement or restart guarantee | Replaced stays false; local release resolution is lost after adapter restart. | #66 records the reproduced release result and residual restart gap; #48 consumes it. |
| Application outcome persistence is outside RuntimeBackend | Legacy in-memory phases cannot serve as durable claims or facts. | Run-service/control-plane follow-up; no shared-contract change in #66. |
| Pool counters are legacy approximations | They cannot substitute for per-identity resource release evidence. | Adapter setup/future pool work, outside the five-operation mapping. |
| Filesystem evidence is Unsupported; other isolation is unknown | No real containment, retention or credential-isolation claim is justified. | #48 maps layout/profile; #51 proves real worker behavior. |

## Filesystem Boundary After #89

The [filesystem handoff](../../work/0089-filesystem-boundary/handoff-0048-0051.md) and [evidence summary](../evidence/89/filesystem/summary.md) define the boundary this report must preserve:

| Field or level | Meaning and current mapping |
| --- | --- |
| WorkingDirectory | Backend-selected worker-visible task directory, never caller-selected host path. Reference uses /workspace; Agent Sandbox returns no supported directory. |
| OutsideBoundary | RuntimeReadOnlyOtherUnavailable permits runtime files to be readable but read-only; other task data outside the directory is unavailable. No Agent Sandbox enforcement is verified. |
| Ephemeral | Cleanup releases the task directory without supported workspace retention. Agent Sandbox does not assert this guarantee. |
| Simulated | Reference-model checks only; not native process isolation. |
| Unsupported | Current Agent Sandbox allocation/observation result. Empty other fields are absence of a supported mapping, not permission or evidence of retention. |
| BackendVerified | Requires real worker evidence under #51; not earned by a passing reference test, ordinary Git/Go fixture or #99 substrate run. |

The trusted local Git/Go fixture proves tool and artifact-collector compatibility only. Explicit collector closure is not worker termination. The new #124 lifecycle evidence does not change that distinction.

The source-based handoff checklist for #48/#51 remains open: worker directory; writable/read-only mounts; HOME, temp and cache placement; host-path/credential exposure; pre-Start denial; direct outside/traversal/alias denial; fresh-claim isolation; pre-termination artifact export and post-termination denial; descendant stop and cleanup refusal on incomplete termination. #66 records these gaps rather than implementing a filesystem API or expanding its bounded experiment into the full #51 isolation suite.

## Remaining Open Questions

The bounded E8-S1 run answered the Ready-worker Start/Terminate question and observed one successful allocation/readiness/cleanup path. These broader questions remain outside that result:

1. Allocation/readiness/identity: what ordering and stable resource identifiers accompany claim creation, sandbox assignment and Ready? Can a mismatched binding be distinguished without attributing another worker's readiness?
2. Cleanup failure: what happens on incomplete deletion or an unresolvable worker? Keep upstream pool replenishment separate.
3. Adapter restart: which allocation/worker/release correlations become unresolvable when in-memory maps disappear, despite retained upstream resources? Do not confuse this with controller restart or claim-outcome persistence.
4. Filesystem/runtime/network: which layout and enforcement properties are actually observed? Keep general isolation unknown and the adapter filesystem level Unsupported until the #48/#51 evidence requirements are met.

## Reconciliation with Merged #30 and #89

#30 is merged; its reduction is the current baseline, not a future dependency. #89 and #124 are merged too. The former draft's AddClaim/BindClaim/StartClaim and SucceedClaim/FailClaim/ExpireClaim descriptions are superseded for the shared backend mapping:

- Allocate owns acquisition and neutral identity; Observe owns identity-matched readiness/resource evidence.
- Start requires actual work-start acknowledgement; Terminate requires worker-stop evidence. Both are currently unsupported in this adapter.
- Cleanup reports confirmed release independently of work outcome. It does not infer replacement, termination, or a filesystem guarantee.
- Durability analysis targets allocation/recovery/release correlation separately from application claim and fact storage.
- FilesystemBoundary is an evidence value returned by Allocate/Observe, not a sixth operation.

#48 can consume this source and bounded real-cluster report while deciding the final supported mapping. The committed MVP is a single governed claim; parent/child governance and a production adapter are outside this ticket.

## Bounded Reproduction

The [mapping experiment](../../harness/spike/agent-sandbox-mapping/README.md) ran on the merged E8-T3 substrate with an explicit Kubernetes context. Its [captured evidence](../evidence/E8-S1/agent-sandbox-mapping/summary.md) records the four installed CRD schemas and demonstrates that a valid Ready worker still returns `ErrUnsupported` from both Start and Terminate. Cleanup separately confirms resource release; it is not treated as work-stop or application-outcome evidence. This closes the reproduced-negative requirement raised in PR #105 without expanding into #51's adapter restart or isolation proof.
