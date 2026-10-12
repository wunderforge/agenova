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

In scope: invite-only managed directory, browser/CLI code+PKCE login, HTTPS CLI target, verified personal identity, active membership and role authorization, own-evidence default, durable Work/audit/session data, restart recovery, backup/restore, revocation, limits and reviewed production deployment.

Out of scope: public signup/billing, runtime-specific shared contracts, worker cloud credentials, arbitrary policy/template editing, silent fixed-principal fallback, blanket production-readiness claims.

## Acceptance Criteria

- Invite Alice and Bob; each uses Portal and CLI with no AWS keys, kubeconfig or installation state.
- Every operation checks verified identity and current active membership; Work and evidence attribution are correct and user/project boundaries hold.
- CLI login uses browser code+PKCE, state binding and protected OS credential storage; browser tokens remain server-side.
- Membership removal and logout have an explicit tested revocation bound; token expiry alone is not described as immediate revocation.
- Evidence survives restart; active Work reconciliation does not re-dispatch uncertain external calls; backup restoration and rollback are exercised.
- Real EKS/Bedrock positive and zero-call negative evidence, rendered Portal/CLI proof, focused and full gates, independent security/operational review precede production cutover.

## Negative Case

Uninvited/disabled users, reused/expired invitation, forged/expired/wrong-issuer/wrong-client/wrong-token-use JWT, caller-authored identity, cross-user/project reads, redirect/token leaks and management escalation fail closed. Failure never falls back to Team A, grants authority or silently duplicates a model call.

## Execution Todo

- [x] Scout fixed-principal service, loopback-only CLI, process-memory evidence and external identity scope.
- [x] Assignee self-review: Owner explicitly reprioritized production/invite-only scope on 2026-10-12. Preserve architecture; do not change #203/#204 assignments or producer contracts.
- [x] Define shared behavior, dependencies and production release gates in spec/design.
- [x] Implement bounded Cognito access-token verification and a required current-membership/revocation source interface with negative fixtures.
- [ ] Supply the durable membership/revocation source and integrate it with each API request.
- [x] Add authenticated HTTPS transport to the connected client without a Kubernetes target; reject token-bearing redirects and ambiguous transport configuration.
- [ ] Integrate request-local principal, action/ownership checks and CLI login/profile selection.
- [ ] Implement PKCE login, OS credential storage, refresh/revocation and Portal sessions.
- [ ] Implement durable storage, migrations, restart reconciliation and backup/restore.
- [ ] Review and deploy bounded infrastructure after budget/SLO decisions.
- [ ] Execute two-user real-backend, negative, recovery and rendered tests.
- [ ] Run focused gates and ./scripts/check.ps1 -All; independent review and cutover.

## Quality Gates

- Planning/source links: `pwsh -NoProfile -File scripts/check.ps1 -Docs`.
- Implementation: package-specific authentication/authorization/storage/client tests, race tests, real IdP and installed two-user E2E, backup/restore and revocation probes.
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
