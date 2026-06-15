# Backend Capability Matrix

Status labels:

- `Supported`: appears covered by Agent Sandbox design or Phase 0 reference behavior.
- `Needs verification`: plausible but must be proven with upstream docs, installation, or behavior evidence.
- `CRD installed`: CRD confirmed present in a local kind cluster (Phase 2 install gate); runtime behavior still needs E2E evidence.
- `Not supported`: not expected from Agent Sandbox as an execution substrate.
- `Agenova-owned`: must remain above the backend adapter.

Phase 2 install gate completed 2026-06-15 on `worker/phase2-upstream-install`. Agent Sandbox v0.4.6 installed on kind cluster `agenova-k8s-lab` (k8s v1.36.1). Source: https://github.com/kubernetes-sigs/agent-sandbox. Evidence: `docs/evidence/phase-2/upstream-agent-sandbox-install-or-blocker/`.

| Capability | Required for any backend | In-memory reference | Agent Sandbox status | Owner | Evidence / next check |
| --- | --- | --- | --- | --- | --- |
| Worker image and command template | Yes | Supported | CRD installed | Backend adapter | `sandboxes.agents.x-k8s.io` CRD installed (Phase 2). Field mapping from Agenova template to Sandbox spec still needs adapter implementation and E2E verification. |
| Warm idle capacity | Yes | Supported | CRD installed | Backend adapter | `sandboxwarmpools.extensions.agents.x-k8s.io` CRD installed (Phase 2). Warm pool semantics need E2E behavioral verification. |
| Claim as one worker-run lease | Yes | Supported | CRD installed | Agenova contract plus backend adapter | `sandboxclaims.extensions.agents.x-k8s.io` CRD installed (Phase 2). Acquisition semantics and status mapping need E2E verification. |
| Claim lifecycle observation | Yes | Supported | CRD installed | Backend adapter | `agent-sandbox-controller` running (Phase 2). Status phase mapping (`Pending`→`Bound`→`Running`→terminal) needs E2E runtime verification. |
| Sandbox cleanup / replacement evidence | Yes | Supported | Needs verification | Backend adapter | Controller manages singleton pod lifecycle. Cleanup primitives need E2E runtime evidence. |
| Backend-neutral contract tests | Yes | Supported | Needs verification | Agenova contract | `internal/runtime/contracttest` suite runs against in-memory backend. Same tests must pass every new backend. |
| Scheduled deletion / cleanup | No | Not implemented | Needs verification | Backend adapter | Verify upstream cleanup primitives. |
| Stateful sandbox storage | No | Not implemented | Needs verification | Backend substrate | Verify storage model and lifecycle ownership. |
| runtimeClass / placement / node pool integration | No | Not implemented | Needs verification | Backend substrate | Verify isolation knobs and scheduling controls. |
| Strong hostile-agent isolation | No | Not supported | Needs verification | Backend substrate | Ordinary Pod isolation is insufficient; verify stronger substrate options. |
| External credential isolation behind gateways | Yes | Static fixture only | Not supported | Agenova-owned | Gateways own upstream credentials; sandboxes must not hold them. |
| Tool Gateway and `ToolInvocation` facts | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Model Gateway and `ModelInvocation` facts | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Claim-anchored authorization | Yes | Not implemented | Not supported | Agenova-owned | Future gateway/control logic. |
| Parent/child claim governance | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Memory and checkpoint interfaces | No | Not implemented | Not supported | Agenova-owned | Future phase. |
| Control Plane / Runtime Plane separation | Yes | Documented | Not supported | Agenova-owned | Product architecture constraint above backend. |
