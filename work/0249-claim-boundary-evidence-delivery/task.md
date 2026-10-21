# Task: Deliver #198 claim-boundary evidence (partial)

- Ticket: [#249](https://github.com/wunderforge/agenova/issues/249), evidence-delivery sub-ticket of [#198](https://github.com/wunderforge/agenova/issues/198); #198 stays open.
- Mission: Deliver the #198 kind evidence and replayable harness with each result's commit, environment and origin, without claiming #198 acceptance.
- Target: `harness/security/`, `docs/evidence/198/`; no product code changes.
- User value: Reviewers see what each #177 Demo group currently proves, on which version, and what is still missing.
- PRD outcome: [4. Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [5. Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- Parent packet [`../0198-claim-escape-denial/task.md`](../0198-claim-escape-denial/task.md) with its spec and design.
- Maintainer decisions on [#177](https://github.com/wunderforge/agenova/issues/177) (21 October 2026): sequential Works are acceptable for the Demo; the concurrency acceptance stays open; terminal refusal without a record is tracked in [#248](https://github.com/wunderforge/agenova/issues/248); partial evidence is delivered through this sub-ticket.

## Scope

In scope:

- Round 1 (`ae24a67`) evidence, retained as the historical baseline: `docs/evidence/198/summary.md`, `supplement/`.
- Round 2 (`d2b3d90`, kind) evidence organized by the three #177 groups: `docs/evidence/198/r2/`.
- Harness used by both rounds: scripted worker (workload tokens, `foreign-scope`, bundled reference worker), boundary probes and campaign drivers.

Out of scope:

- Product fixes (#248 and other follow-ups).
- Re-testing on the final Demo version or environment; this continues under #198.
- Same-service concurrent-claim acceptance.

## Acceptance Criteria

- `docs/evidence/198/r2/summary.md` lists results and gaps per group. Each result names its commit, environment and origin (A1 scripted or A2 real model).
- Each result maps to a Work ID, requested/effective authority, Policy version and independent oracles (Ollama count, MCP receipts, allocation watches).
- #198 remains open and links this ticket.

## Negative Case

- A refusal before the Gateway, or an out-of-scope action the model never attempted, is never reported as a Gateway Deny or as an intercepted model attack.

## Quality Gates

- `go test -race -count=1 ./harness/security/...`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

- PR #221 gates pass; `docs/evidence/198/` reports and raw files are complete and contain no token values.

## Constraints

- Preserve `docs/product/architecture-contract.md`; do not broaden #198 or its acceptance criteria.
- E15 reports and re-tests only; it does not fix product behavior.
