# Task: Ship approved June Portal and public landing after member-login integration

- Ticket: [#233](https://github.com/wunderforge/agenova/issues/233)
- Mission: Ship the approved June Portal and public landing on the merged member-login foundation, without changing authority or login semantics.
- Target: `ui/`, `internal/landing/`, `internal/websession/`, member handler composition and focused documentation.
- User value: Visitors understand the product before signing in; members inspect real Work and recorded activity through a clearer explorer/inspector.
- PRD outcome: [evidence console](../../docs/product/prd.md#7-read-only-claim-console) and [approved public presentation](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Evidence and credential boundaries](../../docs/product/architecture-contract.md).
- [Approved June layout, scope and prior evidence](../0143-midterm-vertical/june-portal.md).
- [Approved landing scope and prior evidence](../0215-invited-team-access/landing.md).
- [Member spec](../0215-invited-team-access/spec.md).
- [Member release prerequisites](../../deploy/production/member-service.md).
- Browser tests: `ui/smoke/june.spec.ts`, `landing.spec.ts`, `connected.spec.ts`.
- Authentication boundary tests: `internal/websession/landing_test.go`, `source_test.go`.

## Scope

In scope:

- Integrate the existing approved local UI commits after #216; preserve both public landing and trusted-proxy configuration in the overlapping composition.
- Reuse real evidence contracts, existing sign-in and account/admin routes.
- Fixed-light June styling, responsive explorer and recorded-activity inspector.
- Finite anonymous landing allowlist with explicitly illustrative product captures.
- Regression tests, rendered proof, documentation and separate PR.

Out of scope:

- Deployment, shared database migration, cloud resources, real Google login or membership operations.
- New authority/evidence schemas, task policy features, external gateways and provider integration.
- Fake traces, invented durations, synthetic records inside Connected mode.

## Acceptance Criteria

- Anonymous root shows only public product information; all sign-in links use `/login`.
- Authenticated root, forms, POST logout and account/admin navigation retain their existing behavior.
- Explorer counts describe the loaded list; admission decision and execution outcome remain distinct.
- Inspector shows recorded targets/status/evidence, including updates during polling.
- Dark system preference does not switch the application palette; desktop/mobile and keyboard navigation remain usable.
- Preserve #216 source-bound login limits and opt-in trusted local proxy behavior.
- Create a normal review PR against main after tests; do not merge it or deploy.

## Negative Case

- Anonymous private APIs stay 401; private routes redirect to login; unknown public assets are 404 and missing allowlisted assets are 503, never a private-Portal fallback.
- Landing sends no automatic API/identity requests, stores no identity and grants no membership.
- Disconnected Connected mode never substitutes demo fixtures or successful evidence.
- Pinned-host and CSRF checks, verified native-code capacity and source-bound login limits remain intact.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [x] Merge #216 into main with specifically approved administrator bypass, then resolve the two overlapping login composition changes on the UI branch.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`; record the existing Windows-only directory-sync failure without calling that run a pass.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/landing ./internal/websession ./cmd/agenova-control-plane`
- `python harness/integration/identity/local-postgres.py --openssl <local openssl> --image postgres@sha256:<reviewed digest>`
- `npm --prefix ui run check`
- `node harness/integration/identity/google-login.mjs`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Exact outcomes and residual platform-specific limitations in this packet and PR.
- Playwright desktop/mobile rendering under `.tmp/ui-smoke/`; synthetic/local-only provenance stays explicit.
- PostgreSQL summary under `.tmp/cicd-integration/local-postgres-summary.json`, including cleanup result.
- Private API denial, public allowlist, authenticated root and login handoff tests.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Do not read user credential files. Tests use newly generated disposable fixture material only.
- Original local approvals were commit-only; on 20 October the Owner separately authorized merging #216 then creating this UI PR. This supersedes the historical no-push/no-PR notes, not the no-deployment boundary.

## Decisions and Blockers

- #216 merged at `f3d1457d1b4b47c450670c91013259ce1f2976cf`; original approved UI commits are `c79f348` and `fa4f0b0`.
- Keep both `TrustLocalProxyClientIP` and `Landing` in configuration and both member composition paths. Do not replace upstream login/session changes with the older UI base.
- Windows first focused run exposed upstream `TestOwnerBootstrapDeliveryGuards`: directory `Sync` in #216 fails on Windows. Landing and websession tests passed. Do not weaken that security behavior just to make a UI test run green; distinguish Windows results from Linux CI.
- Release still requires #216 migration 009 and compatible proxy/binary coordination. This PR performs none of those operations and must not be described as a deploy-ready UI-only rollout.

## Integrated verification — 20 October 2026

- `npm --prefix ui run check`: passed canonical bindings, types, 130 unit tests,
  production build and all 59 Playwright tests. Desktop/mobile June and landing
  captures were visually inspected, including fixed-light dark-system preference.
- `go test ./internal/landing ./internal/websession ./cmd/agenova-control-plane`:
  landing/session packages passed; control plane failed only the unchanged
  `TestOwnerBootstrapDeliveryGuards` on Windows directory `Sync`.
  `go test -count=1 -skip '^TestOwnerBootstrapDeliveryGuards$' ./cmd/agenova-control-plane`
  passed separately. This diagnostic exclusion is not a full gate pass.
- `./scripts/check.ps1 -All`: docs/architecture/contracts, formatting, tidy and
  vet passed; all Go packages except that same control-plane test passed. The
  full gate stopped there; frontend checks were run independently as above.
- Disposable TLS PostgreSQL: authentication/postgres, websession, console,
  workstore and connectedclient packages passed with real local PostgreSQL.
  Control-plane package again failed only the non-SQL directory-sync test.
  The campaign therefore reports `tests_passed: false`, `container_removed: true`.
  No shared database or cloud resources were touched.
- `node harness/integration/identity/google-login.mjs`: passed existing login
  labels, CSP, fragment removal, same-origin POST and mobile regressions with
  synthetic provider transport; no real Google login was attempted.
- Local anonymous rendering of the merged Go handlers passed homepage → existing
  `/login`, Google form retention, mobile/dark preference and private API 401,
  with no automatic API/IdP requests. Evidence: `.tmp/landing-preview/`.
- #216 main release run [39349156962](https://github.com/wunderforge/agenova/actions/runs/39349156962)
  passed source validation; snapshot, publish and deploy jobs were all skipped.
- Linux PR CI remains the complete unskipped gate for the new UI candidate.
  Independent approval and all deployment prerequisites remain separate.

