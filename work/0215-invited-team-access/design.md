# Technical Design: Production invite-only team access

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

The installed control plane selects a fixed Team A principal at process startup. Console setup/Prepare closures are global, and evidence is held in process memory. Connected CLI opens a Kubernetes tunnel from applied installation state; it cannot yet authenticate directly to HTTPS. The demo ALB/session sidecar authenticates one shared password, not an individual application identity. A production rollout must replace each of these constraints explicitly.

## Decision

Use Google OpenID Connect for personal sign-in, as selected by the Owner on
12 October 2026. There is no Agenova password or Cognito directory in the target
deployment. Google Cloud project `agenova-login` contains OAuth application
registration only; application/runtime/database hosting remains on AWS.

The server performs authorization-code exchange with PKCE, state and nonce bound
to a short-lived, one-use login attempt. Pin Google's issuer, endpoints, signing
algorithm and the configured web client audience; verify the authorized party
when present. Google ID tokens are login assertions, NEVER API bearer credentials.
Only `openid email` is requested; do not request Gmail, Drive or offline access.
Discard Google access/refresh tokens after verifying the login assertion.

Agenova issues opaque, revocable application sessions after current membership
and invitation validation. Portal gets an HttpOnly Secure same-site cookie;
native CLI completes a separate one-use PKCE-bound application authorization
handoff through the browser and stores its application credential in the OS
keychain. Never embed a Google client secret in the CLI. A CLI callback receives
only a short-lived one-use application code, never a Google token or session.

Invitations are application-owned and must not create Google accounts. Bind a
verified Google issuer+subject atomically on first acceptance; require the
invited verified email and one-use invitation proof. Do not auto-link accounts
by matching email, strip Gmail dots/plus aliases, or reactivate disabled users.
A Google login without membership/invitation grants no Work access. Bootstrap
Owner identity remains explicit private operator input. Google account MFA is
controlled by Google; do not claim the application enforces Cognito TOTP.

Retain PostgreSQL memberships, roles, audit and encrypted sessions. Use managed
Amazon RDS for PostgreSQL, with encrypted persistent storage retained during
scheduled stops and no automated backups per Owner decision. Keep human
identity, application authorization and AWS workload IAM separate. Review the
complete costed topology before provisioning. Existing Cognito implementation
below is historical/in-progress migration context, not an enabled target.

Application membership, project roles and revocation are server-owned records. Member removal is checked independently of token lifetime. JWT validation alone cannot enforce revocation. Define the active-claim cancellation and concurrent-read boundary together with accepted E14 lifecycle/authorization contracts; do not add a new shared terminal phase merely to simplify restart handling.

Persist Work/facts behind the shared evidence representation, with tenant+owner-qualified reads and immutable fact attribution. Introduce durable submission idempotency before background dispatch. Persist a dispatch lease/state and backend correlation; recover uncertain runs by reconciliation, never blind provider replay. Prefer database transaction/outbox boundaries, with explicit acknowledgement and duplicate tests. Durable history is a separate delivery gate from login.

Production uses its own Terraform root/state and lifecycle. It must not be attached to the disposable demo cleanup. Keep permanent domain resources separate. Reuse the existing EKS deployment adapter only where its behavior has upgrade/rollback evidence; reference installation readiness is not a production release gate.

## Ownership and Contract Boundaries

