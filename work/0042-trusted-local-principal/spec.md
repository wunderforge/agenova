# Feature Specification: Supply a trusted local principal boundary

- Ticket: [#42](https://github.com/wunderforge/agenova/issues/42)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability).

## Intent

Make the upstream identity boundary explicit in the deterministic local reference path. It constructs trusted identity independently of ClaimRequest; #27 decides whether that principal may create the requested assignment. Trust is limited to local operator/test composition, without a production authentication claim.

## In Scope

- One local reference identity provider with fixed Team A and Team B presets chosen through composition setup outside YAML/JSON.
- Separate canonical Principal and ClaimRequest inputs to existing assignment authorization.
- Reproducible Allow/Deny evidence and proof that YAML edits cannot impersonate Team A.

## Out of Scope

- OIDC, SSO, production authentication/security, user directory, sessions, login UI or identity administration.
- Request identity extensions, caller-supplied authoritative principal objects, duplicated authorization, full submission/issuance or runtime/fact-storage implementation.

## Requirements

1. Given explicit local A/B setup, the single reference boundary returns a fresh canonical Principal with deterministic subject, team and authentication context. It accepts no ClaimRequest or task-input identity. Missing/unknown setup errors rather than selecting a privileged default.
2. Given the same valid ClaimRequest bytes and canonical active policy, switching only trusted reference setup changes the Principal supplied to #27. Project/template stay identical; `spec.projectRef` uses the accepted #96 schema and is requested context, not authority.
3. The composition passes request reference, separate trusted Principal and Action (`claim.create`, validated project/template references) to #27's Gate. It consumes the returned Decision and non-forgeable Admission without another allowlist, team comparison or manufactured Allow token.
4. Team A reaches assignment authorization and, when allowed, the downstream continuation once. This slice proves admission reachability; #28/#29 own authority and issuance. Full-demo issuance requires their real integration.
5. Team B is denied before any continuation, claim creation or allocation. Output records actual Principal, Action, Decision and Evidence in canonical IssuedState with real active policy ID/version and reason. Evidence correlates request and decision; claim, effective authority, claim ID and backend identity are absent. Runtime/tool/model collections remain canonical empty arrays.
6. The same fixed principal remains authoritative for the admission attempt. Metadata, requested access, task input, project/template edits or identity-looking strings cannot replace it. Existing strict request validation rejects reserved principal fields; permitted opaque task data is never used to authenticate.
7. Label the executable reference smoke path local/demo-only. A local operator's preset selector is test setup, not proof of remote caller authentication; do not expose it as a production authentication service.

## Negative Cases

| Case | Expected behavior | Claim/backend effects |
| --- | --- | --- |
| Team B, byte-identical canonical YAML | #27 Deny with valid request/decision evidence | None |
| Team B with Team A at reserved principal paths | Existing parser rejects self-assertion | None |
| Team B with identity strings in permitted task input | Principal stays Team B; denial persists | None |
| Missing or unknown preset | Boundary error; no privileged fallback | None |
| Missing/invalid project or template | Existing request/admission validation fails closed | None |
| Existing gate returns non-Allow or error | No continuation; preserve actual decision/error | None |

#27 retains ownership of policy mismatch, ApprovalRequired and malformed admission tests. #42 tests actual composition and identity tampering without duplicating its evaluator.

## Compatibility

- ClaimRequest remains identity-free; YAML and JSON retain one intent-only schema.
- Reuse Principal, Action, Decision, Evidence and IssuedState rather than creating a parallel evidence model.
- Consume the merged #27 gate/policy; do not copy or reinterpret its implementation.
- Preserve #40's backend-neutral composition and keep provider types out of shared contracts.
- Inspectable reference output is not append-only storage, claim issuance or real-backend proof.

## Open Decisions

- The previously stacked #96 dependency is resolved; #42 is reconciled with the merged gate contract on `main`.
- Coordinate exact smoke entrypoint syntax with #41 during implementation. It must preserve out-of-band setup and have executable commands recorded before implementation review.
