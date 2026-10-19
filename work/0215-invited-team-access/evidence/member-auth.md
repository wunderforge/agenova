# Member authentication implementation evidence

Date: 12 October 2026. Ticket: #215. This is local implementation evidence, not
production release or invitation completion.

Passed:

- Patched-toolchain verification uses `GOTOOLCHAIN=go1.27.2`. The PostgreSQL
  SQL driver is `github.com/lib/pq v1.12.3`; keyring transitive `jose2go` was
  upgraded to v1.7.0 after the initial vulnerability scan failed.
- `govulncheck@v1.1.4` over authentication, native login and web sessions found
  zero reachable vulnerabilities and zero affected imported packages, with two
  module-level findings outside imported packages: [scan output](member-auth-vulnerabilities.txt).
  This is a scoped scan, not an independent production security assessment.
- `terraform -chdir=infra/database validate` succeeded. The RDS root has not
  been planned with production network inputs or applied.

- `AGENOVA_TEST_POSTGRES=<disposable local DSN> go test -race -count=1
  ./internal/authentication/... ./internal/login ./internal/websession
  ./internal/console ./internal/cli ./cmd/agenova`: [focused output](member-auth-focused.txt).
- `pwsh -NoProfile -File scripts/check.ps1 -All`: [full output](member-auth-all.txt),
  including 52 existing browser smoke cases. This gate was rerun after the database
  driver and keyring dependency updates using Go 1.27.2.
- Real PostgreSQL 16 in a loopback-only disposable Docker container:
  `postgres:16-alpine@sha256:721873c34ceb9f8d8fc265984940dc982404c105f19ad51be9fdc5970a6080ea`.
  Membership/role persistence, database failure denial, disable/re-enable old-login
  rejection, refresh-family revocation, role escalation rejection, encrypted web
  session data and atomic one-use login callbacks were exercised against SQL.
- Synthetic OAuth issuer/native-browser callback: PKCE/state binding, protected
  member check, refresh, remote logout and redirect/token isolation. OS credential
  backends compile but a real keychain/Cognito login is not yet verified.
- API own/foreign request+claim evidence, reference-mode bypass rejection, member
  revocation and cooperative cancellation of an active provider call.
- `terraform -chdir=infra/identity validate` succeeded; authenticated AWS plan:
  **8 to add, 0 change, 0 destroy**. No apply, user creation or invitation occurred.
- Static login rendering checked at 1100×760 and 390×844, with a visible Sign in
  button and no mobile horizontal overflow. [Desktop](login-render.png),
  [mobile](login-mobile-render.png). This is not a real IdP/browser E2E result.

Remaining: application startup/deployment composition, invitation and admin UI/API,
durable Work/idempotency/reconciliation, real RDS and Cognito provisioning, member
end-to-end login/inference/denial, retained data across stop/start and independent
release review. Existing same-name requests still conflict rather than providing
independent per-user reference namespaces. Logout revocation does not yet cancel
all already-running assignments associated with that grant. Do not claim those
features from the passing package tests. Database backups are excluded by the
Owner's latest decision; this does not remove persistence/restart recovery gates.
