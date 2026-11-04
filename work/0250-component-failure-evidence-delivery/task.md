# Task: Deliver #199 component-failure evidence (partial)

- Ticket: [#250](https://github.com/wunderforge/agenova/issues/250), evidence-delivery sub-ticket of [#199](https://github.com/wunderforge/agenova/issues/199); #199 stays open.
- Mission: Deliver the #199 reference-kind failure evidence and replayable harness with its version, environment, unproven cells and the P2 cleanup finding, without claiming #199 acceptance.
- Target: `harness/security/`, `docs/evidence/199/`; no product code changes.
- User value: Reviewers can see how installed Work behaves under Provider, worker, controller and control-plane interruption on the tested version, and what remains unproven.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- Parent packet [`../0199-provider-failure/task.md`](../0199-provider-failure/task.md).
- Maintainer decisions on [PR #227](https://github.com/wunderforge/agenova/pull/227) and [#177](https://github.com/wunderforge/agenova/issues/177) (21 October 2026): deliver the report through this sub-ticket; keep #199 open; route the P2 idle-Worker finding to E8; E15 records and re-tests, it does not fix.

## Scope

In scope:

- `docs/evidence/199/full/`: report, matrix and hashed raw evidence on main `22355d5` (kind).
- Harness: component-failure, controller-timeout, Provider stop/recovery and terminal-replay probes.
- Earlier pre-Demo evidence retained as historical.

Out of scope:

- Product fixes, including the E8 idle-Worker finding.
- Re-testing after #246 (dedicated cold governed workers, stricter cleanup confirmation) or on the final Demo version; this continues under #199.
- Optional member-mode PostgreSQL durability and EKS.

## Acceptance Criteria

- `docs/evidence/199/full/summary.md` lists each failure cell's result with its commit (`22355d5`) and environment (kind).
- Unproven cells and the P2 finding are explicit and never reported as passes.
- #199 remains open and links this ticket.

## Negative Case

- Work submitted while the Provider is stopped is never reported as Succeeded, and the independent Provider count does not increase.

## Quality Gates

- `go test ./harness/security/terminalprobe`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

- PR #227 gates pass; `docs/evidence/199/full/` report, matrix and hash manifest are complete.

## Constraints

- Preserve `docs/product/architecture-contract.md`; do not broaden #199 or its acceptance criteria.
- E15 reports and re-tests only; it does not fix product behavior.
