# PR216 review corrections — 19 October 2026

This supersedes the open login-capacity finding in acceptance-20261019.md.
It does not declare all production acceptance complete or a live rollout.

## Corrections

- Anonymous browser/native starts share eight outstanding attempts per trusted
  IPv4 address or IPv6 /64, bounded globally at 128. Verified native codes have a
  separate 128-entry pool. PostgreSQL transactions serialize limits across Store
  instances/restarts. No caller-supplied forwarding header is trusted by default.
- Migration 009 adds kind/source hash columns; prior migrations remain unchanged.
  The same-Pod proxy must overwrite the source header after resolving ALB append
  XFF through the pinned VPC, with NodePort ingress restricted to the ALB SG.
- A terminal fact/Finish failure poisons durable request/claim/list reads with a
  storage-unavailable response. The actual connected client returns HTTP 503 rather
  than continuing to poll incomplete durable evidence. No provider replay occurs.
- Bootstrap rejects a team different from configured registrationTeam. Operator
  recovery can rotate only an exact unaccepted Owner reservation. File delivery
  uses exclusive creation, file/directory sync and precedes proof-hash commit.
  Existing Owner cannot be reissued; output is retained after commit uncertainty.
- Invitation acceptance atomically approves a matching pending access request;
  cross-team/rejected registrations cannot be bypassed by this path.
- nginx accepts governance documents up to 1 MiB + 4096 bytes; other routes keep
  their 128 KiB limit. Application authorization/validation remains unchanged.

## Reproduction and evidence

Uncached real PostgreSQL race regressions passed for internal/console,
internal/websession, internal/authentication/postgres and cmd/agenova-control-plane
using disposable schemas in a local PostgreSQL 16 container. Tests cover 129 hostile
starts versus another source, forwarded-header spoofing, concurrent source limits,
restart/expiry/replay, verified-code reservation, terminal write/Finish failures
through connected-client polling and authenticated HTTP, exact Owner recovery,
recipient mismatch/accepted Owner denial, and pending registration reconciliation.

Run with a disposable AGENOVA_TEST_POSTGRES:

```sh
go test -count=1 -race ./internal/console ./internal/websession ./internal/authentication/postgres ./cmd/agenova-control-plane
python3 work/0215-invited-team-access/evidence/verify-login-proxy.py
pwsh -NoProfile -File scripts/check.ps1 -All
```

Full `scripts/check.ps1 -All` passed: Go checks/tests, 125 frontend unit tests,
production build and 56 browser smokes.

The proxy smoke passed using the exact manifest nginx image in isolated Docker
networks: trusted ALB tail selection, untrusted peer spoof rejection, private-header
overwrite, 200 KB governance acceptance and oversized/ordinary-body rejection.
The independent agent's static follow-up found no additional confirmed regressions
in these corrections; it is not a maintainer approval or live deployment evidence.

Private logs: `.tmp/acceptance-20261019/login-capacity.log`, `login-proxy.log`,
`final-review-gate.log`. No real user grants/sessions, cloud resources or live SQL
were changed. CI configuration remains identical to main.

## Release boundaries

Deploy migration 009, updated nginx, trustLocalProxyClientIP=true and a new control
plane image together as documented in deploy/production/member-service.md. An
image-only upgrade is insufficient. Verify actual ALB append mode and SG ingress.
Old binaries reject schema 009; rollback needs a schema-compatible binary, never
schema/data deletion. Shared NAT users share eight pending slots; this is not a
claim of distributed DoS protection. Worker image/protocol remains unchanged.

Previously recorded whole-service crash/real Bedrock non-replay, compatible live
rollback, interactive Windows/WSL Google login and budget/availability acceptance
remain release gaps. The Owner declined repeating live dual-user revocation;
Fisher was not changed. Maintainer approval is required on the final PR commit.
