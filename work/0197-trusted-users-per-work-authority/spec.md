# Feature Specification: Trusted users and per-Work organizational authority vertical

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- PRD outcome: [`Declarative request and authorization resolution`](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [`Claim-scoped authority`](../../docs/product/prd.md#4-claim-scoped-authority), and [`Facts and accountability`](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

Agenova must make organization rules meaningful for both assignment admission and temporary capability without defining one universal Policy language. Two authenticated users may request the same Work through one service and one AgentTemplate, yet receive different authority because one immutable evaluation applies different constraints. The worker must then prove which issued claim it represents before a Gateway uses that authority. Human identity, Policy evaluation, worker identity, and external provider credentials remain separate replaceable trust boundaries.

## In Scope

- External signed-JWT validation and a backend-neutral verified principal containing bounded subject, issuer/authentication context, team/groups, and roles.
- Operation authorization for Work submission, evidence reads, Policy registration, and AgentTemplate registration.
- A backend-neutral Policy evaluation context/result/evaluator contract with optional authority constraints.
- The current exact-match PolicyBundle as a compatible reference evaluator, including optional reference-rule ceilings for the E14 proof.
- Immutable authority resolution and narrowing provenance using the exact evaluation that admitted the assignment.
- Short-lived workload identity bound to one claim/worker and verified by both Tool and Model Gateways.
- CLI/API/Portal evidence parity and secret-free adapter-selected integration evidence.

## Out of Scope

- Token issuance, OIDC discovery, refresh/rotation UI, user directory synchronization, or an IdP.
- General provider-secret management, EKS/Bedrock adapter implementation, and AWS infrastructure.
- Kubernetes Operator, Kubernetes CRDs/controllers, OPA/Cedar integrations, and a general Policy DSL.
- Broad administration, policy editing UI, persistent Work history, HA, or workflow/claim lineage.

## Requirements

- The HTTP boundary accepts only `Authorization: Bearer` for protected application routes. It validates a configured signing algorithm/key, exact issuer, intended audience, expiry, and bounded claims before constructing a principal. It never trusts `ClaimRequest`, query parameters, cookies, or arbitrary identity headers as authority.
- Required identity fields are subject, issuer, authentication context, one team, zero or more groups, and zero or more roles. Duplicate/conflicting team sources, unknown critical claims, invalid UTF-8, oversized values, or ambiguous mappings fail closed. Evidence exposes the bounded principal fields needed for accountability, not the raw JWT or raw metadata.
- Protected operations use stable actions: `work.submit`, `evidence.read.own`, `evidence.read.any`, `policy.register`, and `agent-template.register`. An owner is the verified principal subject recorded at admission. Listing returns only records visible to that principal unless `evidence.read.any` is authorized.
- A Policy evaluation context contains only trusted principal attributes, a stable action, a backend-neutral resource descriptor, and bounded trusted environment facts. It contains no Kubernetes object, provider SDK type, credential, raw JWT, or caller-authored authority.
- A Policy evaluation returns one typed decision, immutable Policy ID/version, stable reason codes, and optional `AuthorityConstraints`. The result is cryptographically or internally bound to the complete evaluation context so it cannot authorize another request.
- `AuthorityConstraints` can limit tools, resource scopes, model profiles, memory scopes, runtime profiles, and maximum timeout. Effective authority is the deterministic intersection of request, AgentTemplate ceiling, returned constraints, and runtime restrictions.
- The reference evaluator maps its existing team/action/project/template rules into the generic context. It selects exactly one rule; ambiguous matches fail closed. A missing reference ceiling preserves admission-only compatibility, while explicitly empty limits grant nothing in that dimension.
- An evaluator cannot add an unrequested value, exceed the template, select an unrequested model/runtime profile, or change task input. If a requested non-empty required dimension resolves completely empty, issuance fails before allocation rather than silently producing unusable authority.
- A workload credential is signed by a control-plane key unavailable to the worker, contains issuer, allowed Gateway audiences, issued/expiry time, claim ID, and worker ID, and expires no later than the claim deadline. The raw token is neither evidence nor a public API field.
- Gateway authentication verifies the credential before claim/authority lookup. Verified claim ID is authoritative; a caller-nominated claim ID is only a consistency check. The Gateway then confirms worker binding, active `Running` phase, authority reference, and operation scope before invoking an adapter.
- Terminal claims, revoked credentials, expired credentials, missing worker binding, and mismatched nominated targets deny with a stable category and zero external calls. Denials are attributed only when the verified credential supplies a safe claim identity.
- CLI reads a user token from an explicitly configured protected file and adds it only to HTTPS or the existing loopback tunnel. Portal development/connected mode uses a server-side proxy that reads the token; browser code does not receive or persist it. Tokens are redacted from errors, logs, snapshots, evidence, and test artifacts.
- The installed reference mode must explicitly choose authenticated multi-user mode or the existing labelled fixed-principal local mode. Live authenticated mode never falls back to the fixed principal on verification failure.
- Adapter evidence is selected from the resolved Platform lock. Shared tests and Policy contexts never branch on Kubernetes, AWS, Ollama, Bedrock, OPA, or Cedar vocabulary. Provider/evaluator-specific gates prove only the selected implementation's claims and report unsupported/missing implementations as blockers.

## Negative Cases

- Missing, malformed, expired, wrong-issuer, wrong-audience, invalid-signature, unsupported-algorithm, forged-team, conflicting-team, and oversized identity inputs fail before application authorization or side effects.
- An ordinary submitter cannot register Policy/templates or read another subject's evidence. A forged owner reference in a URL or request body does not change visibility.
- A Policy evaluation cannot expand request/template authority; context/result mismatch, invalid decisions, malformed constraints, missing snapshots, ambiguous reference rules, and same-version changed reference content fail closed.
- A claim-A workload token paired with claim B's nominated ID, a different worker, wrong Gateway audience, terminal claim, or expired time never reads claim B for execution and produces zero provider calls.
- Authentication/authorization failures never include JWTs, signing keys, raw trusted metadata, provider credentials, task input, or untrusted adapter errors.

## Compatibility

- `ClaimRequest` remains unchanged and principal-free. `SandboxClaim`, `RuntimeBackend`, lifecycle phases, and backend-neutral evidence ownership remain unchanged.
- Existing admission-only Policy documents remain valid and retain their current authority behavior. New ceiling-bearing documents are versioned content and same-identity/different-content conflicts remain errors.
- `PolicyBundle` remains the bundled reference document, not the shape required of external evaluators. Adding OPA, Cedar or a company engine must not change ClaimRequest, SandboxClaim or Gateway contracts.
- The explicitly selected local fixed-principal mode remains available for its existing demo/tests and is labelled non-production. Authenticated mode has no fallback to it.
- Existing in-process Gateway tests may use a trusted verifier fixture, but live paths cannot accept bare claim IDs as authentication.
- Current kind/Agent Sandbox/Ollama behavior remains supported. EKS/Bedrock vocabulary and credential mechanics remain inside future E13 adapters.

## Open Decisions

- Owner/Reviewer approval is required for the selected compatibility strategy: keep richer verified identity internal and project only stable subject/team/authentication context into current public evidence.
- Owner/Reviewer approval is required for the boundary that #197 supplies management-route authorization against current private routes while E13/#146 own any later protected routing integration.
- Kubernetes Operator and repository reorganization require a separate Ticket/task packet; they are intentionally not hidden inside E14.
