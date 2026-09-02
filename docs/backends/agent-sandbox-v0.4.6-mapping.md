# Provisional RuntimeBackend Mapping to Agent Sandbox v0.4.6

Status: research draft for E8-S1 ([#66](https://github.com/wunderforge/agenova/issues/66)); real-cluster validation is pending E8-T3 ([#99](https://github.com/wunderforge/agenova/pull/99)).

This report maps the current Agenova `RuntimeBackend` semantics to the pinned Kubernetes SIGs Agent Sandbox `v0.4.6` API. It is advisory input to E8-T1 ([#48](https://github.com/wunderforge/agenova/issues/48)), not a final adapter contract or a claim of production support.

## Pinned Baseline

- Upstream release: [`v0.4.6`](https://github.com/kubernetes-sigs/agent-sandbox/releases/tag/v0.4.6), annotated tag object `9d2acac65b7903ab2c8d4a7e55ad2d8c06568d26`, resolving to commit [`d0c124d4a1fded4ed4aecb92753696b4dd8de17b`](https://github.com/kubernetes-sigs/agent-sandbox/commit/d0c124d4a1fded4ed4aecb92753696b4dd8de17b).
- Extension API: [`extensions.agents.x-k8s.io/v1alpha1`](https://github.com/kubernetes-sigs/agent-sandbox/tree/v0.4.6/extensions/api/v1alpha1).
- Core API: [`agents.x-k8s.io/v1alpha1`](https://github.com/kubernetes-sigs/agent-sandbox/tree/v0.4.6/api/v1alpha1).
- Agenova boundary: [`internal/runtime/backend.go`](../../internal/runtime/backend.go), including its internal `BackendClaim` projection, interpreted under [Backend Neutrality](../product/architecture-contract.md#backend-neutrality) and [Claim Lifecycle](../product/architecture-contract.md#claim-lifecycle).
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
| Allocation | `native` | A `SandboxClaim` selects `spec.sandboxTemplateRef` and an optional `spec.warmpool`; the controller records the acquired sandbox in `status.sandbox.name`. See the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go) and [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | `AddClaim` maps the internal Agenova `BackendClaim` to an upstream claim; `BindClaim` observes a non-empty sandbox name and records `Bound`. | Source-verified; cold and warm paths still require cluster evidence. |
| Readiness | `translated` | `Sandbox.status.conditions` defines `Ready`; the claim controller mirrors the backing Sandbox `Ready` condition into `SandboxClaim.status.conditions`. See the [core Sandbox API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/api/v1alpha1/sandbox_types.go) and [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | `StartClaim` polls for `Ready=True`. This is valid infrastructure readiness evidence, but the architecture contract forbids treating readiness alone as agent-work success. | Source-verified; transition timing requires cluster evidence. |
| Backend identity | `native` | `SandboxClaim.status.sandbox.name` and `podIPs` expose the assigned backend resource identity and addresses. See [`SandboxClaimStatus`](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go). | `BindClaim` stores `status.sandbox.name` as `SandboxID`; `Claim` returns that ID through the internal `BackendClaim` as backend evidence. | Source-verified; stability across cleanup requires cluster evidence. |
| Start | `adapter-held` | Upstream exposes infrastructure `Ready`, but no field records that the Agenova runner started claim work. | `StartClaim` currently converts observing `Ready=True` into local `Running`. The Agenova architecture instead defines `Running` from runner start, so the exact work-start fact must remain in Agenova-controlled state. | Semantic gap identified; runner-integrated proof is outside this spike. |
| Termination | `adapter-held` | v0.4.6 defines a `Finished` condition on Sandbox and mirrors it to SandboxClaim; reasons include `PodSucceeded` and `PodFailed`. It also represents expiry through `Ready=False` with `ClaimExpired`. These are observable substrate outcomes, not Agenova's explicit `SucceedClaim`, `FailClaim`, and `ExpireClaim` operations. See the [core API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/api/v1alpha1/sandbox_types.go), [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go), and [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go). | The current adapter deletes the upstream claim and stores `Succeeded`, `Failed`, or `Expired` plus an error summary only in its in-memory `claimEntry`. | Source-verified gap; condition behavior and deletion ordering require cluster evidence. |
| Cleanup | `native` | Claim lifecycle supports `Delete`, `DeleteForeground`, and `Retain`; the controller deletes owned Sandbox resources on expiry, while `SandboxWarmPool` reconciles actual replicas back toward `spec.replicas`. See the [claim API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxclaim_types.go), [claim controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxclaim_controller.go), and [warm-pool controller](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/controllers/sandboxwarmpool_controller.go). | `SucceedClaim` and `FailClaim` delete the upstream claim and locally set `SandboxReplaced=true`; `PoolStatus` expects replenished capacity. | Source-verified mechanism; deletion, ownership, and replenishment must be demonstrated. |
| Durability | `unsupported` | Kubernetes persists CRD spec/status while retained, including `Finished` or expired conditions, but it does not persist Agenova's complete terminal phase, failure summary, original Agenova claim spec, or `SandboxReplaced` fact. | The current adapter stores these facts in process memory and deletes the upstream claim on explicit success/failure/expiry, so restart loses the Agenova terminal record. | Source and local-code verified gap; a restart experiment will demonstrate it. |
| Isolation | `unknown` | `SandboxTemplate` carries a Kubernetes `PodSpec`, defaults service-account-token automount off when unspecified, and can manage a default-deny-oriented NetworkPolicy. These are configuration capabilities, not evidence that the selected kind environment enforces the intended runtime/network boundary. See the [template API](https://github.com/kubernetes-sigs/agent-sandbox/blob/v0.4.6/extensions/api/v1alpha1/sandboxtemplate_types.go). | The spike adapter maps only image and command and does not itself prove runtime class, workload identity, network enforcement, or hostile-workload isolation. | Requires the merged #99 substrate plus explicit runtime and network observations. |

## Verified Facts

These facts are established from the pinned v0.4.6 source and current Agenova code; they are not yet real-cluster evidence:

1. Upstream `SandboxClaim` has conditions rather than an Agenova phase field.
2. v0.4.6 does have a `Finished` condition and distinguishes `PodSucceeded` from `PodFailed`; it still has no Agenova `SucceedClaim` or `FailClaim` primitive.
3. `Ready=True` is infrastructure readiness. Agenova `Running` additionally requires the runner to start claim work.
4. Upstream warm-pool status exposes `replicas`, `readyReplicas`, and `selector`, not Agenova's idle/bound/running/replaced breakdown.
5. The current adapter keeps claim phases, error summary, replacement fact, pool mappings, and original Agenova-to-upstream name mappings in memory.
6. The current `Claim()` view does not return the original `PoolRef` or input, and current `PoolStatus()` counts claims across adapter memory rather than deriving a durable per-pool breakdown.
7. The merged E1-T4 work separates the public `api/v1alpha1.SandboxClaim` authority/decision contract from the internal `runtime.BackendClaim`; this spike maps only the latter runtime projection to the upstream CRD.

## Provisional Gap Report

| Gap | Consequence | Candidate owner after this spike |
| --- | --- | --- |
| Work start is not an upstream fact. | `Ready=True` cannot by itself prove Agenova `Running`. | Runner/control-plane integration; re-check after #30. |
| Explicit Agenova terminal transitions are adapter-held. | Upstream pod completion and expiry conditions cannot silently redefine work outcome. | #48 supported mapping plus a later durable claim-state ticket. |
| Terminal record is not durable in the current adapter. | Adapter restart loses phase, summary, original spec, and replacement evidence. | Later E8 adapter implementation; not #66. |
| Pool detail is synthesized and not safely partitioned per pool. | Multi-pool status can be misattributed. | #48 or later adapter implementation. |
| Cleanup/replenishment is not yet reproduced from the pinned substrate. | `SandboxReplaced=true` remains a local assertion until cluster evidence exists. | #66 after #99 merges. |
| Isolation is configuration-only at this stage. | No hostile-workload or network-isolation claim is justified. | Separate bounded evidence ticket unless #48 explicitly includes it. |

## Open Questions for Cluster Validation

1. For warm allocation, what exact ordering is observed among claim creation, `status.sandbox.name`, `Ready=True`, ownership transfer, and warm-pool replenishment?
2. When a backing Pod exits successfully or unsuccessfully, how long are `Finished`, its reason, and the sandbox identity observable on the claim under each shutdown policy?
3. Does deleting a claim with the spike's `Delete` policy reliably delete only the owned Sandbox and replenish the selected pool without transient over-counting?
4. Which resource remains available after controller or adapter restart, and which Agenova facts can be reconstructed without in-memory state?
5. Which runtime class and NetworkPolicy behavior does the #99 kind substrate actually enforce? Until observed, isolation remains `unknown`.

## Sensitivity to E3-T1 (#30)

The current `RuntimeBackend` now uses the internal `BackendClaim` projection, but still mixes substrate operations with Agenova lifecycle mutation. E3-T1 remains open and may reduce that interface. Re-check these rows after #30:

- `start` and `termination`, because explicit `StartClaim` / `SucceedClaim` / `FailClaim` / `ExpireClaim` may move out of the backend boundary;
- `durability`, because the durable claim store may become explicitly control-plane-owned;
- `cleanup`, because cleanup evidence may remain backend-owned even if work outcome does not;
- pool details, because `PoolStatus` may be reduced to only the backend evidence required by the MVP.

`allocation`, upstream readiness observation, and backend identity are least sensitive because they are direct substrate capabilities, although their final Agenova DTOs may still change.

## Pending Reproduction

After #99 merges, add the experimental manifests and exact commands under `harness/spike/agent-sandbox-mapping/`, then capture evidence under `docs/evidence/E8-S1/agent-sandbox-mapping/`. At minimum, the run must demonstrate one unsupported or ambiguous case rather than inferring it from source. The preferred negative demonstration is adapter restart durability; cleanup/replenishment and condition ordering should be captured in the same bounded run where practical.
