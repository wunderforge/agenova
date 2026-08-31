# Evidence Summary

- Ticket: [#40](https://github.com/wunderforge/agenova/issues/40)
- Gate: cli-smoke
- Date: 2026-08-30
- Branch: main
- Command: `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova`
- Binary: `go build -o .tmp/agenova ./cmd/agenova` then `--help`, `version`, `not-a-command`, `--backend kubernetes version`
- Result: pass

Raw output: `output.txt` (focused tests) and `binary-output.txt` (executable smoke).

## Observed behavior

- `agenova --help` exits 0 and prints usage.
- `agenova version` exits 0 and reports `runtime-backend: memory`.
- `agenova not-a-command` exits 2 with actionable stderr.
- `agenova --backend kubernetes version` exits 2 and does not load a provider adapter.

## Limitations

- Evidence was captured on Linux with Go 1.22.2. `scripts/evidence.ps1` could not wrap the command here because it invokes Windows `powershell` rather than `pwsh`.
