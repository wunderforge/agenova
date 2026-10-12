# Feature Specification: Production invite-only team access

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215)
- PRD outcome: [Production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Intent

An invited member operates Agenova over HTTPS with a personal application identity. Cluster administration stays an operator capability. Login alone does not confer task, project, model or evidence authority.

## In Scope

- One invite-only team service with project memberships and extensible organization isolation; no public tenant creation.
- Web and native CLI sign-in, session lifecycle, membership state, per-operation authorization, persistent evidence and production operation.

## Out of Scope

- Public registration and billing, arbitrary user-supplied OIDC issuers, task-authored identity, in-Portal Policy/AgentTemplate CRUD and altered claim semantics.

## Requirements

1. Owner/admin invites an explicit email into a specified team and role. Public self-signup is disabled. Pending invitations grant no Work access; acceptance verifies the intended account. Subject identity is issuer+sub, never mutable email.
2. An active membership is required on each protected request. IdP groups/claims cannot independently elevate membership or roles. Disabled membership denies new operations even while an old JWT is otherwise cryptographically valid.
3. Member may submit to authorized projects/templates and list/read own Work. Project-wide reading is an explicit role; platform management remains the operator path. Roles do not grant requested tools/models beyond existing claim resolution.
4. Native CLI has an HTTPS connection profile independent of operator state. `login` uses system browser code+PKCE S256 with unpredictable state, bounded loopback callback and exact redirect/issuer/client binding. Token refresh secrets use OS credential storage; no silent plaintext fallback. `logout` clears local credentials and invalidates the applicable server session/grant.
5. Browser uses an authenticated server-side session/BFF. No bearer/refresh token enters browser persistent storage, URL, logs, Work or evidence. Cookie flags, CSRF, session fixation, origin checks and rate limits have named executable tests.
6. API accepts only explicitly configured bearer access tokens with verified signature, issuer, allowed algorithms, expected client/audience semantics, token use, expiry and required scope. ID tokens are not API access tokens. Bounded JWKS cache/rotation and network failures fail closed; no caller-selected discovery URL.
7. Database persists membership, invitation state, policy references, idempotency keys, Work ownership/lifecycle/facts and session revocation. Concurrent operations enforce ownership in the data query/transaction, not merely UI filtering. Request-name collisions across users never return another user's evidence.
8. Admission, durable acceptance and work dispatch use explicit transactional/idempotency boundaries. A crash with an uncertain provider result must not automatically repeat inference. Restore cannot reactivate terminal authority. Restart recovery records uncertainty and remains fail-closed until reconciled.
9. Membership removal denies new requests immediately at the authorization read boundary; already-running claim termination/revocation and concurrent reads require a specified, tested bounded mechanism. Production release cannot claim instant revocation from JWT signature validation alone.
10. Every protected management, submission and read surface is inventoried, including setup/configuration/provider diagnostics. Anonymous access is limited to health and explicit authentication flow endpoints; detailed health is private.
11. Migration/rollback, audit retention, incident handling, usage/concurrency ceilings and observability have concrete ownership and recovery tests before release.

## Negative Cases

Two users with identical template/request content remain distinct. Guessing references, changing team/subject payloads, replaying an invitation/code, changing callback state, an inactive membership, cross-project access, expired/tampered JWT and interrupted persistence produce no unauthorized evidence or provider invocation. Broken authenticated mode never downgrades to demo mode.

## Compatibility

ClaimRequest and SandboxClaim remain backend-neutral and retain existing authority/lifecycle meaning. Existing local reference mode is explicitly selected and visibly labelled. OIDC verifies people, Pod Identity authenticates trusted AWS server components, and claim-bound worker identity remains a separate boundary. Reuse accepted E14 identity/policy contracts; do not close #203 or #204 from configuration work.

## Open Decisions

- Owner confirmed AUD100/month and proposed Sydney 10:00–24:00 hours. Initial member/concurrency count, recovery expectations and a feasible costed topology remain open.
- Maintainer: exact shared E14 integration contract if the producer is not accepted before implementation reaches that boundary.
- Delivery: account-wide managed login/SES configuration, invitation sender and explicitly authorized recipient list before any mail is sent.

## Project Account and Deployment Roles

The project/team account is a membership and resource-ownership container. Each invited member has a separate SSO subject. Application roles never create IAM/EKS access entries. Ordinary CLI Work uses HTTPS application credentials. `platform apply` remains a deployment-operator operation using independently established target-cluster credentials and RBAC. Document the operator installation flow and member connection flow separately; test that an ordinary invited member cannot install or mutate the cluster.

The daily availability window must preserve identity, membership and durable Work across shutdown. EKS node scale-down is not cluster pause. No nightly destruction or startup automation is enabled from this specification alone; choose a costed lifecycle and prove data/recovery behavior first.

Owner decision (2026-10-12): only explicitly designated deployment administrators install/upgrade the shared EKS service. Project administrator membership alone is not deployment authorization. Ordinary invited users must be denied installation and cluster-management operations, with no automatic IAM/EKS/RBAC grant.

## Confirmed initial users and application roles

Owner confirmed on 12 October 2026:

- the confirmed Owner email (private operator input): initial account Owner.
- the confirmed invited Admin email (private operator input): account Admin and Admin of the initial authorized project,
  not merely Member. Must use both Portal and native CLI without AWS/EKS credentials.

Owner appoints/removes Admins. Admin invites and manages Member/Viewer project grants,
submits Work and reads the projects they administer. Member submits and reads own
Work in explicitly granted projects. Viewer only reads evidence in explicitly granted
projects. Account-level Admin membership alone does not grant access to every project;
project grants remain explicit. Neither Owner nor Admin implies deployment IAM/RBAC,
and no application role bypasses policy/template/model/claim authority checks.

Owner chose budget-first production hosting within AUD100/month while retaining the
EKS installation option. All future cutover targets are real environments; shared demo
identity/fallback is not an acceptable completion path. The old demo-destruction
heartbeat was paused to prevent accidental destruction under an obsolete deadline.
No new daily stop/start schedule is active. Existing resource costs must be accounted
for during migration, not ignored when estimating the new service.

## Owner storage decision

On 12 October the Owner authorized a low-cost AWS database within the existing
AUD100/month budget and explicitly excluded database backups. Managed Amazon RDS for PostgreSQL is the selected hosting approach, using
Single-AZ and a small instance with encrypted persistent storage. The Owner
explicitly rejected self-hosted PostgreSQL on EC2. Daily stop/start retains all member/Work data;
it must not destroy/recreate the database. Automatic backups and backup-restore
acceptance are removed from this feature scope. Durable admission/facts, crash
reconciliation, migrations, rollback and restart recovery remain required.
