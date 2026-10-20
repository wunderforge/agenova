# Install the Agenova CLI

The CLI uses your personal Agenova login. Work submission needs no AWS account,
kubeconfig, Docker or shared state directory. Deployment operators additionally
need their own AWS/Kubernetes identity. Packages are previews, not signed stable releases.

## macOS

```bash
brew install Leo1piece/agenova/agenova
# Existing installation:
brew update && brew upgrade agenova
```

Credentials use macOS Keychain. Allow the installed binary when prompted.

## Linux (desktop, or an unlocked Secret Service session)

Install Homebrew for Linux following <https://docs.brew.sh/Homebrew-on-Linux>, then:

```bash
brew install Leo1piece/agenova/agenova
agenova --version
```

On Ubuntu/Debian, install `dbus-user-session`, `gnome-keyring` and `xdg-utils`
with your system package manager. Your desktop normally starts D-Bus and unlocks
the login keyring. A headless shell must have an unlocked Secret Service session;
installation alone does not create or unlock one. Credentials never fall back to
plaintext. If your environment cannot provide a keyring/browser, use a supported
interactive workstation instead. Do not put real passwords in shell scripts.

## Windows x64

The Windows package contains `agenova.exe`, a terminal application. Docker, WSL,
Homebrew and an administrator shell are not required.

### PowerShell installer

Run in Windows PowerShell from a writable folder:

```powershell
$installer = Join-Path $PWD 'install-agenova.ps1'
Invoke-WebRequest 'https://github.com/Leo1piece/homebrew-agenova/releases/download/v0.1.0-preview.20261013.1/install-windows.ps1' -OutFile $installer
Get-Content $installer # Review the installer before running it.
& $installer
agenova --version
```

The script verifies the downloaded ZIP's SHA256, installs `agenova.exe` to
`%LOCALAPPDATA%\Agenova\bin`, and adds it to your user PATH. Open a new PowerShell
window for subsequent use. It does not change your execution policy.

### ZIP installation (if scripts are blocked)

Download both:

- [Windows x64 ZIP](https://github.com/Leo1piece/homebrew-agenova/releases/download/v0.1.0-preview.20261013.1/agenova_0.1.0-preview.20261013.1_windows_amd64.zip)
- [SHA256 checksum](https://github.com/Leo1piece/homebrew-agenova/releases/download/v0.1.0-preview.20261013.1/agenova_0.1.0-preview.20261013.1_windows_amd64.zip.sha256)

Compare `Get-FileHash -Algorithm SHA256` with the checksum, extract the ZIP, then
run `.\agenova.exe --version` from its folder. Add that folder to your **user PATH**
through Windows Environment Variables to use `agenova` from any folder.
Windows Credential Manager stores the login; it is not a shared credentials file.

The installer selects x64. ARM64 emulation and native ARM64 are not acceptance-tested.
Uninstall by logging out, removing the installed directory and its user PATH entry.

## WSL2

Choose one CLI environment and keep its login/configuration there:

1. **Linux CLI:** use the Linux Homebrew instructions. D-Bus and an unlocked
   Secret Service keyring are required inside WSL. Install `wslu` where available
   to provide `wslview`; Agenova uses it to open the Windows browser. Otherwise it
   uses `xdg-open`. The browser must reach the CLI callback at `127.0.0.1:38123`.
2. **Windows CLI from WSL:** install the Windows ZIP first, then invoke
   `agenova.exe` through Windows interoperability. This uses Windows Credential
   Manager, the Windows browser and Windows paths (including Work YAML paths).
   It is not the Linux Homebrew binary and does not share Linux login state.

WSL networking/desktop availability varies. WSL1 and a disabled Windows
interoperability/localhost-forwarding setup are not supported by this guide.
Native Linux and Windows CI do not constitute a real WSL acceptance run.

For an interactive Ubuntu WSL shell without a desktop-managed keyring, install
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

Use the existing login-keyring password, or choose one when initializing a new
keyring. It is a local encryption password, not your Google password. Keep using
that D-Bus shell for CLI commands; closing it ends the session. Never save the
password in shell startup files. If this desktop setup is unsuitable, choose the
Windows CLI route above instead. This WSL procedure still needs a real WSL2 check.

## Login and run

```bash
agenova login --server https://app.agenova.app
agenova work list
agenova run -f work.yaml
agenova logout
```

Use `agenova.exe` instead when calling the Windows binary from WSL. The browser
performs Google sign-in; administrator approval and project access are required.
Download [example Work YAML](examples/member-work.yaml), change its request name,
and use an approved project/template. Login remembers the server. No state-dir
argument or shared credentials are needed for Work commands.
