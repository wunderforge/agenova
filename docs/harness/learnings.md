# Harness Learnings

Add an entry only when a failure or repeated steering should change future work.

## 2026-08-12 — Shared-repo cleanup

What slowed work:

- Phase documents, progress logs, worker packets, and product notes described different repository states.
- The check script enforced historical documents instead of current user value.

Root cause:

- Harness design drift.

Harness amendment:

- Route all work through one PRD, architecture contract, current-status file, compact task template, and current quality gates.
- Keep only current accepted evidence or an explicit blocker.

## Escalation Rule

- First occurrence: record a concise learning.
- Second occurrence: add or update a gotcha/playbook.
- Third occurrence: add a mechanical check, fixture, or stronger evidence gate when practical.
