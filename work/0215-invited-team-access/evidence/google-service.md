# Google service, team administration and session cancellation evidence

Date: 2026-10-12. Worktree branch: `codex/invited-team-access`.

## Verified locally

- Google callback exchanges code/PKCE and validates nonce-bound assertions; raw
  Google tokens are not API credentials. The Portal stores an encrypted server-side
  application session and sends only a Secure/HttpOnly cookie to the browser.
- Native CLI receives an application authorization code at its fixed loopback
  callback, then exchanges it with PKCE. Wrong callback, replay and stolen-code
  verifier mismatch fail. Google secrets never enter CLI configuration. Application
  credentials have a twelve-hour absolute lifetime and use OS credential storage.
- Real disposable PostgreSQL tests cover atomic invitation acceptance, current
  inviter authority, proof replay/expiry/replacement/cancellation, member disable
  and fresh-login requirements, cross-team/project administration denial, and audit.
- Real PostgreSQL session revocation while a synthetic provider is running cancels
  the originating Work within six seconds, with one provider attempt and no replay.
  The unrelated second login still authenticates. This is cooperative cancellation,
  not a rollback of a call already accepted by an external provider.
- Rendered browser fixtures exercise Continue with Google, CSP, invitation fragment
  removal, same-document invitation navigation, same-origin POST, mobile overflow,
  member disable/enable/Admin assignment, invitation replacement and POST logout.
  API/Google transports in these rendered fixtures are synthetic.

Commands (Go 1.27.2; local disposable PostgreSQL only):

```sh
GOTOOLCHAIN=go1.27.2 go test -race -count=1 ./internal/authentication/... ./internal/console ./internal/websession ./internal/invitations ./cmd/agenova-control-plane
GOTOOLCHAIN=go1.27.2 go test -race -count=1 ./internal/login
node harness/integration/identity/google-login.mjs
node harness/integration/identity/invitations.mjs
pwsh -NoProfile -File scripts/check.ps1 -All
```

Set `AGENOVA_TEST_POSTGRES` to the disposable database for SQL tests; otherwise
those tests explicitly skip. Local run used PostgreSQL 16 on loopback port 55432
and isolated test schemas. Do not use the local test TLS mode for AWS RDS.

Focused suites passed. Final full gate passed, including 52 browser smoke cases.
Logs are `/tmp/agenova-google-final-gate.log`,
`/tmp/agenova-session-work-focused.log`, `/tmp/agenova-admin-final-focused.log` and
`/tmp/agenova-session-work-final-focused.log`. Rendered proof was inspected at
`.tmp/identity/google-ui/{desktop,mobile}.png` and
`.tmp/identity/invitation-ui/google-mobile.png` (ignored local artifacts).

## Cloud state and remaining release gates

Google OAuth client registration exists in project `agenova-login`. Read-only
Chrome inspection showed External/Testing with zero configured test users; no
Google sign-in has yet been exercised against the installed service. Client secret
is in private ignored storage, not in Git or this evidence. No invitation was sent
or activated for the two real recipients. No new cloud resources were applied.

Durable Work/facts, idempotent restart recovery, budgeted AWS hosting/RDS,
least-privilege SQL deployment, real Google/native-keychain/two-user EKS+Bedrock
campaign and independent review remain required. These tests do not establish
production readiness. Cognito provisioning remains superseded and must not be applied.

## 12 October Google form redirect repair

Google login GET now permits the pinned accounts.google.com destination in its
form-action CSP, retaining the existing same-origin POST and callback checks.
The browser harness uses a real HTTP POST/303 across two local fixture origins;
redirect route mocks did not model the browser navigation reliably. The unit
assertion checks the production Google CSP allowlist. No fixture is live IdP proof.

- `node harness/integration/identity/google-login.mjs`: passed.
- `GOTOOLCHAIN=go1.27.2 go test ./internal/websession`: passed.
- Full `scripts/check.ps1 -All` with disposable PostgreSQL: passed, 53 browser cases.
- Deployed image `sha256:8693375789d487e68f6493d53622f709e17acdc251431facb6daeacec4be6d50`;
  member Deployment rollout completed.
- Live GET /login: 200 with Google form-action allowance. Same-origin POST /login:
  303 to accounts.google.com/o/oauth2/v2/auth, openid/email only, callback
  https://app.agenova.app/auth/callback. No domain or OAuth callback change.
- Actual automated Chrome click still produced ERR_BLOCKED_BY_CLIENT; IAB did not
  navigate. User manual click requested to distinguish client interception.
  Real Google authentication and two-user acceptance are NOT yet verified.

### Follow-up: native form Origin

A second real-browser fixture reproduced `Origin: null` for the HTML POST when
Referrer-Policy was `no-referrer`. The synthetic handler previously accepted the
POST without the production Origin check, hiding this failure. Use `same-origin`
on BFF responses to preserve Origin on same-origin forms while suppressing external
referrers. The fixture now asserts the actual POST Origin; do not accept null
Origin or relax the service's pinned-origin check. Focused browser and websession
tests pass. Live rollout and Google navigation evidence follow separately.

- Follow-up full gate passed (53 browser smoke cases).
- Follow-up live image:
  `sha256:bb9bc348da700c6bc0262d1debef223151eb51ad26120592b46bc6f57f5e4a3b`.
  Kubernetes rollout completed. In the actual Codex browser, clicking Continue with
  Google reached accounts.google.com and rendered “Choose an account” for Agenova.
  This supersedes the earlier navigation blocker; account selection/consent,
  Owner bootstrap and invited Admin acceptance remain separate pending tests.
