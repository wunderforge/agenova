# Task: Consolidate the AIDLC source of truth and repository harness

- Ticket: [#71](https://github.com/wunderforge/agenova/issues/71)
- Mission: Give ten contributors and their coding agents one efficient, reviewable path from an existing Ticket to verified implementation.
- Target: Agent routing, AIDLC collaboration docs, task-packet templates, scaffolding, and delivery checks.
- User value: Contributors can start independent work without private prompting or competing sources of truth.
- PRD outcome: [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [AIDLC collaboration contract](../../docs/development/AIDLC.md)
- [Quality gates](../../docs/harness/quality-gates.md)
- [Harness playbooks](../../docs/harness/playbooks.md)
- `scripts/checks/docs.ps1` and `scripts/checks/contracts.ps1`

## Scope

In scope:

- Separate human/public documentation from minimal coding-agent context.
- Define Ticket and task-packet ownership without duplicating Project state.
- Provide Task, Spec, and Design templates plus deterministic scaffolding.
- Enforce routing, template structure, Issue linkage, and no-overwrite behavior.

Out of scope:

- Changing Agenova product semantics.
- Installing a vendor-specific AIDLC framework or copying external skills into the repository.
- Creating task packets for every existing Ticket in this PR.

## Acceptance Criteria

- `docs/development/AIDLC.md` defines sources of truth, roles, context loading, execution, review, and PRD update rules.
- `AGENTS.md` routes implementation work through PRD plus the active task packet without requiring README or the complete documentation set.
- A contributor can create a correctly named Task, Task + Spec, or Task + Spec + Design packet from canonical templates.
- GitHub remains authoritative for owner, priority, dependency, status, and accepted evidence.
- Documentation and full repository checks pass.

## Negative Case

- The scaffolder rejects an existing packet instead of overwriting it.
- Documentation checks reject missing AIDLC routing, required template sections, unresolved scaffold tokens, or Project fields copied into the task template.

## Execution Todo

- [x] Research official AI-DLC, spec-driven, and agent-instruction practices.
- [x] Audit the retained Agenova Markdown and deterministic checks.
- [x] Confirm the Ticket-to-task-packet model with the project lead.
- [x] Add the AIDLC contract, work convention, and task/spec/design templates.
- [x] Optimize `AGENTS.md` for minimal execution context.
- [x] Add and mechanically test the task scaffolder.
- [x] Run the focused gate and `./scripts/check.ps1 -All` after final review edits.
- [ ] Update PR #73 and Issue #71 with final evidence.

## Quality Gates

- `.\scripts\check.ps1 -Docs`
- `.\scripts\check.ps1 -All`
- GitHub `CI / baseline`

## Evidence Required

- Generated three-file packet with correct Issue link and no unresolved template tokens.
- Demonstrated overwrite rejection.
- Passing local Docs and All profiles plus passing PR CI.

## Constraints

- Preserve `docs/product/architecture-contract.md` and all existing product behavior.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep external skill text outside the repository; retain only Agenova-specific routing and enforceable behavior.

## Decisions and Blockers

- Decision: every implementation Ticket gets `task.md`; spec and design remain adaptive.
- Decision: `AGENTS.md + PRD + active task.md` is the default Agent context.
- Decision: GitHub tracks team delivery; the packet tracks Agent execution.
- Blockers: none.
