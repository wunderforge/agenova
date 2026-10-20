# Public tap clean CLI installation — 13 October 2026 (Sydney)

Requested scope: uninstall, install from the public Homebrew tap, test from blank
CLI credentials/configuration. This is an actual host test, not a mock.

1. `agenova logout` succeeded. Checked that `current-server` was absent and the
   specific `agenova`/`https://app.agenova.app` Keychain item was absent (security
   lookup exit 44). No secret value was requested or printed.
2. Uninstalled `Leo1piece/agenova/agenova` and untapped `Leo1piece/agenova`.
   `shutil.which("agenova")` returned None. No executable remained on PATH.
3. First reinstall used a source archive whose filename differed from the expected
   cache pattern. To remove ambiguity, resolved the exact archive with `brew
   --cache Leo1piece/agenova/agenova`, deleted only that archive, and repeated
   uninstall/untap/install. The archive was confirmed absent before installation.
4. `brew install Leo1piece/agenova/agenova` cloned the public tap, fetched the pinned
   source, verified its configured checksum and compiled the macOS arm64 executable.
   Version: `0.1.0-preview.20261012.1`. Homebrew formula test passed.
5. Before login, explicit-server `work list` failed with exit 1 and
   `login credentials are unavailable; sign in again`.
6. `agenova login --server https://app.agenova.app` succeeded. A new credential item
   and saved origin were present afterward. Browser login reused the existing
   browser session; no fresh Google-account registration was claimed.
7. `agenova work list`, `agenova run -f work.yaml --json`, `agenova work show
   <request-ref> --json`, and another list all succeeded without `--server`,
   `--state-dir`, AWS credentials or kubeconfig arguments.

Actual request:
`work-9cb46309b7f285b594e6ab3ab7554286d39c7eda0983abd8a92f5602b76f4700`.
Admission/terminal result: Allow / Succeeded.
Backend: agent-sandbox, worker `agenova-pool-pool-engineer-bvqfw`.
Model: amazon.nova-micro-v1:0; final response
`609e1413-6492-4787-a440-f90572f2aeff`, 616 input / 118 output tokens.
The answer described cumulative retry time exceeding the overall deadline and
recommended tracking total elapsed time without resetting the deadline. This run
answered the sample task, unlike the preceding guide walkthrough.

Boundary: existing macOS user, Homebrew, Go toolchain/build cache and browser
sessions were retained. This is blank Agenova client state, not a clean OS image,
fresh Homebrew bootstrap or new member approval test. No cloud data or other
applications' credentials were deleted. CLI remains installed and signed in for
the user's continued testing. Public tap: https://github.com/Leo1piece/homebrew-agenova.
