# Technical Design: Prove opt-in controlled worker start and stop on kind

- Ticket: [#135](https://github.com/wunderforge/agenova/issues/135)
- Feature spec: `spec.md`

## Current State and Constraints

- Agent Sandbox v0.4.6 exposes readiness and deletion but no Agenova work-start or worker-stop acknowledgement.
- #48 froze ordinary Start/Terminate as unsupported. #51 still requires broader filesystem and isolation evidence.
- The protocol is process-local and uses `kubectl exec`; it must not weaken the shared claim ID contract or the normal integration gate.

## Decision

Wrap `SpikeAdapter` with an opt-in `ControlledAdapter`. Map the full system-issued claim ID to `sha256:<hex>` for the worker protocol, retain the original claim at the adapter boundary, and require exact start/status/stop acknowledgements. A condition variable serializes in-flight Start and Terminate calls; Cleanup refuses any unconfirmed work state. Tag the real test with `integration && controlled` and compile that tag in the repository baseline.

## Ownership and Contract Boundaries

- `internal/runtime/agentsandbox`: owns protocol state and worker control; no shared RuntimeBackend change.
- `harness/integration/agentsandbox/testworker`: disposable single-claim worker only.
- `harness/integration/agentsandbox/controlled_run_test.go`: opt-in real-kind proof.
- backend notes distinguish ordinary unsupported behavior from the controlled extension.

## Alternatives Considered

- Treat Ready as Start or deletion as Terminate: rejected because neither proves work execution.
- Pass the raw claim ID: rejected because line/argv validation would narrow the shared contract.
- Include the test in the normal integration tag: rejected because ordinary environments do not have its image.

## Verification Strategy

- Unit tests cover arbitrary claim IDs, stale identity, pre-start cancellation, concurrent Start/Terminate, concurrent Terminate, uncertain acknowledgements and cleanup ordering.
- Compile both ordinary and controlled integration variants in the baseline.
- Run the controlled test on an explicit kind context and retain the transcript.

## Risks and Compatibility

- A SHA-256 token binds protocol acknowledgements without revealing or constraining raw claim text; collision risk is negligible for this test slice.
- State is process-local and kubectl acknowledgements are not production workload identity. Documentation keeps those limitations explicit.

