# Kubernetes Agent Sandbox Adapter

Status: verified spike, not a production backend.

Kubernetes Agent Sandbox is the first real substrate used to test the `RuntimeBackend` boundary. It is not the Agenova product API and is not required by application agents.

## Verified Spike Behavior

- Agenova runtime image and command map to an upstream `SandboxTemplate`.
- A selected runtime template can have a `SandboxWarmPool`.
- One Agenova claim maps to one upstream sandbox acquisition.
- Upstream sandbox identity is returned as backend evidence.
- Upstream readiness can support the `Bound` to `Running` observation path.
- Claim deletion triggers sandbox cleanup and warm-pool replenishment.
- Upstream API details remain confined to `internal/runtime/agentsandbox`.

The path was exercised with Agent Sandbox v0.4.6 on a local kind cluster. It must be re-run when promoting or changing the adapter.

To reproduce the pinned upstream test substrate itself (no adapter involved), see [harness/spike/agent-sandbox-substrate/RUNBOOK.md](../../harness/spike/agent-sandbox-substrate/RUNBOOK.md).

## Known Gaps

1. Upstream claims use conditions rather than Agenova work phases.
2. Upstream has no Agenova `SucceedClaim` or `FailClaim` primitive; terminal state is currently held in adapter memory and is lost after restart.
3. Pool status exposes less detail; Agenova counts are approximated from local state.
4. The spike is only accurate for the validated single-pool path.
5. `Claim()` returns status but not the original claim spec.
6. Gateway transport, claim identity, external-egress controls, and durable facts are not integrated with the Kubernetes path.

## Integration Gate

Prerequisites:

- `kubectl`;
- a reachable cluster context;
- compatible Agent Sandbox CRDs and controller;
- permission to create and delete test resources.

Run:

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

## Promotion Criteria

Before describing this as a supported backend:

- preserve terminal claim state across adapter restart;
- return complete claim identity/spec data;
- calculate per-pool status correctly;
- run the applicable shared contract cases;
- prove gateway-only external access or clearly state the network limitation;
- capture reproducible cluster, resource, event, and log evidence.
