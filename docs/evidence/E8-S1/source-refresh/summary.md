# E8-S1 source refresh evidence

Date: 2026-09-12.

- Task: [#66 mapping spike](../../../../work/0066-agent-sandbox-mapping/task.md).
- Report: [provisional mapping](../../../backends/agent-sandbox-v0.4.6-mapping.md).
- Merged source baseline: `0ee9042c2f6a11b722d0b5875e28f652dcfd2caf` (#30, #89 and #124).
- Scope: source-based reconciliation only. No runtime code, shared API or cluster manifests changed. The PR remains a draft and the real-backend negative case is outstanding until #99 merges.
- Source review: `internal/runtime/backend.go`, `internal/runtime/agentsandbox/allocation.go`, `allocation_test.go`, the shared contract suite, and the #89 handoff/evidence were checked against the report. Start/Terminate return ErrUnsupported; Cleanup confirms both resource absences; filesystem evidence remains Unsupported.

## Validation

Tools: PowerShell 7.4.7, Go 1.22.12 (matching the repository Go 1.22 baseline), Node 24.19.0. PowerShell/Go/npm were staged under ignored `.tmp/toolchain`; Go caches, npm cache and Playwright browsers also use `.tmp`.

Run from the repository root with those executables on PATH, GOPATH set to `.tmp/gopath`, GOCACHE to `.tmp/gocache`, and PLAYWRIGHT_BROWSERS_PATH to `.tmp/playwright` (absolute paths).

| Check | Exact command | Result |
| --- | --- | --- |
| Documentation | `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -Docs` | Passed; [raw output](docs-output.txt). |
| Existing runtime/reference cases | `go test -count=1 -v ./internal/operator/... ./internal/runtime/...` | Passed; [raw output](focused-output.txt). |
| Full repository baseline | `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All` | Passed; [raw output](baseline-output.txt), including 7 browser smoke cases. |
| Whitespace | `git diff --check` | Passed. |

Initial baseline attempts stopped at dependency download because the sandbox could not resolve proxy.golang.org. Dependencies were then downloaded with network authorization. This is an environment/setup failure, not a passing gate.

[Source hashes](source-sha256.txt) pin the reviewed contract, adapter, tests and filesystem handoff. The existing readiness negative test asserts Start and Terminate return ErrUnsupported for a Ready allocation; cleanup tests check release and identity failures. These are local simulated-controller tests, not the real-cluster reproduction required to complete #66. No new test is introduced for this documentation-only refresh.
