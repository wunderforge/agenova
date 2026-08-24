# Architecture Contract

This document is authoritative for stable product and system boundaries. The PRD may change; these rules require an explicit architecture decision and maintainer approval. It does not own ticket state or feature-specific implementation plans. See the [AIDLC source-of-truth map](../development/AIDLC.md).

## Product Center

Agenova provides a **claim-scoped governance contract** for reusable agent work.

- A reusable agent role is long-lived.
- A `SandboxClaim` is one agent worker run / scoped assignment.
- A claim is not one tool call.
- Agent code and its framework own prompts, reasoning, plans, and task semantics.
- Agenova owns claim lifecycle, scoped authority, gateway boundaries, facts, lineage, and backend evidence.
- Runtime backends own process execution and substrate capabilities.

## Submission and Resolution

- `ClaimRequest` is the application-facing declaration of one task, requested access, and backend-neutral runtime requirements.
- Caller identity comes from a trusted upstream authentication boundary and must not be self-declared as authoritative data in `ClaimRequest`.
- Possessing or invoking the Agenova CLI grants no authority; Agenova authorizes the caller's action, project, and template at the trusted application boundary.
- A denied submission must not create a claim or allocate a runtime backend.
- Task input does not grant resource access; requested resource scopes must be resolved through the same authority rules as tools, models, and memory.
- YAML, API JSON, and a future `agenova run -f <file>` command must use the same request schema.
- The task remains embedded in `ClaimRequest` for the MVP; a standalone Task resource requires a separately justified lifecycle.
- `SandboxClaim` is the system-managed record of one resolved worker run. Callers do not self-issue claim status or effective authority.
- Request resolution precedes claim creation and must remain backend-neutral.
- The MVP may use a static, versioned, default-deny reference policy bundle. Identity-provider integration and self-service policy administration are separate concerns.

## Backend Neutrality

`RuntimeBackend` is an internal replacement boundary.

- Application-facing types must not depend on Kubernetes CRDs, provider SDK objects, or backend status shapes.
- Backend-specific code and vocabulary stay inside the adapter package.
- Adding or swapping a backend must not change the meaning of a claim.
- The in-memory backend is the reference oracle for shared lifecycle semantics.
- A backend gap must be documented and tested; it must not silently weaken the shared contract.

Kubernetes Agent Sandbox is one adapter target, not an Agenova product dependency. Agenova must not build a competing sandbox lifecycle platform unless verified evidence shows that no suitable backend can carry a required semantic.

## Claim Lifecycle

Shared phases are:

```text
Pending -> Bound -> Running -> Succeeded
Pending -> Bound -> Running -> Failed
Pending -> Bound -> Failed
Pending / Bound / Running -> Expired when the relevant timeout applies
```

- Backend readiness is infrastructure evidence, not agent-work success.
- `Running` starts when the Agenova runner starts claim work, not merely when a Pod or process exists.
- `Succeeded`, `Failed`, and `Expired` are terminal claim outcomes.
- Cleanup, deletion, replenishment, or sandbox replacement is resource evidence, not a claim phase.

## Authority and Credentials

- Requested access is intent, not granted authority.
- Effective claim authority is the intersection of Agent Template limits, applicable caller/project/platform policy, requested access, and runtime restrictions.
- A request may narrow authority but cannot create authority.
- Requests contain scopes and references, never external secret values.
- Authority is anchored to an active claim, not an idle sandbox or network location.
- Warm workers must not hold standing external authority.
- External system credentials remain behind Tool and Model Gateways or the future Memory Interface.
- A sandbox may receive only scoped identity material required to authenticate to Agenova components.
- Gateway policy and tests do not replace network controls, workload identity, or backend isolation evidence.

## Facts and Lineage

- `ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` are append-only facts below a claim.
- Facts must be attributable to the correct claim and must not be cross-assigned between workers.
- Parent/child claims express authority scope and accountability.
- Claim lineage must not grow into workflow scheduling without a separately approved product scope.

## Evidence Surfaces

- CLI JSON, the read-only evidence API, and the React console must consume the same backend-neutral evidence representation.
- An API or UI may transport, validate, and render governance evidence; it must not create a second claim, policy, decision, or evidence model.
- The MVP console is read-only. Claim mutation, policy editing, workflow control, and broad administration require separately approved scope.
- A reference evidence endpoint must bind locally or internally by default and must not be exposed publicly without an upstream authentication boundary.

## Reference Installation and Bootstrap

- The MVP install path targets an existing, explicitly selected test cluster; Agenova does not create the cluster.
- The caller's existing Kubernetes authentication and RBAC authorize installation mutations. An installation file or `ClaimRequest` cannot grant that privilege.
- Installation may seed the first versioned Agenova PolicyBundle. That bundle governs later Agenova control-plane actions; it does not retroactively authorize the bootstrap operation that installed it.
- The supported reference install must be idempotent and must not require embedding administrative credentials in the Agenova CLI or configuration.
- Production upgrades, rollback, high availability, multi-cluster administration, and general policy management require separately approved scope.

## Scope Discipline

- Prefer small interfaces, explicit behavior, and contract tests over broad frameworks.
- Consume backend lifecycle, warm-pool, checkpoint, snapshot, placement, and isolation capabilities when available.
- Do not claim production readiness from in-memory behavior, static manifests, or one local-cluster spike.
- Do not implement future surfaces merely because they appear in the long-term vision or technology list.
