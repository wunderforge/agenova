# Backend Capability Matrix

Status labels:

- `Supported`: appears covered by Agent Sandbox design or Phase 0 reference behavior.
- `Needs verification`: plausible but must be proven with upstream docs, installation, or behavior evidence.
- `Not supported`: not expected from Agent Sandbox as an execution substrate.
- `Agenova-owned`: must remain above the backend adapter.

| Capability | Required for any backend | In-memory reference | Agent Sandbox status | Owner | Evidence / next check |
| --- | --- | --- | --- | --- | --- |
| Worker image and command template | Yes | Supported | Needs verification | Backend adapter | Map Agenova template fields without exposing upstream CRD shape. |
| Warm idle capacity | Yes | Supported | Needs verification | Backend adapter | Verify warm pool or equivalent capacity behavior. |
| Claim as one worker-run lease | Yes | Supported | Needs verification | Agenova contract plus backend adapter | First spike target: acquisition semantics and status mapping. |
| Claim lifecycle observation | Yes | Supported | Needs verification | Backend adapter | Map to `Pending`, `Bound`, `Running`, terminal phases. |
| Sandbox cleanup / replacement evidence | Yes | Supported | Needs verification | Backend adapter | Must stay resource evidence, not a terminal claim phase. |
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
