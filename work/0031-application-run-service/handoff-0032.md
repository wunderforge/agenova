# Run-service handoff to Ticket #32

- Producer: [Ticket #31](https://github.com/wunderforge/agenova/issues/31) and [PR #131](https://github.com/wunderforge/agenova/pull/131).
- Consumer: [Ticket #32](https://github.com/wunderforge/agenova/issues/32).
- Authoritative reader: `(*internal/app.RunService).Claim(claimID string) (v1alpha1.SandboxClaim, bool)`.

## Contract

- The reader returns `Running` while the application work callback executes and the terminal application phase after the outcome is selected.
- Returned claims and backend identities are defensive snapshots; callers cannot mutate authoritative state.
- Unknown claims return `false`.
- Backend readiness is never exposed as claim authority. `Running` is published only after `RuntimeBackend.Start` succeeds.
- Terminal outcome is published before termination and cleanup. Their success or failure is evidence and never rewrites the outcome.

## Ticket #32 integration boundary

Adapt Tool and Model Gateway lifecycle eligibility to this public-claim reader and the separately trusted invocation context. Require the authoritative phase to be exactly `Running`; fail closed for unknown, Pending, Bound, terminal, or context-mismatched claims.

Do not read `runtime.BackendClaim`, infer authority from `RuntimeBackend.Observe`, accept a caller-supplied claim ID as worker identity, or change capability-policy enforcement in #32.

## Evidence

`internal/app/run_service_test.go` proves that the reader exposes Running during work, terminal state after work, defensive copies under concurrent reads, and no Running state after Start failure or deadline expiry.
