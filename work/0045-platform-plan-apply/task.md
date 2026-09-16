# Task: Implement idempotent `agenova platform plan/apply`

- Ticket: [#45](https://github.com/wunderforge/agenova/issues/45)
- Mission: Reconcile one complete, validated Platform desired state onto an explicitly selected existing Kubernetes target through the shared adapter lifecycle.
- Target: `internal/platformapply`, bundled Kubernetes deployment adapter, CLI composition, reference deployment artifact, and focused kind evidence.
- User value: An operator can inspect, apply, and safely repeat one declarative Agenova installation without hidden authority or provider-specific CLI branches.
- PRD outcome: [`docs/product/prd.md` §6](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Context to Read

- `AGENTS.md`
- `docs/product/prd.md`
- `docs/product/architecture-contract.md#reference-installation-and-bootstrap`
- `internal/platform/resolve.go`
- `internal/adapterregistry/lifecycle.go`
- `internal/adapters/bundled/registry.go`
- `harness/fixtures/contract/v0/inputs/platform/valid-reference.yaml`
- this task packet

## Scope

In scope:

- `platform validate`, `platform plan`, and `platform apply -f` with stable human and JSON output.
- Automatic activation of exact adapter requirements through #151's lifecycle.
- A narrow deployment-adapter interface and Kubernetes reference implementation using the caller's current credentials and adapter-owned context/namespace config.
- Namespace, immutable desired-state/policy records, one minimal internal reference control-plane Deployment and ClusterIP Service.
- Recorded Platform revision, adapter lock, per-component configured/available/used/failed status, confirmation unless `--yes`, and idempotent second apply.
- Focused fake-target tests plus real kind first/second apply and denied-RBAC evidence.

Out of scope:

- Cluster creation, Helm/CRDs/controllers, production upgrades/rollback/HA, external plugins, policy CRUD, AgentTemplate management, credentials, or unimplemented tool/memory/telemetry providers.
- Rebinding or expanding existing claims.
- Replacing the mandatory Agenova Model Gateway with a model backend.

## Acceptance Criteria

- Validation and planning are read-only and run complete generic plus adapter-owned validation.
- A first apply on the selected existing cluster reaches Ready; the same apply reports no changes and creates no duplicate named resources.
- Missing adapters are activated through the shared lifecycle; command code contains no provider identity switch.
- Unknown adapter/version, capability/config errors, wrong target, missing `kubectl`, and denied RBAC fail with actionable, secret-free diagnostics and bounded partial reconciliation.
- Status and persisted records expose the effective revision and adapter lock without private adapter configuration.
- A compiled `agenova` executable performs the workflow.

## Negative Case

- A caller without required Kubernetes RBAC receives a preflight denial and no Agenova resources are created.

## Execution Todo

- [x] Scout implementation, contracts, risks, and dependencies.
- [x] Confirm scope with the Owner through the explicit instruction to continue #45 without another approval pause.
- [x] Expose capability construction through the shared lifecycle.
- [x] Implement pure validate/plan and target-neutral orchestration tests.
- [x] Implement Kubernetes reference reconciliation and internal Ready/status surface.
- [x] Add CLI commands, confirmation, and stable human/JSON output.
- [x] Add fake-target failures and real kind first/second apply plus denied-RBAC evidence.
- [x] Run focused gates and `.\scripts\check.ps1 -All`.
- [ ] Review the diff and record exact evidence in PR and Ticket.

## Quality Gates

- `go test ./internal/platformapply ./internal/adapters/bundled ./internal/cli ./cmd/agenova/...`
- `.\scripts\check.ps1 -All`
- compiled CLI transcript against kind

## Evidence Required

- Stable validate/plan JSON, first apply Ready, second apply no changes, resource-count comparison, and denied-RBAC transcript.
- Test doubles proving deployment/runtime/model selection remains independent and provider-neutral above the adapter boundary.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Kubernetes values live only in deployment/runtime adapter config; no root kube flags.
- Use current kube credentials/RBAC and never persist secrets.
- The reference service binds only inside the cluster through a ClusterIP Service.

## Decisions and Blockers

- Decision: implement the minimum reference packaging as one internal health/status binary, Deployment, Service, desired-state record, and initial policy record. This satisfies the approved install outcome without claiming a production control plane.
- Decision: runtime and model adapters are validated/activated independently; this ticket does not deploy their external prerequisites.
- Evidence: `docs/evidence/45/platform-apply-kind.md` records Ready first apply, zero-change second apply, stable resource count, internal status, and denied-RBAC behavior.
- Full `.\scripts\check.ps1 -All` passed, including Go, frontend unit/build and 48 browser smoke tests (after installing locked UI dependencies in this worktree).
- Blocker: #151 must merge before this stacked branch is rebased and published.
