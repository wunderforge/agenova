# Agenova MVP PRD

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
submit task and requested access as one ClaimRequest
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

## Required Outcomes

### 1. Declarative request resolution

- One backend-neutral `ClaimRequest` carries the task, requested access, and runtime requirements.
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

- Runtime, tool, and model facts are attributable to the correct claim.
- Users can query a claim's lifecycle, invocations, outcome, lineage, and backend evidence through a demo surface.
- Denied requests produce inspectable evidence in the target MVP.

### 6. Demonstrable contributor path

- A new contributor can run the reference scenario locally.
- Work is split into bounded tasks with acceptance criteria and evidence.
- The final demo follows one reproducible golden path rather than disconnected modules.

## MVP Deliverables

- Backend-neutral `ClaimRequest`, `SandboxClaim`, and `RuntimeBackend` contracts plus reference implementation.
- Contract, authorization, facts, and lineage tests.
- One runnable local golden demo exposed through CLI or an equivalent executable entrypoint.
- One real backend adapter evidence path; Kubernetes Agent Sandbox is the current candidate.
- A queryable claim/evidence output suitable for CLI and later UI use.
- One example multi-agent scenario, such as engineer plus reviewer.
- Quickstart, contributor guide, and quality gates.

## Out of Scope Unless Re-Prioritized

- General workflow DAG orchestration.
- Multiple production-grade backend adapters.
- A complete Memory platform or rollback system.
- Managed cloud control plane, tenancy, billing, or enterprise policy authoring.
- Production-grade Kubernetes controllers, CRDs, Helm release, or high availability.
- Claims of hostile-agent isolation without runtime and network evidence.
- A large chaos lab independent of the golden demo.

## Acceptance Scenario

The MVP is accepted when a teammate can reproduce this behavior:

1. Submit an `engineer` `ClaimRequest` from YAML and observe its resolved, system-managed claim.
2. Show that requested access outside the applicable limits is absent from effective authority.
3. Observe backend allocation and claim transition to `Running`.
4. Execute one allowed tool call and one allowed model call.
5. Attempt at least one denied request and observe evidence without an external call.
6. Submit a child `reviewer` request and show facts attributed to the correct claim.
7. End the parent or worker claim and prove further governed access is denied.
8. Query a single evidence view containing claim outcome, effective authority, lineage, invocations, and backend identity.
9. Run the reference path locally and demonstrate the supported runtime portion on the selected real backend.

## Success Measures

- New contributor reaches a passing reference scenario from the README.
- Every merged MVP task has reproducible evidence.
- The golden demo runs without manual code changes.
- The team can explain which behavior is reference-only, backend-verified, or not implemented.

## Open Product Decisions

- Final product name for the public claim resource: keep `SandboxClaim` or evolve toward `RuntimeClaim`.
- Exact minimal `ClaimRequest`, effective-authority, fact, and evidence schemas for the executable slice.
- Whether the first demo gateway uses HTTP, gRPC, or an in-process boundary behind a stable interface.
- Which real backend behaviors are committed for the final demo versus recorded as gaps.
- Minimum UI scope after the CLI/evidence schema stabilizes.
