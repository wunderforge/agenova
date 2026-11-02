# Install or upgrade the Agenova CLI

Current preview: **`0.1.0-preview.20261102.1`**, built from merged main
[`66936e9`](https://github.com/wunderforge/agenova/commit/66936e91386233b5514ba005eba3436548c98e43).
The [upstream release](https://github.com/wunderforge/agenova/releases/tag/v0.1.0-preview.20261102.1)
contains packages, SHA256 checksums, a Windows installer and the build manifest.
These are unsigned previews, not signed stable releases.

The CLI uses your personal Agenova login. Ordinary Work submission needs no AWS
account, kubeconfig, Docker or shared state directory. Installation and
`agenova --version` work while the service is offline; login and remote Work
commands require a running service. Operators additionally need their own
AWS/Kubernetes identity for deployment.

| Platform | Current installation route | Verification for this release |
| --- | --- | --- |
| Windows x64 | ZIP or PowerShell installer below | Native regression, Credential Manager and installer tests passed |
| Linux x64 | Checked tarball below | Cross-built on Windows; native runtime/login not tested |
| macOS | Pinned source build below | No binary package; native build/login not tested |

The separately maintained `Leo1piece/agenova` Homebrew tap currently distributes
the older `0.1.0-preview.20261013.1`. `brew update && brew upgrade agenova` does
**not** install the current preview. Use the upstream package or pinned source
build, then check which executable your terminal selects.

## Windows x64

Docker, WSL, Homebrew and an administrator shell are not required. Close running
Agenova CLI processes before replacing the executable.

### New installation

From a writable folder in PowerShell:

```powershell
$version = '0.1.0-preview.20261102.1'
$base = "https://github.com/wunderforge/agenova/releases/download/v$version"
$installer = Join-Path $PWD 'install-agenova.ps1'
Invoke-WebRequest "$base/install-windows.ps1" -OutFile $installer
Get-Content $installer # Review before running.
& $installer -Version $version
agenova --version
```

The installer checks the ZIP's SHA256, installs to
`%LOCALAPPDATA%\Agenova\bin`, and adds that directory to your user PATH and the
current session. Open a new terminal for other sessions. It does not change
execution policy. If scripts are blocked, use the ZIP route below instead.

For a custom location, replace the installer invocation with:

```powershell
& $installer -Version $version -InstallDirectory 'D:\Tools\Agenova\bin'
```

### Upgrade an existing installation

Check the active executable and duplicates first:

```powershell
Get-Command agenova -All | Select-Object Source
agenova --version
```

Download and review the new installer as above, then upgrade the active binary
in its existing directory:

```powershell
$installDir = Split-Path -Parent (Get-Command agenova -CommandType Application | Select-Object -First 1).Source
& $installer -Version $version -InstallDirectory $installDir -NoPathUpdate
agenova --version
(Get-Command agenova -CommandType Application | Select-Object -First 1).Source
```

Check the selected path before running the installer. `-NoPathUpdate` leaves
persistent PATH unchanged; use it for a directory already on PATH. Upgrading
preserves OS-stored login entries. You do not need to log out first; sign in again
only if your session has expired and the service is running.

### ZIP installation or upgrade

Download both [Windows x64 ZIP](https://github.com/wunderforge/agenova/releases/download/v0.1.0-preview.20261102.1/agenova_0.1.0-preview.20261102.1_windows_amd64.zip)
and [SHA256 checksum](https://github.com/wunderforge/agenova/releases/download/v0.1.0-preview.20261102.1/agenova_0.1.0-preview.20261102.1_windows_amd64.zip.sha256)
into the same folder. From that folder:

```powershell
$version = '0.1.0-preview.20261102.1'
$archive = "agenova_${version}_windows_amd64.zip"
$expected = ((Get-Content "$archive.sha256" -Raw).Trim() -split '\s+')[0]
if ($expected -notmatch '^[a-fA-F0-9]{64}$' -or
    (Get-FileHash $archive -Algorithm SHA256).Hash -ne $expected) {
    throw 'Checksum verification failed; do not install.'
}
Expand-Archive $archive -DestinationPath "agenova-$version"
& ".\agenova-$version\agenova.exe" --version
```

After verification, copy `agenova.exe` into the chosen installation directory,
replacing its old binary for an upgrade. For a new installation, add that
directory to your **user PATH** through Windows Environment Variables. Open a new
terminal and confirm the path and version. Choose a fresh extraction folder if
one already exists.

Windows Credential Manager stores login entries, without a shared credentials
file. Native ARM64 and ARM64 emulation were not acceptance-tested. To uninstall,
log out while the service is available, then remove your installation directory
and its user PATH entry.

## Linux x64

This is the current upstream preview tarball. Native Linux runtime/login were
not tested for this cross-built release. From a writable folder:

```bash
version=0.1.0-preview.20261102.1
base="https://github.com/wunderforge/agenova/releases/download/v$version"
archive="agenova_${version}_linux_amd64.tar.gz"
curl -fL "$base/$archive" -o "$archive"
curl -fL "$base/$archive.sha256" -o "$archive.sha256"
sha256sum -c "$archive.sha256" || exit 1
tar -xzf "$archive" agenova
mkdir -p "$HOME/.local/bin"
install -m 755 agenova "$HOME/.local/bin/agenova"
export PATH="$HOME/.local/bin:$PATH"
agenova --version
command -v agenova
```

For upgrades, run `type -a agenova` first. These commands replace
`$HOME/.local/bin/agenova`; choose the existing directory if you intend to upgrade
a different installation. Add the selected directory to your shell's startup
PATH if needed. Existing keyring entries remain intact.

On Ubuntu/Debian, install `dbus-user-session`, `gnome-keyring` and `xdg-utils`
with your package manager. Your desktop normally starts D-Bus and unlocks the
login keyring. A headless shell needs an unlocked Secret Service session;
installation alone does not create or unlock one. Credentials never fall back
to plaintext. Do not put real passwords in shell scripts.

## macOS: current version from source

The old Homebrew formula is not updated by this upstream release. To build the
current tag, install Go 1.27.2 and Xcode Command Line Tools first. Native macOS
build/login for this tag remains unverified.

```bash
version=0.1.0-preview.20261102.1
mkdir -p "$HOME/.local/bin"
GOTOOLCHAIN=go1.27.2 CGO_ENABLED=1 GOBIN="$HOME/.local/bin" go install \
  -ldflags="-X github.com/wunderforge/agenova/internal/cli.Version=$version" \
  "github.com/wunderforge/agenova/cmd/agenova@v$version"
export PATH="$HOME/.local/bin:$PATH"
agenova --version
type -a agenova
```

Keep CGO enabled for macOS Keychain support. This pins the release tag, not a
moving branch. For upgrades, rerun with the desired version. Add the selected bin
directory to your shell's startup PATH. If Homebrew is also installed, check
the active path so you do not run its older binary. Allow the installed binary
when Keychain prompts; an upgrade may require renewed approval.

## WSL2

Choose one CLI environment and keep its login/configuration there:

1. **Linux CLI:** use the Linux tarball instructions. An unlocked Secret Service
   keyring and D-Bus session are required inside WSL. Install `wslu` for `wslview`
   to open the Windows browser; otherwise the CLI uses `xdg-open`. The browser
   must reach the callback at `127.0.0.1:38123`.
2. **Windows CLI from WSL:** install the Windows ZIP, then invoke `agenova.exe`
   through Windows interoperability. It uses Windows Credential Manager, the
   Windows browser and Windows paths, not the Linux CLI's login state.

WSL networking/desktop availability varies. WSL1 and disabled Windows
interoperability/localhost forwarding are not supported by this guide. Windows
tests and a cross-built Linux package do not constitute WSL acceptance.

For interactive Ubuntu WSL without a desktop-managed keyring, install
`dbus-user-session`, `gnome-keyring` and `xdg-utils`, then start a dedicated session:

```bash
dbus-run-session -- bash
set +x
read -r -s -p "Login keyring password: " AGENOVA_KEYRING_PASSWORD
printf '\n'
printf '%s' "$AGENOVA_KEYRING_PASSWORD" | gnome-keyring-daemon --unlock --components=secrets
unset AGENOVA_KEYRING_PASSWORD
agenova login --server https://app.agenova.app
agenova work list
```

Use the existing keyring password or choose one when initializing a new keyring;
this is a local encryption password, not your Google password. Keep using that
D-Bus shell. Never save the password in shell startup files. This procedure still
needs real WSL2 acceptance; use the Windows CLI if it is unsuitable.

## Verify the upgrade, then sign in

`agenova --version` should begin with:

```text
agenova 0.1.0-preview.20261102.1
```

`agenova dev` or an older version means your shell selected another binary.
Check `Get-Command agenova -All` on Windows or `type -a agenova` on macOS/Linux.
The `runtime-backend: memory` line in version output describes the standalone
reference backend; it does not mean remote Work uses a simulated model. Default
Work commands use the signed-in service. `--backend memory` explicitly selects
local reference execution.

When the team service is running:

```bash
agenova login --server https://app.agenova.app
agenova work list
agenova run -f work.yaml
```

Google login uses your approved Agenova account and project access. Download the
[example Work YAML](examples/member-work.yaml), change its request name and use
an approved project/template. Login remembers the server; no state-dir argument
or shared credentials are needed. `agenova logout` signs out when required;
it is not an upgrade step.
