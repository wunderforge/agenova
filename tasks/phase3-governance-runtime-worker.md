# Task

Mission: Implement the smallest Phase 3 governance runtime on the in-memory reference path, with evidence for gateways, facts, authorization, claim lineage, and a multi-agent reference scenario.
Branch or worktree: `worker/phase3-governance-runtime`
Target: `api/v1alpha1`, `internal/toolgateway`, `internal/modelgateway`, `internal/facts`, `internal/governance`, `harness/phase-3-governance-runtime`, `docs/evidence/phase-3`, `docs/status/implementation-status.js`, `scripts/check.ps1` only for lightweight evidence checks.
User value: Agenova demonstrates its own governance value above sandbox execution without implementing UI, cloud control plane, memory, rollback, or workflow DAG orchestration.

Acceptance criteria:

- Tool Gateway MVP authorizes requests by active claim and records `ToolInvocation` facts.
- Model Gateway MVP authorizes requests by active claim and records `ModelInvocation` facts.
- Runtime events can be recorded and queried by claim.
- Claim lineage supports parent/child relationships for multi-agent governance, without becoming a workflow DAG engine.
- Negative authorization tests prove unbound, terminal, unknown, or child-out-of-scope claims are denied.
- Multi-agent reference scenario creates parent/child claims and records facts under the right claim lineage.
- Kubernetes-backed multi-agent support is either tested or recorded through the existing `multi-agent-kubernetes-or-blocker` evidence gate.

Evidence required:

- `.\scripts\evidence.ps1 -Phase phase-3 -Gate tool-gateway -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate model-gateway -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate authorization-negative -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate claim-lineage -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate facts-query -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate multi-agent-reference -Command "<test command>"`
- `.\scripts\evidence.ps1 -Phase phase-3 -Gate multi-agent-kubernetes-or-blocker -Command "<test or blocker command>"`
- `.\scripts\check.ps1 -All`
- `.\scripts\check.ps1 -Phase3Evidence`

Constraints:

- Preserve RuntimeBackend boundary.
- Keep application-facing APIs backend-neutral.
- Do not merge to main.
- Do not implement UI, cloud control plane, memory, rollback, billing, or workflow DAG orchestration.
- Claim graph semantics may support future optional orchestration, but this phase only implements governance relationships.
- Keep implementation simple, readable, and in-memory unless evidence requires more.

Report:

- Changed files
- Evidence commands and results
- Risks or blockers
- Suggested next task
