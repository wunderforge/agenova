# Feature Specification: Backend-neutral Policy evaluation kernel

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- PRD outcome: [`Declarative request and authorization resolution`](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [`Claim-scoped authority`](../../docs/product/prd.md#4-claim-scoped-authority)

## Intent

Agenova must stop coupling authorization consumers to the concrete reference `PolicyBundle`. M1 introduces a small, backend-neutral evaluation contract and an immutable admission snapshot. The existing exact-match YAML continues to behave as it does today, while a fake external evaluator proves that future engines do not reshape claim, Gateway, or evidence contracts.

## In Scope

- A backend-neutral Policy evaluation context, result, evaluator, and immutable source/snapshot contract.
- Optional `AuthorityConstraints` carried as a trusted cap in the evaluation snapshot.
- Exact-context binding between evaluation and admission.
- The current exact-match `PolicyBundle` as a compatible reference evaluator.
- A reusable contract suite exercised by the reference evaluator and a fake external evaluator.

## Out of Scope

- OIDC/JWT verification, HTTP route authorization, evidence ownership, CLI token transport, or Portal proxying (#203).
- Reference-YAML constraint authoring and the two-user organizational authority scenario (#204).
- Worker JWTs or Gateway authentication (#205).
- kind/Ollama installation and team release packaging (#206).
- OPA/Cedar/company-engine integrations, a general Policy DSL, Kubernetes Operator, EKS, or Bedrock.

## Requirements

- `EvaluationContext` contains a trusted principal projection, stable action, backend-neutral resource descriptor, and bounded trusted environment facts. It contains no Kubernetes object, provider SDK type, credential, raw token, or caller-authored authority.
- `Evaluation` contains one typed decision, immutable Policy ID/version, stable reason data, and optional `AuthorityConstraints`.
- The authorization boundary binds an allowed evaluation to the complete context it evaluated. A different principal, action, resource, or trusted environment cannot reuse it.
- Evaluator and snapshot/source interfaces do not expose reference rule fields or Policy-engine-specific types.
- `AuthorityConstraints` can represent caps for tools, resource scopes, model, memory, runtime, and timeout. M1 validates and defensively copies this data; it does not add reference-YAML authoring semantics.
- Constraints are not authority grants. Downstream code may only intersect them with requested, template, and runtime limits. An absent constraint preserves current admission-only behavior.
- The reference evaluator maps the existing team/action/project/template exact-match document into the generic context and preserves current allow, deny, ambiguity, and immutable-version behavior.
- Evaluator outputs with invalid decisions, invalid Policy references, malformed constraints, or context mismatch fail closed before claim issuance or backend allocation.
- A fake external evaluator can satisfy the same contract without importing `PolicyBundle` or changing public domain contracts.

## Negative Cases

- Context/result mismatch, invalid decisions, invalid Policy references, malformed constraints, missing snapshots, ambiguous reference rules, and same-version changed reference content fail closed.
- Mutating caller-owned context, result, or constraint objects after evaluation cannot alter the bound admission snapshot.
- A permissive fake evaluator cannot make constraints act as a grant or bypass the existing request/template/runtime authority boundaries.

## Compatibility

- `ClaimRequest`, `SandboxClaim`, `RuntimeBackend`, lifecycle phases, Gateway contracts, and public evidence schemas remain unchanged.
- Existing Policy documents remain valid and retain current behavior.
- `PolicyBundle` remains the bundled reference document, not the schema required of external evaluators.
- Existing local, kind, Agent Sandbox, and Ollama paths continue through the reference evaluator without configuration changes.

## Open Decisions

- None. The assignee self-reviewed the narrowed M1 boundary against #197, the PRD, and the architecture contract under the workflow introduced by #186.
