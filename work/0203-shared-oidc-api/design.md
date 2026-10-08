# Technical Design: Shared OIDC API for two trusted users

- Ticket: [#203](https://github.com/wunderforge/agenova/issues/203)
- Feature spec: `spec.md`

## Current State and Constraints

- The control-plane HTTP composition currently supplies a configured trusted principal rather than authenticating each request.
- Console submission, list, request evidence, and claim evidence share one in-memory service and backend-neutral `evidence.View`.
- Connected CLI and Portal use that HTTP service, while registration uses a separately trusted operator path.
- M1 introduces the evaluator snapshot boundary and must remain the admission source of truth.
- `ClaimRequest` cannot accept authoritative identity and browser/worker environments cannot retain company bearer tokens.

## Decision

Add a small `internal/identity` boundary with `Provider.Verify(ctx, bearer) (VerifiedPrincipal, error)`. The OIDC implementation receives immutable issuer, audience, JWKS URL, HTTP client, clock, and explicit mapping configuration. It verifies JOSE signature and registered claims before mapping bounded trusted attributes to one Agenova team. Provider errors expose stable categories and sanitized messages only.

Wrap protected user HTTP routes with authentication middleware. The middleware stores the verified identity in request context; handlers pass its stable `Principal` projection to the existing application boundary. The service records an internal owner key derived from verified issuer+subject alongside each Work record and filters list/read/claim lookup by that key before returning evidence. Cross-owner and absent lookups share the same response.

Connected CLI reads a token from an explicitly supplied protected file for each request and sets `Authorization: Bearer`; it never accepts the token as a command argument. The Portal browser calls a same-origin local proxy, and only that server-side proxy reads/injects the token. Authenticated and fixed-principal handlers are separate compositions, so verifier failure cannot fall through.

## Ownership and Contract Boundaries

- `internal/identity`: verified identity contract, OIDC/JWKS implementation, stable error categories, deterministic test issuer/JWKS fixtures.
- `internal/console`: authenticated route wrapper and owner-aware service/query behavior; it does not parse JWTs or change evidence DTOs.
- `cmd/agenova-control-plane`: explicit authenticated versus fixed-principal composition and deploy-time identity configuration.
- `cmd/agenova`: protected token-file loading and bearer transport only; no claim interpretation.
- Portal server/proxy boundary: token injection; browser source continues consuming the canonical evidence API.
- Installation/config adapter: provider-specific issuer/audience/JWKS/mapping values remain configuration and never enter domain contracts.

## Alternatives Considered

- OIDC discovery: rejected for M2 because fixed explicit configuration is testable and the Ticket excludes a general SSO integration surface.
- Passing raw claims into Policy: rejected because it couples Policy to one provider and risks treating unbounded identity metadata as authority.
- Storing JWTs with Work for later ownership checks: rejected because it expands secret lifetime and leaks identity-provider data into evidence/state.
- Browser local/session storage: rejected because the accepted design requires server-side proxy injection and no persistent bearer token.
- Returning 403 for cross-owner evidence: rejected because it leaks that a guessed reference exists.

## Verification Strategy

- Identity contract tests use a local deterministic issuer/JWKS server and a controllable clock for valid dual-user, signature, issuer, audience, expiry, key, mapping, and redaction cases.
- HTTP tests assert authentication happens before service mutation and that owner filtering is identical for list, request, and claim evidence routes.
- Spy backend/provider counters prove named authentication/ownership denials create no claim, allocation, or external invocation.
- CLI tests validate protected-file permissions, header injection, sanitized errors/output, and absence of token command arguments.
- Portal tests validate same-origin proxy usage and absence of browser persistence/URL token material.
- Installed kind smoke runs two sanitized test identities through one service and compares API/CLI/Portal-visible principal and ownership behavior.

## Risks and Compatibility

- JWKS rotation/cache behavior can create availability or stale-key risks; use bounded caching/refetch behavior without implementing discovery, and fail closed when a trusted key cannot be established.
- Owner filtering added only at presentation would be bypassable; ownership must be enforced in the service/query boundary shared by all transports.
- Subject collision across issuers is avoided by the internal issuer+subject owner key while public evidence continues exposing only the stable subject/team/authentication context projection.
- Existing fixed-principal mode remains a separate explicit composition for local/reference compatibility and never serves as authenticated-mode fallback.
- M2 preparation is based on M1; before PR creation it must be rebased onto merged `main` and all focused/full gates rerun.
