# Phase 1 Progress

## Status

Runtime contract evidence loop complete. Agent Sandbox adapter remains a scaffold/research target.

## Completed

- Pivot decision recorded.
- Phase 1 spike scope documented.
- Backend capability matrix scaffolded.
- Harness scenario scaffolded for capability matrix evidence.
- `RuntimeBackend` interface defined in `internal/runtime/backend.go`.
- Reusable contract test suite extracted to `internal/runtime/contracttest/run.go`.
- `internal/operator.Runtime` named as reference backend with compile-time `RuntimeBackend` assertion.
- In-memory reference backend passes all six contract test cases via `contracttest.Run`.
- `check.ps1 -All` static checks updated to verify contract code artifacts.

## Contract Evidence

The following code artifacts form the Phase 1 contract evidence loop:

| Artifact | Path | Role |
| --- | --- | --- |
| `RuntimeBackend` interface | `internal/runtime/backend.go` | Pluggable boundary definition |
| Contract test suite | `internal/runtime/contracttest/run.go` | Reusable backend-neutral tests |
| Reference backend | `internal/operator/runtime.go` | In-memory implementation |
| Compile-time assertion | `internal/operator/doc.go` | Proves `Runtime` satisfies `RuntimeBackend` |
| Contract test wiring | `internal/operator/runtime_test.go` | Proves reference backend passes all tests |

Evidence command: `go test ./...`

## Agent Sandbox Adapter

Status: Phase 2 spike implemented and evidence-backed.

- Agent Sandbox v0.4.6 installed on kind cluster `agenova-k8s-lab`.
- `internal/runtime/agentsandbox` contains the only adapter package allowed to know upstream CRD shapes.
- SpikeAdapter e2e verified `AddTemplate`, `AddWarmPool`, `AddClaim`, `BindClaim`, `StartClaim`, `SucceedClaim`, and `ExpireClaim` against the real controller.
- Confirmed semantic gaps: upstream has no explicit terminal claim phase field, terminal state is adapter-local, and pool status granularity is limited.
- `contracttest.Run` still passes against the in-memory reference backend; full contract parity for Agent Sandbox requires a durable Agenova overlay or upstream CRD change for terminal phases.

## Next Verification

Decide how to close the explicit terminal-phase durability gap before promoting Agent Sandbox from spike backend to production default backend.
