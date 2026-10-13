# Task: Define the governed BAU/SRE investigation contract and threat boundary

- Ticket: [#220](https://github.com/wunderforge/agenova/issues/220)
- Mission: Define and prove the smallest backend-neutral operation, resource-scope, and evidence contract needed for a governed read-only BAU/SRE investigation.
- Target: `internal/gateway`, `internal/toolgateway`, sanitized contract fixtures, and task-local specification/design evidence.
- User value: An existing troubleshooting agent can request concrete enterprise read operations while Agenova rejects ambiguous scope, mutation, cross-claim use, and terminal-claim use before provider dispatch.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md)
- [`design.md`](design.md)
- [`docs/product/architecture-contract.md`](../../docs/product/architecture-contract.md), especially Submission and Resolution, Authority and Credentials, and Evidence Surfaces
- [`work/0033-governed-invocation-contracts/spec.md`](../0033-governed-invocation-contracts/spec.md), the existing governed invocation contract
- [`internal/gateway/validate.go`](../../internal/gateway/validate.go) and [`internal/toolgateway/gateway.go`](../../internal/toolgateway/gateway.go)
- [`internal/toolgateway/gateway_test.go`](../../internal/toolgateway/gateway_test.go)
- [`docs/harness/playbooks.md`](../../docs/harness/playbooks.md), Start a GitHub Ticket and Change a Core Contract

## Scope

In scope:

- A generic logical-operation vocabulary for the E21 read-only categories without vendor or company-specific types.
- A canonical, exact resource-scope grammar for work items, repositories, Kubernetes resources, metric targets, dashboards, and GitOps applications.
- Structural validation and stable fail-closed categories before claim attribution or provider dispatch.
- Sanitized allow/deny fixtures and focused tests, including a positive provider-call control.
- An explicit E14/E16/E18 dependency and gap matrix.

Out of scope:

- Real work-tracker, source-control, Kubernetes, observability, or GitOps adapters.
- Company-specific skills, topology, identifiers, policies, ticket contents, or credentials.
- Worker image, kind deployment, Helm/Argo packaging, draft merge requests, or live mutation.
- Replacing E14 identity, E16 provider transport/catalog, or E18 durable evidence.

## Acceptance Criteria

- Every E21 pilot operation and resource category has one documented backend-neutral representation and validation rule.
- A malformed, wildcard, non-canonical, or unsupported resource scope is denied before claim attribution, policy evaluation, and provider dispatch with a stable category.
- Existing exact `repo:owner/name` scopes remain compatible.
- Out-of-scope resources, mutation operations, cross-claim requests, and non-Running claims remain denied with zero provider calls.
- At least one in-scope read operation reaches a deterministic adapter spy exactly once and records one correlated Allow fact.
- Sanitized fixtures contain no company identifier, credential, kubeconfig, private endpoint, or secret value.
- The dependency matrix identifies which remaining capabilities belong to E14, E16, and E18.

## Negative Case

- A Running claim granted `kubernetes.get` for one exact namespace resource cannot use a wildcard, a differently encoded equivalent, another namespace, or a mutation operation; each attempt is denied before adapter invocation and cannot acquire authority from task/model/tool content.

## Execution Todo

- [x] Scout the existing authority resolver, tool Gateway, worker protocol, facts/evidence, E14/E16/E18 dependencies, and tests.
- [x] Assignee self-review: the task elaborates existing PRD authority/evidence outcomes, preserves exact grants and backend neutrality, and does not introduce a company integration or production claim.
- [x] Freeze the logical-operation and resource-scope contract in `spec.md`.
- [x] Implement canonical resource-scope parsing/validation behind the existing Tool Gateway boundary.
- [x] Add sanitized contract fixtures and the allow/deny matrix tests.
- [x] Record the dependency/gap matrix and implementation decisions in `design.md`.
- [x] Run focused gates and all Go packages; record the unavailable PowerShell wrapper precisely.
- [x] Review the diff for scope, regressions, confidential data, and source-of-truth changes.

## Quality Gates

- `go test ./internal/gateway ./internal/toolgateway ./internal/authority`
- `go test ./api/v1alpha1 ./internal/app ./internal/facts ./internal/workerprotocol`
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`
- `git diff --check`

## Evidence Required

- Focused test output for canonical scopes, structural denial, exact authority enforcement, terminal claims, cross-claim behavior, and positive/negative provider call counts.
- Full repository gate output or the exact missing-environment blocker.
- A sanitized fixture scan and confirmation that no company-specific identifier or credential material is present.
- The E14/E16/E18 dependency/gap matrix and residual risks that block M2.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep provider/vendor shapes in adapters; shared scope types express logical resources only.
- Do not treat a bare claim ID as worker authentication or static YAML as runtime/network evidence.
- Do not commit company-specific data to this public repository.

## Decisions and Blockers

- Decision: reuse the existing `Tool.Action` operation model, exact effective-authority enforcement, invocation IDs, decisions, and facts; E21 does not create a parallel policy or evidence engine.
- Decision: validate a closed set of E21 resource kinds at the Tool Gateway boundary while preserving the existing `repo:owner/name` form.
- Decision: capability ceilings may retain one terminal wildcard during resolution, but individual governed invocations must always name one exact canonical resource.
- Blocker: the branch baseline does not yet contain E16's real provider transport/catalog implementation; M1 can define and test the shared contract, while M2 must integrate the accepted E16 boundary instead of duplicating it.
- Verification: focused packages and all Go packages passed on 2026-10-14. Two loopback `httptest` packages initially failed under the filesystem/network sandbox and passed when rerun with local-port permission. See `docs/evidence/220/README.md`.
- Environment blocker: `pwsh` is not installed on this macOS host, so the repository-owned `scripts/check.ps1 -Docs/-All` wrapper and its frontend/browser portions were not executed here.
