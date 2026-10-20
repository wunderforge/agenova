# Task: Verify automatic main release with uncached status probes

- Ticket: [#225](https://github.com/wunderforge/agenova/issues/225)
- Mission: Prove a small merged application change automatically reaches the existing shared EKS service.
- Target: CP status handlers/tests, existing main release switches and release evidence.
- User value: Uncached readiness/status and a demonstrated merge-to-deployment path.
- PRD outcome: [demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture](../../docs/product/architecture-contract.md), [release runbook](../../docs/development/shared-eks-release.md), cmd/agenova-control-plane/main.go and status tests, scripts/release/release.mjs and tests.

## Scope

In scope:

- Add Cache-Control: no-store to readiness/status, retaining member metadata restrictions.
- Temporarily activate existing main publication/deployment gates immediately before merge; verify push-triggered delivery and restore disabled switches afterward.

Out of scope:

- Worker/schema/auth changes, Secret reads, new infrastructure, external MCP/OTEL, or permanent release-policy changes. Manual dispatch is not evidence of automatic main delivery.

## Acceptance Criteria

- Focused GET/HEAD tests prove no-store while existing body/status/member restrictions remain intact.
- Actual Git/Go dependency discovery selects Control Plane only from the recorded deployed source, never an invented baseline.
- PR merge triggers main-push validation, OIDC/ECR publication and guarded CP rollout.
- AWS verifies immutable ECR digest; Kubernetes verifies unchanged UID/configuration/Worker/registry, new CP image/source/generation and readiness.
- Real readiness returns no-store and existing authenticated Work remains queryable; switches return to disabled after the trial.

## Negative Case

- Remote /v1/status remains inaccessible even with spoofed forwarding headers. Preserve rollback/Worker/drift guards; never relabel failed rollout as success.

## Execution Todo

- [x] Scout code, contracts and risks.
- [x] Self-review bounded acceptance against PRD and Owner's authorization.
- [ ] Capture baseline, add uncached status and regression tests.
- [ ] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./cmd/agenova-control-plane`
- `node --test scripts/release/release.test.mjs scripts/release/candidate.test.mjs`
- PR/Main Linux gates and actual AWS/Kubernetes/HTTP/Agenova CLI acceptance.
- `.\scripts\check.ps1 -All`

## Evidence Required

- Safe pre/post Deployment projections, push run/event/source/ECR digest, readiness headers and retained Work query. No new model call is necessary for this status-only change.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Reviewed login baseline remains fixed; no secret-bearing file or Secret object reads, no template overwrite or other worktree edits. One single-replica Recreate rollout briefly interrupts service.

## Decisions and Blockers

- Owner requests this bounded automatic main trial on 19 October 2026, not indefinite unattended deployment. Existing main-only OIDC and disabled candidate mode remain unchanged. Return both ordinary switches to disabled after testing; ongoing automatic releases require operator activation.

