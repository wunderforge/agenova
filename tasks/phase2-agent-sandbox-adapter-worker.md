# Task

Mission: Implement the smallest Agent Sandbox adapter slice needed to exercise a real Kubernetes claim lifecycle while preserving Agenova's RuntimeBackend boundary.
Branch or worktree: `worker/phase2-agent-sandbox-adapter`
Target: `internal/runtime/agentsandbox`, `deploy/`, `harness/phase-2-agent-sandbox-runtime`, `docs/evidence/phase-2`, `docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md`, `scripts/check.ps1` only for lightweight evidence checks.
User value: Agenova can prove that Agent Sandbox can be used as a backend substrate without leaking upstream CRD shape to application-facing APIs.

Acceptance criteria:

- A dedicated `internal/runtime/agentsandbox` package is the only place that may import or reference upstream Agent Sandbox CRD types or manifests.
- The adapter exposes an Agenova `RuntimeBackend` implementation or a clearly named spike backend that can create/read the upstream CRDs needed for a minimal claim lifecycle.
- A local e2e harness runs against `kind-agenova-k8s-lab` and records claim lifecycle evidence.
- `kubectl-runtime-state` evidence shows relevant upstream resources, pods/events/logs where available.
- `backend-neutral-api` evidence proves no upstream CRD shape appears outside the adapter package.
- If the upstream CRD semantics cannot carry Agenova's required claim lifecycle, record the blocker instead of faking support.

Evidence required:

- `.\scripts\evidence.ps1 -Phase phase-2 -Gate claim-lifecycle-e2e -Command "<e2e command>"`
- `.\scripts\evidence.ps1 -Phase phase-2 -Gate kubectl-runtime-state -Command "<kubectl state/log command>"`
- `.\scripts\evidence.ps1 -Phase phase-2 -Gate backend-neutral-api -Command ".\scripts\check.ps1 -All"`
- `.\scripts\check.ps1 -All`

Constraints:

- Preserve RuntimeBackend boundary.
- Keep application-facing APIs backend-neutral.
- Do not merge to main.
- Do not implement Tool Gateway, Model Gateway, memory, rollback, UI, or cloud control plane.
- Do not build a competing Kubernetes sandbox controller.
- Prefer a thin, explicit adapter over a broad abstraction.

Report:

- Changed files
- Evidence commands and results
- Risks or blockers
- Suggested next task
