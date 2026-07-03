# Backend Capability Matrix

Status labels:

- `Supported`: appears covered by Agent Sandbox design or Phase 0 reference behavior.
- `Needs verification`: plausible but must be proven with upstream docs, installation, or behavior evidence.
- `CRD installed`: CRD confirmed present in a local kind cluster (Phase 2 install gate); runtime behavior still needs E2E evidence.
- `E2E verified`: behavior confirmed by running SpikeAdapter against real kind cluster (Phase 2 adapter gate).
- `E2E gap`: partially supported; semantic difference documented in `internal/runtime/agentsandbox/doc.go`.
- `Not supported`: not expected from Agent Sandbox as an execution substrate.
- `Agenova-owned`: must remain above the backend adapter.
- `Not required`: intentionally outside the current MVP requirement.
- `Future work`: reserved for later specs; do not implement under Phase 1 scope.

Phase 2 install gate completed 2026-06-15 on `worker/phase2-upstream-install`. Agent Sandbox v0.4.6 installed on kind cluster `agenova-k8s-lab` (k8s v1.36.1). Source: https://github.com/kubernetes-sigs/agent-sandbox. Evidence: `docs/evidence/phase-2/upstream-agent-sandbox-install-or-blocker/`.

Phase 2 adapter gate completed 2026-06-15 on `worker/phase2-agent-sandbox-adapter`. SpikeAdapter implemented in `internal/runtime/agentsandbox/`. E2E tests passed against `kind-agenova-k8s-lab`. Evidence: `docs/evidence/phase-2/claim-lifecycle-e2e/`, `docs/evidence/phase-2/kubectl-runtime-state/`, `docs/evidence/phase-2/backend-neutral-api/`.

| Capability | Required for any backend | In-memory reference | Agent Sandbox status | Owner | Evidence / next check |
| --- | --- | --- | --- | --- | --- |
| Worker image and command template | Yes | Supported | E2E verified | Backend adapter | `AddTemplate` creates `SandboxTemplate` CRD with `podTemplate.spec.containers`. E2E confirmed with `busybox:stable`. Mapping: Agenova `Image`+`Command` -> upstream `podTemplate.spec.containers[0].image`+`command`. |
| Image-based runtime templates | Yes | Supported | E2E verified | Agenova contract plus backend adapter | MVP uses image-based runtime templates. Shared runtime image plus claim-time agent artifact loading is explicitly future work, not a required backend capability for Phase 1. |
| Warm idle capacity | Yes | Supported | E2E verified | Backend adapter | `AddWarmPool` creates `SandboxWarmPool` CRD. Controller pre-creates idle sandbox pods (OnReplenish). Replenishment confirmed: after SucceedClaim, a new idle pod appeared within 2s. |
| Per-agent dedicated warm pools | No | Not required | Not required | Operational policy | Warm pools are selected runtime/template capacity, not a semantic guarantee for every agent role. Low-frequency agents may use cold acquisition. |
| Claim as one worker-run lease | Yes | Supported | E2E verified | Agenova contract plus backend adapter | `AddClaim` creates `SandboxClaim` CRD. Controller emits `SandboxAdopted` event when assigning sandbox. `status.sandbox.name` carries the sandbox pod name (Agenova `SandboxID`). |
| Claim lifecycle observation | Yes | Supported | E2E gap | Backend adapter | `BindClaim`/`StartClaim` poll for controller-driven transitions (no direct API trigger). Gap: upstream uses `status.conditions` (Ready=True), not a phase field. Adapter may map sandbox adoption to `Bound` and readiness to conditions, but `Ready=True` must not become `Succeeded`. See `internal/runtime/agentsandbox/doc.go` gap 1. |
| Sandbox cleanup / replacement evidence | Yes | Supported | E2E verified | Backend adapter | OnReplenish strategy confirmed: controller created replacement pod within 2s of claim deletion. Sandbox replacement evidence recorded in adapter-local `SandboxReplaced=true` state. |
| Backend-neutral contract tests | Yes | Supported | E2E gap | Agenova contract | `contracttest.Run` passes against in-memory backend. The SpikeAdapter cannot pass the full contract test suite because SucceedClaim/FailClaim have no upstream equivalent. See gap 2 in `internal/runtime/agentsandbox/doc.go`. Requires upstream CRD spec change or local state overlay for full compliance. |
| Explicit terminal phases (Succeeded/Failed) | Yes | Supported | E2E gap | Backend adapter | Gap: upstream `SandboxClaim` has no Succeeded/Failed phase field. Adapter deletes the claim on terminal transitions and records phase locally. State is not durable in the upstream CRD. If the adapter process restarts, terminal claim state is lost. |
| Pool status granularity | Yes | Supported | E2E gap | Backend adapter | Gap: upstream `SandboxWarmPool.status` only exposes `readyReplicas` and `replicas`. Agenova's `IdleSandboxes`/`BoundClaims`/`RunningClaims`/`ReplacedSandboxes` breakdown is approximated from local adapter claim state. |
| Scheduled deletion / cleanup | No | Not implemented | E2E verified | Backend adapter | `lifecycle.shutdownPolicy=Delete` + `ttlSecondsAfterFinished` confirmed in SandboxClaim spec. Controller cleans up pods after claim deletion. |
| Stateful sandbox storage | No | Not implemented | Needs verification | Backend substrate | Upstream `SandboxTemplate.spec.podTemplate.spec` accepts standard pod volumes. Storage lifecycle ownership not tested. |
| runtimeClass / placement / node pool integration | No | Not implemented | Needs verification | Backend substrate | Standard pod scheduling fields available via `podTemplate.spec`. Not tested in spike. |
| Isolation tiers | No | Not implemented | Future work | Agenova contract plus backend substrate | Isolation tiers are not MVP fields. Future specs may map tier requirements to runtimeClass, dedicated nodes, stronger backends, or other substrate controls. |
| Strong hostile-agent isolation | No | Not supported | Needs verification | Backend substrate | Ordinary Pod isolation is insufficient; verify stronger substrate options before claiming hostile-agent isolation. |
| External credential isolation behind gateways | Yes | Static fixture only | Not supported | Agenova-owned | Gateways own upstream credentials; sandboxes must not hold them. |
| Deny-by-default sandbox external egress | Yes | Static fixture only | Needs verification | Agenova contract plus backend adapter | MVP posture: sandbox code does not directly access external tools, models, cloud APIs, or arbitrary internet destinations. Adapter must prove backend controls for local and EKS environments. |
| Tool Gateway and `ToolInvocation` facts | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Model Gateway and `ModelInvocation` facts | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Shared runtime image plus claim-time agent artifact | No | Not implemented | Future work | Agenova-owned plus runner | May reduce warm pool fragmentation later, but adds artifact verification, dependency isolation, caching, bootstrap, and failure-mode complexity. Not required for Phase 1. |
| Claim-anchored authorization | Yes | Not implemented | Not supported | Agenova-owned | Future gateway/control logic. |
| Parent/child claim governance | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Memory and checkpoint interfaces | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Control Plane / Runtime Plane separation | Yes | Documented | Not supported | Agenova-owned | Product architecture constraint above backend. |
