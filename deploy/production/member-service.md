# Google sign-in member service

Operator wiring for Ticket #215. This is tested service composition, not a
completed production release. Managed RDS/hosting, durable Work recovery and real
two-account acceptance remain release gates. Do not apply the historical Cognito
root in `infra/identity`; Google OIDC is the selected identity provider.

Google authenticates a person. Agenova invitations, current memberships and project
roles authorize application access; they do not grant AWS or Kubernetes permissions.
Google tokens are discarded after callback verification. APIs use revocable Agenova
credentials, not Google ID tokens.

## Startup

Build the Portal assets and place them in a dedicated read-only directory containing
only public build output. Configure `AGENOVA_AUTH_MODE=member` and
`AGENOVA_MEMBER_CONFIG=/etc/agenova/member-service.json`:

```json
{
  "origin": "https://app.agenova.app",
  "googleOAuthFile": "/run/agenova-secrets/google-oauth.json",
  "databaseUrlFile": "/run/agenova-secrets/database-url",
  "sessionKeyFile": "/run/agenova-secrets/session-key",
  "portalDirectory": "/opt/agenova/portal"
}
```

The private Google JSON contains `provider` (`google`), `project_id`, `client_id`,
`client_secret`, and `redirect_uri` (`https://app.agenova.app/auth/callback`). Register
that exact callback on a Google OAuth **Web application** client. Keep the secret in
a mounted secret file, never in the Portal bundle, CLI, Terraform state or logs.
Only `openid email` is requested. There is no separate Agenova password. If the
Google app is in Testing, configure the intended test accounts in its audience
before attempting real sign-in; testing-mode access is not a public release.

The database URL file contains the dedicated runtime user's URL, including `sslmode=verify-full`,
`sslrootcert=/etc/agenova/rds-ca.pem`, and `connect_timeout=5`; URL-encode credentials.
Never use the RDS master/migration role as the running application. The key file is
a base64-encoded 32-byte random secret retained across restarts. Secret values do
not belong in Terraform variables, images, task evidence, CLI arguments or logs.

The protected Work/Portal listener is `127.0.0.1:8081`. Terminate HTTPS at a reverse
proxy on the **same host/pod**, preserve the configured public Host, and proxy all
paths including `/auth/*` and `/login`. Do not proxy directly from a remote ALB to
this loopback HTTP listener. Port 8080 keeps `/readyz` reachable for existing Kubernetes Pod-IP probes. Detailed
operator status on that port requires an actual loopback peer; forwarded headers
cannot satisfy that restriction. Do not publish it through the proxy. The health check
is installation readiness only, not a production capability assessment.

An empty/malformed member configuration, unknown authentication mode, unavailable
DB, schema mismatch, missing Portal build or invalid encryption key prevents
startup. There is no fixed Team A fallback in member mode. The old private `submit`
command receives an authentication rejection; members use the HTTPS CLI instead.

## Explicit migration

Run as a designated database migration operator, with that role's protected URL file:

```sh
agenova-control-plane identity-migrate --database-url-file /run/agenova-secrets/migration-url
```

Migration is transactional, takes a database advisory lock, records its checksum,
and rejects an incompatible database version or changed applied migration. Repeating
an unchanged migration preserves data. API startup only checks the schema; the
runtime role should have no CREATE/ALTER/DROP privileges. Do not alter an applied
migration to deliver later schema changes. Rollback compatibility/recovery and the
production least-privilege SQL grant installation still need release evidence.

## Member commands

After the service and invitations are live:

```sh
agenova login --server https://app.agenova.app
agenova run -f work.yaml --server https://app.agenova.app
agenova work list --server https://app.agenova.app
agenova logout --server https://app.agenova.app
```

No AWS keys, kubeconfig, port-forward or installation `--state-dir` is used for these
commands. OS credential storage is required; the CLI does not fall back to a plaintext
token file. Operators retain the separately authorized EKS installation flow.

## Initial Owner and invitations

Bootstrap exactly once with the designated operator, using a private JSON file
containing `email`, `team` and `project` (real recipients stay out of repository docs):

