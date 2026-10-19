# Google OIDC replacement foundation

Date: 2026-10-12. Ticket: [#215](https://github.com/wunderforge/agenova/issues/215).
The Owner selected existing Google accounts, no Cognito or separate Agenova passwords.
This is implementation evidence, not production acceptance.

## Implemented

- Callback-only Google RS256 ID-token verification with pinned issuer/JWKS/client,
  exact nonce, issued/expiry times, authorized party and verified email. Google
  tokens deliberately cannot implement the API credential-verifier interface.
- Server authorization-code/PKCE exchange requests only `openid email`; no offline
  access. It returns identity metadata, never Google API/refresh tokens. The caller
  must atomically consume the server-owned login attempt before exchange.
- Hashed opaque Agenova application credentials in migration 003, current active
  membership/revocation checks, absolute lifetime up to twelve hours and 32-session
  per-member cap. Restart reads do not depend on an in-process credential cache.
- Application-owned Google invitations in migration 004, one-use random proof,
  verified authoritative email plus stable Google subject binding. Existing
  identities are never linked/reactivated by email. Acceptance rechecks inviter
  authority; concurrent acceptance commits exactly once. Owner bootstrap is a
  private operator capability, not a public signup endpoint.
- Prior migrations remain unchanged. Prior Cognito wiring is marked superseded;
  no Cognito Terraform apply is authorized by the replacement design.

## Reproducible tests

Use only a disposable local PostgreSQL database for this command:

```sh
GOTOOLCHAIN=go1.27.2 \
AGENOVA_TEST_POSTGRES='postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable' \
go test -race -count=1 ./internal/authentication/... ./cmd/agenova-control-plane
```

[Focused output](google/focused.log) covers fake Google signatures/endpoints,
wrong audience/nonce/party/signature, token expiry, unsupported headers, key
rotation/outage, bounded JWKS, confidential code exchange, real disposable SQL
migrations, session revocation/disable/re-enable, invitation binding/replay,
parallel acceptance, expired invitations and inactive inviters. TLS disabling is
local test configuration only, never an AWS database setting.

## Cloud state and release blockers

Google project `agenova-login` exists and its OAuth branding configuration was
created in the browser. Web OAuth client creation is prepared, not yet confirmed.
No Google compute/database or billing upgrade was created. No real Google token,
invitation email, production session or new AWS resource was exercised here.

The installed service still needs replacement composition: Google callback and
one-use attempt integration, application invitation delivery/acceptance UI,
CLI authorization handoff, full member administration, Work persistence/recovery,
production secret provisioning, costed hosting and two-user live acceptance.
The Google client intentionally does not consume state/nonce itself: the HTTP
login controller must do so atomically before using it. Do not expose the earlier
Cognito composition or describe these package tests as an end-to-end login.

Full repository gate passed with Go 1.27.2, Node 24 and the same disposable SQL
fixture: `pwsh -NoProfile -File scripts/check.ps1 -All`. This includes 52 existing
browser smoke cases, not a new live Google login browser test.
[Full output](google/full-gate.log).
