# Task: Freeze the Agent Sandbox v0.4.6 mapping

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- Mission: Turn the #66 substrate spike into an explicit, evidence-backed map from each neutral RuntimeBackend operation to upstream v0.4.6 behavior or an honest gap.
- Target: `docs/backends/agent-sandbox.md` and the version-pinned mapping note.
- User value: Integrators can tell what actually works without inferring execution from infrastructure readiness.
- PRD outcome: `docs/product/prd.md` §3 Backend-neutral execution.

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet, `internal/runtime/backend.go`, #48, and the #66 mapping report.

## Scope and Acceptance

- Classify Allocate, Observe, Start, Terminate, Cleanup, identity, and filesystem evidence as native, translated, adapter-held, or unsupported.
- Record exact upstream resource/condition and gap. Promote a capability only with pinned-version kind evidence.
- Keep application outcome separate from resource release. Ready never implies Running; deletion request never proves worker stop.
- Negative case: missing or mismatched worker cannot be reported Ready or Released.
- Out of scope: changing shared contracts, a second backend, or claiming hostile-process isolation.

## Execution Todo

- [x] Scout contract, adapter, spike and PR #105.
- [x] Record this packet; owner explicitly allowed tonight's direct sprint execution without separate planning approval.
- [x] Consume #66 mapping and reconcile with new kind evidence.
- [x] Run focused tests and `.\scripts\check.ps1 -All`.
- [x] Review final diff and record evidence in #48 and PR #134.

## Quality Gates and Evidence

- `go test ./internal/runtime/agentsandbox/...`
- `.\scripts\check.ps1 -All`
- Version-pinned source/CRD map and kind observations with claim and worker IDs.

## Constraints

Preserve `docs/product/architecture-contract.md`; keep backend-specific types in the adapter; do not call unsupported behavior complete.
