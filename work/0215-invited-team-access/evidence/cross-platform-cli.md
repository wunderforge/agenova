# Cross-platform CLI distribution — 13 October 2026

## Published artifacts

[Preview release](https://github.com/Leo1piece/homebrew-agenova/releases/tag/v0.1.0-preview.20261013.1)
contains native Windows x64 ZIP, Linux x64 tar.gz, per-archive SHA256 files and a
per-user Windows installer. Public tap commit 74fa5ea removes the macOS-only
restriction and pins source 3921df511f3d5d0d7425eaf0163997ee4d378450 with verified
archive SHA256. Subsequent source commits only change CI/packaging/docs.

Windows installer verifies SHA256 before extracting, installs to LocalAppData and
updates user PATH; it does not change execution policy or need elevation.
Linux uses the existing unlocked login Secret Service collection with namespaced
keys; no plaintext backend is allowed. WSL detects wslview where available and
otherwise uses xdg-open. macOS Keychain and Windows Credential Manager remain.

## Evidence

[Native CI run](https://github.com/wunderforge/agenova/actions/runs/38369627349)
completed successfully: baseline, Windows and Ubuntu jobs all passed.

- Windows: actual Credential Manager Set/Get/Delete, reopened store persistence,
  synthetic OAuth PKCE/session/logout tests, native build and ZIP packaging,
  clean per-user installer execution, installed version/help.
- Ubuntu: actual D-Bus + encrypted GNOME login keyring Set/Get/Delete, synthetic
  OAuth/session tests, native build/archive, actual Homebrew source install and
  brew test from the same formula published to the public tap.
- Local macOS: focused login tests and full check.ps1 -All passed (55 browser
  smokes); Windows/Linux cross-compilation also succeeded.
- Downloaded Windows/Linux CI archives were checked against their SHA256 before
  publishing. Release listing verified all five assets after publication.

The first Linux test exposed an unwanted collection-creation prompt; using the
existing login collection fixed it. Two Homebrew fixture failures concerned Git
identity setup; the final fixture initializes a disposable local tap without
creating a Git commit. Neither failure was suppressed.

## Limits

Tests use synthetic OAuth servers and synthetic credentials in isolated native
stores, not real Google user login on Windows/Linux. No real WSL2 host is available
in this session. WSL2 localhost browser callback, wslview launch and complete
Google login/run/list require a teammate's WSL2 acceptance run. Headless Linux
requires an unlocked Secret Service session. Windows ARM64 and macOS Intel install
are unverified. Packages are unsigned previews, not a stable product release.
No cloud permissions or application memberships were changed.
