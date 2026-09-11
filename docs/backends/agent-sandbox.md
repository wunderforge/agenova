# Kubernetes Agent Sandbox Adapter

Status: verified spike, not a production backend.

Kubernetes Agent Sandbox is the first real substrate used to test the `RuntimeBackend` boundary. It is not the Agenova product API and is not required by application agents.

## Verified Spike Behavior

- Agenova runtime image and command map to an upstream `SandboxTemplate`.
- A selected runtime template can have a `SandboxWarmPool`.
- One Agenova claim maps to one upstream sandbox acquisition.
- Upstream sandbox identity is returned as backend evidence.
- Upstream readiness is `Bound`-level infrastructure evidence. Both `Start` and the legacy `StartClaim` report unsupported work-start semantics; neither promotes readiness to Running.
- Claim deletion triggers sandbox cleanup and warm-pool replenishment.
- Upstream API details remain confined to `internal/runtime/agentsandbox`.

The earlier spike was exercised with Agent Sandbox v0.4.6 on a local kind cluster. That historical run does not verify the reduced contract or the changed legacy start behavior. The current integration gate must be re-run; see the [current environment blocker](../evidence/30/agent-sandbox/summary.md).

## Reduced RuntimeBackend Contract (Ticket #30)

Support of the five reduced operations against the upstream controller, verified with a simulated controller at the `kubeClient` seam (`internal/runtime/agentsandbox/allocation_test.go`); real-cluster evidence for these operations is still a blocker (no verified test kube context in the current owner environment).

| Operation | Support | Notes |
| --- | --- | --- |
| Allocate | supported, with an unprovable recovery case | Creates the upstream claim and waits for sandbox assignment. Recovery checks and reserves the assigned worker before deleting the claim, and completes only after both resources are confirmed absent. Unknown identities stay recovery-pending; conflicting identities retain their upstream claim without deletion. |
| Observe | supported (readiness only) | `Ready` mirrors the upstream `Ready=True` condition, and only when the claim still carries the recorded worker; a different or missing worker is reported as an identity mismatch. Replacement is not observable and stays `false`. Query failures are returned as errors. |
| Start | unsupported | The controller starts the pod on its own; there is no channel to acknowledge actual work start. |
| Terminate | unsupported | No worker-stop evidence exists apart from resource deletion. |
| Cleanup | supported (release) | Rechecks the recorded worker binding before deletion; changed/missing bindings or query failures prevent deletion. `Released` is reported only after both the claim and the assigned sandbox are confirmed absent; failures are explicit and retryable after identity/release can be confirmed. |
| Filesystem boundary | unsupported | Allocation and observation explicitly report `FilesystemEvidenceUnsupported`. Ticket #48 must map the worker-visible directory and mount/profile behavior; #51 must provide real isolation evidence before this can become `BackendVerified`. |

`kubectl` invocations carry a per-command deadline and resource absence is classified by exit status and empty output, never by error text.

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

The integration gate now checks Allocate, identity-matched Observe, explicit unsupported Start/Terminate, and independent absence of both claim and sandbox after Cleanup. It also checks cleanup without work start and idempotent release. The former expiry bookkeeping case is preserved as a unit regression. Historical assertions that Ready proves Running or deletion proves replacement are removed from the integration gate.

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
