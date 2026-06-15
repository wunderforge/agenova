# Task

Mission: Complete the Phase 1 RuntimeBackend contract evidence loop without expanding beyond the adapter spike.
Branch or worktree: `worker/phase1-runtime-contract`
Target: `internal/runtime`, `internal/operator`, `docs/phases/phase-1-agent-sandbox-adapter-spike`, `harness/phase-1-agent-sandbox-adapter-spike`, `scripts/check.ps1` only if a small evidence gate is needed.
User value: Agenova can prove backend pluggability before any Kubernetes integration is treated as real product behavior.

Acceptance criteria:

- The in-memory runtime is clearly named as the reference backend and passes the reusable RuntimeBackend contract tests.
- Phase 1 docs and progress accurately reflect implemented contract evidence.
- The Agent Sandbox adapter spike remains a scaffold/research target; no fake upstream verification is claimed.
- Any added checks are lightweight and run through `.\scripts\check.ps1 -All`.
- No application-facing API depends on upstream Agent Sandbox CRD shape.

Evidence required:

- `go test ./...`
- `.\scripts\check.ps1 -All`
- If new Phase 1 evidence is captured, use `.\scripts\evidence.ps1 -Phase phase-1 -Gate runtime-contract -Command "go test ./..."`

Constraints:

- Preserve RuntimeBackend boundary.
- Keep application-facing APIs backend-neutral.
- Do not merge to main.
- Do not broaden scope into Phase 2 Kubernetes deployment, Tool Gateway, Model Gateway, memory, rollback, UI, or cloud control plane.
- Do not mark Agent Sandbox behavior verified without upstream docs, install output, or runtime evidence.

Report:

- Changed files
- Evidence commands and results
- Risks or blockers
- Suggested next task
