# Portal demo slice — execution contract

Mission: Turn the reviewed HTML exploration into a React user journey without implying that illustrative records are live Agenova evidence.

Target: `ui/src/Portal.tsx`, its typed demo data, styles, and browser smoke. Preserve the existing #59 contract storyboard and #60 single-claim console.

User value: A first-time viewer can start at Work, inspect one assignment and its access/activity, then understand the platform capabilities and their current connection state.

Acceptance criteria:

- Work list, new-work demo, work detail, access comparison, work activity/record, agents, policy, platform, platform activity, and demo identity are navigable and responsive.
- Work activity is scoped to one work item. Platform activity aggregates only request-resolution, tool, model, and runtime examples across work items.
- The narrowed example shows requested `shell.exec` and 45 minutes, a template ceiling without `shell.exec` and max 30 minutes, and a 30-minute illustrative effective result. The detailed resolution record contains the effective scope and timeout.
- A newly submitted example remains Pending with no claim, grant, backend, or gateway record. No form invokes a backend.
- The UI distinguishes validated canonical fixture viewing from illustrative portal records and missing live API/gateway/memory integration. It does not infer policy results from requests or represent mock records as execution evidence.
- Existing #59/#60 behavior remains reachable and regression tests pass.

Quality gates: focused React tests; Playwright desktop/mobile flows and screenshots; `npm --prefix ui run check`; `./scripts/check.ps1 -All`; `git diff --check`.

Evidence required: exact command results, rendered desktop/mobile screenshots, a negative Pending/no-grant browser assertion, scoped activity assertions, and a status map of connected versus example-only surfaces.

Constraints: This is a user-approved demonstration slice, not completion of #60's live API/polling acceptance or an expansion of the MVP product contract. The original `.tmp` HTML prototypes stay untouched. Fixture/illustrative data stays behind UI adapters; no new governance DTO or policy evaluator is introduced.

Known gotchas: #60 is not yet merged to main, so this branch is stacked on its reviewed fixture-console implementation. `docs/project-status.md` is a merged-state snapshot and must not be advanced for this unmerged UI. Browser screenshots are evidence of rendered behavior, not backend proof.

## Verification — 14 Sep 2026

- `npm --prefix ui run build`: passed after the final form change.
- `npm --prefix ui run test:smoke -- portal.spec.ts`: 5/5 passed, including a non-repository Researcher request, Pending/no-grant, scoped Work activity, cross-work Platform activity, authority narrowing, and mobile layout.
- `$env:GOFLAGS='-buildvcs=false'; ./scripts/check.ps1 -All`: passed. Go tests, contract checks, 72 frontend tests, production build, and 28 browser tests passed. The flag only avoids local VCS stamping because this sandbox cannot read Git's global ignore path; the first unmodified `-All` attempt failed solely in the CLI smoke's VCS read.
- `git diff --check`: passed.
- Rendered Playwright captures: `.tmp/ui-smoke/portal-portal-journey-keep-5b3b8-nd-shows-narrowed-authority/portal-work-list.png`, `portal-resolution.png` in the same directory, `.tmp/ui-smoke/portal-platform-activity-a-87126--and-links-back-to-one-work/portal-platform.png`, and `.tmp/ui-smoke/portal-portal-remains-usable-at-mobile-width/portal-mobile-work.png`. Desktop Work, Platform, resolution record, and mobile Work captures were visually inspected.

Residual boundary: The portal's example records are intentionally distinct from the canonical fixture console. Live evidence, persistence, submission, policy evaluation, and observed gateway traffic remain unconnected. No real-backend behavior is claimed by this slice.

## Connected-view extension — 14 Sep 2026

Mission: Let the team inspect the actual connection state from the same navigation without mistaking example content for live data.

Acceptance: A persistent Demo/Connected switch preserves the current route; Connected never renders the `portal-data.ts` stories or demo identity; each page explains missing capability in user language; Platform summarizes readiness by user task; the mobile status summary requires no horizontal scrolling. An unconnected view is never labeled as an empty successful result or a transient connection failure.

Boundary: No live portal endpoint is available in this branch. Connected therefore renders only `Not connected`; this task does not fake a successful request or add an HTTP contract. Future slices must attach accepted live sources and represent loading, successful empty, and failure states separately before claiming that a row is connected.

Evidence: `npm --prefix ui run test:smoke -- portal.spec.ts` passed 8/8 after the responsive repair. `$env:GOFLAGS='-buildvcs=false'; ./scripts/check.ps1 -All` passed with all 31 browser tests. Screenshots include `.tmp/ui-smoke/portal-connected-pages-exp-b6768-ithout-leaking-demo-records/portal-connected-work.png`, `portal-connected-platform.png`, and `.tmp/ui-smoke/portal-connected-status-stays-readable-on-mobile/portal-connected-mobile.png`. The final mobile capture was visually inspected after converting the status table to vertically stacked items.
