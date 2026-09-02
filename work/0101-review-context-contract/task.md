# Task: Define durable context for automatic PR review

- Ticket: [#101](https://github.com/wunderforge/agenova/issues/101)
- Mission: Make PR review context explicit and mechanically complete so automatic and human reviewers start from the same durable project sources.
- Target: `AGENTS.md`, PR template/validator fixtures, and `docs/development/AIDLC.md`.
- User value: A contributor can open a reviewable PR that tells reviewers which task, contract, fixtures, and evidence actually govern the change.
- PRD outcome: [Required Outcome 8 — Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [#101 decision comment](https://github.com/wunderforge/agenova/issues/101#issuecomment-5507625261)
- `docs/development/AIDLC.md` — source-of-truth and review responsibilities
- `.github/pull_request_template.md` and `scripts/check-pr-body.ps1` — existing PR delivery contract
- `harness/fixtures/contracts/pr-valid.md` — valid PR-body fixture

## Scope

In scope:

- Declare the durable review-context chain and contract-sensitive review rules.
- Require a task packet, optional one-off review focus, changed boundary, deferred non-goal, and evidence in a PR body.
- Validate the new PR-body fields through the existing deterministic PR contract check.
- Explain the CI / automatic review / human review responsibility split in AIDLC.

Out of scope:

- Enabling or implementing a GitHub Action with an OpenAI API key.
- AI auto-approval, auto-merge, or replacing human product/architecture review.
- Rewriting product contracts or adding a second process handbook.

## Acceptance Criteria

- A PR template routes reviewers to the active task packet without duplicating the detailed context owned there; it allows an optional one-off review focus.
- The PR-body validator rejects missing review-context or scope/deferral fields; its valid fixture proves the accepted format.
- `AGENTS.md` gives a concise, stable checklist for contract-sensitive review and defers mechanical checks to CI.
- AIDLC explains the distinct responsibilities of CI, automatic review, and human approval.

## Negative Case

- A PR body that omits a required review-context or scope/deferral field is rejected by `scripts/check-pr-body.ps1`.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm the final approach in the #101 decision comment with the Owner.
- [x] Add the smallest review rules, PR template fields, validator checks, fixture, and AIDLC pointer.
- [x] Run the PR-body contract check and the repository baseline.
- [x] Review the diff for process duplication, scope drift, and current-source accuracy.

## Quality Gates

- `.\scripts\check-pr-body.ps1 -BodyPath harness/fixtures/contracts/pr-valid.md`
- `.\scripts\check.ps1 -All`

## Evidence Required

- `./scripts/check-pr-body.ps1 -BodyPath harness/fixtures/contracts/pr-valid.md` — pass (2026-09-02).
- `./scripts/check.ps1 -All` — pass (2026-09-02); includes negative assertions for an omitted task packet and deferred non-goal.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep review guidance short, stable, and scoped to non-mechanical review concerns; CI remains the source of deterministic enforcement.

## Decisions and Blockers

- The Owner approved implementation through the #101 decision comment and follow-up request on 2026-09-02.
