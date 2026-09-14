# Technical Design: Agent Sandbox v0.4.6 mapping

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- Feature spec: `spec.md`

PR #105 supplies a provisional map and reproducible kind spike. The adapter currently translates upstream SandboxClaim creation/status/deletion into Allocate/Observe/Cleanup, holds identity in memory, and returns ErrUnsupported for Start/Terminate. Use the report as baseline; update any classification only with implementation plus runtime evidence. `internal/runtime/backend.go` owns neutral semantics; `internal/runtime/agentsandbox` owns upstream translation; the run service owns application outcome. Reject Pod Ready as work start and delete-request as worker stop. Focused Go tests and kind observations guard the mapping. Warm-pool reuse and volatile maps remain explicit risks.
