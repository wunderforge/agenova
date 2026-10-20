# Scoped role foundation

The Owner requested six distinct duties. Existing account Owner/Admin and project
Admin/Member/Viewer remain compatible. Migration 006 adds separate Governance Admin
and Platform Operator grants without changing existing assignments or migrations.

| Duty | Scope | Enforced boundary |
| --- | --- | --- |
| Account Owner | Account | Sole grant/revoke authority for additional duties |
| Project Admin | Project | Existing member administration and Work/evidence access |
| Governance Admin | Account or existing project | Explicit policy/template registration authorization predicate |
| Member | Project | Submit Work/read own evidence, subject to claim policy |
| Viewer | Project | Read project evidence, no submission |
| Platform Operator | Installation | Explicit configure/diagnose/recover authorization predicate; no IAM/RBAC grant |

`POST /api/members` accepts `grant-role` and `revoke-role`, `principal`, `role`,
`scopeKind` and `scopeRef`. Identity and team come from authentication. Only an
active Owner can delegate; membership locks serialize edits and audits commit in
the same transaction. Disabled users and unknown actions fail closed. An ordinary
Admin cannot disable a member carrying additional privileged duties. Existing
project Work access is neither added nor removed by an additional duty.

Member management renders and revokes scoped grants, and offers role-compatible
scope choices to Owner only. Server authorization is independent of that UI.

## Verification

Focused real disposable PostgreSQL race tests cover explicit grants, implicit
Owner/Admin denial, forbidden delegation, project/account/installation scope
isolation, unknown operation denial, immediate grant revocation, inactive-member
denial, privileged-member protection and audit records. Command:

```sh
GOTOOLCHAIN=go1.27.2 go test -race ./internal/authentication/postgres ./internal/invitations
node harness/integration/identity/invitations.mjs
```

Set `AGENOVA_TEST_POSTGRES` to a disposable database. The browser harness uses
synthetic API responses; it is not live Google or production authorization proof.
Rendered output: `.tmp/identity/invitation-ui/google-mobile.png`.

## Deliberately unavailable operations

The new authorization predicates are not policy/template mutation endpoints or
cloud administration APIs. Current registration is installation-global and is
still guarded by the existing trusted operator boundary. The Owner explicitly deferred project-isolated
registry/activation and member Policy/Template management. No user receives
cluster permissions, provider secrets or claim issuance authority from these
roles. This foundation does not complete the approved role-management feature
or establish production readiness.

Verification on 12 October 2026: focused PostgreSQL/invitation race suites passed;
rendered invitation/member harness passed and mobile screenshot was inspected;
`pwsh -NoProfile -File scripts/check.ps1 -All` passed, including 53 browser smoke
cases; the final documentation gate passed. Logs are local files
`/tmp/agenova-roles-focused.log`, `/tmp/agenova-roles-ui.log`,
`/tmp/agenova-roles-full-gate.log` and `/tmp/agenova-roles-docs.log`.

## Fixed role directory

The Owner selected a read-only permission directory and existing member assignment,
not custom role editing. `/members#roles` explains six duties and the compatibility
Account Admin role. Policy/Template and remote platform management are explicitly
unavailable; links lead to `/members#member-section` assignment controls. No
backend permission changed. The updated executable browser harness verifies six
role cards, unavailable states, no permission-edit controls, assignment navigation
and mobile overflow. Rendered proof was inspected at
`.tmp/identity/invitation-ui/roles-mobile.png`; desktop proof is
`.tmp/identity/invitation-ui/roles-desktop.png`.
