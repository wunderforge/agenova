# Agenova MVP PRD

This document is authoritative for committed MVP outcomes, scope, and product acceptance. It does not own architecture invariants, implementation status, or mutable ticket state. Accepted work that changes an MVP requirement must update this PRD; feature-level elaboration that stays within the requirement belongs in the applicable Ticket task packet under `work/`.

Implementation order and stable Epic boundaries are summarized in the [MVP Epic Map](mvp-epics.md). Mutable ticket state remains in GitHub. See the [AIDLC source-of-truth map](../development/AIDLC.md) for conflict and update rules.

## Product Statement

Agenova gives one agent worker run a standard, backend-neutral governance boundary: a claim, temporary authority, governed interfaces, auditable facts, and execution-backend evidence.

## Problem

Teams can build agents, but each team repeatedly assembles runtime isolation, temporary access, credential proxies, tool/model controls, audit records, and backend integrations. The result is tightly coupled to one runtime and difficult to explain after a run finishes.

## Target Users

- Agent developers who need a safe, consistent way to run reusable agents.
- Platform engineers who need backend choice without changing application contracts.
- Reviewers and operators who need evidence of authority, behavior, outcome, and runtime placement.

## MVP User Journey

```text
receive a trusted principal and one ClaimRequest
  -> authorize claim.create for the requested project and template
  -> validate the request and resolve the Agent Template
  -> derive effective authority from request and policy limits
  -> create one system-managed claim
  -> allocate a runtime
  -> execute governed requests
  -> record facts and outcome
  -> revoke authority
  -> query evidence
```

Canonical target input:

```yaml
apiVersion: agenova.io/v1alpha1
kind: ClaimRequest
metadata:
  name: fix-payment-timeout
spec:
  templateRef: engineer
  projectRef: payments
  task:
    type: repository-change
    input:
      repository: acme/payments
      objective: Fix the payment timeout bug
      baseBranch: main
  requestedAccess:
    tools: [git.read, git.write, github.pull-request]
    resourceScopes: [repo:acme/payments]
    modelProfile: approved-coding-model
    memoryScopes: [team-docs]
  runtime:
    profileRef: standard-isolated
    timeout: 20m
```

```text
agenova run -f tasks/fix-payment-timeout.yaml
```

YAML and API JSON represent one `ClaimRequest` schema. The CLI is a submission client, not a second flag-based contract.

The trusted principal comes from an upstream authentication boundary and is not a caller-controlled field in `ClaimRequest`. The MVP uses one Agenova-side, versioned reference policy bundle with default-deny behavior; it proves policy evaluation, not identity-provider integration or self-service policy administration.

## Required Outcomes

### 1. Declarative request and authorization resolution

- One backend-neutral `ClaimRequest` carries the task, requested access, and runtime requirements.
- An upstream authentication boundary supplies a trusted principal outside `ClaimRequest`.
- Agenova authorizes the principal's action, project, and template against a versioned reference policy before claim creation or backend allocation.
- Requested access is never treated as granted authority.
- Task input identifies what the agent should work on; requested resource scopes identify what it is asking to access.
- Effective claim authority is the intersection of the request, Agent Template limits, applicable caller/project/platform policy, and runtime restrictions.
- Requests contain references and scopes, never long-lived external secret values.

### 2. Claim lifecycle

- One run creates one `SandboxClaim`.
- Valid and invalid lifecycle transitions are explicit and tested.
- Work outcome remains separate from backend cleanup evidence.

### 3. Backend-neutral execution

- The in-memory backend passes the full shared contract suite.
- At least one real backend demonstrates the supported allocation/readiness/cleanup path.
- Backend types do not appear in shared application-facing APIs.
- Unsupported backend semantics are visible gaps, not hidden local assumptions.

### 4. Claim-scoped authority

- Only an active `Running` claim can use governed Tool or Model interfaces.
- Unknown, pending, terminal, and out-of-parent-scope claims are denied.
- Long-lived external credentials are not placed in sandbox configuration.

### 5. Facts and accountability

- Control-plane authorization decisions record the trusted principal, action, allow/deny result, policy ID/version, and reason.
- Runtime, tool, and model facts are attributable to the correct claim.
- Users can query a request or claim's lifecycle, invocations, outcome, lineage, and backend evidence through the CLI and a read-only console backed by the same evidence representation.
- Denied requests produce inspectable evidence in the target MVP.

