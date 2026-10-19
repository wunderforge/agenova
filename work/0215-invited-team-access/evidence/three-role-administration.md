# Three-role administration and sidebar, 19 October 2026

Owner requested a backend administration page, three operational roles, existing-user
migration, and subsequently allowed Platform Admins to manage other administrators.
Owner identity and self-role changes remain protected; project scope and separate
AWS/Kubernetes authorization remain enforced. Existing legacy grant APIs retain
compatibility; new role transitions clear incompatible grants transactionally.

## Verification

- Real PostgreSQL + race: `go test -race ./internal/authentication/postgres ./internal/invitations ./cmd/agenova-control-plane` with disposable `AGENOVA_TEST_POSTGRES`: passed.
- Positive cases: administrator approval of another administrator, administrator
  downgrade to operator, member/operator/admin transitions and capability checks.
- Negative cases: Owner/self modification, cross-team, unknown role, disabled user,
  wrong installation, operator business access, and audit failure rollback.
- `node harness/integration/identity/invitations.mjs` and `access-requests.mjs`: passed.
- `pwsh -NoProfile -File scripts/check.ps1 -All`: exit 0; 56 browser smoke cases passed.
- Sidebar browser test verifies Administration/POST Log out inside Main navigation,
  no separate Account bar, desktop/mobile rendering and no horizontal overflow.
- UI permission hints do not authorize requests; server-side checks remain authoritative.

## Deployment and migration

The earlier live migration explicitly targeted the five existing approved accounts,
preserved the Owner and active flags, and promoted the four non-Owner users to the
Platform Admin bundle. Before-state and completion markers are in identity audit.
The script rejects replay and was tested on a disposable PostgreSQL schema before
execution. Temporary operator credentials and migration deployment were removed.
No new user registration is automatically promoted.

The sidebar/admin-delegation revision changes the control-plane image and Portal
assets only, without another membership migration. Image:
`sha256:b10cb3368e65b9b1298fb6e0047d52df2fbb904f8ba537d15ccb444b5bd91dc6`.
Local operational logs and screenshots are under `.tmp/three-role-deploy`,
`.tmp/admin-nav-postgres.log`, `.tmp/admin-nav-all.log` and `.tmp/ui-smoke`.

These changes do not grant cloud IAM access, introduce a model configuration editor,
or claim a new real Windows/WSL Google-login acceptance run. Existing midnight
EKS/ALB retirement authorization is unchanged; production RDS remains persistent.

Live post-deployment verification: rollout completed; authenticated Fisher session
loaded existing persisted Work, sidebar Administration/Log out were present in
Main navigation, and other administrators exposed three-role selectors. Owner and
Fisher's own row had no role editor. Saving another existing administrator's
unchanged Platform Admin/payments assignment returned `User access updated.`;
no live user was demoted for testing. Actual demotion/denial is covered by the
real PostgreSQL tests above. Rendered proof: `nav-live.png` and
`admin-role-save-live.png` under `.tmp/three-role-deploy`.

## Browser governance editor and dark administration theme

Administration now shares the Portal's dark palette, sidebar, navigation spacing,
inputs and cards. Policy/Template editor loads project-scoped `/api/setup` data,
converts the Policy bundle to its canonical document envelope, and submits JSON
(valid YAML) to the existing `/api/governance` registration endpoint. There is no
new write endpoint, database mutation behavior, overwrite or permission expansion.
Changed Policy requires a new version; changed Template requires a new name and
explicit Policy/Work selection. Immutable conflict is displayed instead of ignored.

Focused browser proof: `node harness/integration/identity/governance-editor.mjs`:
policy and template save payloads, invalid JSON, 403, 409, target-change reset,
desktop/mobile rendering passed. Existing administration/approval browser harnesses
also passed. Fresh real PostgreSQL/race tests passed for authentication store,
HTTP handlers and control-plane packages (`.tmp/governance-editor-postgres.log`).
Screenshots: `.tmp/identity/governance-editor/{desktop,mobile}.png` (synthetic data).

Live governance verification: Fisher's Platform Admin browser session loaded
`payments` Policy `reference-default-deny` version `1` and template `engineer`,
then successfully saved both without changing their contents/capability ceilings.
Policy response reported activation; template response reported saved. These writes
use the same authenticated project registration API as the CLI. No new privileges
were added to Work during the smoke check.

Full gate passed with 56 browser tests. A final CSS-only `[hidden]` precedence fix
was followed by the focused governance browser harness (passed). Final deployed
image: `sha256:7b3cbc344585b3cdac872437f8c2d343df06c9f4665fbe00320d9819b9c82771`.

Windows distribution follow-up: downloaded public `v0.1.0-preview.20261013.1`
Windows ZIP and SHA256, verified digest and PE `agenova.exe` entry, and verified
published `install-windows.ps1` matches the repository script. Added explicit
PowerShell and ZIP instructions to `docs/cli-installation.md`; macOS retains
Homebrew. This does not substitute for interactive Windows Google-login acceptance.
