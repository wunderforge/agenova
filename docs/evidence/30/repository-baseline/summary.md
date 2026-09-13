# Ticket #30 final repository evidence

- Task: [RuntimeBackend MVP](../../../../work/0030-runtime-backend-mvp/task.md)
- Date: 2026-09-10
- Branch: `tomtian/e3-t1-runtime-backend-mvp`
- Baseline: `e2e8af43fa624544ed3239e1b4994c160d2cead2` plus the wrapper's explicit-context fix. All PR Go sources remain unchanged and match the [Go manifest](../reference-contract/source-sha256.txt); the wrapper and its regression script match the [script manifest](source-sha256.txt).

| Gate | Exact command | Result | Output |
| --- | --- | --- | --- |
| Build | `go build ./...` | exit 0, no output | — |
| Repository | `pwsh -File scripts/check.ps1 -All` | exit 0, 13 checks pass | [output.txt](output.txt) |
| Race | `go test -count=1 -race ./...` | exit 0 | [race-output.txt](race-output.txt) |
| Consumers | `go test -count=1 -v ./internal/app/... ./internal/cli/... ./internal/toolgateway/... ./internal/modelgateway/... ./harness/e2e/...` | exit 0 | [consumers-output.txt](consumers-output.txt) |
| Integration compile | included in `pwsh -File scripts/check.ps1 -All` | pass; no cluster tests run | [excerpt from repository output](integration-compile-output.txt) |
| Explicit-context guard (retained from Slice 2) | `go test -count=1 -tags integration -run '^TestRuntimeBackend_AllocateObserveCleanup$' ./harness/integration/agentsandbox/` | expected exit 1 before any cluster call; rejection verified | [guard output](integration-context-guard-output.txt) |
| Wrapper context guard | `pwsh -NoProfile -File scripts/tests/check-context.ps1` | exit 0, 9 cases pass | [wrapper output](wrapper-context-guard-output.txt) |

Build, focused groups and race were run after the cleanup fix on the unchanged Go sources. The wrapper regression and repository gate were run again after removing the wrapper's context default. The Go context-guard output is the earlier accepted run at `91977b8`; it proves only the unchanged Go preflight, not wrapper behavior. Contract output is recorded [separately](../reference-contract/summary.md).

The wrapper regression copies the actual entry point into a temporary directory and replaces its check modules with recording doubles. For both `-Integration` and `-Profile Backend`, omission, empty strings and whitespace reject before any recorded check; explicit context/namespace values are forwarded unchanged. Ordinary `-All` still works without a cluster. The new regression rejected the old wrapper because `Integration/omitted` incorrectly exited 0. All nine cases pass after the fix; no Go or kubectl commands run inside these isolated cases.

A passing compile or context guard is not an integration pass. The current environment has no kubectl and no confirmed test context; [real-backend verification remains blocked](../agent-sandbox/summary.md). The [technical review and #31 handoff](../../../../work/0030-runtime-backend-mvp/review.md) are complete; independent human review and acceptance remain outstanding.
