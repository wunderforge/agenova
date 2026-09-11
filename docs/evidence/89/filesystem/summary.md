# Ticket #89 filesystem-boundary evidence

## Result

- The shared runtime seam now reports one backend-selected, claim-scoped working directory and an explicit filesystem evidence level.
- The reference backend passes reusable model cases for pre-Start denial, task read/write, readable-but-not-writable runtime data, traversal/direct outside denial, replacement freshness, post-termination export denial, and cleanup refusal while termination remains incomplete. These cases are labelled `Simulated`; they do not claim process isolation.
- A separate trusted local fixture successfully initialized and committed a small Git repository, edited it, ran `go test ./...`, exported one bounded regular file, closed its test collector, rejected a later collection attempt, deleted the workspace, and retained only the acknowledged bytes and digest. Its subprocess environment is allowlisted and excludes host credentials/helpers; symbolic links, hard links, escape paths and oversize outputs are rejected. This proves ordinary tool and collector compatibility, not worker termination or isolation.
- The Agent Sandbox adapter reports `Unsupported` until #48 maps the substrate and #51 supplies real worker isolation evidence.

## Commands

All commands completed successfully on 2026-09-10:

```text
go test -count=1 -v ./internal/operator/... ./internal/runtime/...
go test -count=1 ./internal/operator/... ./internal/runtime/...
go test -count=1 ./internal/toolgateway/... ./internal/modelgateway/... ./harness/e2e/...
pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All
```

The local compatibility run records every command, cwd, exit and bounded output plus the resolved Git and Go versions. The 2026-09-10 Windows run used Git `2.54.0.windows.1` and Go `1.26.4`, reported `git_diff_bytes=192`, exported SHA-256 `5501a550dfeaa6aa1cf476e3c06bcb5c3e05d038b4f40402dc43aa6c458b9186`, and recorded `workspace_retained=false`.

## Evidence boundary

The reference model and local fixture are deterministic contract and compatibility evidence only. They do not prove containment against a hostile process or termination of real worker descendants. Ticket #51 owns that proof on the real Agent Sandbox backend using the mandatory cases listed in `work/0089-filesystem-boundary/handoff-0048-0051.md`.