```sh
agenova-control-plane identity-bootstrap --config-file /etc/agenova/member-service.json --owner-file /run/agenova-secrets/initial-owner.json
```

Google bootstrap creates a pending application invitation and writes its one-time
link to `<owner-file>.invitation` with mode 0600 and exclusive creation. It sends no
email and creates no Google account. Deliver that link privately to the intended
Owner, who signs in with the matching verified Google account. A second bootstrap
is refused, including while the first is pending. Do not delete a pending row or
repeat bootstrap to work around a failed link-file write; reconcile it explicitly.

The authenticated Portal links account administrators to `/members`. `Create
invitation` returns a one-time link once; the database stores only its hash. Share
it privately with the specified recipient. A Google login and matching verified
email are both required. The proof starts in a URL fragment, is removed from the
address bar and posted only to the same-origin login handler. List responses never
return the proof. The app does not claim to have emailed the recipient.

Only Owner may invite an account/project Admin. Admin can invite Member or Viewer
into an existing project. Invitations are bounded to ten per team/hour and 200
retained records/team, with a seven-day lifetime. Accepted links cannot be reused,
merge an existing identity or reactivate a disabled member. The management page can cancel or replace a pending/expired link. Replacement
invalidates the old proof atomically, preserves roles, and is capped at ten per
actor/hour. Accepted invitations cannot be replaced.

Browser sessions use Secure, HttpOnly, SameSite cookies and server-side encrypted
records. CLI login explicitly opens the browser, then receives a one-use code at
`http://127.0.0.1:38123/callback`; PKCE protects its exchange. The Google client secret
never leaves the server. Both application sessions expire after at most twelve hours;
there is **no Google refresh token**. Run `agenova login` again after expiry. Logout
revokes the application grant and clears the local credential/cookie. The originating session is checked before each governed operation and every three
seconds during active Work. A revocation check has a three-second timeout, so
cooperative cancellation is requested within six seconds. An already-started
external call may still finish; cancellation is not a promise to undo it.

Build with Go 1.27.2. Verify service composition using the focused SQL/authentication
suite and the rendered login/invitation harnesses described in the task evidence.
Synthetic Google exchanges prove the contract, not real Google sign-in or cloud readiness.

The bootstrap team/project must match the operator-registered governance policy.
An invitation creates membership; it does not create or widen Policy/AgentTemplate
permissions. A mismatch must deny Work rather than synthesize a permissive policy.

## Member administration

`/members` lists active/disabled members and their account/project roles. Owner can
appoint/demote account Admins and edit project grants. Admin can edit Member/Viewer
grants only in projects they administer. Disabling a member is allowed to Admin
only when that person has no project Admin role and all their project memberships
are administered by the actor. Owner itself cannot be disabled/demoted here.
Enabling a disabled account never restores old sessions; a fresh login is required.
These controls use authenticated `POST /api/members`, not caller-supplied team or
actor fields. Changes and their actor are audited in the same database transaction.

## Durable Work and interrupted startup

Member mode requires migration 005 and an exclusive PostgreSQL writer session.
Stop the previous writer cleanly before replacement; a second live writer cannot
accept Work. The service persists admission before dispatch and records governed
calls before invoking providers. User-chosen names are scoped to the member; use
returned Work references for later `agenova work show`. Retrying identical input
with the same name returns its existing Work, including after restart.

Startup repairs lost terminal publication and reconciles recorded allocations by
stopping and releasing their original worker. It never repeats inference, allocates
another worker, or restores a terminal claim's authority. Unknown allocation identity,
changed worker ownership, unconfirmed release and previously failed cleanup block
startup with an operator recovery diagnostic. Do not delete database rows, mark them
complete manually, or resubmit under a new name to evade that block. Preserve the
record and inspect its exact backend correlation. The operator procedure and real
cluster recovery campaign remain release gates.

A failed recovered Work can contain a committed ProviderAttempt with no outcome.
The Portal displays `Outcome unknown`; an Allow with no attempted call is `Not
started`. These are observations about missing execution evidence, not a new
permission or a successful inference result. The original provider effect may have
completed, so recovery does not automatically retry it.

## Current EKS validation deployment (12 October)

