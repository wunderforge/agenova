# Task: Daily delivery triage skill

- Ticket: [#126](https://github.com/wunderforge/agenova/issues/126)
- Mission: Make the recurring Agenova delivery review executable without autonomous product-scope changes.
- Target: `.agents/skills/agenova-delivery-triage/SKILL.md` and the thread's nightly automation.
- User value: The team receives a checked delivery queue and explicit questions when a scope decision is needed.
- PRD outcome: [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [AIDLC source of truth and review contract](../../docs/development/AIDLC.md)
- [Delivery playbooks](../../docs/harness/playbooks.md)

## Scope

In scope:

- Project-local skill for one pass over Issues, PRs, Actions, and Board status.
- Evidence-based review, merge, and post-merge reconciliation with a PRD scope-interruption gate.
- Nightly local thread automation invoking the skill.

Out of scope:

- Independent product decisions, automatic approval of own PRs, or protection bypass.
- New application behavior, product requirements, or a second project board.

## Acceptance Criteria

- A future pass can identify actionable changes, review against the linked task packet, and report exact evidence.
- A product-scope conflict pauses only the affected item pending Owner approval.
- One nightly Australia/Sydney automation is active and avoids duplicate schedules.

## Negative Case

- A PR proposing a new MVP outcome must not be merged, reticketed into scope, or used to edit the PRD without the Owner's decision.

## Execution Todo

- [x] Scout repository context, PR conventions, and existing automations.
- [x] Confirm scope-interruption requirement with the Owner in this thread.
- [x] Add a short project skill; do not duplicate mutable GitHub state.
- [ ] Create and verify one nightly automation.
- [x] Run the focused documentation gate.
- [x] Run `./scripts/check.ps1 -All` and review the diff; Go checks passed, frontend gate is blocked by missing local npm/Chromium dependencies.

## Quality Gates

- `./scripts/check.ps1 -Docs`
- `./scripts/check.ps1 -All`

## Evidence Required

- The skill file and validation result.
- Exact check output and automation confirmation.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Do not store credentials or mutable Issue/Board snapshots in the skill.

## Decisions and Blockers

- Owner explicitly requested interruption-and-resume for product, requirement, scope, and direction decisions before this implementation.
- `-Docs` passed. `-All` passed Go tests and Agent Sandbox integration compile after rerunning with Git access, then stopped because this worktree lacks installed frontend dependencies/Chromium. PR CI must run the full baseline.
