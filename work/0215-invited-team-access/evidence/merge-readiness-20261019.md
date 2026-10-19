# Merge readiness audit — 19 October 2026

PR #216, implementation head `245d0d1` before this documentation audit.
No submitted reviews, general comments or inline review comments were returned by
GitHub at audit time. GitHub reports MERGEABLE (no textual conflict) but
REVIEW_REQUIRED; this is not approval to merge. PR remains draft.

## Resolved now

The baseline job stopped at `Validate PR delivery contract`, before code tests:
PR text omitted the required `Linked ticket` section and other template structure.
Restored the canonical headings/labels, closing ticket, verification table and
backend-neutrality confirmation. Validated with
`pwsh -NoProfile -File scripts/check-pr-body.ps1 -BodyPath <saved body>`.
No workflow checks were disabled. The subsequent run 39232189020 passed all three jobs (baseline, Linux CLI, Windows CLI).

## Acceptance matrix

| Area | Current evidence | Remaining work |
| --- | --- | --- |
| Google registration and approved access | google-service.md, access-approval.md; real Owner/Fisher sessions | One consolidated two-user registration-to-revocation campaign, not just separate successful sessions |
| Native macOS client and real model call | new-member-guide.md, public-tap-clean-install.md, platform-operator-upgrade.md | Do not label a retained browser login as fresh registration |
| Role/governance boundaries | team-admin-bundle.md, three-role-administration.md; real PostgreSQL negative/transaction tests and live saves | Independent security review; full live member isolation/revocation/zero-provider-call campaign |
| Durable history across clean upgrade | platform-operator-upgrade.md; three prior Work records survived two actual upgrades | Abrupt interruption of active EKS Work, cleanup-only reconciliation and no model replay, plus schema-compatible rollback |
| Recovery invariants | durable-work.md, recovery.md; executable fault injection and SQL tests | Real EKS cleanup-only reattachment passed: see acceptance-20261019.md. Whole-service interruption and real provider non-replay still unverified |
| macOS/Windows distribution | public-tap-clean-install.md, cross-platform-cli.md; published Windows checksum and native CI | Interactive Windows Google login and real WSL2 acceptance remain unverified |
| Administration/editor UI | three-role-administration.md; rendered browser tests and real authenticated saves | No additional UI blocker identified in this audit |
| Hosting budget and lifecycle | Current EKS/RDS rollout; Owner authorized ten days continuous operation on October 19 | AUD100/month sustainable service topology/availability and operational release decision remain open |
| CI and review | Head a9eeb26 baseline passed; CI file matches main. No reviews yet | Latest-head green required checks and maintainer approval |

## Merge decision

Do not mark the PR production-ready solely because CI becomes green. To merge
under the current ticket acceptance, complete the live interruption/revocation
campaign and obtain required independent review. If maintainers want to merge
an incremental implementation earlier, they must explicitly accept a narrowed
release scope and track remaining acceptance in follow-up tickets; do not silently
close #215 as fully accepted. No reliable merge time can be promised before this
review/acceptance decision.

Do not manufacture another user's session, revoke a teammate, interrupt live Work,
or destroy persistent resources merely to make this matrix green. Coordinate any
live destructive/revocation test with a designated test account and recorded scope.

Latest evidence: [acceptance continuation](acceptance-20261019.md). Owner declined live Fisher revocation; retain this gap instead of changing his permissions.
