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

Status: scaffold/research target. No upstream install, CRD install, or behavior verification has been performed.
The adapter path is reserved at `internal/runtime/agentsandbox` (allowed by `Test-RuntimeBoundary`).
Any future adapter must pass the same `contracttest.Run` suite without changing application-facing APIs.

## Next Verification

Validate whether Kubernetes SIG Apps Agent Sandbox acquisition semantics can represent an Agenova `SandboxClaim` as one agent worker run / sandbox execution lease while preserving separate terminal claim outcome and sandbox cleanup evidence.

