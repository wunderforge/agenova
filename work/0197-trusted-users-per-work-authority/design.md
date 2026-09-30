# Technical Design: Trusted users and per-Work organizational authority vertical

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- Feature spec: `spec.md`

## Current State and Constraints

- The installed control plane constructs one process-wide Team A `PrincipalSource`. The console HTTP handler has same-origin checks but no per-request authentication or authorization; connected CLI/Portal use a loopback Kubernetes port-forward, which is transport isolation rather than identity.
- `PolicyBundle.Rule` exactly matches team/action/project/template and returns a boolean. Admission carries the decision, but authority resolution only intersects request and AgentTemplate; Policy cannot currently produce different authority for identical requests.
- RunService and Gateways already preserve authoritative claim state and context-mismatch checks, but the live worker has no cryptographically verified claim-bound credential. A claim ID remains caller-controlled unless a trusted host constructs the invocation context.
- The bundled registry currently contains Kubernetes deployment, Agent Sandbox runtime, and OpenAI-compatible model adapters. There is no EKS or Bedrock adapter in this repository state; E13 owns them.
- Public contract changes require fixtures, generated UI types, compatibility tests, and architecture review. Provider and deployment shapes must remain adapter-owned.

## Decision

Implement three explicit trust boundaries and compose them without sharing credentials:

1. **Human to API:** an `identity.Verifier` validates externally issued compact JWTs using configured local trust material, exact issuer/audience, an algorithm allow-list, and a clock. An HTTP authenticator extracts the Bearer value, calls the verifier, and places an internal immutable verified principal in request context. Token creation and discovery are out of scope.
2. **Worker to Gateway:** a separate `workloadidentity.Issuer/Verifier` signs and validates short-lived claim-bound JWTs. The claim deadline caps expiry. A verified workload principal, not a header claim ID, creates the existing trusted invocation context.
3. **Gateway to provider:** the selected trusted adapter owns provider authentication. This ticket adds conformance seams and absence/redaction assertions only; it neither serializes provider credentials into Work nor implements the general #155 resolver.

Use standard-library cryptography and JSON/base64 processing with a deliberately narrow JWT profile rather than a broad identity framework: one configured asymmetric algorithm for user JWT validation and one separately keyed Agenova workload signer. Reject algorithm/key confusion, duplicate JSON members, non-canonical critical claims, and tokens outside strict size/time bounds. Keep verifier configuration outside Platform business contracts until #146/E13 owns the protected deployment configuration surface.

## Ownership and Contract Boundaries

- `internal/identity`: verified principal, external JWT verifier, HTTP extraction errors, bounds, redaction-safe categories, and operation authorization inputs. It has no ClaimRequest or provider dependency.
- `internal/policy` and `internal/authorization`: validated optional `AuthorityCeiling`, stable operation actions, deterministic single-rule match, and an internal immutable matched-policy capability. Public decisions continue to expose only Policy reference/result/reason.
- `internal/authority`: intersect request/template with the matched Policy ceiling and runtime restrictions; emit deterministic per-dimension provenance. It never loads Policy itself.
- `internal/workloadidentity` plus issuance/runtime composition: sign a bounded credential only after system issuance and worker binding are known; inject only that token and non-secret Gateway location into the worker. Signing/verification keys remain host-side.
- Tool/Model Gateways: authenticate workload identity first, build the trusted invocation context from verified claims, then perform existing active-claim/authority checks. Adapter interfaces and facts remain backend-neutral.
- Console/control-plane HTTP: middleware maps routes to operation actions, binds submissions to the verified principal, filters list/read results by recorded subject, and protects management writes. Fixed-principal behavior requires explicit local-reference mode.
- Connected CLI/Portal: a token-file option supplies Bearer authentication over HTTPS/loopback; the UI dev proxy holds the token server-side. JSON/evidence contracts never include token material.
- Adapter verification: a common matrix records resolved deployment/runtime/model adapter identities and invokes registered adapter-specific evidence gates. Unknown or unavailable gates report unsupported/blocker, never success or fallback.

## Detailed Data Flow

1. Client loads an externally obtained JWT from a protected file and sends a canonical `ClaimRequest` plus Bearer header.
2. HTTP authentication validates the JWT and produces one immutable verified principal. Operation authorization checks `work.submit` before parsing can cause any downstream side effect.
3. Authorization evaluates the request action against one Policy snapshot, selects exactly one matching rule, and returns an internal admission capability containing the decision and cloned optional ceiling.
4. Authority resolution intersects the validated request, cloned AgentTemplate, admission-bound Policy ceiling, and runtime limits. Issuance records the public principal projection, Policy reference, effective authority, and narrowing changes.
5. Runtime allocation binds a backend worker. The host issues a workload credential for that exact claim/worker with expiry capped by the claim deadline and supplies it to the worker without provider credentials.
6. A Tool/Model request presents the workload credential and may nominate a claim ID. The Gateway verifies the token and uses its claim ID as authoritative; mismatch denies before authority/provider lookup for the nominated claim.
7. The Gateway confirms authoritative `Running` state and worker binding, evaluates effective authority, invokes the selected adapter only on Allow, and appends claim-attributed facts.
8. Terminal transition/revocation makes subsequent invocations fail even if the token signature and expiry remain valid.

