# Operational role layers — 12 October 2026

Task: [#215 packet](../task.md). Branch: `codex/invited-team-access`.

## Verified behavior

- Independent account/project invitation grants; omitted account role stays Member.
  Project-only administrators cannot invite account/project administrators or read
  another project's membership/invitation records.
- Explicit account/project Governance Admin and installation Platform Operator
  grants. Unknown actions, foreign scopes, disabled members and revoked grants deny.
- Immutable project Policy/Template registration, create/equal/conflict behavior,
  runtime compatibility rejection and transactionally audited policy activation.
  Audit failure rolls back both registration and activation. Registry reads are
  scoped; absent defaults do not panic and database failure does not use defaults.
- Protected ownership transfer tested on disposable identities, including wrong
  confirmation, inactive target, concurrent transfers and audit failure rollback.
  No real ownership transfer was performed.
- HTTPS management client refuses redirects; Platform CLI checks installation
  identity before using local cloud/Kubernetes credentials. A permitted application
  grant does not bypass a simulated local Kubernetes RBAC denial.
- Portal project/template selectors request the corresponding setup. Lack of a
  template no longer prevents reading otherwise authorized Work evidence.

## Executed checks

With disposable PostgreSQL on local port 55432:

```text
go test -race ./internal/authentication/postgres ./internal/cli ./internal/connectedclient ./cmd/agenova-control-plane: PASS
go test -race ./internal/authentication/postgres ./internal/console ./cmd/agenova-control-plane: PASS
node harness/integration/identity/invitations.mjs: PASS
npm --prefix ui run test:smoke -- --grep 'project configuration selector|without a template': 2 PASS
pwsh -NoProfile -File scripts/check.ps1 -All: PASS; 55 browser smoke tests
```

Go toolchain 1.27.2; Node 24. Focused tests used actual disposable PostgreSQL;
browser fixture responses are synthetic. Local rendered proof is under ignored
`.tmp/identity/invitation-ui/`; scoped policy rendering is in Playwright results.
The full gate does not substitute for two-human Google/CLI/Bedrock acceptance.

## Actual deployment

- STS confirmed designated non-root deployment operator in the expected AWS account.
- RDS migration 007 applied; seven checksum-checked migrations, one active Owner.
- Runtime registry INSERT and pointer UPDATE permissions verified true. Registry
  DELETE, audit UPDATE and public-schema CREATE permissions verified false.
- Initial migration connection failed before DDL. A deployment-managed retry
  subsequently completed successfully; the precise transient cause was not proven.
  Temporary migration Deployment and operator Secret removed after verification.
- EKS member deployment rolled out successfully; one available replica. Image:
  `sha256:358d0c1a893a95121b61f23cf591fb1a05e5a4845d79f5aa5de5cd914a91e8d8`.
- Public login returned 200; anonymous governance POST returned 401. Existing
  Owner session survived rollout. Real browser showed eight role cards, separate
  account/project invitation selectors, and payments/engineer Portal selectors.
- Fisher administrator invitation is prepared but not submitted. Its acceptance,
  native CLI login, permitted Work and denial/revocation campaign remain pending.

## Limits

No new permanent cloud infrastructure or recurring schedule was created here.
The current EKS/ALB/RDS deployment still does not demonstrate AUD100/month hosting.
Old cleanup automation stays paused. Schema 007 cannot use the previous binary
as a rollback image: retain data and build a schema-compatible rollback binary.
General Policy/Template CRUD, remote hosted cloud deployment and implicit IAM/RBAC
are not implemented. Current CLI performs narrow immutable registration using
explicit duties. Independent review and remaining production release gates in the
packet are still required; this is not an assertion that all of #215 is complete.
