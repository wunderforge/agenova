# Phase 1 Spec: Runtime Backend Boundary

## Contract Shape

Phase 1 introduces a minimal `RuntimeBackend` boundary around the existing Phase 0 lifecycle. The boundary should be shaped by current behavior, not by Kubernetes or upstream Agent Sandbox resource names.

Any implementation satisfying this interface is a valid Agenova runtime backend. Adding or swapping a backend must not require application-facing Agenova API changes.

Minimal Go-shaped contract sketch:

```go
type RuntimeBackend interface {
	AddTemplate(AgentSandboxTemplate) error
	AddWarmPool(SandboxWarmPool) error
	AddClaim(SandboxClaim) error
	BindClaim(name string) error
	StartClaim(name string) error
	SucceedClaim(name string) error
	FailClaim(name string, summary string) error
	ExpireClaim(name string, summary string) error
	Claim(name string) (SandboxClaim, bool)
	PoolStatus(name string) (SandboxWarmPoolStatus, bool)
}
```

Contract rules:

- `AddClaim` creates a pending claim for one worker run / sandbox execution lease.
- `BindClaim` binds a pending claim to exactly one sandbox lease when capacity is available.
- `StartClaim`, `SucceedClaim`, `FailClaim`, and `ExpireClaim` preserve the Phase 0 lifecycle transitions.
- Terminal claim phases remain `Succeeded`, `Failed`, or `Expired`.
- Sandbox cleanup, deletion, or replacement is resource evidence, not a terminal claim phase.
- Backend-specific identifiers, CRD shapes, status names, or SDK objects must not appear in application-facing Agenova APIs.
- Contract tests must run against the in-memory reference backend and be reusable for every future backend.

## Backend Targets

- `in-memory`: reference backend and contract test target.
- `agent-sandbox-adapter`: spike target for Kubernetes SIG Apps Agent Sandbox.
- `future backend adapters`: valid when they satisfy the same `RuntimeBackend` contract without changing application-facing APIs.

## Adapter Boundary

The adapter may translate Agenova templates, pools, claims, and status into upstream resources, labels, annotations, or SDK calls. Application-facing APIs, facts, gateways, and future policy logic must continue to speak Agenova concepts.

## MVP Runtime Rules

The detailed product contract lives in `docs/product/runtime-backend-mvp-contract.md`. Phase 1 implementations must preserve these rules:

- Agent Sandbox and Kubernetes state are infrastructure evidence, not the Agenova product contract.
- Agent Sandbox `Ready=True` must not be mapped to claim `Succeeded`.
- Kubernetes `Pod Running` must not be treated as Agenova claim `Running` in warm-pool mode.
- Upstream sandbox adoption maps to Agenova claim `Bound` and a `SandboxAllocated` condition.
- Upstream sandbox or pod readiness maps to a readiness condition, not a terminal claim phase.
- `Running`, `Succeeded`, `Failed`, and `Expired` are Agenova-owned claim execution phases.
- Failure details should be reasons, conditions, or runtime facts, not many extra top-level phases.
- Warm pool replacement is capacity evidence, not a terminal claim outcome.
- Warm pools are configured for selected runtime templates, not automatically for every agent role.
- MVP sandbox external egress is deny-by-default except for Agenova runtime, Tool Gateway, and Model Gateway paths.
- MVP runtime templates are image-based. Shared runtime images plus claim-time agent artifacts are future work, not a Phase 1 requirement.
- Isolation tiers, dedicated node pools, runtime classes, and stronger sandbox backends are future capability questions, not Phase 1 MVP fields.

## Non-Goals

- No competing Kubernetes sandbox controller until the spike proves one is required.
- No gateway implementation.
- No external credential injection into sandboxes.
- No direct sandbox access to external tools, model providers, cloud APIs, or arbitrary internet destinations.
- No per-agent dedicated warm pool requirement.
- No dynamic agent artifact loading requirement.
- No isolation tier implementation.
- No memory, checkpoint, rollback, UI, tenancy, billing, or managed control plane.
