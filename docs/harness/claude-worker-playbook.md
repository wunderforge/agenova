# Claude Worker Playbook

Claude Code workers execute bounded tasks. Codex controls task design, integration, and acceptance.

## Worker Packet

Every worker starts from a packet:

```md
# Task

Mission: <one sentence>
Branch or worktree: <worker branch>
Target: <files/modules>
User value: <why it matters>

Acceptance criteria:
- <observable outcome>

Evidence required:
- <exact command or artifact>

Constraints:
- Preserve RuntimeBackend boundary.
- Keep application-facing APIs backend-neutral.
- Do not merge to main.
- Do not broaden scope without reporting a blocker.
- Do not mark Agent Sandbox behavior verified without upstream docs, install output, or runtime evidence.

Report:
- Changed files
- Evidence commands and results
- Risks or blockers
- Suggested next task
```

## Execution Rules

- Read `AGENTS.md`, this playbook, and only the task-relevant docs.
- Keep patches small enough for Codex review.
- Prefer existing project patterns over new abstractions.
- Add tests before or with behavior changes.
- Do not install upstream dependencies silently. Record install commands and output.
- Do not mark Agent Sandbox behavior verified without real upstream docs, install output, or runtime evidence.
- For parallel work, stay inside the assigned package, doc area, or evidence gate. Report overlap before editing.

## Handoff Format

Use this report shape:

```md
## Summary
<short summary>

## Evidence
- `<command>`: pass/fail and important output path

## Files
- `<path>`: <what changed>

## Risks
- <remaining risk or none>
```

## Cross-Validation

For high-risk changes, Codex may ask a second worker for review. The reviewer should focus on:

- Backend-neutral API leakage.
- Claim lifecycle semantic drift.
- Missing negative tests.
- Overbuilt abstractions.
- Evidence gaps.
