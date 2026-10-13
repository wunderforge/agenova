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

Download the Windows ZIP and matching `.sha256` file from the
[preview releases](https://github.com/Leo1piece/homebrew-agenova/releases).
Verify with `Get-FileHash -Algorithm SHA256`, extract `agenova.exe` into a directory
on your user PATH, then open a new terminal.

Alternatively download `install-windows.ps1` from the same release, review it,
and run it in Windows PowerShell. It verifies the archive checksum, installs to
`%LOCALAPPDATA%\Agenova\bin` and adds that directory to your user PATH. No admin
rights or execution-policy changes are required; if local policy prevents running
scripts, use the ZIP method. Windows Credential Manager stores the login.

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
