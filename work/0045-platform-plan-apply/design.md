# Technical Design: idempotent Platform plan/apply

- Ticket: [#45](https://github.com/wunderforge/agenova/issues/45)
- Feature spec: `spec.md`

## Current State and Constraints

The v1alpha1 Platform parser/resolver and bundled adapter lifecycle exist. They validate a backend-neutral desired state but perform no target operations. Kubernetes must remain one deployment adapter, not a command-layer dependency, and bootstrap authority comes only from the caller's existing credentials/RBAC.

## Decision

Add a small `platformapply.Service` that owns parse, resolve, activation, planning, confirmation-independent apply orchestration, and safe status. It receives an adapter lifecycle plus a capability constructor; it does not branch on adapter IDs. A narrow `DeploymentAdapter` constructed by the registry owns target preflight, plan, apply, and readiness.

The Kubernetes deployment implementation uses fixed-argument `kubectl` process execution (no shell), adapter-owned context/namespace, declarative `kubectl apply`, and fixed Agenova resource names. It reconciles:

1. namespace;
2. Platform revision/adapter-lock ConfigMap;
3. versioned reference policy ConfigMap;
4. one minimal internal `agenova-control-plane` Deployment;
5. one ClusterIP Service;
6. rollout readiness.

The control-plane binary exposes only `/readyz` and `/v1/status` inside the cluster. This is reference-install evidence, not the full product API. The public status excludes private adapter configuration.

## Ownership and Contract Boundaries

- `internal/platformapply`: orchestration types, parser, stable plan/status, activation, and target-neutral tests.
- `internal/adapterregistry`: exposes construction through the existing lifecycle.
- `internal/adapters/bundled`: Kubernetes deployment implementation and schemas.
- `internal/cli`: argument/confirmation/output only.
- `cmd/agenova-control-plane`: internal reference health/status process.
- `deploy/reference`: reproducible image packaging and reference fixture/readme.

Runtime and model implementations are independently selected and activated but are not called by the deployment adapter. Model calls remain governed by the existing Model Gateway.

## Alternatives Considered

- ConfigMaps only: rejected because it could not truthfully demonstrate a Ready Agenova deployment.
- Helm/operator/CRDs: rejected as production lifecycle scope beyond MVP.
- Kubernetes branches in CLI/orchestrator: rejected because it violates adapter neutrality.
- Deploy the loopback demo console: rejected because its host-local identity/runtime assumptions are not an in-cluster control-plane contract.

## Verification Strategy

- Pure/fake tests: parse/resolve, stable plan/JSON, exact activation, independent adapter choices, no-op second apply, wrong implementation, preflight denial, partial failure, confirmation, and CLI compiled smoke.
- Real kind: build/load reference image, first apply Ready, inspect records/status, second apply no changes and stable resource counts, then a restricted service account demonstrates RBAC denial with zero resources.
- Repository baseline after focused tests.

## Risks and Compatibility

- `kubectl` output/version drift: use JSON output for reads and stable fixed manifests for writes; cover command construction with fakes.
- Partial target failure: order mutations, report completed/failed/pending components, and make re-apply convergent.
- Image availability: the documented kind path builds and loads a local image; remote distribution is not claimed.
- Existing commands and Platform contract remain compatible.
