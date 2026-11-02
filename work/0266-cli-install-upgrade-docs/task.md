# Task: Current CLI installation and upgrade guidance

- Ticket: [#266](https://github.com/wunderforge/agenova/issues/266)
- Mission: let teammates install and upgrade the published current CLI without following the older tap.
- Target: README.md, docs/cli-installation.md, docs/team-quick-guide.md.
- User value: current downloads, verified checksums and explicit in-place upgrade steps.
- PRD outcome: [user-facing CLI delivery](../../docs/product/prd.md); no behavior or scope change.

## Context to Read

AGENTS.md; docs/product/prd.md; this packet; [installation guide](../../docs/cli-installation.md); [team guide](../../docs/team-quick-guide.md); scripts/install-windows.ps1; published v0.1.0-preview.20261102.1 installer and release-manifest.json.

## Scope

In scope: upstream release pointers, Windows new/custom/existing-path installs, Linux tarball checksums, pinned macOS source-build guidance, PATH/version troubleshooting and truthful platform verification limits.
Out of scope: code changes, tap maintenance, cluster lifecycle, deployment, credentials access and new release automation.

## Acceptance Criteria

- README exposes the current preview and installation guide.
- Windows instructions verify checksums and support upgrades at the existing active path without changing persistent PATH.
- Linux instructions verify the current tarball; macOS route pins the current source tag and preserves CGO/Keychain support.
- The older separate tap is clearly distinguished; no promise that brew upgrade provides the current preview.
- Users can check version and PATH; login is separate from installation and requires an available service.
- Links match public release assets; documentation and PR checks pass.

## Negative Case

Do not disable execution policy, bypass checksums, overwrite a different installation silently, erase login credentials, or claim native Linux/macOS/WSL validation from a Windows cross-build.

## Execution Todo

- [x] Scout release assets, current install path, script options and source tag.
- [x] Assignee self-review: documentation-only scope fits the Ticket and PRD; no contract change.
- [x] Update README, install guide and nearby quick-guide pointers.
- [x] Verify links, documented commands and focused docs gate; record the pre-existing Windows full-gate failure.
- [ ] Review diff, commit/push and create PR; merge after required checks.

## Quality Gates

- Published release metadata/assets and source tag verification.
- Documented Windows ZIP checksum, installer and existing-path upgrade smoke in a temporary directory; no real credentials.
- PowerShell syntax parsing of documented snippets.
- `./scripts/check.ps1 -Docs`; `./scripts/check.ps1 -All`; Linux PR CI.

## Evidence Required

[Evidence](evidence.md): exact commands and results, including any pre-existing local gate failure; PR CI links. No private runtime identifiers or credential values.

## Constraints

Preserve docs/product/architecture-contract.md, all pre-existing files including debug.log, existing execution/login and cloud state. Cache and evidence on D:. No cluster recovery or deployment. Branch-protection bypass requires explicit authorization if the normal merge is blocked.

## Decisions and Blockers

The old tap has no write permission in this account. Route current installations to the upstream published preview instead of implying the tap was updated. No native macOS host is available; mark source-build instructions as unverified for this release. No application image files change.
