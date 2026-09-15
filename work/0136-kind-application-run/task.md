# Task: Compose the governed application run with Agent Sandbox on kind

- Ticket: [#136](https://github.com/wunderforge/agenova/issues/136)
- Mission: Prove the canonical ClaimRequest can use the same application governance and lifecycle path while only the RuntimeBackend is replaced by the controlled Agent Sandbox adapter on kind.
- Target: `harness/integration/agentsandbox/`, focused evidence, and only the smallest application composition seam needed by the test.
- User value: A teammate can see that Agenova governs one request before Kubernetes execution, rather than merely launching a Pod.
- PRD outcome: [MVP User Journey](../../docs/product/prd.md#mvp-user-journey) and [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Technical design](design.md)
- [Architecture backend neutrality](../../docs/product/architecture-contract.md#backend-neutrality)
- [#41 run submission](../0041-run-submission/task.md)
- [#31 application run service](../0031-application-run-service/task.md)
- [#135 controlled kind worker](../0135-controlled-worker-kind/task.md)
- [Agent Sandbox mapping](../../docs/backends/agent-sandbox.md)

## Scope

In scope:

- One separately tagged integration test using the canonical Team A ClaimRequest, trusted reference principal/policy, system-managed issuance, `RunService`, and `agentsandbox.ControlledAdapter`.
- One Team B denial that proves zero backend allocation.
- Correlated lifecycle, worker identity, actual worker result, and confirmed cleanup evidence.
- Exact kind preparation and reproduction commands.

Out of scope:

- Full #51 filesystem/mount hardening, gateways, evidence-view assembly, live UI, arbitrary agent images, production controllers, or multi-agent behavior.

## Acceptance Criteria

- Team A reaches `Succeeded` through the canonical application submission path on kind.
- The same system-issued claim ID is correlated with the one backend allocation and worker identity through Start, Terminate, and Cleanup.
- Team B is denied before any Kubernetes allocation.
- Provider types remain inside the adapter/composition edge; the ClaimRequest, authority, claim, and RuntimeBackend contracts do not change.

## Negative Case

- Team B denial must not call any RuntimeBackend method or create a claim-backed Kubernetes workload.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation; the Owner explicitly directed uninterrupted progress on the #107 critical path.
- [ ] Add the application-to-controlled-backend integration gate.
- [ ] Capture a fresh real-kind transcript with independent cleanup checks.
- [ ] Run focused compile/tests and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -run '^$' -tags 'integration controlled' ./harness/integration/agentsandbox/`
- `go test -count=1 -v -tags 'integration controlled' -timeout 5m ./harness/integration/agentsandbox/ -run '^TestGovernedApplicationRun_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace agenova-run136`
- `.\scripts\check.ps1 -All`

## Evidence Required

- A fresh kind transcript naming the request, decision, issued claim, worker identity, terminal phase, controlled result, denied zero-allocation path, and confirmed cleanup.
- Focused compile/test output and repository baseline result.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Replace only the RuntimeBackend in the real-backend path; do not fork governance logic for Kubernetes.
- Never infer work start from Ready or cleanup from a delete request.

## Decisions and Blockers

- PR #134 supplies the opt-in controlled worker protocol because Agent Sandbox v0.4.6 does not natively acknowledge task start/stop.
- #123 remains Future Backlog because its gateway and multi-agent/lineage scope exceeds the single-claim mid-term demo.
- #41 / PR #128 is the dependency under review; this branch is intentionally based on its verified implementation and will be rebased after merge.
