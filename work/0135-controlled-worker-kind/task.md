# Task: Prove opt-in controlled worker start and stop on kind

- Ticket: [#135](https://github.com/wunderforge/agenova/issues/135)
- Mission: Prove one opt-in, claim-bound work start/stop protocol on Agent Sandbox without treating ordinary readiness or deletion as those semantics.
- Target: `internal/runtime/agentsandbox/controlled.go`, the disposable test worker, the opt-in kind test, backend documentation, and retained #51 evidence.
- User value: A compatible worker can demonstrate actual work acknowledgement and confirmed stop on kind while unsupported ordinary images still fail explicitly.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature specification](spec.md) and [technical design](design.md)
- [Architecture backend neutrality](../../docs/product/architecture-contract.md#backend-neutrality)
- [RuntimeBackend contract](../../internal/runtime/backend.go)
- [Frozen Agent Sandbox mapping](../../docs/backends/agent-sandbox.md)
- [Parent #51 task packet](../0051-kind-runtime-proof/task.md)

## Scope

In scope:

- A `ControlledAdapter` that activates only for a protocol-conforming image.
- Fixed claim-control tokens that accept every system-issued claim ID.
- Deterministic Start, Terminate and Cleanup serialization.
- A separately tagged real-kind test and reproducible evidence.

Out of scope:

- Completing #51 filesystem/isolation acceptance, production workload identity, restart durability, arbitrary-image support, or a shared-contract change.

## Acceptance Criteria

- A Ready controlled worker returns a claim-bound start/result acknowledgement, then confirms child-process stop before Cleanup reports release.
- Concurrent Start/Terminate and Terminate/Terminate calls cannot leave work running or duplicate stop.
- The normal Agent Sandbox integration gate does not require the disposable controlled-worker image.

## Negative Case

- Ordinary images retain `ErrUnsupported`; unknown identities, stale bindings, uncertain start/stop and cleanup-before-stop fail closed.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm the bounded corrective scope with the Owner and use Codex review findings as the independent review input.
- [x] Add the opt-in adapter and disposable worker without changing ordinary adapter behavior.
- [x] Encode arbitrary shared-contract claim IDs into fixed worker-control tokens.
- [x] Serialize Start/Terminate/Cleanup and separate the controlled integration tag.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 ./internal/runtime/agentsandbox/... ./harness/integration/agentsandbox/testworker`
- `go test -run '^$' -tags 'integration controlled' ./harness/integration/agentsandbox/`
- `go test -count=1 -v -tags 'integration controlled' -timeout 5m ./harness/integration/agentsandbox/ -run '^TestControlledRuntimeBackend_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace default`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused test output covering claim tokens, concurrency, uncertain results and cleanup.
- A real-kind transcript showing correlated identity, actual child result, confirmed stop and release.
- Full repository and CI gates.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep the controlled protocol opt-in and adapter-held; never promote it to upstream-native or arbitrary-image support.

## Decisions and Blockers

- #48 is complete and freezes the ordinary adapter mapping. This ticket owns only #134's controlled-worker slice and advances, but does not close, #51.
- The Owner authorized completing #134 from Codex review on 2026-09-15. Two Codex review rounds produced ten findings; all define corrective acceptance cases within this packet.
- Focused tests, both integration compile variants, and `./scripts/check.ps1 -All` pass locally. A fresh real-kind transcript remains the publication gate.
- The final fresh real-kind run passed against the code committed as `8d4eee7`; `docs/evidence/51/kind-run.md` records the correlated worker, control token, child result, confirmed stop, and release.

