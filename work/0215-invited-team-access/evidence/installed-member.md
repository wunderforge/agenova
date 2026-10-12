# Installed member and invitation implementation evidence

12 October 2026, Ticket #215. This is implementation and local container evidence;
no production cutover, real Cognito mail, real RDS provisioning or two-account live
acceptance has occurred.

- [Focused race tests](installed-member-focused.txt) use disposable PostgreSQL and
  exercise authenticated composition, deny-before-side-effect invitation checks,
  one-time Owner bootstrap, subject-bound acceptance, replay/expiry/disabled-member
  rejection, migration checksum checks and protection of operator diagnostics.
- [Browser contract](invitation-ui.txt) is executable with
  `node harness/integration/identity/invitations.mjs`. It checks the actual embedded
  invitation HTML/CSS/JS against **synthetic API responses**, including submitted
  role/email/project, denial messaging, XSS-safe text, mobile overflow and POST
  logout. [Desktop](invitation-desktop.png), [mobile](invitation-mobile.png).
- Authentication startup has no Team A fallback. Database URL TLS verification,
  schema compatibility, key material and Portal build are required. Pod-IP health
  probes remain compatible; detailed status requires an actual loopback peer.
- Owner invites Admin; Admin invites Member/Viewer. There is no automatic IAM/EKS
  grant or policy widening. Delivery uncertainty does not enable membership.
- Patched Go 1.27.2 is pinned. EventStream/Bedrock dependencies were updated for
  GO-2026-5764; minimum module Go is 1.24.0. Historical foundation build versions
  are not evidence of the current production binary.

Still required: full member role editing; unknown/expired/cancelled invitation
reconciliation; durable Work/facts and crash-safe idempotency; logout grant-bound
active-Work cancellation; costed host/network deployment, least-privilege DB/IAM
bootstrap, actual RDS/Cognito/Bedrock positive/negative and restart tests; two
personal browser/native-keychain logins; independent review. Work history remains
process-local. Do not release this branch as production-ready yet.

## Final checks for this slice

- `GOTOOLCHAIN=go1.27.2 AGENOVA_TEST_POSTGRES=<disposable PostgreSQL URL>
  pwsh -NoProfile -File scripts/check.ps1 -All`: [passed](installed-member-all.txt),
  including 52 existing browser smoke cases. The invitation page has its separate
  executable browser contract linked above.
- Scoped `govulncheck@v1.1.4` over control-plane, authentication, invitations,
  native login and web sessions: [zero reachable vulnerabilities](installed-member-vulnerabilities.txt),
  zero affected imported packages; two module-level findings outside imported packages.
- Linux arm64 [control-plane image](installed-member-container-build.txt) and
  [compatible worker image](installed-member-worker-build.txt) built successfully
  with Go 1.27.2. The control-plane build used the existing trusted corporate CA as
  a build secret; TLS verification stayed enabled and that CA was not copied to
  the runtime layer. Local images only: no ECR push or cloud rollout.
- [Container startup smoke](installed-member-container-smoke.txt) confirmed that
  member mode without configuration and an unknown mode both exit before service
  startup. This is not a live AWS or runtime-isolation test.