- Authentication adapter: IdP validation, discovery/key rotation and stable verified subject.
- Application authorization: current membership, action, project, ownership; consumes accepted policy evaluation contract (#207), does not invent competing policy semantics.
- API/BFF: request-local context, safe sessions, typed failures and redacted transport; #203 compatibility.
- CLI: explicit connection profile, browser PKCE, OS secret storage, refresh/logout; no Kubernetes requirement for members.
- Persistence: transactional ownership and facts, migration, idempotency, restart reconciliation.
- Operator infrastructure: IAM workload permissions, Cognito/database/runtime resources, TLS, logs, alarms, release and recovery. AWS/SQL types do not enter ClaimRequest.

## Alternatives Considered

- Per-member Kubernetes tunnel: rejected by Owner; grants an infrastructure entry and keeps shared application identity.
- Shared demo login or personal static API keys: insufficient identity/revocation/authorization design.
- Bespoke password server: unnecessary credential-management risk; use managed directory.
- Merge #207 and call it authentication: incorrect; it is a policy kernel, not OIDC or user sessions.
- One large untested cutover: rejected; preserve demo until all production release evidence passes.

## Verification Strategy

Deliver independently reviewable slices: (1) accepted identity/membership interfaces and adversarial fixtures, (2) per-request API/CLI identity and owner reads, (3) native/web login and revocation, (4) persistent Work/recovery, (5) reviewed infrastructure and live two-user campaign. These are implementation slices, not reduced production requirements.

Local tests cover malformed JWT/JWKS limits, exact issuer/client/scopes, key rotation, cache/network failure, token redaction, CSRF/PKCE state, invite replay, owner-query filtering, revocation races and transaction/restart faults. Installed proof uses two real accounts with no cluster access, positive inference plus backend-side zero-call denials, browser rendering and CLI parity. Exercise migration/rollback and lost-node/service restart. Run focused/race/full repository gates and independent security/operational review before public cutover.

## Risks and Compatibility

Owner retained AUD100/month and proposed Sydney 10:00–24:00 operation. This is a hard design constraint, not a validated EKS cost estimate. EKS control-plane hourly billing continues with zero nodes; daily cluster re-creation adds startup/teardown time and must not remove persistent identity/storage. A costed topology and recovery choices are still required. Email delivery/sandbox limits require verification. Existing in-memory runners cannot be made durable by serializing their final output alone. A single registered template/global setup does not prove multi-user isolation. The active demo cleanup remains unchanged. No production capability is claimed by this planning packet.

Owner resolved deployment ownership on 2026-10-12: designated administrators operate the shared EKS deployment. Project membership/SSO role management cannot grant infrastructure administration. Validate member-versus-operator denial separately from successful member Work submission.

## First implementation boundary

`internal/authentication` accepts a verified private identity and requires a current membership/revocation resolver on every request. It returns only the public principal projection; it never grants policy authority. No persistence substitute is provided. `internal/authentication/cognito` pins the issuer/client/scope, accepts only RS256 access tokens, retains token/session identifiers for revocation lookup, and bounds JWKS size/count/refresh frequency and HTTP time. It does not treat token email/groups/team as application roles. Known keys expire after fifteen minutes; failed refresh after expiry denies instead of using stale keys. Unknown-key refresh is limited to once per thirty seconds, so newly rotated keys can temporarily fail closed.

The connected-client library accepts one explicit HTTPS origin and a token source, mutually exclusive with all Kubernetes tunnel settings. Redirects are rejected even with an injected HTTP client; transport/errors do not emit bearer secrets. CLI selection/keychain/login integration is still a separate unfinished slice. Existing explicit reference clients retain their loopback behavior.

## Membership, client and session implementation

`internal/authentication/postgres` owns stable issuer+subject to principal/team mapping,
current project roles, membership audit and refresh-family revocation tombstones.
Disabling a member advances the minimum original authentication time; merely refreshing
or re-enabling membership cannot revive the old login. Cognito `auth_time` handling
remains in its adapter. Owner cannot be disabled through the generic membership setter.

Authenticated console composition requires a request-local preparation function AND
current project authorizer. The legacy reference handler/Submit cannot bypass an
already-authenticated Service. Work evidence is filtered before querying runtime state;
operator provider counters are unavailable to members. Runtime checks current project
access at governed operations, with a three-second cancellation poll for active Work.
Database queries are bounded to three seconds; this is a cooperative cancellation
bound, not proof that an external provider never completed an already-started request.
Logout grant-to-active-Work cancellation still needs the persistent ownership/session
integration; do not describe the membership poll as that missing behavior.

Native `agenova login/logout --server <https-origin>` uses a fixed IPv4 loopback
callback and PKCE/state binding; remote run/work selection rejects installation state
and reference-backend flags. OAuth credentials/config live in OS keychain/WinCred/
Secret Service only, with no file fallback. Browser OAuth tokens are AES-GCM encrypted
in PostgreSQL and keyed by hashed opaque session handles. Login attempts are one-use,
five-minute, capped records; refresh compare-and-swap cannot resurrect a deleted
session. Web/native use distinct clients and the API rechecks membership per request.

Current limits and incomplete surfaces: no production wiring or invitation management
API yet; Work is still process-local. Browser sessions have a twelve-hour maximum;
expired session/attempt cleanup occurs at new-session/attempt creation. Key rotation,
remote state, DB migrations/rollback, persistent Work and real IdP acceptance are
release gates rather than implied by the new packages. No high-availability claim is
made for the budget-first hosting design.

Latest Owner override: do not configure database backups. Earlier references to
backup/restore gates in this design are superseded by the storage decision in the
spec. Keep restart/crash recovery, safe migrations and non-destructive rollback.
The database is managed RDS, Single-AZ; it is not described as HA. Automated backups are disabled per the Owner decision. Storage remains billable while stopped.

## Operational role layers (Owner decision, 12 October)

Account membership and project grants are independent. An invitation defaults to
account Member even when its project role is Admin. Only Owner appoints account
or project administrators and additional duties. Transfer requires a named active
Account Admin, explicit confirmation and an audited transaction; generic edits
cannot remove the last Owner. Project administrators see/manage only their projects.

Governance Admin is an explicit account or project grant, not inherited from
Owner. Migration 007 adds account/project-qualified immutable Policy and Template
records. Policy version registration, active-pointer change and audit commit
atomically; changing content at an existing identity conflicts. A project writer
cannot register another team's/project's rules. Runtime compatibility remains an
operator-controlled check. New Work resolves its own project registry, falling
back to installed defaults only for absent records, never database errors. Portal
configuration is selected by authorized project and template and uses the same
registry; Work admission independently rechecks membership and claim policy.
This is narrow create/identical-reapply and version activation, not arbitrary CRUD.

Platform Operator adds an authenticated CLI authorization check for a named
installation before validate/plan/apply/status. The exact validated configuration
or saved status snapshot is reused after this check. Deployment still runs
locally under the operator's own cloud and Kubernetes identity; no API executes
cluster mutations or returns cloud credentials. An application grant does not
restrict or replace separately held cloud administrator powers. Existing local
operator workflows remain available under those independent IAM/RBAC boundaries.

Migration 007 is additive but older binaries reject newer schemas. Do not roll
back by dropping tables/data; a rollback binary must understand migration 007.
Runtime needs SELECT/INSERT on immutable registry tables and SELECT/INSERT/UPDATE
on the active pointer; it must not receive DDL or audit UPDATE/DELETE privileges.

## Superseding Owner decision: registration and administrator approval

Automatic email delivery is cancelled. No SES resources or emails were created.
For the operator-pinned account, verified Google sign-in creates an idempotent
pending access request, bound to issuer and subject, with no application session,
CLI authorization code, project grants or Work access. Existing disabled identities
cannot register around revocation. Account Owner/Admin review requests; only Owner
appoints administrators, and other approvals require the reviewer’s project Admin
grant. Approval atomically writes membership, project grants, status and audit.
Rejected requests do not automatically reopen. New accounts see a contact-admin
message and sign in again after approval. Existing invitation-only installations
retain their explicit configuration. Public tenant creation stays excluded.
Tests must cover pending browser/native denial, repeated login, concurrent review,
role escalation, team/project isolation, audit rollback and rendered approval UI.

### Remembered CLI service

A successful login writes only its HTTPS origin to the OS user configuration
folder at `agenova/current-server`, atomically with owner-only file permissions.
Tokens remain in the OS credential store. Subsequent `run`, `work` and `logout`
commands use it unless `--server`, `--state-dir` or `--backend` is explicit.
Deployment and governance commands retain explicit targets. Logout removes a
matching saved origin after server revocation succeeds. Invalid saved configuration
fails closed instead of selecting local installation state. Help/version never
read the saved server or credentials.

On macOS, newly created credentials trust the creating Agenova executable through
Keychain ACLs. Existing records retain their access list; choose Always Allow for
the installed executable when macOS asks. A locked keychain or changed executable
can require consent again. This setting does not trust all applications and does
not provide an unencrypted credential fallback.

## Superseding Owner decision: Team administrator bundle, 13 October

Team administrator (Account Admin plus Project Admin) includes governance only in
its administered projects and application deployment authorization only for the
serving installation pinned by trusted server configuration. Neither account-only
nor project-only Admin qualifies. Owner remains separate. No AWS/IAM/RBAC grants
are created. Existing explicit scoped duties remain additive and independently
revocable. Current membership is checked on each operation; demotion/disable
removes derived authority. Governance rechecks inside its write transaction.
Update UI/docs and test real PostgreSQL positive/negative/revocation paths plus
rendered browser flows and the full gate. Operator examples omit optional
state-dir; verify default-directory behavior without sharing local state.
