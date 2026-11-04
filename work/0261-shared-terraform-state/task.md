# Task: Shared Terraform state and on-demand EKS operations

- Ticket: [#261](https://github.com/wunderforge/agenova/issues/261)
- Mission: Move existing infrastructure state into a private shared backend and provide safe team lifecycle procedures.
- Target: infra backend configuration, scripts/infra, operations documentation.
- User value: Team operators can initialize the same state with AWS SSO instead of relying on one laptop.
- PRD outcome: [Owner-approved production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension); infrastructure operations only, no shared product contract changes.

## Context to Read

- AGENTS.md and docs/product/prd.md
- [Spec](spec.md), [design](design.md)
- infra/production/README.md, infra/production/eks/*.tf, infra/production/edge/*.tf
- infra/database/main.tf, infra/domain/main.tf, infra/member-edge/main.tf
- docs/development/shared-eks-release.md and deploy/production/member-service.md

## Scope

Private S3 backend, migration of five existing state roots, plan guard and team runbook. No application-role changes, CI changes, cluster rebuild, further deletion, or automatic schedule. State migration does not authorize applying infrastructure drift.

## Acceptance Criteria

1. Back up states and preserve their lineage, serial and resource identities in separate locked S3 keys.
2. Verify bucket encryption/versioning/public-access restrictions and state locking with real AWS evidence.
3. Document SSO access, inputs, on/off order and recovery; deny persistent-resource deletion/replacement in plan guard.
4. Report live UI status separately from source or publication status.

## Negative Case

Reject unsafe plan actions, unexpected roots/accounts, and non-S3/unlocked backends. Do not treat an absent cluster as a working UI or a successful rebuild test.

## Execution Todo

- [x] Scout existing roots and authoritative state files.
- [x] Self-review: infrastructure-only delivery fits authorized production operations, with no product contract expansion.
- [x] Implement S3 configuration and guard.
- [x] Migrate backed-up states without infrastructure apply.
- [x] Verify backend and focused tests; run ./scripts/check.ps1 -All.
- [x] Review diff and prepare reviewable changes/evidence.

## Quality Gates

- python3 -m unittest discover -s scripts/infra -p 'test_*.py'
- terraform fmt -check and terraform validate for affected roots
- ./scripts/check.ps1 -All

## Evidence Required

Redacted migration equality and backend controls, native lock contention, plan guard positive/negative tests, exact remaining blockers. Private state and credentials never enter Git.

## Constraints

Preserve architecture contracts, resource addresses, RDS and shared network. User cancelled automatic cleanup on 30 October; no new delete/apply of infrastructure in this ticket. CI remains unchanged.

## Decisions and Blockers

- 2026-10-30: EKS and app/demo A records absent; no currently running app UI at that URL. main at 32b59bf contains recent UI updates.
- GitHub CLI active enterprise identity cannot create this repository issue; used existing Leo1piece credential only for the scoped command, without changing global account.

- Evidence: [verification](evidence/verification.md); private backups in `.tmp/shared-state/backup`.

## Authorized lifecycle verification follow-up

2026-10-30: user explicitly requested actual startup, latest-main UI verification,
then shutdown. This supersedes this packet's earlier no-rebuild limitation for
the bounded verification cycle only. Preserve RDS/data and persistent dependencies;
no automatic schedule. Record actual plans, runtime digest, UI evidence, teardown
and failures. Local SSO token is expired; designated operator credentials remain
valid, so this cycle does not certify teammate SSO access.

User subsequently confirmed “没事，关吧，晚上我再测试。” Continue shutdown;
do not reopen for the delayed manual login. No automatic evening restart.
