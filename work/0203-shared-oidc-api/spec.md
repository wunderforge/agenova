# Feature Specification: Shared OIDC API for two trusted users

- Ticket: [#203](https://github.com/wunderforge/agenova/issues/203)
- PRD outcome: [trusted-principal submission](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [shared evidence surfaces](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

Connected Agenova clients authenticate to one control plane with company-issued bearer JWTs. The service verifies identity once at its trusted HTTP boundary, supplies that identity separately from canonical Work input, and applies owner isolation to evidence. Authentication is transport-neutral after verification and does not redefine Policy, claim, or evidence semantics.

## In Scope

- Fixed issuer/audience/JWKS verification and explicit configured claims-to-team mapping.
- A rich internal verified identity containing stable subject, mapped team, authentication context, and bounded trusted attributes.
- Submission and evidence authorization for authenticated ordinary users.
- CLI token-file and server-side Portal proxy transports.
- Stable sanitized authentication failures and owner-hidden not-found evidence behavior.

## Out of Scope

- Identity-provider hosting, browser login flow, token lifecycle, OIDC discovery, account administration, delegated management, or enterprise Policy authoring.
- Per-user authority ceilings, worker JWTs, provider credentials, Operator/CRDs, or cloud deployment.

## Requirements

- Given a valid JWT from the configured issuer for the configured audience, when the HTTP boundary authenticates it, then it verifies the signature and temporal claims and produces one immutable verified identity.
- Given configured claim mapping, when multiple or conflicting raw claims exist, then only the server configuration determines the single public Agenova team or authentication fails closed.
- Given canonical Work JSON, when submitted by an authenticated user, then the verified identity is supplied separately and caller-authored team/subject values have no authority.
- Given owned evidence, when the same subject lists or reads it, then API, CLI, and Portal receive the same backend-neutral view.
- Given evidence owned by a different subject, when a user lists or guesses its request/claim reference, then the record is omitted or returned as not found without revealing its existence.
- Given an invalid or absent bearer token in authenticated mode, when any protected route is requested, then no claim, allocation, provider call, or evidence mutation occurs and fixed-principal mode is not attempted.
- Given a management registration route, when called with an ordinary Work bearer token, then it remains unauthorized unless the existing independent operator trust boundary authorizes it.

## Negative Cases

- Missing/malformed bearer syntax; expired/not-yet-valid token; wrong issuer/audience; unknown key; invalid signature; absent subject; unmapped or ambiguous team.
- User A token plus Team B values in JSON, headers, query, or metadata.
- User A guesses User B request reference or claim ID.
- Token, signature material, raw claims, JWKS configuration, or authorization headers appear in logs, URLs, error bodies, evidence, CLI output, Portal storage, or provider configuration.

## Compatibility

- Canonical `ClaimRequest`, `SandboxClaim`, Policy evaluation, authority resolution, gateway, and evidence DTOs keep their existing backend-neutral meanings.
- The existing fixed-principal local/reference mode remains available only by explicit mode/configuration and retains its current tests.
- Existing admission-only reference Policy YAML stays valid; M2 changes identity provenance and evidence ownership, not Policy language or authority constraints.
- Management registration continues to rely on the current operator/Kubernetes trust path.

## Open Decisions

- None. Provider-specific claim names and mappings are deployment configuration, not shared contract fields.
