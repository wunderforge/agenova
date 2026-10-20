# Task: Complete June styling across Administration and remaining pages

- Ticket: [#238](https://github.com/wunderforge/agenova/issues/238)
- Mission: Finish the approved June visual treatment without changing authority or login behavior.
- Target: embedded Administration assets, access-status documents, fixture/evidence CSS and browser regression tests.
- User value: Every reachable page feels like the same application, including administration and failure states.
- PRD outcome: [team access and presentation](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet.
- `internal/invitations/page.{go,html,css,js}`: server authorization, CSP hashes and management controls.
- `internal/websession/handler.go`, `login.{html,css}`: login and pending/declined states.
- `ui/src/portal-june.css`, `style.css`, `console.css`; `ui/smoke/` and `ui/playwright.config.ts`.
- `docs/development/shared-eks-release.md`: pinned release and compatibility checks.

## Scope

In scope: June palette, typography, navigation spacing, controls, responsive overflow and theme-lock consistency across all rendered page families; regression/evidence and scoped rollout.

Out of scope: business/API/schema changes, IAM/Secrets, Worker protocol upgrade, new services, replacing existing authentication, automatic deployment redesign.

## Acceptance Criteria

- Administration Users, Access requests, Policy & Templates and Roles match June under light/dark preference, desktop and mobile.
- Login and access-status pages remain legible and lock the intended light palette; fixture/evidence pages use matching tokens while retaining provenance.
- No regressions in administrative role updates, approval controls, governance loading/errors, disabled states or navigation.
- CSP stays hash-based; administration still rejects anonymous and unauthorized users.
- Green source gates, rendered screenshots and verified SHA/digest-pinned presentation-only deployment.

## Negative Case

Changing CSS or markup must not grant admin access, bypass CSRF/CSP, expose private data or turn fixtures into apparently live evidence. Browser management data is synthetic and never submitted to the shared service.

## Execution Todo

- [x] Scout page families, authorization boundaries and release dependencies.
- [x] Self-review against the accepted Owner request, #238 and PRD; no product boundary changes.
- [x] Unify missed server/fixture styles and theme metadata.
- [x] Add permission/CSP and browser visual/interaction regression coverage.
- [ ] Run focused gates and `./scripts/check.ps1 -All`.
- [ ] Review diff, merge green PR and deploy the presentation-only patch; record exact evidence.

## Quality Gates

- `go test ./internal/invitations ./internal/websession`
- `npm --prefix ui run check`
- `./scripts/check.ps1 -All`

## Evidence Required

Desktop/mobile Administration screenshots with populated/empty/error states, login and access-status checks, page-family audit, preserved auth/CSP tests, Actions results and release record.

## Constraints

Preserve the architecture contract. Do not read secret values or change membership in live smoke. Existing schema 009 and Worker pair remain unchanged. Owner authorized administrator merge only for this UI fix PR after all CI passes.

## Decisions and Blockers

- /admin is server-rendered and did not inherit portal-june.css. Repair its embedded styles rather than moving its authorization boundary into the client.
- Pending/declined output reused light CSS but lacked the login page's theme metadata. Fixture/evidence pages retained the pre-June green palette.
- Main includes a separate Worker upgrade absent from the deployed #234 source. Production delivery must isolate this presentation patch on the actual live baseline; no untested cumulative backend promotion.
- Windows full gate reaches `go test ./...` but fails the unchanged `TestOwnerBootstrapDeliveryGuards` directory-sync path. Preserve the failure; Linux CI must pass before merge. Do not weaken bootstrap durability for this presentation ticket.
