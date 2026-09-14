# Kubernetes Agent Sandbox Adapter

Status: version-pinned kind spike, not a production backend.

Kubernetes Agent Sandbox is the first real substrate used to test the `RuntimeBackend` boundary. It is not the Agenova product API and is not required by application agents.

To reproduce the pinned upstream substrate itself (disposable `kind` cluster + one upstream-native sandbox lifecycle, no Agenova adapter), see [`harness/spike/agent-sandbox-substrate/RUNBOOK.md`](../../harness/spike/agent-sandbox-substrate/RUNBOOK.md) (E8-T3 / #50).

## Verified Spike Behavior

- Agenova runtime image and command map to an upstream `SandboxTemplate`.
- A selected runtime template can have a `SandboxWarmPool`.
- One Agenova claim maps to one upstream sandbox acquisition.
- Upstream sandbox identity is returned as backend evidence.
- Upstream readiness is `Bound`-level infrastructure evidence. The ordinary `SpikeAdapter.Start` and legacy `StartClaim` remain unsupported; neither promotes readiness to Running. An opt-in test-worker protocol can separately acknowledge actual work start.
- Claim deletion triggers sandbox cleanup and warm-pool replenishment.
- Upstream API details remain confined to `internal/runtime/agentsandbox`.

The [#66 mapping experiment](agent-sandbox-v0.4.6-mapping.md) exercised the reduced adapter on v0.4.6 and proved that a Ready worker still returned `ErrUnsupported` for ordinary Start/Terminate. A separate #51 kind test uses a protocol-conforming disposable worker to prove an opt-in, adapter-held work-start/stop path; it does not change upstream semantics.

## Reduced RuntimeBackend Contract (Ticket #30)

The five reduced operations are tested at the `kubeClient` seam (`internal/runtime/agentsandbox/allocation_test.go`). #66 and #51 also provide bounded real-kind evidence. The support categories below are frozen for the v0.4.6 MVP slice; see the [pinned source and detailed gaps](agent-sandbox-v0.4.6-mapping.md).

| Operation | Support | Notes |
| --- | --- | --- |
| Allocate | supported, with an unprovable recovery case | Creates the upstream claim and waits for sandbox assignment. Recovery checks and reserves the assigned worker before deleting the claim, and completes only after both resources are confirmed absent. Unknown identities stay recovery-pending; conflicting identities retain their upstream claim without deletion. |
| Observe | supported (readiness only) | `Ready` mirrors the upstream `Ready=True` condition, and only when the claim still carries the recorded worker; a different or missing worker is reported as an identity mismatch. Replacement is not observable and stays `false`. Query failures are returned as errors. |
| Start | `SpikeAdapter`: unsupported; `ControlledAdapter`: adapter-held, opt-in | The upstream controller has no task-start channel. Only an image implementing `/agenova-workerctl` can return a claim-bound child-process start/result acknowledgement. This is demonstrated by the disposable #51 test worker, not arbitrary agents. |
| Terminate | `SpikeAdapter`: unsupported; `ControlledAdapter`: adapter-held, opt-in | Upstream deletion is not stop evidence. The test worker confirms the child process exited before Cleanup; ordinary images still have no supported stop channel. |
| Cleanup | supported (release) | Rechecks the recorded worker binding before deletion; changed/missing bindings or query failures prevent deletion. `Released` is reported only after both the claim and the assigned sandbox are confirmed absent; failures are explicit and retryable after identity/release can be confirmed. |
| Filesystem boundary | unsupported | Allocation and observation explicitly report `FilesystemEvidenceUnsupported`. Ticket #48 must map the worker-visible directory and mount/profile behavior; #51 must provide real isolation evidence before this can become `BackendVerified`. |

`kubectl` invocations carry a per-command deadline and resource absence is classified by exit status and empty output, never by error text.

### Frozen v0.4.6 mapping boundary (E8-T1 / #48)

| Neutral semantic | Classification | Source of evidence | Residual gap |
| --- | --- | --- | --- |
| Allocate and worker identity | translated | Upstream `SandboxClaim.status.sandbox.name`, identity-matched local reservation and real kind observation | Adapter restart reconstruction and atomic identity preconditions |
| Observe Ready | translated | Upstream `Ready=True` for the recorded worker | Ready proves Bound only |
| Start / Terminate on ordinary images | unsupported | No upstream work-control API; #66 reproduced `ErrUnsupported` on Ready worker | Needs a compatible worker-control channel |
| Start / Terminate on the #51 test image | adapter-held | Explicit `kubectl exec` to claim-bound `/agenova-workerctl`; child start/result and stop acknowledgements before deletion | Test protocol is not production identity/isolation, and arbitrary images are unsupported |
| Cleanup | translated | Delete the bound claim, then confirm both claim and Sandbox absent; #51 also checks Pod absence | Warm-pool replacement and restart durability not established |
| Filesystem boundary | unsupported | `FilesystemEvidenceUnsupported` in Allocation/Observation | No real inside/outside, cross-claim, HOME/cache or retention proof |

No shared `RuntimeBackend` or application-facing type changes are needed. Application phases, outcome and durable audit facts remain run-service/control-plane concerns, not upstream condition translations.

### Recovery evidence limits

This adapter reads a worker only through the claim's `status.sandbox.name`, which yields two cases it cannot resolve:

- The controller may bind a sandbox between a status read and the delete, so one empty status snapshot does not prove that no worker was assigned.
- A failed create request looks identical whether the server rejected it or accepted it, bound a worker and lost the response; once the claim is absent its status can no longer be read.

An attempt whose worker was never observed stays recovery-pending with an explicit error, and its ClaimID is not reused. If the claim still exists, recovery leaves it in place so a later retry can read a late binding, check ownership, and confirm cleanup. If the claim has disappeared before any worker identity was learned, this adapter cannot resolve the attempt automatically. Enumerating sandboxes through verified owner references or equivalent correlation is a promotion requirement for that case.

A worker that the controller assigns to a second claim while it still belongs to a first one is a conflict: the second attempt fails closed and its upstream claim is deliberately left in place, because deleting it under `shutdownPolicy: Delete` can destroy the worker the first claim is still using. This check applies to normal binding, initial compensation, and recovery retries, including identities already saved by an earlier recovery. Such an attempt is terminal and needs operator intervention.

Recovery retains its worker reservation across deletion failures and until release is confirmed, preventing another local allocation from acquiring a worker still being removed. These reservations do not expose a usable allocation identity through Observe, Start, Terminate, or Cleanup. A changed or missing binding on a retained claim also stops recovery before deletion.

The binding check and deletion are separate kubectl requests, not an atomic upstream identity precondition. The spike assumes the binding/resource name remains stable between them. Production promotion needs verified immutable identity/precondition handling and real-cluster evidence for concurrent resource changes.

### Legacy compatibility audit (Ticket #30)

| Concrete methods | Current interpretation |
| --- | --- |
| AddTemplate / AddWarmPool | Adapter setup, outside the shared contract. |
| AddClaim / BindClaim | Legacy local Pending/Bound bookkeeping; allocation correlation in the reduced path uses Allocate. |
| StartClaim | Returns ErrUnsupported after checking the legacy claim; a Ready worker remains Bound. |
| SucceedClaim / FailClaim / ExpireClaim | Retained for source compatibility. Their local phases and replacement flags are not verified work or cleanup evidence; new application code must use its own outcome state and the reduced resource operations. |
| Claim / PoolStatus | Legacy local/approximate views, not the future authoritative run-service or backend evidence API. |

The ordinary integration gate checks Allocate, identity-matched Observe, explicit unsupported Start/Terminate, and independent absence of claim and Sandbox after Cleanup. The opt-in #51 gate additionally builds/loads its disposable worker, observes a real claim-bound task result and stop acknowledgement, then checks claim, Sandbox and Pod absence. It does not prove the filesystem boundary or final demo-agent integration.

## Known Gaps

1. Upstream claims use conditions rather than Agenova work phases.
2. Upstream has no Agenova `SucceedClaim` or `FailClaim` primitive; terminal state is currently held in adapter memory and is lost after restart.
3. Pool status exposes less detail; Agenova counts are approximated from local state.
4. The spike is only accurate for the validated single-pool path.
5. `Claim()` returns status but not the original claim spec.
6. Gateway transport, claim identity, external-egress controls, and durable facts are not integrated with the Kubernetes path.
7. Workers cannot be enumerated independently of their claim, so if the claim disappears before the worker is observed, its release cannot be confirmed and non-allocation cannot be proven.
8. The worker filesystem layout, writable task directory, synthetic HOME/cache placement, and outside-boundary enforcement are not yet mapped or verified.

## Integration Gate

Prerequisites:

- `kubectl`;
- a reachable cluster context;
- compatible Agent Sandbox CRDs and controller;
- permission to create and delete test resources.

The harness requires an explicit context, uses unique resource names for each run, and bounds every kubectl command. If allocation identity or resource release cannot be confirmed, it retains resources for manual recovery rather than bypassing adapter checks with a blind delete.

Run:

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

For the opt-in #51 worker-control proof, build and load the disposable test image first, then run only the controlled test against an explicitly selected context and namespace:

```powershell
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
go test -count=1 -v -tags integration -timeout 5m ./harness/integration/agentsandbox/ -run '^TestControlledRuntimeBackend_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace default
```

The test uses unique names and cleans only its own resources. It never creates or deletes the cluster. Do not run the #50 cluster teardown from a checkout without that cluster's ownership receipt.

## Promotion Criteria

Before describing this as a supported backend:

- preserve terminal claim state across adapter restart;
- enumerate workers independently of the claim (owner reference or label selector) so recovery can confirm release without a prior status observation;
- return complete claim identity/spec data;
- calculate per-pool status correctly;
- run the applicable shared contract cases;
- map and verify the Ticket #89 filesystem boundary, including outside and cross-claim negative cases;
- prove gateway-only external access or clearly state the network limitation;
- capture reproducible cluster, resource, event, and log evidence.