### 6. Reference installation and initial policy bootstrap

- An authorized operator can install the minimum reference Agenova deployment on an existing test cluster from one declarative configuration.
- The install validates the selected cluster context and relies on the cluster's existing authenticated identity and RBAC; the installation file does not grant its caller administrative authority.
- The first install seeds one versioned default-deny Agenova PolicyBundle for later control-plane actions.
- Repeating the same install is idempotent and does not duplicate named resources or silently change the desired configuration.
- Cluster creation, production lifecycle management, and general policy administration remain outside the MVP.

### 7. Read-only claim console

- A minimal read-only API exposes the stable evidence view by request reference or claim ID.
- A React console uses bounded polling to display live lifecycle, requested versus effective authority, decisions, invocations, lineage, outcome, and backend identity.
- CLI, API, and UI share one evidence contract; the frontend does not create a separate governance model.
- Claim search, mutation controls, policy editing, WebSocket/SSE streaming, and broad administration remain outside the MVP.

### 8. Demonstrable contributor path

- A new contributor can run the reference scenario locally.
- Work is split into bounded tasks with acceptance criteria and evidence.
- The final demo follows one reproducible golden path rather than disconnected modules.

## MVP Deliverables

- Backend-neutral `ClaimRequest`, `SandboxClaim`, and `RuntimeBackend` contracts plus reference implementation.
- Minimal trusted-principal, static policy-bundle, action-authorization, and decision-evidence contracts plus reference evaluator.
- Contract, authorization, facts, and lineage tests.
- One runnable local golden demo exposed through CLI or an equivalent executable entrypoint.
- One supported, idempotent reference installation/bootstrap command for an existing test cluster.
- One real backend adapter evidence path; Kubernetes Agent Sandbox is the current candidate.
- One evidence representation consumed by CLI JSON, a minimal read-only API, and the React claim console.
- One example multi-agent scenario, such as engineer plus reviewer.
- Quickstart, contributor guide, and quality gates.

## Out of Scope Unless Re-Prioritized

- General workflow DAG orchestration.
- Multiple production-grade backend adapters.
- A complete Memory platform or rollback system.
- Managed cloud control plane, tenancy, billing, or enterprise policy authoring.
- OIDC/SSO implementation, user management, self-service policy CRUD, delegated administration, and general privileged cluster-management commands beyond the narrowly scoped reference install.
- Kubernetes cluster creation, production install upgrades/rollback, multi-cluster administration, or embedding administrative credentials in the Agenova CLI.
- Production-grade Kubernetes controllers, CRDs, Helm release, or high availability.
- Claims of hostile-agent isolation without runtime and network evidence.
- A large chaos lab independent of the golden demo.

## Acceptance Scenario

The MVP is accepted when a teammate can reproduce this behavior:

1. On an explicitly selected existing test cluster, install the reference Agenova deployment, seed its initial policy, and repeat the install without duplicating resources.
2. Submit an `engineer` `ClaimRequest` as the trusted Team A principal and observe its resolved, system-managed claim.
3. Submit the same request as Team B and observe denial before claim creation or backend allocation.
4. Show that requested access outside the applicable limits is absent from effective authority.
5. Observe backend allocation and claim transition to `Running`.
6. Execute one allowed tool call and one allowed model call.
7. Attempt at least one denied governed request and observe evidence without an external call.
8. Submit a child `reviewer` request and show facts attributed to the correct claim.
9. End the parent or worker claim and prove further governed access is denied.
10. Query the same evidence representation through CLI JSON and the live read-only console.
11. Run the reference path locally and demonstrate the supported runtime portion on the selected real backend.

## Success Measures

- New contributor reaches a passing reference scenario from the README.
- Every merged MVP task has reproducible evidence.
- The reference install and golden demo run without manual code changes on their documented environments.
- The team can explain which behavior is reference-only, backend-verified, or not implemented.

## Open Product Decisions

- Final product name for the public claim resource: keep `SandboxClaim` or evolve toward `RuntimeClaim`.
- Exact minimal principal, reference-policy, authorization-decision, `ClaimRequest`, effective-authority, fact, and evidence schemas for the executable slice.
- Whether the first demo gateway uses HTTP, gRPC, or an in-process boundary behind a stable interface.
- Which real backend behaviors are committed for the final demo versus recorded as gaps.
- Exact minimum components and packaging used by the reference installation path.
