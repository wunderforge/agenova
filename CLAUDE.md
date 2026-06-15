# CLAUDE.md

Use `AGENTS.md` as the primary agent context. Keep loaded context focused on the current phase.

## Preferred Load Order

1. `README.md`
2. `docs/product/purpose.md`
3. `docs/product/architecture-contract.md`
4. `docs/phases/phase-0-foundation-alpha/README.md`
5. `docs/phases/phase-0-foundation-alpha/spec.md`
6. `harness/phase-0-foundation-alpha/README.md`
7. The smallest code files needed for the task

## Non-Default Planning Context

`docs/product/roadmap.md` and `docs/human-design-decisions/` are intentionally tracked but excluded from default context. Load them only when the task asks for planning, product positioning, core-subject rationale, or architecture philosophy.

## Phase Discipline

Phase 0 is a local foundation alpha. Do not infer later-phase requirements from archived design notes.

## Worker Discipline

When running as a Claude Code worker, read `docs/harness/claude-worker-playbook.md` before editing. Keep changes scoped to the task packet, preserve the RuntimeBackend boundary, and report exact evidence commands plus results. Do not merge to `main`.
