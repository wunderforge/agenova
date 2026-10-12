# Technical Design: Production invite-only team access

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

The installed control plane selects a fixed Team A principal at process startup. Console setup/Prepare closures are global, and evidence is held in process memory. Connected CLI opens a Kubernetes tunnel from applied installation state; it cannot yet authenticate directly to HTTPS. The demo ALB/session sidecar authenticates one shared password, not an individual application identity. A production rollout must replace each of these constraints explicitly.

## Decision

Use managed Cognito for directory/password/MFA and protocol endpoints; retain backend-neutral application authentication/authorization interfaces. Use a separately managed durable relational store for application memberships, sessions, audit and Work records. The final AWS database availability/topology depends on budget and recovery targets; do not provision from this draft.

Separate ordinary member and operator paths. A member's HTTPS client supplies a verified access token/session; the service projects issuer+subject into a current membership and an immutable per-request principal. It then authorizes action/project/template and reuses claim resolution, runtime and Gateway boundaries. Never install a global mutable current-user variable.

Cognito public signup is disabled. Administrative invitation is a distinct audited application/operator action; the AWS directory adapter receives only a narrowly scoped invitation capability, never the human operator's admin credentials. No directory password or token is stored in Terraform. Invitations remain unsent until explicitly instructed for named recipients.

Native clients use a secretless public OAuth client and code+PKCE; web uses a distinct server-side client/session. Cognito-specific `token_use`, `client_id`, scope and optional resource-audience rules stay inside its validation adapter. Do not apply generic ID-token audience checks to an access token. Validate exact issuer and app-client allowlist; unknown keys trigger one bounded refresh rather than an unbounded request per invalid token. The library selection and algorithms must be pinned and tested before implementation.

Application membership, project roles and revocation are server-owned records. Member removal is checked independently of token lifetime. JWT validation alone cannot enforce revocation. Define the active-claim cancellation and concurrent-read boundary together with accepted E14 lifecycle/authorization contracts; do not add a new shared terminal phase merely to simplify restart handling.

Persist Work/facts behind the shared evidence representation, with tenant+owner-qualified reads and immutable fact attribution. Introduce durable submission idempotency before background dispatch. Persist a dispatch lease/state and backend correlation; recover uncertain runs by reconciliation, never blind provider replay. Prefer database transaction/outbox boundaries, with explicit acknowledgement and duplicate tests. Durable history is a separate delivery gate from login.

Production uses its own Terraform root/state and lifecycle. It must not be attached to the disposable demo cleanup. Keep permanent domain resources separate. Reuse the existing EKS deployment adapter only where its behavior has upgrade/rollback evidence; reference installation readiness is not a production release gate.

## Ownership and Contract Boundaries

- Authentication adapter: IdP validation, discovery/key rotation and stable verified subject.
- Application authorization: current membership, action, project, ownership; consumes accepted policy evaluation contract (#207), does not invent competing policy semantics.
- API/BFF: request-local context, safe sessions, typed failures and redacted transport; #203 compatibility.
- CLI: explicit connection profile, browser PKCE, OS secret storage, refresh/logout; no Kubernetes requirement for members.
- Persistence: transactional ownership and facts, migration, idempotency, restart reconciliation, backups.
- Operator infrastructure: IAM workload permissions, Cognito/database/runtime resources, TLS, logs, alarms, release and recovery. AWS/SQL types do not enter ClaimRequest.

## Alternatives Considered

- Per-member Kubernetes tunnel: rejected by Owner; grants an infrastructure entry and keeps shared application identity.
- Shared demo login or personal static API keys: insufficient identity/revocation/authorization design.
- Bespoke password server: unnecessary credential-management risk; use managed directory.
- Merge #207 and call it authentication: incorrect; it is a policy kernel, not OIDC or user sessions.
- One large untested cutover: rejected; preserve demo until all production release evidence passes.

## Verification Strategy

Deliver independently reviewable slices: (1) accepted identity/membership interfaces and adversarial fixtures, (2) per-request API/CLI identity and owner reads, (3) native/web login and revocation, (4) persistent Work/recovery, (5) reviewed infrastructure and live two-user campaign. These are implementation slices, not reduced production requirements.

Local tests cover malformed JWT/JWKS limits, exact issuer/client/scopes, key rotation, cache/network failure, token redaction, CSRF/PKCE state, invite replay, owner-query filtering, revocation races and transaction/restart faults. Installed proof uses two real accounts with no cluster access, positive inference plus backend-side zero-call denials, browser rendering and CLI parity. Exercise migration/rollback, database backup restore and lost-node/service restart. Run focused/race/full repository gates and independent security/operational review before public cutover.

## Risks and Compatibility

Owner retained AUD100/month and proposed Sydney 10:00–24:00 operation. This is a hard design constraint, not a validated EKS cost estimate. EKS control-plane hourly billing continues with zero nodes; daily cluster re-creation adds startup/teardown time and must not remove persistent identity/storage. A costed topology and recovery choices are still required. Email delivery/sandbox limits require verification. Existing in-memory runners cannot be made durable by serializing their final output alone. A single registered template/global setup does not prove multi-user isolation. The active demo cleanup remains unchanged. No production capability is claimed by this planning packet.

Owner resolved deployment ownership on 2026-10-12: designated administrators operate the shared EKS deployment. Project membership/SSO role management cannot grant infrastructure administration. Validate member-versus-operator denial separately from successful member Work submission.

## First implementation boundary

`internal/authentication` accepts a verified private identity and requires a current membership/revocation resolver on every request. It returns only the public principal projection; it never grants policy authority. No persistence substitute is provided. `internal/authentication/cognito` pins the issuer/client/scope, accepts only RS256 access tokens, retains token/session identifiers for revocation lookup, and bounds JWKS size/count/refresh frequency and HTTP time. It does not treat token email/groups/team as application roles. Known keys expire after fifteen minutes; failed refresh after expiry denies instead of using stale keys. Unknown-key refresh is limited to once per thirty seconds, so newly rotated keys can temporarily fail closed.

The connected-client library accepts one explicit HTTPS origin and a token source, mutually exclusive with all Kubernetes tunnel settings. Redirects are rejected even with an injected HTTP client; transport/errors do not emit bearer secrets. CLI selection/keychain/login integration is still a separate unfinished slice. Existing explicit reference clients retain their loopback behavior.
