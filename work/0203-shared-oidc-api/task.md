# Task: Shared OIDC API for two trusted users

- Ticket: [#203](https://github.com/wunderforge/agenova/issues/203)
- Mission: Authenticate connected Work and evidence requests with externally issued OIDC JWTs while keeping caller identity outside `ClaimRequest` and management writes outside the user API.
- Target: `internal/identity`, `internal/console`, connected CLI transport, Portal proxy/connection code, installation configuration, and focused contract/E2E tests.
- User value: Two company users can concurrently use one Agenova service and can see only their own Work evidence.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [Evidence surfaces](../../docs/product/architecture-contract.md#evidence-surfaces).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- `spec.md`
- `design.md`
- `docs/product/architecture-contract.md#submission-and-resolution`
- `docs/product/architecture-contract.md#evidence-surfaces`
- `internal/console/http.go`
- `internal/console/http_test.go`
- `cmd/agenova-control-plane/main.go`
- `cmd/agenova/main.go`
- `ui/src/connected-source.ts`
- `ui/src/ConnectedPortal.tsx`
- `docs/reference-cli-kind-ollama.md`

## Scope

In scope:

- A backend-neutral verified-human-identity boundary and fixed OIDC/JWKS verifier configuration.
- Bearer JWT verification for signature, issuer, audience, expiry, subject, and configured claim-to-team mapping.
- Authenticated Work submission plus owner-filtered evidence list/read in one long-running control plane.
- Connected CLI token-file support with file-safety checks and no token serialization into commands, URLs, evidence, or logs.
- A same-origin server-side Portal proxy contract that injects the bearer token without browser persistence.
- Explicit separation between authenticated connected mode and labelled fixed-principal local/reference mode.
- Sanitized two-user HTTP/CLI/Portal contract evidence and installed kind smoke coverage.

Out of scope:

- Login or authorization-code UI, token issuance, refresh, revocation, discovery, user directory, and general SSO administration.
- Policy-derived per-user authority constraints (M3), workload identity (M4), Kubernetes Operator/CRDs, OPA/Cedar, EKS/Bedrock, or a general secret manager.
- Moving PolicyBundle or AgentTemplate registration into the ordinary user bearer-token surface.

## Acceptance Criteria

- Two valid configured company identities submit identical Work to one service concurrently; evidence attributes each Work to its verified subject/team.
- Missing, malformed, expired, wrong-issuer, wrong-audience, invalid-signature, unmapped-team, forged-team, and conflicting identity inputs fail before claim creation, backend allocation, or provider invocation.
- Authenticated users can list and read only their own evidence; guessed request or claim references owned by another user are indistinguishable from absent records.
- CLI, HTTP API, and connected Portal render the same public principal projection without bearer tokens or raw OIDC claims.
- Policy and AgentTemplate registration remain protected by the existing operator/Kubernetes trust path.
- Fixed-principal local/reference mode remains explicit and cannot be selected as a fallback after authenticated-mode failure.

## Negative Case

- User A's valid token combined with Team B data in request metadata, headers, query parameters, or task input cannot change the verified principal, and a guessed User B evidence reference returns no evidence with zero Work/provider side effects.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies: the current console service binds one preset principal at construction, so M2 must introduce per-request identity without changing the evidence DTO or allowing owner checks only in handlers.
- [x] Assignee self-review: Ticket and PRD agree on externally issued identity, fail-closed authorization, owner-only evidence, and no management expansion; M1 merge remains the PR publication dependency.
- [x] Add the verified principal and OIDC verifier boundary with deterministic JWKS test fixtures.
- [ ] Protect Work submission and owner-filter evidence routes without changing canonical `ClaimRequest`.
- [ ] Add connected CLI token-file transport and Portal server-side proxy injection.
- [ ] Add installed kind two-user authentication and evidence-isolation smoke evidence.
- [ ] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/identity ./internal/console ./cmd/agenova-control-plane ./cmd/agenova`
- `npm --prefix ui test -- --run`
- M2 kind authentication/evidence-isolation smoke command recorded by the implementation
- `./scripts/check.ps1 -All`

## Evidence Required

- Focused test output covering two valid users and every named JWT/ownership failure with zero-side-effect assertions.
- Sanitized CLI/API/Portal parity transcript showing only the stable public principal projection.
- Installed kind transcript proving owner-only evidence access through the real service boundary.
- Full repository gate output, or an explicit environment blocker that does not overclaim installed behavior.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Agenova verifies externally obtained tokens and never becomes the issuer or browser token store.
- `ClaimRequest` remains principal-free, provider-free, and Kubernetes-free.
- Authentication failure must not downgrade to the fixed-principal reference path.
- This branch may be prepared on M1 but its PR is not opened until #197/#207 merges and the branch is rebased onto `main`.

## Decisions and Blockers

- Decision: use configured issuer, audience, JWKS URL, and explicit claim-to-team mapping; OIDC discovery is intentionally excluded.
- Decision: return not-found semantics for cross-owner evidence references to avoid disclosure through existence or status differences.
- Decision: keep rich verified claims internal and persist only the current stable public `Principal` projection plus an internal immutable ownership key.
- Decision: the bounded initial verifier accepts only RS256 keys/tokens from the explicitly configured JWKS endpoint; unsupported algorithms fail closed rather than widening the cryptographic profile implicitly.
- Blocker: M1 PR #207 must merge before this milestone can be proposed for review.
