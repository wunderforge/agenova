# #48 Agent Sandbox Supported Mapping Evidence

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- Pull request: [#133](https://github.com/wunderforge/agenova/pull/133)
- Tested revision: `f78b42cb3e9c62743fdabf6437ecffb402fb4967`
- Tested tree: `24276f90f3ba3b8f9ef7c7c78b2f87176338d4fd`
- Test start: `2026-09-15T00:05:51Z`
- Pinned upstream substrate: Agent Sandbox `v0.4.6`

## Accepted mapping

| Semantic | Classification | Evidence boundary |
| --- | --- | --- |
| Allocate | `translated` | Source, simulated controller and retained #66 kind run. Recovery and atomic binding gaps remain. |
| Observe | `translated` | Identity-matched Ready condition is Bound-level evidence only. |
| Backend identity | `translated` | Assigned sandbox name becomes neutral `Backend/WorkerID`; correlation is process-local. |
| Start | `unsupported` | A valid Ready allocation returned `ErrUnsupported` in the retained #66 kind run. |
| Terminate | `unsupported` | The ordinary adapter has no worker-stop acknowledgement distinct from deletion. |
| Cleanup | `translated` | Release requires confirmed absence of both claim and recorded sandbox; replacement is not inferred. |
| Durability/restart | `unsupported` | Allocation, reservation and release correlation are process-local. |
| Filesystem boundary | `unsupported` | The adapter returns `EvidenceLevel=Unsupported` and no asserted directory/outside/retention fields. |
| General isolation | `unknown` | Worker filesystem, process, credential and network probes remain #51 work. |

The complete real-cluster transcript is retained in the [#66 evidence](../../E8-S1/agent-sandbox-mapping/output.txt). It verifies the bounded ordinary-adapter mapping; it is not evidence for the separate #51 controlled-worker protocol.

## Verification

| Command | Exit | Captured output |
| --- | --- | --- |
| `go test -count=1 -v ./internal/runtime/agentsandbox/...` | 0 | [focused-output.txt](focused-output.txt) |
| `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -Docs` | 0 | [docs-output.txt](docs-output.txt) |
| `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All` | 0 | [baseline-output.txt](baseline-output.txt) |

The local full gate sets `GOFLAGS=-buildvcs=false` because Go cannot obtain VCS stamping through the sandbox-owned worktree metadata. This changes build metadata only, not source selection or tests. CI runs the repository baseline without that local workaround.

[Source SHA256 values](source-sha256.txt) pin the shared backend contract, adapter implementation/test, formal mapping, provisional source report, and filesystem handoff used for this classification.
