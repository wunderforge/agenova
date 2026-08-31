# Evidence Summary

- Ticket: [#40](https://github.com/wunderforge/agenova/issues/40)
- Gate: cli-smoke
- Date: 2026-08-31
- Branch: main
- Command: `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova`
- Binary: OS-correct `go build -o .tmp/agenova ./cmd/agenova` (Windows smoke uses `agenova.exe` via `runtime.GOOS` and `t.TempDir()`)
- Result: pass

Raw output: `output.txt` (focused tests) and `binary-output.txt` (executable smoke).

## Observed behavior

- `agenova --help` exits 0 and prints usage.
- `agenova version` exits 0 and reports `runtime-backend: memory`.
- `agenova not-a-command` exits 2 with actionable stderr.
- `agenova --backend kubernetes version` exits 2 and does not load a provider adapter.
- `agenova --backend --version` exits 2 with `flag --backend requires a value`.

## Limitations

- Evidence was captured on Linux with Go 1.22.2. The smoke test selects `agenova.exe` when `GOOS=windows` and cleans the build directory with `t.TempDir()`.
