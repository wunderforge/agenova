# Google registration and administrator approval

Owner replaced email invitations with Google registration followed by explicit
administrator assignment. No SES identity, DNS records, mail permissions or
emails were created by the cancelled email work.

## Behavior

Operator-pinned `registrationTeam` enables the flow. A verified Google issuer and
subject create one pending record; email is display/contact metadata, not the
membership key. Registration creates no membership, browser session or native
CLI code. Existing active members still log in; disabled identities cannot
register around revocation. Account Owner/Admin review the pending queue. Owner
alone appoints administrators; other approvals require the reviewer's existing
Project Admin grant. Approval, project grants and audit commit atomically.
A declined request does not reopen on another login. Existing invitation-only
configuration remains available, and previously issued links are not revoked.

## Executable evidence

- `AGENOVA_TEST_POSTGRES=... GOTOOLCHAIN=go1.27.2 go test -race ./internal/authentication/postgres ./internal/websession ./internal/invitations ./cmd/agenova-control-plane` passed against disposable PostgreSQL on port 55432, using isolated schemas.
- New database tests prove repeated login is idempotent, pending credentials deny,
  verified identity and team binding, disabled-member denial, one winner among four
  concurrent approvals, rejection persistence, Account Admin escalation denial,
  cross-project denial and full rollback when audit insertion fails.
- Browser/native callback tests prove pending identities redirect to the pending
  page without a browser credential or native authorization-code redirect.
- `node harness/integration/identity/access-requests.mjs` passed: pending queue,
  explicit independent account/project roles, approval payload, hidden invitation
  form, desktop/mobile rendering. Uses synthetic identities/API responses.
- Existing `invitations.mjs` and `google-login.mjs` passed for legacy compatibility.
- `pwsh -NoProfile -File scripts/check.ps1 -All` passed with Go 1.27.2, Node 24 and
  the disposable PostgreSQL URL; 55 browser smoke cases passed.
- `git diff --check` passed. Rendered local proof is under ignored
  `.tmp/identity/access-ui/` (desktop, mobile and live administrator view).

## Actual deployment

- STS verified expected non-root deployment operator and AWS account.
- Migrated managed RDS through version 008; runtime granted SELECT/INSERT/UPDATE
  on the access-request table. Active Owner count remains one. Temporary migration
  Deployment and operator Secret were removed after successful migration.
- EKS member deployment rolled out successfully with image
  `sha256:0651bc8f78e032e0e5c6750eaa69fbe69a7db36598c3c51b1ff0647a839bc68d`.
- Enabled the existing trusted team in operator configuration. Preserved database,
  existing memberships, sessions and keys. No real roles were granted during tests.
- HTTPS `/login` and `/access-pending` return 200 with the new copy;
  anonymous `/api/access-requests` returns 401. TLS verification remained enabled
  (Python's strict local CA check failed; system curl with the configured CA passed).
- Existing Owner browser session survived rollout. Live `/members` shows Pending
  access requests, no pending requests, one active Owner, and no invitation form.

## Remaining acceptance

The second person must complete their real Google login to populate the live
queue; no fake production Google identity was inserted. Real two-person approval,
OS-keychain CLI login and Work/Bedrock acceptance remain unproven. The pre-existing
pending invitation was not cancelled; the user may instead sign in normally to
exercise registration. Full production cost/availability/rollback gates remain
as recorded in the task packet. Earlier binaries reject schema 008, so rollback
requires a compatible binary, never dropping persistent tables.
