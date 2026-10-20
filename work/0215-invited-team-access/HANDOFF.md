# Agent handoff: hosted access and administration

Read repository AGENTS.md, PRD, this ticket's task.md, then linked spec/design before
implementation. This note is navigation, not a replacement architecture contract.

## Start with the user workflow

- [Team Quick Guide](../../docs/team-quick-guide.md): Portal/CLI login, approval,
  macOS/Windows installation, Work example, separate AWS SSO/EKS access.
- [CLI installation](../../docs/cli-installation.md): executable download links,
  checksum verification, OS credential stores and Windows/WSL acceptance limits.
- [Use cases](../../docs/team-use-cases.zh-CN.md): role/deployment/governance mapping.
- [Latest evidence](evidence/three-role-administration.md): tests and live saves.

Portal: https://app.agenova.app/ ; administration: /admin ; editor: /admin#governance.
Use an approved personal Google account. Pending registration requires an admin.
Never reuse another user's token, state directory, cloud credentials or browser session.

## Current implementation

Three assignable roles: Platform Admin, Platform Operator, Member. Owner is a
protected identity. Admins can change other users' roles in administered projects,
including other admins; cannot change Owner or their own role. Cloud IAM/EKS RBAC
remain separate. Role changes atomically remove incompatible legacy grants.
Storage preserves old grant shapes for compatibility; do not add another migration
that promotes all new users. The five-user October 19 migration was one-off and audited.

The browser editor uses `/api/setup` and `/api/governance`; JSON is submitted as a
canonical PolicyBundle/AgentTemplate document. Policies have immutable id/version;
templates have immutable names. Changed content needs a new identity. Saving Policy
activates it for new Work; a new template needs Policy allowance and Work selection.
Never turn conflicts into silent overwrites. Worker image/entrypoint restrictions apply.

Relevant sources: `internal/invitations/page.{html,css,js}`, PostgreSQL
`platform_roles.go` and `governance.go`, control-plane `member_auth.go` and
`governance.go`, and Portal sidebar `ui/src/Portal.tsx`.

## Verify and continue

```sh
node harness/integration/identity/invitations.mjs
node harness/integration/identity/access-requests.mjs
node harness/integration/identity/governance-editor.mjs
# Set AGENOVA_TEST_POSTGRES to a disposable database; never production.
go test -race ./internal/authentication/postgres ./internal/invitations ./cmd/agenova-control-plane
pwsh -NoProfile -File scripts/check.ps1 -All
```

Database tests skip without the disposable connection; a green skipped test is not
PostgreSQL acceptance. Browser harnesses use synthetic responses; live saves and
rendered proof are recorded separately. Windows package checksum/PE checks do not
prove interactive Windows Google login. Do not claim broad production readiness.

## Deployment and retention

Current application is the member control plane on EKS with persistent managed
PostgreSQL. Latest image and verification are in the evidence document. Do not
restore retired demo/public deployments or historical Cognito scaffolding.

Before cloud changes, read `work/0175-aws-demo-bootstrap/cleanup-deadline.txt` in the
main operational checkout and the latest user instructions. October 19 authorization
removes EKS/ALB after Sydney October 20 00:00, preserving RDS/data, dependency
network, domain/certificates, SSO/IAM, ECR and logs. Do not run blanket Terraform
destroy. Hosting URLs may be unavailable after scheduled retirement.

Do not commit `.tmp` files, credentials, kubeconfigs, private deployment snapshots,
or raw database backups. Keep the existing PR #216 for this feature; independent
review and remaining release acceptance are distinct from a successful rollout.
