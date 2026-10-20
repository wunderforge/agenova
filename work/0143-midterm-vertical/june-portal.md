# June Portal — owner-approved visual follow-up (20 October)

## Outcome and scope

Apply the approved local June preview to the actual React Portal: light surfaces,
violet navigation, a compact Work explorer, and recorded activity with an inspector.
Keep the existing API, authentication, administration links, submission and evidence
semantics unchanged. This is an owner-requested continuation of the Portal slice,
not a new backend feature or a change to the original checkpoint acceptance.

Context: [task](task.md), [spec](spec.md), [design](design.md),
[PRD: read-only claim console](../../docs/product/prd.md#7-read-only-claim-console).

## Acceptance and negative cases

- Demo and Connected share layout, never data; disconnected views do not show fixtures.
- Summary counts describe only the loaded list, not global platform metrics.
- Admission decisions, provider completion and overall Work outcome remain distinct.
- Inspection uses recorded targets, statuses and links. No invented spans or durations.
- Preserve every existing route, form, POST logout and administration entry.
- Fixed light palette under normal light/dark browser preferences; browser extensions
  or forced-dark rendering can still override page appearance.
- Usable desktop and mobile layout, keyboard focus and reduced-motion support.

## Execution and evidence

- [x] Scout current components, API projections, browser tests and approved preview.
- [x] Self-review: presentation only, no contract change or cloud operations.
- [x] Implement shell, explorer and activity inspection.
- [x] Run unit/type/build/browser gates, inspect desktop/mobile screenshots.
- [x] Run `./scripts/check.ps1 -All`, review diff and prepare local-only commit.

No PR, push, deployment, paid resource, cloud authentication or live Work submission.
Existing deterministic API fixtures verify the real Connected components, not a live
cloud provider.

### Verified locally

- `./scripts/check.ps1 -All`: passed, including Go formatting/tidy/vet/tests,
  canonical contracts, TypeScript, production build, 130 frontend unit tests and
  58 Playwright browser tests.
- Browser regressions cover existing submission/denial/authority comparison,
  pending calls and polling stability, failed outcome versus successful calls,
  account navigation/POST logout, unavailable/no-fixture-fallback behavior,
  fixed-light system preferences, keyboard inspection and mobile overflow.
- Connected inspector updates the selected call's status and evidence link when
  its actual API fixture receives a completion; it does not assume one beforehand.
- Desktop (1440px) and mobile (390px) renderings inspected under `.tmp/ui-smoke/`:
  `june-explorer-desktop.png`, `june-detail-desktop.png`, `june-detail-mobile.png`,
  `june-policy-mobile.png`, `june-connected-list.png`, `june-connected-detail.png`.
  These are reproducible local test artifacts, not live execution evidence.
- No backend/provider/security behavior changed. No live cluster/login/provider
  validation was performed for this presentation-only change.

Visual references: the public [June preview](https://getdesign.md/design-md/june)
and [Datadog trace inspection](https://docs.datadoghq.com/tracing/trace_explorer/trace_view/).
Use the approved preview's light surfaces and trace/list-inspector relationship,
not proprietary design files, synthetic preview records or fabricated time bars.
