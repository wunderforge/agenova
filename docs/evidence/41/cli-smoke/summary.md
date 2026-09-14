# Evidence Summary

- Ticket: [#41](https://github.com/wunderforge/agenova/issues/41)
- Gate: cli-smoke
- Date: 2026-09-14
- Branch: main
- Commit: f56ad505def151c56721d9420c7151a37000e153
- Command: `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova`
- Result: pass

Raw output: `output.txt` (focused tests) and `binary-output.txt` (executable smoke).

## Observed behavior

- Team A `run -f` on the payment-timeout YAML exits 0 with `decision: Allow` and `allocated: false`.
- Team B (`AGENOVA_LOCAL_PRINCIPAL=team-b`) exits 1 with `decision: Deny` and `allocated: false`.
- Secret-value, self-asserted principal, missing template, and malformed YAML fail before allocation (allocation spy stays at 0).
- `agenova run` without `-f`, `run -f --help` as a missing value, unknown `--backend`, and `--repo` exit 2.
- A handler that reports `allocated: true` is treated as a product error.

## Limitations

- Evidence was captured on Linux with Go 1.22.2. `run -f` admits the request; it does not issue a SandboxClaim or call `RuntimeBackend.Allocate` (#29 / #31 remain open).
