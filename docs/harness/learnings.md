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

- Define one authority per question, keep delivery state in GitHub, use feature specs only when ambiguity or parallel dependencies justify them, and test routing/structure instead of repeated prose.

## Escalation Rule

- First occurrence: record a concise learning.
- Second occurrence: add or update a gotcha/playbook.
- Third occurrence: add a mechanical check, fixture, or stronger evidence gate when practical.
