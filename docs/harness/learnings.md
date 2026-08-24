# Harness Learnings

Add an entry only when a failure or repeated steering should change future work.

## 2026-08-12 — Shared-repo cleanup

What slowed work:

- Phase documents, progress logs, worker packets, and product notes described different repository states.
- The check script enforced historical documents instead of current user value.

Root cause:

- Harness design drift.

Harness amendment:

- Route work through the PRD, architecture contract, implementation snapshot, GitHub Issue form, and current quality gates.
- Keep only current accepted evidence or an explicit blocker.

## 2026-08-24 — Document count was not the main problem

What slowed work:

- The repository had already removed most historical documents, but several retained files still answered the same routing, status, and workflow questions.
- Exact-phrase checks encouraged those documents to repeat product language.

Root cause:

- Document ownership was implicit, and task/spec depth was undefined.

Harness amendment:

- Define one authority per question, keep delivery state in GitHub, give every Agent a Ticket-derived task packet, use feature specs only when ambiguity or parallel dependencies justify them, and test routing/structure instead of repeated prose.

## 2026-08-24 — Documentation routing was not execution context

What failed or slowed down:

- A comprehensive source-of-truth map did not tell a coding Agent which minimal files to load or where to keep its Todo.
- Removing the local task template confused a duplicated team backlog with a useful Agent execution contract.

Root cause:

- Human coordination context and Agent execution context were treated as the same layer.

Harness amendment:

- Keep GitHub as the team tracker, require `AGENTS.md + PRD + task.md` as the Agent baseline, and scaffold one reviewable task packet per Ticket.

## 2026-08-25 — Dependency changes confused the full gate

What failed or slowed down:

- `scripts/check.ps1 -All` reported that `go.mod` or `go.sum` needed `go mod tidy` even after tidy had run, because the intended dependency changes were still unstaged and the gate checks their working-tree diff.

Root cause:

- The full Go gate assumes normalized dependency changes are staged, but that precondition is not stated in the playbook or gate output.

Harness amendment:

- On the next harness-maintenance Ticket, either compare module files before and after `go mod tidy` without depending on Git staging state, or explicitly document the staging precondition.

## Escalation Rule

- First occurrence: record a concise learning.
- Second occurrence: add or update a gotcha/playbook.
- Third occurrence: add a mechanical check, fixture, or stronger evidence gate when practical.
