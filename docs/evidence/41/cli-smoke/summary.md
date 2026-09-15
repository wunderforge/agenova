# Evidence Summary

- Ticket: [#41](https://github.com/wunderforge/agenova/issues/41)
- Gate: cli-smoke
- Date: 2026-09-15
- Branch: `cursor/e6-t2-run-submission` (takeover update)
- Command: `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova`
- Result: pass

Executable output: `binary-output.txt`. Focused test output is reproduced by the command above and by PR CI.

## Observed behavior

- Team A `run -f` on the payment-timeout YAML exits 0, issues one claim, allocates once, and reports `phase: Succeeded`.
- Team B (`AGENOVA_LOCAL_PRINCIPAL=team-b`) exits 1 with `decision: Deny` and `allocated: false`.
- Secret-value, self-asserted principal, missing template, and malformed YAML fail before allocation (allocation spy stays at 0).
- `agenova run` without `-f`, `run -f --help` as a missing value, unknown `--backend`, and `--repo` exit 2.
- The CLI reports the system-issued claim ID and backend-neutral terminal phase.

## Limitations

- This reference path uses the memory backend and a deterministic no-op work callback. The controlled Kubernetes composition is verified separately.
