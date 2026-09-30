# Feature Specification: Trusted users and per-Work organizational authority vertical

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- PRD outcome: [`Declarative request and authorization resolution`](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [`Claim-scoped authority`](../../docs/product/prd.md#4-claim-scoped-authority), and [`Facts and accountability`](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

Agenova must make organization rules meaningful for both assignment admission and temporary capability. Two authenticated users may request the same Work through one service and one AgentTemplate, yet receive different authority because one immutable organizational Policy version applies different ceilings. The worker must then prove which issued claim it represents before a Gateway uses that authority. Human identity, worker identity, and external provider credentials remain three separate trust boundaries.

## In Scope

- External signed-JWT validation and a backend-neutral verified principal containing bounded subject, issuer/authentication context, team/groups, and roles.
- Operation authorization for Work submission, evidence reads, Policy registration, and AgentTemplate registration.
- Policy rules that may carry an organizational authority ceiling in addition to assignment-match fields.
- Immutable authority resolution and narrowing provenance using the exact Policy match that admitted the assignment.
- Short-lived workload identity bound to one claim/worker and verified by both Tool and Model Gateways.
- CLI/API/Portal evidence parity and secret-free adapter-selected integration evidence.

## Out of Scope

- Token issuance, OIDC discovery, refresh/rotation UI, user directory synchronization, or an IdP.
- General provider-secret management, EKS/Bedrock adapter implementation, and AWS infrastructure.
- Broad administration, policy editing UI, persistent Work history, HA, or workflow/claim lineage.

## Requirements

- The HTTP boundary accepts only `Authorization: Bearer` for protected application routes. It validates a configured signing algorithm/key, exact issuer, intended audience, expiry, and bounded claims before constructing a principal. It never trusts `ClaimRequest`, query parameters, cookies, or arbitrary identity headers as authority.
- Required identity fields are subject, issuer, authentication context, one team, zero or more groups, and zero or more roles. Duplicate/conflicting team sources, unknown critical claims, invalid UTF-8, oversized values, or ambiguous mappings fail closed. Evidence exposes the bounded principal fields needed for accountability, not the raw JWT or raw metadata.
- Protected operations use stable actions: `work.submit`, `evidence.read.own`, `evidence.read.any`, `policy.register`, and `agent-template.register`. An owner is the verified principal subject recorded at admission. Listing returns only records visible to that principal unless `evidence.read.any` is authorized.
- Policy evaluation selects exactly one rule for an allowed operation. Ambiguous same-precedence matches fail closed. The selected rule and Policy ID/version are captured in the internal admission capability; downstream resolution cannot reread a newer Policy.
- An optional Policy ceiling can limit tools, resource scopes, model profiles, memory scopes, runtime profiles, and maximum timeout. A missing ceiling preserves the existing admission-only rule. An explicitly present empty dimension grants nothing in that dimension.
- Effective authority equals the deterministic intersection of the request, AgentTemplate ceiling, matched Policy ceiling when present, and runtime restrictions. It preserves request order, removes duplicates, rejects wildcard requested scopes, and records stable reasons including `outside-template-ceiling`, `outside-policy-ceiling`, `policy-timeout-cap`, and existing runtime narrowing.
- Policy cannot add an unrequested value, exceed the template, select an unrequested model/runtime profile, or change task input. If a requested non-empty required dimension resolves completely empty, issuance fails before allocation rather than silently producing unusable authority.
- A workload credential is signed by a control-plane key unavailable to the worker, contains issuer, allowed Gateway audiences, issued/expiry time, claim ID, and worker ID, and expires no later than the claim deadline. The raw token is neither evidence nor a public API field.
- Gateway authentication verifies the credential before claim/authority lookup. Verified claim ID is authoritative; a caller-nominated claim ID is only a consistency check. The Gateway then confirms worker binding, active `Running` phase, authority reference, and operation scope before invoking an adapter.
- Terminal claims, revoked credentials, expired credentials, missing worker binding, and mismatched nominated targets deny with a stable category and zero external calls. Denials are attributed only when the verified credential supplies a safe claim identity.
- CLI reads a user token from an explicitly configured protected file and adds it only to HTTPS or the existing loopback tunnel. Portal development/connected mode uses a server-side proxy that reads the token; browser code does not receive or persist it. Tokens are redacted from errors, logs, snapshots, evidence, and test artifacts.
- The installed reference mode must explicitly choose authenticated multi-user mode or the existing labelled fixed-principal local mode. Live authenticated mode never falls back to the fixed principal on verification failure.
- Adapter evidence is selected from the resolved Platform lock. Shared tests never branch on Kubernetes, AWS, Ollama, or Bedrock vocabulary. Provider-specific gates prove only the selected adapter's claims and report unsupported/missing adapters as blockers.

## Negative Cases

- Missing, malformed, expired, wrong-issuer, wrong-audience, invalid-signature, unsupported-algorithm, forged-team, conflicting-team, and oversized identity inputs fail before application authorization or side effects.
- An ordinary submitter cannot register Policy/templates or read another subject's evidence. A forged owner reference in a URL or request body does not change visibility.
- A Policy ceiling cannot expand request/template authority; ambiguous matching rules, malformed ceilings, missing Policy snapshots, and same-version changed content fail closed.
- A claim-A workload token paired with claim B's nominated ID, a different worker, wrong Gateway audience, terminal claim, or expired time never reads claim B for execution and produces zero provider calls.
- Authentication/authorization failures never include JWTs, signing keys, raw trusted metadata, provider credentials, task input, or untrusted adapter errors.

## Compatibility

- `ClaimRequest` remains unchanged and principal-free. `SandboxClaim`, `RuntimeBackend`, lifecycle phases, and backend-neutral evidence ownership remain unchanged.
- Existing admission-only Policy documents remain valid and retain their current authority behavior. New ceiling-bearing documents are versioned content and same-identity/different-content conflicts remain errors.
- The explicitly selected local fixed-principal mode remains available for its existing demo/tests and is labelled non-production. Authenticated mode has no fallback to it.
- Existing in-process Gateway tests may use a trusted verifier fixture, but live paths cannot accept bare claim IDs as authentication.
- Current kind/Agent Sandbox/Ollama behavior remains supported. EKS/Bedrock vocabulary and credential mechanics remain inside future E13 adapters.

## Open Decisions

- Owner/Reviewer must approve the exact public `Principal` compatibility strategy: extend the current v0 shape with bounded issuer/groups/roles fields or introduce a separate internal verified-principal type and project only stable fields into issued evidence. The design recommends the latter to minimize public-contract churn.
- Owner/Reviewer must confirm whether management-route authorization lands in this one PR if E13/#146 changes the HTTP routing concurrently; otherwise #197 must provide the shared middleware and tests against the current private routes and record the integration blocker.
