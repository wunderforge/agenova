# Member service composition (superseded Cognito wiring)

**Do not deploy this Cognito configuration or run its bootstrap command.** The
Owner selected Google OIDC with no separate password on 12 October 2026. This
file records the earlier implementation until replacement composition is tested.
See [current design](../../work/0215-invited-team-access/design.md).

This is operator wiring for Ticket #215, not a completed production release.
Live Cognito/RDS provisioning, invitation recovery, durable Work/reconciliation and the live two-account
campaign must be completed before public cutover. Existing EKS runtime and immutable
Policy/AgentTemplate registration are reused; application membership grants no
Kubernetes rights and does not replace the claim policy.

## Startup

Build the Portal assets and place them in a dedicated read-only directory containing
only public build output. Configure `AGENOVA_AUTH_MODE=member` and
`AGENOVA_MEMBER_CONFIG=/etc/agenova/member-service.json`:

```json
{
  "origin": "https://app.agenova.app",
  "issuer": "https://cognito-idp.ap-southeast-2.amazonaws.com/ap-southeast-2_REPLACE",
  "oauthOrigin": "https://REPLACE.auth.ap-southeast-2.amazoncognito.com",
  "nativeClientId": "REPLACE_NATIVE_CLIENT_ID",
  "webClientId": "REPLACE_WEB_CLIENT_ID",
  "scope": "agenova/api",
  "databaseUrlFile": "/run/agenova-secrets/database-url",
  "sessionKeyFile": "/run/agenova-secrets/session-key",
  "portalDirectory": "/opt/agenova/portal"
}
```

Obtain the issuer/client/domain values from `infra/identity` outputs. The database
URL file contains the dedicated runtime user's URL, including `sslmode=verify-full`,
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

The operator needs Cognito `AdminCreateUser` and `AdminGetUser` on the one configured
User Pool, and the database membership write permissions. No IAM user or cluster
role is created. Cognito first creates a subject with mail suppressed; the application
persists its inactive Owner mapping before requesting an email. The Owner completes
Cognito password/MFA setup and signs in, activating the subject-bound invitation.
A second bootstrap is refused even while the first is pending.

The authenticated Portal links account administrators to `/members`. It sends
`POST /api/invitations` with email/project/role and displays the durable invitation
status. Only Owner may invite an account/project Admin. Admin can invite Member or
Viewer into an existing project. Project roles still pass the existing claim policy.
Invitations are bounded to ten per team/hour and 200 retained records/team. Their
initial lifetime is seven days. Login activates only a sent, unexpired invitation
bound to the verified Cognito subject. Accepted invitations cannot reactivate a
subsequently disabled member. Existing directory identities are never adopted or
have their aliases stolen by a failed create.

An uncertain mail/create result is deliberately not retried automatically. Delivery
reconciliation, cancelled/expired invitation replacement, account-role changes and
full member administration remain unfinished release work. Do not manually alter
DB rows to conceal an unconfirmed directory result or call this complete production
invitation lifecycle evidence. Passwords are generated/delivered by Cognito and never
returned by the application API.

Builds use the patched Go 1.27.2 toolchain; the AWS EventStream security fix raises
the module's minimum Go version to 1.24.0. Historical evidence captured with earlier
Go versions is preserved rather than relabelled as a current build result.

The bootstrap team/project must match the operator-registered governance policy.
An invitation creates membership; it does not create or widen Policy/AgentTemplate
permissions. A mismatch must deny Work rather than synthesize a permissive policy.