`app.agenova.app` is routed through the existing ALB to the separate
`agenova-member-control-plane` Deployment and NodePort 31090. The original service
is retained pending real-user acceptance. `infra/member-edge` owns the additive
entry resources; `infra/database` owns managed RDS. Both depend on the existing VPC.
Do not resume old demo destruction or destroy the old ALB/cluster during validation.

The deployed image includes the Portal: build `deploy/reference/Dockerfile`, then
use `deploy/production/Dockerfile` with `CONTROL_PLANE_IMAGE` pointing to that build
(prefer its immutable registry digest). The bundled `nginx.conf` forwards only the
configured member host over the private same-Pod listener and uses writable `/tmp`
for every nginx temporary directory. All credentials remain mounted Secret files.
The manifest example records the validated deployment shape, not a generic install.

Run migrations 001–006 as the database operator. Give the separate runtime login
only schema USAGE, database CONNECT, required table DML and sequence access. Remove
DDL capability, migration-table writes, and UPDATE/DELETE on audit/fact tables.
Verify these privileges explicitly. Bootstrap uses a temporary private operator Pod;
remove that Pod and its master-credential/SQL Secrets after preserving the private
Owner link. Never mount the database master credential into the running service.

A new Pod's first five-second DB connection failed during this deployment; verify
connectivity from a ready Pod and repeat the transactional migration rather than
weakening TLS or exposing the database. The executed migration was repeated safely.
Runtime-role creation and Owner bootstrap are distinct operations and must not be
blindly repeated after their first success.

### Role-authorized management CLI

Account and project roles are assigned independently. Owner can also grant
`governance-admin` for an account/project and `platform-operator` for an installation.
These duties do not add Work permissions or provider credentials. Governance uses
immutable registration: a new Policy version may become active, while an existing
identity with different content conflicts. Projects must already exist.

```sh
agenova login --server https://app.agenova.app
agenova policy apply -f policy.yaml --server https://app.agenova.app --project payments
agenova agent-template apply -f engineer.yaml --server https://app.agenova.app --project payments
agenova platform plan -f platform.yaml --server https://app.agenova.app --installation YOUR_PLATFORM
agenova platform apply -f platform.yaml --server https://app.agenova.app --installation YOUR_PLATFORM
agenova platform status --server https://app.agenova.app --installation YOUR_PLATFORM
```

Platform operations also require the operator's local AWS profile/kubeconfig and
cluster RBAC. The installation name must match the file or saved applied state.
Ordinary members use `run`/`work` with `--server` and need no cluster credentials.
Portal's project/template selectors display the corresponding registered
configuration; role and policy checks still run on every submission.

Before deploying this extension, run the explicit identity migration with an
operator credential. Migration 007 requires runtime SELECT/INSERT on
`agenova_project_policies` and `agenova_project_templates`, and
SELECT/INSERT/UPDATE on `agenova_project_policy_active`. Older binaries reject
schema 007; retain data and use a compatible rollback build. Do not rerun Owner
bootstrap, rotate sessions or grant database superuser rights during this upgrade.

### Google registration with administrator approval

Set `registrationTeam` to the existing account/team identifier in the private
member-service JSON to enable reviewed registration. Omit it to retain the
invitation-only flow. It requires Google authentication and an existing active
Owner; login never creates an account container or an Owner.

New users sign in with Google at `/login` and receive the Access pending page.
Account Owner/Admin review them under `/members` → Pending access requests,
selecting an existing project and explicit account/project roles. Only Owner may
appoint administrators. Other reviewers need Project Admin for the target project.
After approval users sign in again; CLI users rerun `agenova login --server URL`.
No AWS credentials, invitation token or email delivery are required. Pending and
declined users receive no application session or native CLI authorization code.
Disabled members cannot register around revocation. Declined requests do not
reopen on login. The bounded queue holds at most 10,000 registration records per
account; reaching this limit fails closed and needs operator retention review.

Migration 008 adds `agenova_access_requests`; grant the runtime role
SELECT/INSERT/UPDATE on this table, with no DELETE/DDL. Approval, grants and audit
commit together. Preserve the database and existing encryption keys. As with other
versioned migrations, rollback requires a binary compatible with schema 008.
