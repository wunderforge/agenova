# Owner-approved public landing (20 October)

## Outcome

Integrate the approved June landing into the installed member UI. Anonymous root
shows public product information and explicitly illustrative UI captures. Sign in
opens the existing `/login` flow. Authenticated root still opens Connected Work.
This owner-requested follow-up extends the explicit public surface, not membership,
policy, session, OAuth or admission semantics.

Context: [task](task.md), [spec](spec.md), [design](design.md),
[production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Scope and negative cases

- Serve a finite allowlist of public files from the built Portal's `landing/`.
  No directory listing, arbitrary file access, API proxy or request-time UI screenshots.
- Reuse the existing member authentication handler, pinned origin, login GET/POST,
  callback, CSRF, invitation/native CLI and membership behavior.
- Anonymous `/api/*`, `/auth/me`, private bundles and management routes stay protected.
  No Demo fallback, identity marker, private data or storage write on the landing.
- Login buttons become ordinary same-origin `/login` links; remove mock notices.
- Match the existing login/pending-page palette to June; retain form markup,
  invitation script, CSP hashes, origin checks and all Google/session semantics.
- Keep explicit fixture labels on public screenshots; no customer logos/metrics.
- Publish only product information even if the session is absent/expired. Missing
  landing files produce an unavailable response, never a protected Portal fallback.
- Preserve existing reference/Vite Demo routes. This adds no public tenant creation.
- Initial acceptance was uncommitted. After preview approval, the Owner authorized
  a local commit on 20 October. No push, PR, deployment, cloud resources,
  secret-file access or live Google login.

## Plan / acceptance

- [x] Read entry points, session boundary, packaging and browser tests; self-review.
- [x] Integrate static landing and explicit public allowlist in member composition.
- [x] Test anonymous page/assets, private routes, authenticated entry and login handoff.
- [x] Run focused and full gates; inspect desktop/mobile renderings of built assets.
- [x] Show local actual HTTP handler and login page; receive approval for local commit.

Local browser evidence may use rejecting/synthetic identity fixtures. It must say
so and is not proof of a live Google exchange, shared AWS deployment or a real Work.

## Provenance and focused results

- `./scripts/check.ps1 -All` passed: Go formatting/tidy/vet/tests, canonical
  contracts, frontend types/build, 130 unit tests and 59 Playwright browser tests.
- Reviewed the final diff: no new schema, OAuth parameters, callback URL, identity
  source, invitation semantics or cloud configuration. Commit locally on
  `codex/june-portal-ui-20261020`, following the June Portal base `c79f348`.
- `ui/public/landing/assets/` contains unchanged June React screenshots from local
  fixture Playwright captures at `c79f348`: `june-explorer-desktop.png` and
  `june-detail-desktop.png`. Labels explicitly say illustrative test data. These
  are not screenshots of a member's current Work or production evidence.
- Focused Go tests pass for `internal/landing`, `internal/websession` and member
  composition. Anonymous root/assets, exact allowlist, HEAD, forbidden mutations,
  missing-file 503/no-Portal fallback, pinned host, private API 401, management
  redirect, authenticated root and opt-in compatibility are covered.
- `ui/smoke/landing.spec.ts` passes against the production UI build, covering login
  hrefs, real product-image loading, tabs, enlargement, keyboard, dark preference,
  mobile, and no automatic API/storage activity.
- Existing `harness/integration/identity/google-login.mjs` passes: actual login
  HTML/script/CSP, invitation fragment removal, same-origin POST, rejected invalid
  proofs and mobile rendering. Its provider transport is explicitly synthetic.
- Existing disposable TLS PostgreSQL suites pass for member/session/console/
  client/Work persistence. The local test container was removed; no cloud resource
  was created. Summary: `.tmp/cicd-integration/local-postgres-summary.json`.
- Local screenshots under `.tmp/landing-preview/` come from the actual Go landing
  and websession handlers: anonymous root → `/login`, Google form intact, private
  APIs returning 401, no automatic API or IdP calls. The local read-only rendering
  harness rejects all identity and login submissions. It reads no real OAuth
  files and cannot perform a real Google exchange. This is not a deployed login
  acceptance test.
