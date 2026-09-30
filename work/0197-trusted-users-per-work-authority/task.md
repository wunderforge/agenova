# Task: Trusted users and per-Work organizational authority vertical

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- Mission: Deliver the complete E14 vertical in one reviewable change: authenticate two external users per request, apply an immutable organizational Policy ceiling, bind each worker to one claim at the Gateways, and prove unauthorized activity stops before external calls.
- Target: `api/v1alpha1`, `internal/identity`, `internal/policy`, `internal/authorization`, `internal/authority`, `internal/issuance`, `internal/app`, `internal/gateway`, Tool/Model Gateways, `internal/console`, connected CLI/Portal transport, controlled Agent Sandbox launch, tests, and adapter-selected evidence.
- User value: Two trusted users can reuse one AgentTemplate and submit identical requested access to one Agenova service while receiving different explainable temporary authority, without putting human tokens or provider credentials in Work or Agent Pods.
- PRD outcome: [`Declarative request and authorization resolution`](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [`Claim-scoped authority`](../../docs/product/prd.md#4-claim-scoped-authority), and [`Facts and accountability`](../../docs/product/prd.md#5-facts-and-accountability).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md)
- [`design.md`](design.md)
- [`docs/product/architecture-contract.md`](../../docs/product/architecture-contract.md)
- [`docs/development/AIDLC.md`](../../docs/development/AIDLC.md)
- [`docs/harness/playbooks.md`](../../docs/harness/playbooks.md#change-a-core-contract)
- [`api/v1alpha1/sandbox_claim.go`](../../api/v1alpha1/sandbox_claim.go)
- [`internal/app/reference_admission.go`](../../internal/app/reference_admission.go)
- [`internal/policy/bundle.go`](../../internal/policy/bundle.go)
- [`internal/authority/resolver.go`](../../internal/authority/resolver.go)
- [`internal/gateway/contract.go`](../../internal/gateway/contract.go)
- [`internal/console/http.go`](../../internal/console/http.go)
- [`internal/adapters/bundled/registry.go`](../../internal/adapters/bundled/registry.go)
- [E14 Epic #176](https://github.com/wunderforge/agenova/issues/176) and source requirements [#91](https://github.com/wunderforge/agenova/issues/91), [#121](https://github.com/wunderforge/agenova/issues/121), and [#172](https://github.com/wunderforge/agenova/issues/172)

## Scope

In scope:

- Verify externally issued Bearer JWTs at the HTTP boundary and derive the trusted principal outside `ClaimRequest`.
- Authorize Work submission, own/cross-user evidence reads, Policy registration, and AgentTemplate registration as separate operations.
- Add optional organizational ceilings to Policy rules and resolve one immutable admission/authority snapshot.
- Issue and verify short-lived claim/worker-bound workload identity before Tool or Model Gateway authority lookup.
- Add token-file CLI transport and a Portal server-side proxy path without browser persistence or credential disclosure.
- Prove shared contracts with memory/spy tests, preserve the current kind/Agent Sandbox/Ollama path, and define adapter-selected real-provider evidence consumed by E13.

Out of scope:

- OIDC discovery, enterprise SSO, directories, or Agenova user-token issuance/rotation (#182).
- A general provider credential resolver or closure of #155.
- EKS, Bedrock, or protected-cloud deployment implementation owned by E13/#146.
- Policy editor UI, general multi-tenancy, HA, durable Work history, or parent/child claims.

## Acceptance Criteria

- Two valid externally signed JWTs identify distinct users on one service. With one template, one Policy version, and byte-equivalent `ClaimRequest.spec`, both Work items are admitted and receive different effective authority solely from their matched Policy ceilings.
- Effective authority is the intersection of requested access, template ceiling, matched Policy ceiling, and runtime restrictions. Admission and resolution use the same immutable Policy snapshot; later Policy changes do not alter an issued claim.
- Identity and API permissions are service-established. Anonymous, invalid, expired, wrong-issuer/audience, forged-team, and conflicting-metadata requests fail closed; ordinary users read only their own evidence and cannot perform management writes.
- Each live worker uses a signed, expiring credential bound to one claim, worker identity, and Gateway audience. Missing, malformed, expired, revoked, terminal, wrong-audience, worker-mismatched, and cross-claim credentials cause zero external calls.
- CLI, API, and Portal report the same principal, Policy reference, requested/effective authority, narrowing reasons, claim lifecycle, and outcome without exposing authentication or provider credentials.
- Shared contract tests remain provider-neutral. Current bundled adapters pass regression gates; an E13-selected EKS/Bedrock composition can run the separate real-provider gate without changing shared authority semantics.

## Negative Case

- Present a valid claim-A workload credential while nominating claim B and requesting an operation claim A's matched Policy ceiling excludes. The Gateway must attribute the bounded denial to claim A, make zero adapter calls, record no claim-B invocation fact, and never read claim B's authority for execution.

## Execution Todo

- [ ] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Confirm this packet with the Owner and Reviewer before implementation.
- [ ] Add the verified-principal/JWT boundary and operation-level HTTP authorization.
- [ ] Extend Policy evaluation to return an immutable matched-rule ceiling and include it in authority resolution/provenance.
- [ ] Add claim/worker-bound workload credential issuance and Gateway verification before authority lookup.
- [ ] Connect the installed service, CLI token-file transport, and Portal server-side proxy without secret persistence.
- [ ] Add shared conformance tests and adapter-keyed kind/Ollama plus E13 real-provider evidence entrypoints.
- [ ] Update contract fixtures, generated UI types, reference guidance, and project status only after behavior is verified.
- [ ] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/identity ./internal/policy ./internal/authorization ./internal/authority ./internal/issuance ./internal/app ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/console ./internal/connectedclient ./cmd/agenova-control-plane ./internal/runtime/agentsandbox`
- `npm --prefix ui test`
- Existing opt-in kind/Ollama integration gate plus the adapter-selected E13 EKS/Bedrock gate when that adapter/environment is available
- `./scripts/check.ps1 -All`

## Evidence Required

- Focused output proving JWT validation/redaction, per-operation authorization, identical-request/different-ceiling results, immutable Policy snapshots, workload identity verification, and zero adapter calls for every named denial.
- Reproducible kind transcript for two users, independent claims, different authority, cross-claim and post-terminal denial, CLI/API/Portal parity, and absence of external credentials from worker environment/manifests.
- Adapter-keyed evidence manifest naming the selected Platform/adapters and whether each real-provider gate ran. Lack of the E13 EKS/Bedrock environment is an explicit Epic blocker, never a simulated success.
- Full repository gate output and final scope/security diff review.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- One `SandboxClaim` remains one worker assignment; a claim ID is correlation, not authentication.
- `ClaimRequest` cannot carry principal, trusted metadata, effective authority, user tokens, workload tokens, or provider credentials.
- Policy and identity can only narrow or deny; neither may add authority absent from request/template/runtime limits.
- Backend/provider and JWT transport details stay behind adapters; public governance contracts remain backend-neutral.
- Do not close #155 or #182, and do not claim E14 complete without the required E13 real-provider evidence.

## Decisions and Blockers

- Selected human identity reference: externally issued signed JWT validated by configured issuer, audience, algorithm/key, and expiry. Agenova does not issue the user token.
- Selected worker identity reference: Agenova-signed short-lived JWT whose expiry is no later than the claim deadline; no refresh protocol is introduced in this slice.
- Existing Policy rules without a ceiling remain admission-only for compatibility; ceiling-bearing rules participate in resolution.
- The current repository has no EKS/Bedrock adapter. E13 owns that implementation and environment; its real-provider transcript remains an external completion dependency for E14 Epic closure.
- Implementation is blocked until Owner and Reviewer approve this task/spec/design packet in #197.
