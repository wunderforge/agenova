# Team administrator bundle — 13 October 2026

Owner explicitly requested bundled governance and deployment authority, retaining
separate cloud credentials. Team administrator is the conjunction of active
Account Admin and Project Admin, not a new stored role. Governance is restricted
to administered projects. Deployment additionally pins the requested installation
to trusted AGENOVA_PLATFORM_NAME. Unknown actions, foreign installations and
account-only/project-only administrators deny. Owner remains separately scoped.
Explicit duties remain additive; revoking derived access does not delete them.

## Automated evidence

- Real disposable PostgreSQL on localhost:55432: `go test -race` for
  internal/authentication/postgres, cmd/agenova-control-plane and internal/app passed.
- Tests prove policy registration, HTTP deployment authorization without an explicit
  operator duty, project/installation denial, unknown-action denial and immediate
  demotion/disable handling. Governance checks inside its write transaction.
- The existing explicit-duty revocation tests still pass.
- Invitations and access-requests browser harnesses passed; desktop/mobile rendering
  inspected. Team administrator bundled permissions are explained on member rows.
- `pwsh -NoProfile -File scripts/check.ps1 -All` passed, including 55 browser smokes.
- Empty user configuration directory test proves automatic default state selection
  and retained installation discovery after successful reconciliation is recorded.

## Live deployment and default-directory CLI

AWS STS verified Fisher's AgenovaAdministrator SSO role in account 931228356546.
The default macOS current-platform.json did not exist before the run. No state
was copied from another user or from the earlier operator test directory.

Using Fisher's existing application login and AWS SSO identity, ran platform
plan, apply and status with --server and --installation but NO --state-dir.
The reviewed plan changed only the member control-plane image remotely; local
adapter activation was also expected for the empty default installation directory.

New image digest:
`sha256:a0b03a922ca2805fdef154f1bc0f92fb474712532b60cb6d6d4f502f4e2a731b`

Apply returned applied=true, ready=true. Status returned installationReady=true.
Work list retained four Succeeded records. The live Members page displayed Fisher
as Team administrator with bundled project governance/current-installation access.
Other members remained Developer; no membership rows or explicit duties were changed.

Local receipts: `.tmp/operator-upgrade/team-admin-{plan,apply,status}.json`.
Rendered proof: `.tmp/identity/invitation-ui/live-team-admin.png`.
Gate logs: `/tmp/team-admin-race.log`, `/tmp/team-admin-all-final.log`.

## Scope of proof

Fisher retained his previously granted explicit Platform Operator duty during the
live upgrade. The no-explicit-duty positive/negative test is real PostgreSQL plus
HTTP handler testing, not a claim that the live upgrade isolated the new bundle.
No live policy version was changed solely for this test. No AWS grants were added.
The public Homebrew preview was not republished; member-upgrade requires the newer
CLI already used for these tests. Default local state is installation discovery,
not authorization or shared credentials. Model configuration changes remain outside
the image-only member-upgrade path.
