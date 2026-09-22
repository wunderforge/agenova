# Task: Make pre-implementation task-packet approval advisory

- Ticket: [#184](https://github.com/wunderforge/agenova/issues/184)
- Mission: Make pre-implementation task-packet approval advisory while preserving Ticket scope, authority conflict stops, evidence, and independent merge review.
- Target: `AGENTS.md`, `docs/development/AIDLC.md`, `docs/harness/playbooks.md`, and `scripts/checks/docs.ps1`
- User value: Contributors may start bounded local implementation without waiting for a separate approval comment, while reviewers retain clear merge accountability.
- PRD outcome: [`Demonstrable contributor path`](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`docs/development/AIDLC.md`](../../docs/development/AIDLC.md)
- [`docs/harness/playbooks.md`](../../docs/harness/playbooks.md)
- [`scripts/checks/docs.ps1`](../../scripts/checks/docs.ps1)

## Scope

In scope:

- Replace mandatory pre-implementation Owner/Reviewer approval wording with an explicit recommendation.
- Allow local implementation after the Ticket and complete task packet exist, marking unreviewed packets where applicable.
- Keep maintainer decisions mandatory for PRD/architecture conflicts and keep independent review required before merge.
- Update mechanical text checks to enforce the revised guardrails.

Out of scope:

- Removing the Ticket/task-packet requirement, acceptance criteria, negative cases, evidence, or PR review.
- Relaxing product, architecture, authority, or backend-neutrality boundaries.
- Changing application behavior.

## Acceptance Criteria

- Repository instructions consistently describe pre-implementation Owner/Reviewer approval as recommended, not blocking.
- An agent may begin bounded local implementation from a complete Ticket/task packet and identifies an unreviewed packet in its status/reporting.
- PRD or architecture conflicts still stop for a maintainer decision.
- Independent review and exact evidence remain required before merge.
- Documentation contract checks enforce these distinctions.

## Negative Case

- Mechanical checks reject wording that permits implementation without a Ticket/task packet, bypasses maintainer decisions, or removes independent merge review.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation.
- [x] Update the workflow authorities, task guidance, and scaffold output with consistent advisory language and unchanged hard boundaries.
- [x] Update documentation contract checks for the revised invariants.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All` (full gate reached one unrelated frontend smoke failure; see blocker below and PR CI).
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `pwsh ./scripts/check.ps1 -Docs`
- `pwsh ./scripts/check.ps1 -All`

## Evidence Required

- Diff showing consistent advisory language across workflow authorities.
- Passing documentation contract output and full repository gate output.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- This Ticket changes only the timing of task-packet review; it must preserve all product/architecture stop conditions and merge-review requirements.
- Do not include #183 macOS bootstrap implementation in this branch or PR.

## Decisions and Blockers

- Owner approval recorded in [#184](https://github.com/wunderforge/agenova/issues/184#issuecomment-5770707798).
- Decision: pre-implementation review is advisory; a complete Ticket/task packet, maintainer stop for authority conflicts, exact evidence, and independent merge review remain mandatory.
- Local `pwsh ./scripts/check.ps1 -Docs` passes. Local `-All` passed docs, Go, contracts, types, components, and build; Playwright finished 51/52 with the existing keyboard/loading smoke failing again under unsupported local Node 25.9.0. The PR's Ubuntu/Node 24 CI is the authoritative full-gate result; no UI code is changed by this Ticket.
