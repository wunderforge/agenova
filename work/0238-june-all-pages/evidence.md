# June page-family audit

Presentation-only change; authentication, administrative JavaScript, API payloads, schema and Worker protocol remain unchanged.

| Page family | Before | Repair / coverage |
|---|---|---|
| Administration: Users, Access requests, Policy & Templates, Roles | Independent embedded dark stylesheet; not covered by Portal tests | June tokens, white sidebar/cards, purple controls, theme metadata, keyboard skip link and responsive fields |
| Login | Already June | Existing POST/Google login and hash-based CSP retained |
| Access pending / declined | June login CSS but missing theme-lock metadata | Add the same light-theme/browser-darkening metadata; no credentials issued |
| Fixture storyboard and evidence console, including missing/malformed routes | Older green palette | June neutral/purple/semantic colors; keep fixture provenance and diagnostic states |
| Portal list/new/detail/access/activity, Agents, Policy, Platform, Identity | Already June | Add route-family dark-preference/mobile regression to detect omissions |

## Rendered evidence

These are local renders of the actual embedded Administration HTML/CSS/JS with **synthetic management API fixtures**, not live account screenshots or authorization evidence.

- [Desktop](evidence/administration-desktop.png)
- [Mobile](evidence/administration-mobile.png)

## Focused verification

- `go test ./internal/invitations ./internal/websession`: passed. Includes anonymous rejection, existing member-denied page case, CSP hashes and pending/declined theme metadata with no new cookies.
- `npm --prefix ui run build`: passed.
- `npm --prefix ui run test:smoke -- admin.spec.ts --workers=2`: final focused run 6 passed. All management sections, POST shapes, invalid JSON, empty/unavailable state, fixture/evidence and Portal route families covered; placeholder and denied-label contrast must be at least 4.5:1.
- Initial full frontend check: 132 component tests and 66 browser tests passed. Automated review subsequently identified two small-text contrast issues; both colors were darkened and a sixth focused regression added. Final CI reruns the full suite for the corrected head.
- Existing two June browser tests passed alongside the initial focused run. An initially invalid raw-percent URL hit the preview server rather than the app; the regression now uses the application's malformed-route path.
- `./scripts/check.ps1 -All`: docs/contracts/format/tidy/vet passed; `go test ./...` failed only `TestOwnerBootstrapDeliveryGuards` in the unchanged Windows directory-sync path. All other listed packages passed. Frontend full checks run separately; Linux CI remains a merge gate. No bootstrap code/test workaround is included.
- CI and deployed-source verification are recorded in the PR/release evidence after completion; local browser fixtures do not establish live membership or AWS safety behavior.

## Release boundary

The live baseline is #234 (`53f5842`), while main separately contains #231's Worker migration. Backport only this issue's presentation/test packet to that baseline, merge its PR into main, and publish/deploy that exact compatible backport SHA. Preserve database schema, configuration and Worker binding; keep automatic release switches unchanged. The release SHA remains an ancestor of main, so later cumulative delivery still detects the outstanding Worker upgrade.
