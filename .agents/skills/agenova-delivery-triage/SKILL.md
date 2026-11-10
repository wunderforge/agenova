---
name: agenova-delivery-triage
description: Run Agenova's bounded daily GitHub delivery loop across Issues, PRs, Actions, and the Delivery Board, with evidence-based review and an explicit product-scope interruption gate.
---

# Agenova delivery triage

Use for a scheduled or manual pass over active Agenova delivery. This is an operating loop, not permission to change the product direction.

## Source of truth and scope gate

Read `AGENTS.md`, `docs/product/prd.md`, and `docs/development/AIDLC.md` first. For each actionable item, read its Issue and linked `work/<issue>-<slug>/task.md`, then only the relevant spec/design, architecture sections, code, and tests. GitHub owns mutable status; `docs/project-status.md` describes merged behavior, not planned work.

Before changing a ticket, PR, or code, check the proposed outcome against the PRD and architecture boundary. If it changes target user, committed scope, requirement, authority semantics, acceptance scenario, or product design, **interrupt that item**: record the exact conflict and a small set of options, ask the product Owner to approve one, and resume only after a decision. Continue unrelated in-scope work. Do not silently alter the PRD or use a new ticket to launder a scope change.

## One pass

1. Read the current Delivery Board, open Issues and PRs, recent merges, and Actions. Compare with the last pass; skip unchanged, non-actionable items. Prioritize the next demo's critical path without hiding other blockers.
2. For each changed PR, compare the diff with its Ticket and approved task packet. Check positive and negative behavior, named evidence, CI on the latest head, unresolved review findings, and dependencies. A green check is necessary, not sufficient.
3. Leave a concise, actionable review when something is wrong. Approve only when the work and evidence meet the ticket; never approve your own PR. Keep planning-only and incomplete draft PRs in draft. Merge only when ready, current checks pass, required independent approval exists, comments are resolved, and the branch is mergeable. Do not bypass protection merely to clear the queue.
4. After a merge, verify the linked Issue and Board status, note newly unblocked successors, and ask the appropriate owner for the next bounded step. Reconcile obvious status drift. Do not mark an unverified deliverable Done.
5. Search for an existing Issue before creating one. A new Issue needs a specific gap, PRD fit, acceptance evidence, dependency, and appropriate Board placement. Product-scope questions require Owner approval before adding to the MVP path.
6. End with a short report: merged/done, reviews or changes requested, newly unblocked work, remaining blockers, and Owner decisions needed. Give links and exact evidence. If nothing meaningful changed, stay quiet.

## Operational safety

Preserve dirty worktrees and use a separate branch/worktree for edits. Never inspect secret-bearing files. Do not infer work from assignees or board columns alone; verify code, comments, CI, and merge state. Do not manufacture an approval, dismiss a still-relevant review, or claim a runtime capability from mocks. Use the repo's `scripts/check.ps1` for scoped changes and report the actual checks run.