## Compatibility and Migration

- Add Policy ceiling fields as optional. Decode absent versus explicitly present empty ceiling/dimensions deliberately so old documents retain admission-only behavior and explicit empty limits deny that dimension.
- Clone rule/ceiling slices and maps when loading, matching, and resolving. Policy version changes affect only later admissions.
- Prefer an internal `VerifiedPrincipal` richer than the current public `v1alpha1.Principal`; project the stable subject/team/authentication context into current issued evidence and add public groups/roles only if acceptance cannot be met without them.
- Preserve current constructors through explicit local-reference wrappers. New authenticated constructors require verifiers and fail on nil/misconfiguration; they never silently call the old fixed source.
- Introduce Gateway authenticated entrypoints alongside in-process trusted-context entrypoints long enough to migrate tests. Only the live worker transport may use the authenticated entrypoint.

## Alternatives Considered

- **Trusted reverse-proxy identity headers:** rejected for the reference because correctness depends on deployment topology stripping spoofed headers; the current loopback/private service does not establish that full boundary.
- **mTLS client identity:** deferred because certificate enrollment and Portal/browser handling add operational scope unrelated to the authority result.
- **Agenova-issued user tokens:** rejected for this slice because #182 is optional and would make Agenova an identity lifecycle owner.
- **Bare claim ID or opaque shared worker token:** rejected because it cannot bind one worker to one issued claim and permits authority selection by identifier.
- **Policy admission plus a separate authority-policy lookup:** rejected because Policy could change between decisions or the two evaluations could select different versions.
- **Generic credential resolver in this PR:** rejected because E14 needs one proven host-side credential boundary, while #155 owns the reusable resolver product.
- **Hard-coded AWS acceptance logic:** rejected because EKS/Bedrock belong to adapters and E13; shared E14 contracts must also work with kind/Ollama and future providers.

## Verification Strategy

- Identity unit/contract tests cover signature, algorithm, issuer, audience, expiry/not-before/skew, duplicate claims, conflicting team mapping, bounds, UTF-8, and redaction. Use deterministic local keys and clocks; never snapshot raw tokens.
- Policy/authority tests submit byte-equivalent request specs for two principals against one Policy version and template, proving distinct ceilings, deterministic provenance, no expansion, explicit-empty behavior, ambiguity rejection, and snapshot immutability after loader updates.
- HTTP tests cover anonymous/invalid authentication, own versus cross-user list/detail, operator-only registration, same-origin behavior, fixed-mode isolation, and zero service calls on denial.
- Workload/Gateway tests cover both Tool and Model paths with valid, malformed, expired, wrong-audience, wrong-worker, revoked, post-terminal, and claim-A/token plus claim-B/target cases. Adapter spies and fact stores assert zero external calls and no cross-attribution.
- Runtime tests inspect launch input, Agent Sandbox manifests, and worker environment for forbidden human JWTs, signing keys, AWS/provider credentials, and platform-management credentials.
- CLI/UI tests prove token-file use, HTTPS/loopback restriction, server-side proxy injection, error redaction, and visible evidence parity.
- Adapter matrix:
  - memory/spies: full shared positive and negative contract;
  - Kubernetes + Agent Sandbox + OpenAI-compatible/Ollama: reproducible installed two-user and lifecycle regression;
  - E13 EKS + Bedrock: externally supplied real-backend gate for allowed inference, denied zero-call evidence, workload identity, credential absence, and cleanup.
- Run focused gates, UI tests, opt-in integrations, `./scripts/check.ps1 -All`, generated-contract checks, and a final secret/scope diff review.

## Risks and Compatibility

- JWT parsing is security-sensitive. Keep the accepted profile narrow, cap token/claim sizes, use constant-time/signature-library primitives, inject clocks, and fuzz malformed input. If review prefers a vetted dependency, record that dependency and its validation defaults before implementation.
- Existing HTTP setup/list surfaces expose policy and evidence without identity. Authenticated mode must protect every route consistently; an unclassified route must default deny.
- Policy ceiling semantics can accidentally broaden authority if absence and empty are conflated. Preserve an explicit presence bit/pointer and test every dimension.
- Worker identity issuance occurs only after backend worker binding is known. If the current Agent Sandbox launch API cannot safely inject post-binding identity, stop and revise the runtime handshake rather than placing a reusable credential in the template/warm pool.
- Token-file handling cannot guarantee external issuer delivery security. This slice documents file permissions and avoids argv/URL/browser persistence; #182 remains the optional first-party lifecycle owner.
- The E13 adapter/environment may not exist when code is ready. Merge/closure claims must distinguish shared/local proof from the outstanding real Bedrock evidence; no Ollama or mocked response substitutes for it.
