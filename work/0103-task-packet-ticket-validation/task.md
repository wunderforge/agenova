# Task: Validate PR task packet matches linked ticket

- Ticket: [#103](https://github.com/wunderforge/agenova/issues/103)
- Mission: Make the PR contract reject task-packet references that are malformed, missing, or belong to a different linked ticket.
- Target: `scripts/checks/contracts.ps1` and `harness/fixtures/contracts/pr-valid.md`.
- User value: Human and automatic reviewers can reliably reach the single task context for every accepted PR.
- PRD outcome: [Required Outcome 8 — Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Reviewer finding on PR #102](https://github.com/wunderforge/agenova/pull/102#discussion_r3913157563)
- `scripts/checks/contracts.ps1` — PR-body contract implementation and negative cases
- `harness/fixtures/contracts/pr-valid.md` — accepted PR-body fixture

## Scope

In scope:

- Require the task-packet value to use `work/<issue>-<slug>/task.md` with a lowercase kebab-case slug.
- Require the path issue number to match the ticket closed by the PR, ignoring leading zeroes.
- Require the referenced task packet to exist in the checked-out repository.
- Cover `None`, malformed path, mismatched ticket, and missing file as deterministic negative cases.

Out of scope:

- External URL resolution or validation.
- Changes to task-packet structure, AIDLC ownership, or product contracts.

## Acceptance Criteria

- A matching, existing canonical task-packet reference passes the PR contract.
- Invalid, mismatched, and missing task-packet references fail with direct errors.

## Negative Case

- A non-empty value such as `None`, another ticket's task path, or a missing canonical path must not pass validation.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation.
- [x] Parse and validate the closing ticket and canonical task-packet reference.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `.\scripts\check-pr-body.ps1 -BodyPath harness/fixtures/contracts/pr-valid.md`
- `.\scripts\check.ps1 -Docs`
- `.\scripts\check.ps1 -All`

## Evidence Required

- `.\scripts\check-pr-body.ps1 -BodyPath harness/fixtures/contracts/pr-valid.md` — pass (2026-09-02), including the embedded invalid-reference cases.
- `.\scripts\check.ps1 -Docs` — pass (2026-09-02).
- `.\scripts\check.ps1 -All` — pass (2026-09-02) with worktree Git metadata access.
- GitHub `baseline` — recorded in the PR because GitHub owns that mutable run status.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep the fix deterministic and inside the existing PR contract validator; do not add LLM or network validation.

## Decisions and Blockers

- The Owner approved this follow-up implementation in the request to raise a new PR on 2026-09-02.
- The restricted local run could not read worktree VCS metadata during the CLI smoke build; the identical full check passed when Git metadata access was allowed.

