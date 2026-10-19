# Task: Production invite-only team access

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215)
- Mission: Deliver invited personal access over HTTPS without granting members AWS or Kubernetes access.
- Target: authentication, connected CLI, console/application authorization, durable evidence, operator infrastructure and acceptance.
- User value: Invited members use their own identity and see only authorized Work; identities, permissions and evidence survive service restarts.
- PRD outcome: [Owner-approved production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Context to Read

- AGENTS.md, docs/product/prd.md, this packet.
- [Spec](spec.md), [design](design.md), [architecture](../../docs/product/architecture-contract.md), [delivery contract](../../docs/development/AIDLC.md).
- [#203 identity transport](https://github.com/wunderforge/agenova/issues/203), [#204 authority constraints](https://github.com/wunderforge/agenova/issues/204), [#207 kernel producer](https://github.com/wunderforge/agenova/pull/207).
- cmd/agenova-control-plane/main.go, cmd/agenova/main.go, internal/console/http.go, internal/console/service.go, internal/connectedclient/run.go, internal/app/prepare.go, internal/facts/journal.go.
- Official [Cognito client configuration](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-settings-client-apps.html), [token endpoint](https://docs.aws.amazon.com/cognito/latest/developerguide/token-endpoint.html), [admin invitations](https://docs.aws.amazon.com/cognito-user-identity-pools/latest/APIReference/API_AdminCreateUser.html).

## Scope

In scope: invite-only managed directory, browser/CLI code+PKCE login, HTTPS CLI target, verified personal identity, active membership and role authorization, own-evidence default, durable Work/audit/session data, restart recovery, revocation, limits and reviewed production deployment.

Out of scope: public signup/billing, runtime-specific shared contracts, worker cloud credentials, arbitrary policy/template editing, silent fixed-principal fallback, blanket production-readiness claims.

## Acceptance Criteria

- Invite Alice and Bob; each uses Portal and CLI with no AWS keys, kubeconfig or installation state.
- Every operation checks verified identity and current active membership; Work and evidence attribution are correct and user/project boundaries hold.
- CLI login uses browser code+PKCE, state binding and protected OS credential storage; browser tokens remain server-side.
- Membership removal and logout have an explicit tested revocation bound; token expiry alone is not described as immediate revocation.
- Evidence survives restart; active Work reconciliation does not re-dispatch uncertain external calls; rollback and restart recovery are exercised; automatic database backups are excluded by the Owner decision below.
- Real EKS/Bedrock positive and zero-call negative evidence, rendered Portal/CLI proof, focused and full gates, independent security/operational review precede production cutover.

## Negative Case

Uninvited/disabled users, reused/expired invitation, forged/expired/wrong-issuer/wrong-client/wrong-token-use JWT, caller-authored identity, cross-user/project reads, redirect/token leaks and management escalation fail closed. Failure never falls back to Team A, grants authority or silently duplicates a model call.

## Execution Todo

- [x] Scout fixed-principal service, loopback-only CLI, process-memory evidence and external identity scope.
- [x] Assignee self-review: Owner explicitly reprioritized production/invite-only scope on 2026-10-12. Preserve architecture; do not change #203/#204 assignments or producer contracts.
- [x] Define shared behavior, dependencies and production release gates in spec/design.
- [x] Implement bounded Cognito access-token verification and a required current-membership/revocation source interface with negative fixtures.
- [x] Supply the durable membership/revocation source and integrate it with each API request.
- [x] Add authenticated HTTPS transport to the connected client without a Kubernetes target; reject token-bearing redirects and ambiguous transport configuration.
- [x] Integrate request-local principal, action/ownership checks and CLI login/profile selection.
- [x] Implement PKCE login, OS credential storage, refresh/revocation and Portal sessions.
- [x] Implement durable storage, migrations, restart reconciliation and data-preserving stop/start.
- [ ] Review and deploy bounded infrastructure after budget/SLO decisions.
- [ ] Execute two-user real-backend, negative, recovery and rendered tests.
- [ ] Run focused gates and ./scripts/check.ps1 -All; independent review and cutover.

## Quality Gates

- Planning/source links: `pwsh -NoProfile -File scripts/check.ps1 -Docs`.
- Implementation: package-specific authentication/authorization/storage/client tests, race tests, real IdP and installed two-user E2E, restart/recovery and revocation probes.
- Repository: `pwsh -NoProfile -File scripts/check.ps1 -All`.

## Evidence Required

Exact local tests, real IdP/client/server transcripts with secrets redacted, zero-side-effect negatives, concurrent user isolation, restart and restore output, rendered views, cost/deployment plan and independent review. Plans and mocks cannot satisfy production acceptance.

## Constraints

Preserve architecture invariants, immutable registration identities and existing explicit local-reference mode. Keep provider-specific settings in authentication/infrastructure adapters. Do not send invitations before recipients are explicitly provided. No production cloud apply until a recurring budget and retention are established. Existing disposable demo cleanup is separate and unchanged.

## Decisions and Blockers

- Owner selected invite-only on 2026-10-12. Cognito is the selected planning default; no user pool or invitations have been created.
- Existing AUD100/month and ten-hour demo authorization is not authorization for an unbounded production bill. Owner confirmed AUD100/month, with proposed Sydney 10:00–24:00 service hours. Cost feasibility and safe shutdown/recovery topology remain unresolved; no production schedule is enabled.
- PR #207 is approved but unmerged; #203 has no published implementation. The integration must reuse accepted contracts or record a maintainer decision; no hidden fork of policy semantics.
- Production availability, recovery targets and initial member count remain to be established before sizing/release. Defaults below are proposals, not promised service levels.

- Owner requested a separate project-account/SSO feature with both member CLI use and EKS deployment. These are separate permission planes: members get application roles; deployment operators need explicit target-cluster IAM/RBAC. Owner confirmed designated administrators deploy the shared EKS service. Ordinary members cannot install, upgrade or mutate cluster resources. No invitation grants cluster permissions.

- Initial implementation is deliberately not wired into the deployed service: Cognito verifier, membership boundary and HTTPS client transport only. No `agenova login` command, durable membership store, invitation, SSO pool, browser-session migration or per-user console composition exists yet. Do not expose the reference API based on these package tests.
- JWT dependency is pinned to golang-jwt/jwt v5.3.1. Focused race tests cover access-token type/client/issuer/scope/expiry/signature, key rotation/cache/outage limits, per-request membership denial and HTTPS redirect isolation.

- First foundation slice passed focused race checks and the full repository gate (52 browser smoke cases). Exact results and explicit unimplemented surfaces: [foundation evidence](evidence/foundation.md). Production release gates remain unchecked.

## 12 October continuation

- Owner explicitly requested every production feature and a testable two-email journey.
  Initial Owner: the confirmed Owner email (private operator input); invited account/project Admin:
  the confirmed invited Admin email (private operator input). Admin can run Work through HTTPS CLI and Portal, invite
  Member/Viewer and administer authorized projects; no IAM/EKS grant is implied.
- Budget decision resolved: budget-first production hosting, AUD100/month, retain EKS
  deployment support. The old ten-hour cleanup heartbeat is PAUSED. No production
  cutover, recurring on/off schedule, new AWS identity resource or invitation has
  been executed by this continuation.
- Added PostgreSQL membership/grant/audit/revocation adapters, request-local console
  admission and owner/project evidence filtering, native browser PKCE/OS-keyring login,
  refresh/logout and HTTPS CLI selection, encrypted durable web sessions and CSRF/state
  checks. These are library/CLI integrations; the installed control-plane composition
  still needs wiring. Native CLI endpoints are not advertised as live.
- Focused real PostgreSQL and synthetic-IdP tests passed, including database outage,
  revoked refresh families, disabled/re-enabled membership, project role escalation,
  cross-user evidence, PKCE binding, callback replay, CSRF and refresh/logout races.
- Permanent Cognito Terraform is in `infra/identity`: validated/planned, NOT applied.
  Public signup disabled, required TOTP MFA, separate web/native clients, five-minute
  access tokens and protected User Pool. No user passwords in code/Terraform.
- Remaining release work is substantive: invitation/admin HTTP+UI, installed service
  composition, durable Work/facts/idempotency and crash reconciliation,
  budgeted host migration, real Cognito/mail/native-keychain/browser/EKS acceptance and
  independent review. Current request references still share a service namespace:
  collisions return 409 without disclosing evidence; independent same-name submissions
  and restart-safe identities remain part of the persistent Work implementation.

- Latest Owner decision: AWS database may be provisioned within AUD100/month; no
  database backups requested. Select managed Amazon RDS for PostgreSQL, Single-AZ and a small Graviton
  instance, with encrypted persistent storage; do not host PostgreSQL on EC2. Daily shutdown stops compute and
  retains data; no nightly destroy/recreate. Remove automatic backup/restore as a
  requested delivery gate, retain durable writes, crash recovery, safe migrations
  and rollback. This updates the earlier production scope rather than silently
  omitting an agreed acceptance criterion. No database cloud apply has occurred.

## Installed member composition slice

- Added optional explicit member service composition to the existing installed
  control plane, with a loopback-only BFF/Portal/API listener and private status.
  Fixed reference identity is never constructed for authenticated admission.
- Startup requires strict operator configuration, verify-full PostgreSQL TLS,
  retained encryption key, public Portal build and a compatible migrated schema.
  The reference private-command route cannot bypass member authentication.
- Added transactional, locked, checksum-checked explicit identity migration.
  Runtime startup does not perform DDL. Operator procedure and member commands:
  [member service wiring](../../deploy/production/member-service.md).
- Real disposable PostgreSQL composition tests cover unauthenticated root/API,
  active and disabled membership, protected diagnostics, repeated migrations and
  altered migration rejection. These do not replace live Cognito/EKS evidence.
- Invitation management, durable Work and production provisioning remain unfinished;
  no new cloud resources or invitations have been created by this slice.

## Invitation slice

- Added Cognito directory adapter (suppressed creation followed by explicit email),
  durable invitation intent/subject binding, one-time Owner bootstrap, account-role
  invitation checks, per-team invitation limits and an authenticated invitation page.
- First verified personal login atomically accepts a sent/unexpired invitation and
  activates its stable membership. No mutable email claim selects membership.
  Replays, expired invitations, unknown delivery and management escalation fail closed.
- The invitation page includes a POST logout control; the installed Portal adds
  logout and an invitation link for account administrators, with root entry forced
  into connected mode. The page/browser contract uses synthetic API responses;
  real Cognito email and two-account acceptance remain pending.
- Extended vulnerability scanning exposed GO-2026-5764 in existing AWS EventStream/
  Bedrock dependencies. Updated EventStream to 1.7.8, Bedrock runtime to 1.50.4 and
  the required AWS core dependencies. Minimum Go is now 1.24.0 with Go 1.27.2 pinned
  for patched builds; both build Dockerfiles were updated. The affected scoped scan
  now reports zero reachable vulnerabilities (two unused module-level findings).
- Full member role editing, invitation reconciliation/cancellation/replacement,
  persistent Work/facts/restart recovery, logout-to-running-Work cancellation,
  budgeted cloud hosting and real-user release tests remain required. No production
  cutover or email to the two confirmed recipients has occurred.

- Latest reproducible results and remaining release gaps: [installed member evidence](evidence/installed-member.md).

## Google sign-in replacement (latest Owner decision)

- Owner selected Google OIDC and explicitly rejected Cognito/separate passwords.
  This supersedes the earlier Cognito deployment/invitation decisions above.
- Google Cloud project `agenova-login` was created using the current browser
  account. No Google compute/database resources or billing upgrade were requested.
- Implement Google callback verification, application-owned invitations, revocable
  application credentials and native browser handoff; retain memberships/roles.
- Do not apply `infra/identity` Cognito resources or send Cognito password emails.
- Google OAuth registration and real two-account acceptance remain unfinished.
  Existing Cognito composition must not be described as Google-ready.
- Identity source: [Google OIDC documentation](https://developers.google.com/identity/openid-connect/openid-connect).

- Google replacement foundation now has callback assertion/code-exchange adapters,
  durable opaque application sessions and atomic Google invitation binding.
  [Tests and release gaps](evidence/google-login.md) distinguish this from the
  still-unfinished installed Google login/CLI flow. No real user invitation sent.

## Google service and member administration continuation

- Google OAuth Web client was registered by the Owner. Credentials are stored in
  an ignored private file, never in this packet or committed configuration.
- Installed member mode now composes Google callback verification, invitation proof
  consumption, encrypted Portal sessions and application-native CLI code/PKCE
  handoff. Google assertions are rejected as API credentials; CLI uses OS storage.
- Added member list, role/grant editing, disable/enable, invitation cancel/replacement
  and expiry presentation. Owner alone appoints account/project Admins. Admin
  project writes require an existing Admin grant for that project.
- Real disposable PostgreSQL tests cover old-session invalidation, cross-project
  administration denial, invitation proof replacement/replay/expiry, and audit.
  Rendered tests cover Google login, mobile invitation/member controls and logout.
- No real Google login, production deployment or invitation to the confirmed
  recipients has occurred. Durable Work recovery, logout-to-active-Work cancellation,
  costed production hosting/RDS and the real two-user campaign remain unfinished.

- Added session-bound active Work checks without placing credentials or private
  session identifiers in ClaimRequest/evidence. Real PostgreSQL logout test cancels
  the originating run within the six-second check bound, rejects further requests,
  and leaves an unrelated login usable. Already-started provider effects cannot
  be rolled back. Persistence/crash recovery remains a separate unfinished gate.

- Current slice evidence and remaining gates: [Google service evidence](evidence/google-service.md).

## Durable Work continuation

- Added explicit migration 005, schema-scoped single-writer fencing, owner-scoped
  idempotency and current-membership SQL reads. Admission and governed facts commit
  before execution; lifecycle facts/snapshots commit atomically.
- Real disposable PostgreSQL + HTTPS CLI tests retain completed evidence across
  restart without a duplicate provider call. Failed attempt persistence blocks the
  provider. Lost terminal-publication acknowledgement is repairable without replay.
- Other interrupted Work still blocks startup pending reconciliation. The installed
  production composition deliberately does not enable the incomplete recovery path.
- [Durable Work evidence and remaining gates](evidence/durable-work.md).

## Interrupted allocation reconciliation

- Member-service startup now acquires the durable writer and reconciles known
  interrupted allocations before admission. Cleanup-only adapter reattachment
  verifies the recorded binding and never starts/replays work.
- Missing invocation results stay explicitly unknown. CLI/Portal recognize failed
  recovery evidence without inventing ProviderOutcome records or model success.
- PostgreSQL + HTTPS CLI fault tests and rendered UI checks pass. Unknown allocation
  identity/previous cleanup failure remain operator blockers, not fabricated success.
- [Recovery evidence and remaining production gates](evidence/recovery.md).

## Owner-requested role separation, 12 October

The Owner approved implementation of the six-duty plan: Account Owner, Project
Admin, Governance Admin, Member, Viewer and installation-scoped Platform Operator.
These are additive scoped duties, not implicit cloud grants. Retain existing
account Admin compatibility while separating it from project grants.

This slice adds explicit Owner-only grant/revoke of Governance Admin (account or
existing project) and Platform Operator (installation), durable audit, current
active-membership authorization and member UI. Owner has no implicit governance
or installation capability. Platform duties never grant IAM/RBAC or business
Work access; unknown actions deny.

Architecture blocker: current registration has one installation-wide active
Policy pointer and one compatible template slot. A project-scoped management API
cannot safely reuse these writes: it could replace another project's policy.
Requested maintainer decision: isolate registry/activation by account/project,
or retain operator-only registration for this PR. Until resolved, do not mount
policy/template mutation APIs or advertise the new grants as working management
commands. No shared architecture contract was changed by this slice.

- [Role matrix and scoped grant evidence](evidence/scoped-roles.md).

## Fixed role directory and deferred governance management

Owner explicitly deferred Policy/Template management changes; retain operator-only
registration and do not change the shared registry/activation architecture in this
PR. The prior architecture question is resolved as a deferral, not approval.
Owner selected a fixed Roles & Permissions directory with existing member role
assignment, not custom permission editing. The member page now describes six
duties and compatibility Account Admin behavior, scopes, limitations and unavailable
management capabilities; it links to existing assignment controls. Service
identities remain separate. No backend permissions are expanded by this UI.

## Live member validation deployment

Owner requested full cutover preparation for hands-on testing. Added an independent
member endpoint on the existing EKS/ALB, managed RDS, runtime SQL role and private
Owner bootstrap. Read [deployment evidence](evidence/eks-member-deployment.md) and
[member-edge dependencies](../../infra/member-edge/README.md) before any cleanup.
The old service is retained until acceptance; the paused destruction automation
must stay paused. Real human Google login is now required to continue the two-user
campaign. Persistent database data must not be destroyed to roll back application
code. Long-term AUD100 hosting feasibility is still an explicit unresolved gate.

## Explicit account/project roles and protected ownership transfer

Owner approved separate account/project invitation choices on 12 October. Omitted
accountRole defaults to member, never derives from project Admin. Existing stored
memberships/invitations retain their explicit grants. Separate management controls
must describe both scopes. Project administrators can administer only their own
projects, not account roles; Owner alone appoints administrators. Planned governance
and installation duties are read-only/unassignable until their capabilities exist;
existing grants remain visible and revocable. Add atomic transfer to an existing
active account Admin with explicit confirmation, auditing and no ownerless interval;
ordinary demotion/disable of Owner stays forbidden. Test escalation, cross-project
reads/writes, invitation acceptance, transfer rollback/concurrency and rendered UI.
No live ownership transfer or actual permission changes are part of deployment.

### Superseding decision: operational duties, not placeholders

Owner explicitly requested all three role layers be operational, superseding the
Policy/Template deferral and the proposed unassignable placeholder end state.
Finish role separation first, then implement scoped immutable Policy/Template
registration and installed-service consumption, with genuine API/CLI authorization
and project isolation. Platform Operator uses Agenova installation authorization
AND the operator's own existing AWS/Kubernetes credentials; no hosted deployment
agent, cloud key upload or implicit cluster permission. Do not deploy placeholder
role UI as completion of this request. Existing platform and claim invariants stay
unchanged. Ownership transfer may be tested with disposable identities only;
never transfer the real Owner during validation.

## Operational role implementation and rollout

Implemented the superseding role decision with scoped governance storage/API/CLI,
local-identity platform authorization, separate account/project roles, protected
ownership transfer and project-aware Portal setup. Focused real PostgreSQL/race
checks and the full gate pass (55 browser smokes). Migration 007 and the member
image are deployed; existing Owner login remains usable. See [role-layer evidence](evidence/role-layers.md)
for exact results, immutable image and pending two-user/native CLI acceptance.
The Fisher administrator invitation is prepared pending the browser-required
per-action confirmation; no membership or real ownership was changed by rollout.

## Automatic invitation email (Owner request)

Add automatic transactional invitation email using an isolated SES adapter and
workload IAM, with fixed sender/origin, no cloud keys in Work/browser, no raw proof
in logs or durable plaintext. Persist delivery attempt status independently of
membership acceptance. SES acceptance is not inbox delivery. Unknown outcomes
must not trigger blind retry; explicit replacement invalidates the old proof and
resends under current inviter authorization. Preserve manual-link deployments
when mail is unconfigured. Verify sender/DKIM through existing Route53; SES sandbox
recipient restrictions and production-access approval are external release gates.
Self-review: requested by Owner; preserves claim authority and immutable identity.

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

Registration/approval implementation, race/real-PostgreSQL checks, rendered UI,
full gate and actual member-service rollout are complete for this slice.
[Access approval evidence](evidence/access-approval.md) records the exact boundaries
and the remaining real second-user login/approval/CLI acceptance. No real user
roles were changed and no mail was sent. The existing pending invitation remains
valid; normal Google login now uses the approval queue without that link.

## Owner-requested macOS package

Deliver a native macOS CLI installer for the current member feature. Existing
GitHub Releases are empty; #183 is a local substrate bootstrap, not a distributed
member CLI. Build a versioned host-architecture pkg with Keychain enabled, explicit
PATH entry, checksum and installation instructions. No bundled secrets, cluster
creation, signing claims or public stable release. Current host has no Developer ID
identity, so label preview/unsigned/unnotarized. Verify payload, architecture,
version and executable commands; run the full gate. Installer execution and real
Intel/Windows acceptance are separate from package construction.
Self-review: fits member CLI distribution; no claim or authority boundary changes.

The Owner clarified distribution should use Homebrew (`brew install`), not pkg.
The uninstalled pkg was a local prototype; replace its build script with a Formula
using an immutable public source archive and checksum, native Keychain integration,
and a designated public tap. Verify actual brew installation and CLI commands.
No unsigned pkg is proposed as the delivery path.

Homebrew Formula syntax and an actual local-tap source installation succeeded on
macOS arm64. Installed at `/opt/homebrew/Cellar/agenova/0.1.0-preview.20261012`,
linked as `/opt/homebrew/bin/agenova`; the Formula pins commit 02d4adb with SHA256.
Public distribution is pending Owner choice of team vs personal tap repository.
Full repository gate passed (55 browser cases); package mode changed to Formula.

## CLI login usability follow-up

Remember the last successfully authenticated HTTPS origin outside installation
state; tokens stay exclusively in OS credential storage. Explicit server wins;
explicit state/backend and operator commands retain their local behavior. Missing
saved endpoint preserves existing behavior; malformed endpoint fails closed.
Logout clears the matching default after successful revocation. Enable Keychain
trust for the creating CLI executable, not arbitrary applications. Existing items
retain their ACL in the keyring dependency: users may need Always Allow once;
upgrades/locked keychains can still prompt. Never delete credentials to bypass OS
consent. Verify focused CLI/login tests and the full gate; real macOS prompt
behavior requires user authorization and must not be claimed from unit tests.

CLI usability verification: focused login/CLI/composition tests passed; final
`check.ps1 -All` passed, including 55 browser cases. Homebrew preview
`0.1.0-preview.20261012.1` pins implementation commit 64395c9 and was installed by
an actual local-tap source upgrade. The public tap remains unpublished. After
upgrading, sign in once with `agenova login --server https://app.agenova.app`, then
use `agenova work list` or `agenova run -f work.yaml`. Existing Keychain ACLs may
still require user-selected Always Allow; zero-prompt operation on this user's
existing item has not been verified and is not claimed.

## New-member guide walkthrough

Added a concise [Chinese newcomer guide](../../docs/member-quickstart.zh-CN.md)
and runnable YAML. [Live walkthrough evidence](evidence/new-member-guide.md)
records successful installed CLI login, remembered-server run/list/show and Portal
verification. This reused an existing approved account, not a fresh registration.
The Nova Micro call succeeded but its answer was inadequate; do not treat execution
success as answer-quality acceptance. No live role changes were made.

## Public Homebrew tap

Owner requested public one-command installation. Published
https://github.com/Leo1piece/homebrew-agenova under the authenticated Owner's
personal GitHub namespace, containing only the reviewed Formula and instructions.
It pins the same tested source archive/checksum and labels the release preview.
Actually removed the old local-tap executable and installed with
`brew install Leo1piece/agenova/agenova`; macOS arm64 compilation succeeded.
The guide now uses this public command. No stable release, notarization, bottle,
Intel validation or upstream-organization ownership is implied.
Public-tap verification: Homebrew formula test passed (`--version`, `--help`),
`agenova work list` succeeded against the remembered live server after switching
installations, and the documentation gate passed. No re-login was required in
this run. Source code is unchanged from the previously passed full gate.

Owner-requested uninstall/reinstall validation completed with blank Agenova
credentials and default server, public tap re-clone and exact source archive
cache removal. [Clean-install evidence](evidence/public-tap-clean-install.md)
records the unauthenticated negative, new login, actual run/list/show and Bedrock
result. Existing browser and OS/toolchain state were retained, explicitly scoped.

## Owner-requested live Platform Operator upgrade, 13 October

Grant Fisher the explicit existing installation duty, verify his actual Google
CLI identity, and upgrade the live member control plane through platform apply.
The reference installer cannot replace the member Deployment safely. Add a narrow
Kubernetes adapter member-upgrade mode: pin existing Deployment UID and the
non-image Pod template fingerprint, preserve auth/database/sidecar/workload
identity, enforce single-writer Recreate, reject runtime/model changes and patch
only the named control-plane image with optimistic concurrency tests. No schema
migration or broad production installer is implied. Test denial, stale bindings,
concurrent changes, readiness, no-op repeat, then full gate and real EKS rollout.
Record exact login subject, old/new image and Pod identity, durable history and
HTTPS health. Never use a reference install as proof of member-service upgrade.

The [live Platform Operator evidence](evidence/platform-operator-upgrade.md)
records the explicitly approved Fisher grant, real Google CLI identity, actual
member Deployment image replacement, final ready/status/no-op repeat, preserved
history and a new real Nova Micro Work. The scope is image-only upgrade of an
existing member service, not generic production installation or model reconfiguration.
The [team use cases](../../docs/team-use-cases.zh-CN.md) distinguish application,
governance and infrastructure access and label remaining unsupported scenarios.

## Owner-requested role UI simplification, 13 October

Use Developer, Read-only and Team administrator presets for new member approval
and invitations. Map atomically to the existing account/project payload; keep
custom combinations accessible. Keep existing member grants intact, with explicit
project editing and advanced account settings. Collapse deployment/governance
grants, revocation and ownership controls under Advanced permissions. Preserve
Owner-only rules and service-side checks; no new roles, migrations or automatic
changes to existing members. Verify exact payloads, defaults, non-Owner options,
rendered desktop/mobile and the full gate. Update the team use-case guide.

Implemented and deployed. [Role UI evidence](evidence/role-ui-simplification.md)
records focused/full gates, rendered desktop/mobile checks, real CLI rollout,
preserved Fisher grant and authenticated/anonymous post-deployment checks.

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

Completed and deployed: [Team administrator bundle evidence](evidence/team-admin-bundle.md).
Real CLI plan/apply/status succeeded without state-dir, starting without an applied
record in the default user directory. Full gate and PostgreSQL/race checks passed.

## Owner-requested Linux, WSL and Windows distribution

Extend public CLI packaging beyond macOS: Linux Homebrew, native Windows archive
and per-user installer, WSL2 Linux usage with Secret Service or native Windows
CLI interoperability. Keep tokens in OS credential storage; no plaintext fallback.
Test native credential write/read/delete on disposable platform environments,
OAuth callback/session tests and build/version/help, then full gate. Record native
Windows/Linux versus WSL-only unverified behavior explicitly. Publish checksummed
preview artifacts and update the public tap only after relevant checks pass.

Published preview 0.1.0-preview.20261013.1 and updated public tap.
[Cross-platform evidence](evidence/cross-platform-cli.md) records native Windows/Linux
CI, actual package installation, local full gate and the remaining real WSL2/Google
acceptance boundary. No real-user WSL login is claimed.

## Owner decision: three operational roles, 19 October

Replace everyday role choices with Platform Admin, Platform Operator, Member.
Owner remains a protected identity. Platform Admin is the existing account-admin
and assigned-project-admin bundle (including governance and serving-installation
deployment); Platform Operator has scoped deployment only; Member has assigned
project submission and own evidence only. AWS IAM/RBAC remain independent.
Implement atomic role transitions over the existing persisted grant model, clearing
incompatible grants on demotion, with audit and active-member/team locking.
Keep legacy storage/API compatibility but expose no extra role choices in the new
management flow. Do not mislabel legacy custom grants as canonical roles.
Owner explicitly authorizes migrating all five current team accounts to Platform
Admin, preserving Owner and disabled states; do not apply automatic promotion to
future registrations or other accounts. Record exact targets and before/after
state. Test operator/member denials, downgrade, unknown role/scope/cross-team,
rollback and UI; run focused PostgreSQL and full gate before deployment. Update
quick guide/use cases and verify real deployment/HTTPS. Existing image-only
upgrade cannot change model configuration; no new unsupported capability claims.

## Follow-up, 19 October: navigation and administrator delegation

Owner requests Administration and Log out alongside Work/Agents/Policy/Platform
in the Portal sidebar, sharing its desktop/mobile styles. Server emits only a
non-authoritative navigation hint; all API authorization remains server-side.
Platform Admin may now assign all three operational roles to other users, including
other administrators, within its administered projects. Owner and self-role changes
remain protected. This supersedes the Owner-only administrator appointment rule.
Test administrator approval/role transition, protected Owner/self, and navigation.

## Owner follow-up: browser governance editor, 19 October

Add project Policy/AgentTemplate read/edit/save in Administration through existing
setup and governance APIs. Preserve immutable policy versions/template names:
changed content requires new identity; no overwrite, delete or general CRUD.
Server role/project checks, validation, worker compatibility, audit and atomic
activation remain authoritative. Show conflict/denial/invalid input explicitly,
protect unsaved text during submission and clear stale selection on target change.
Test positive saving, denial, conflict, malformed JSON, project switching and mobile
rendering; run PostgreSQL tests/full gate and deploy with live idempotent save proof.

## Merge-readiness audit, 19 October

The original implementation checkboxes above are reconciled with later evidence;
checked implementation does not imply the whole production acceptance campaign.
See [current acceptance status](evidence/merge-readiness-20261019.md). Keep review,
live interruption/revocation and long-term hosting decisions open. Historical
Cognito and pre-persistence gaps in earlier evidence are not current blockers.

## Owner follow-up: infrastructure directory migration, 19 October

Move active infra/demo roots to infra/production/eks and edge while preserving
local Terraform state, lineage, serial, resource addresses and physical names.
Do not apply infrastructure changes or affect RDS/shared network. Retire obsolete
shared-password deployment scripts and archive their runbook as historical only.
Preserve local-only active network policy/add-on configuration. Update operator
references and scheduled cleanup paths without changing authorized lifetime.
Evidence: private backups, identical state hashes and before/after plan comparison,
Terraform validation, RDS health, documentation and full repository gate.
CI remains enabled; latest pre-migration run 39232189020 passed all three jobs.
