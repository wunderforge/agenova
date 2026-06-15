# Task

Mission: Establish the first Phase 2 deployability evidence gate by verifying local Kubernetes tooling and the upstream Agent Sandbox installation path.
Branch or worktree: `worker/phase2-upstream-install`
Target: `deploy/`, `docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md`, `docs/evidence/phase-2`, `docs/harness/*`, `scripts/check.ps1` only if a small gate needs clarification.
User value: Agenova can decide from evidence whether Kubernetes SIG Apps Agent Sandbox is a real default backend substrate or a blocker that must stay behind the RuntimeBackend boundary.

Acceptance criteria:

- Local tool availability is recorded for `docker`, `kind`, and `kubectl`.
- A local kind cluster is created or a blocker is documented with exact command output.
- Upstream Agent Sandbox install/source path is verified from official sources or documented as a blocker.
- No Agent Sandbox support is claimed without install output, official docs, or runtime evidence.
- The RuntimeBackend boundary remains intact; no upstream CRD shape appears in application-facing Agenova APIs.

Evidence required:

- `.\scripts\evidence.ps1 -Phase phase-2 -Gate cluster-bootstrap -Command "<cluster bootstrap or version command>"`
- `.\scripts\evidence.ps1 -Phase phase-2 -Gate upstream-agent-sandbox-install-or-blocker -Command "<install/source verification command>"`
- `.\scripts\check.ps1 -All`

Constraints:

- Preserve RuntimeBackend boundary.
- Keep application-facing APIs backend-neutral.
- Do not merge to main.
- Do not implement Tool Gateway, Model Gateway, memory, rollback, UI, or cloud control plane.
- Do not build a competing Kubernetes sandbox controller.
- If upstream Agent Sandbox is unavailable, record the blocker and stop; do not invent a fake integration.

Report:

- Changed files
- Evidence commands and results
- Risks or blockers
- Suggested next task
