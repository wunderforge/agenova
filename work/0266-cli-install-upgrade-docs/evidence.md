# Installation documentation evidence

Current preview: `v0.1.0-preview.20261102.1`; source commit
`66936e91386233b5514ba005eba3436548c98e43`.

## Release verification

The published upstream release contains Windows x64 ZIP, Linux x64 tarball,
their SHA256 sidecars, install-windows.ps1 and release-manifest.json. The tag
points to the source commit above. The separately maintained Homebrew tap still
pins the older preview; its account does not grant write access here.

## Gates

- `./scripts/check.ps1 -Docs`: pass, including local Markdown links and task/PR contracts.
- Five PowerShell snippets parsed with `System.Management.Automation.Language.Parser`: pass.
- The exact documented ZIP checksum/extraction/version snippet, using assets downloaded from the public release: pass.
- Public release metadata checked against every current download link: pass.
- Published installer downloaded via the documented URL and run into a disposable installation with `-NoPathUpdate`: pass. ZIP and checksum are fetched by the real installer.
- Existing-path upgrade using the documented first-Application selection with two Agenova directories on PATH: pass. The test supplied the previously verified ZIP through `-ArchivePath` to avoid a redundant download. The selected temporary binary reports `0.1.0-preview.20261102.1`; the real installation is untouched.
- Repository `./scripts/check.ps1 -All`: fail at the pre-existing Windows `TestOwnerBootstrapDeliveryGuards` invitation-file test in `cmd/agenova-control-plane`. Documentation, formatting, module metadata, vet and all other reported Go packages passed. The script stops before frontend checks; this is not a full-suite pass.
- Linux PR CI: required before merge; its result will be attached to the PR.

## Review findings

The upgrade smoke caught a duplicate-PATH case: `Get-Command` with an Application
filter can return multiple matches. The guide now explicitly selects the first
Application before deriving its directory. The corrected duplicate-PATH upgrade
was rerun successfully. No product change was needed.

The evidence concerns installation/version behavior. No live Work or real Google
login was performed while the cluster is stopped.

No application code, image, workflow, credential or cloud resource changes.
Linux native runtime/login, macOS source-build/login and WSL remain unverified
for this CLI distribution; the guide explicitly states these limits.
