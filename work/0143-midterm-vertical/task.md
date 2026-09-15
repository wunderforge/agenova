# Task: UI-to-kind real-model checkpoint

- Ticket: [#143](https://github.com/wunderforge/agenova/issues/143)
- Mission: Complete one observable internal-demo journey from task submission through real kind execution and model inference to inspectable evidence.
- Target: application facts/evidence, local console API, controlled runtime transport, example worker, provider adapter, React portal and acceptance tests.
- User value: The owner can submit, watch and inspect a real governed assignment without reconstructing disconnected module demos.
- PRD outcome: [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, this packet.

- [Checkpoint spec](spec.md) and [design](design.md)
- [Architecture contract](../../docs/product/architecture-contract.md)
- [Reference admission](../../internal/app/reference_admission.go), [submission](../../internal/app/submit.go), [RunService](../../internal/app/run_service.go)
- [Fact store](../../internal/facts/store.go), [Model Gateway](../../internal/modelgateway/gateway.go)
- [Controlled adapter](../../internal/runtime/agentsandbox/controlled.go)
- [Accepted real-kind seam](../../harness/integration/agentsandbox/application_run_test.go)
- [AIDLC](../../docs/development/AIDLC.md), [playbooks](../../docs/harness/playbooks.md)

## Scope

In scope:

- Finish #36's supported credential boundary before integration release.
- Complete #37/#38's immutable correlated facts and shared evidence view, coordinating with the original #37 owner rather than duplicating a live writer.
- Expose #68's local/internal evidence API plus the explicitly approved limited submission endpoint.
- Reuse #136's kind lifecycle and a bounded worker transport, enforce #33/#34/#35 before any provider attempt.
- Add a minimal real-model adapter and worker objective/result interaction, not a vendor-specific governance model.
- Integrate the accepted React product portal from its preview branch; keep demo and connected sources explicit and isolated.

Out of scope: policy editing, login/SSO, broad administration, durable history, production anti-bypass claims, multi-agent, full coding framework, vendor console recreation.

## Acceptance Criteria

- Internal UI submits canonical ClaimRequest input; server supplies trusted principal separately and resolves permissions before allocation.
- Team A reaches one actual kind worker, real model inference and a task-dependent result; UI shows lifecycle, requested/effective authority, decisions, invocations, result and cleanup evidence.
- Team B denial produces request-correlated evidence and zero claim/backend/provider calls.
- Over-ceiling access is narrowed; denied and terminal invocations make zero provider calls and retain the defined attribution semantics.
- Connected failures remain unavailable/failed, never filled with fixture results.

## Negative Case

- Caller-supplied principal/claim authority cannot authorize execution; wrong worker binding, ungranted model and post-terminal attempts fail closed.

## Execution Todo

- [x] Scout merged baseline, dirty worktrees, accepted kind seam and existing UI branch.
- [x] Record independent planning review; owner approved checkpoint and internal submission on 15 Sep 2026; independent blind review approved corrected spec/design.
- [x] Freeze fact/evidence and submission boundaries; finish prerequisite #36, including immutable input snapshots and exact credential aliases.
- [x] Implement facts/evidence and local API with deterministic failure tests.
- [x] Implement bounded real-model worker transport and provider response/result handling.
- [x] Integrate React demo/connected flow and UI submission.
- [x] Add executable integration and browser acceptance gates before claiming live completion.
- [x] Run focused gates, repository baseline, controlled-kind gate and separately authorized live inference gate.
- [ ] Independently review, commit, publish evidence and reconcile linked tickets/PRs.

## Quality Gates

- `.\scripts\check.ps1 -All`
- Existing kind baseline: `go test -count=1 -v -tags 'integration controlled' -timeout 5m ./harness/integration/agentsandbox -run '^TestGovernedApplicationRun_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace <new-owned-namespace>`
- New facts/API/provider and browser acceptance commands must be recorded when introduced; no prose-only replacement for executable gates.
- Real HTTP/kind/model gate: `go test -count=1 -v -tags 'integration controlled live' -timeout 5m ./harness/integration/agentsandbox -run '^TestUIModelCheckpoint_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace <new-owned-namespace> -live-model`
- Real browser gate: `node harness/e2e/ui-kind-checkpoint.mjs --live-model --base-url http://127.0.0.1:5175`

## Evidence Required

- Independent planning and release reviews, focused/full test output and CI.
- Browser submission-to-result flow and pre-allocation denial proof.
- Actual kind worker identity, runtime start, provider response/usage metadata and cleanup confirmation from the same assignment.
- Live inference is a separate authorized gate, never a recorded/mock response substituted into connected mode.

## Constraints

- Preserve stable contracts except the owner's explicit internal-submission exception, recorded in PRD and architecture contract.
- Provider configuration stays private to the host adapter; never read secret-bearing files or include provider credentials in worker input, logs, Git or browser responses.
- No default paid calls; provider/channel and bounded budget require explicit confirmation.
- Keep one writer per path and inspect existing team work before takeover.

## Decisions and Blockers

- Owner approved a bigger checkpoint, internal UI submission and real LLM execution. Tools may remain explicit mocks.
- Public API selection may be made behind the adapter, but organizational compliance is not established by endpoint choice; use public/synthetic inputs for live evidence.
- Provider access is the owner's running local Ollama; existing model only, no download or paid API call. Public-provider compliance/access remain separate configuration.
- Independent blind planning review requested two fixes: freeze transport/Running sequencing and remove any browser identity switch. Both are recorded in spec/design; operator-only identity and pinned exec/stdio are the construction boundary.
- Owner allowed blind Codex review to preserve scarce Claude capacity; use independent Codex and app PR review, without spending Claude quota by default.
- Owner opened local Ollama; the loopback API is available with existing `llama3.1:latest` and `deepseek-r1:7b`. Use existing local llama inference for the real-model gate, without new downloads or API charges. Generic adapter tests remain separate from real-model evidence.
- A recovery heartbeat could not be created because this thread already has an active automation. Preserve the original automation; local recovery state is [sprint.md](sprint.md).

## Verified checkpoint (15 Sep 2026)

- Live HTTP/kind gate passed in 29.92 seconds: Team B denial allocated no claim/worker and made no provider call; Team A obtained an actual llama3.1 answer through the Model Gateway and cleaned up its worker.
- Browser gate passed: request `work-59ff763d-38e9-4b49-aa64-354468e59042`, worker `agenova-pool-reference-engineer-pool-7njm5`, invocation `inv-de4174f7bfd2e37fc521133a8b48b6d7`, 23 input / 24 output tokens, 14 correlated facts and confirmed cleanup.
- Result, authority comparison and Platform screenshots are generated under `.tmp/ui-kind-checkpoint/`; these are local test artifacts, not maintained fixtures.
- Independent core review found lifecycle sink, cleanup-result and shutdown issues; all were fixed with deterministic regressions and re-reviewed. Final integrated release review and repository baseline remain release gates.
- This proves a task-dependent minimal agent answer, not repository editing or a fully integrated vendor coding agent. Tools and Memory remain explicitly unconnected.
- Integrated `scripts/check.ps1 -All` passed, including Go/vet, canonical bindings, 72 frontend unit tests and 37 browser tests. Initial sandbox-only VCS failure was resolved by rerunning with Git access, not by changing the gate.
- Final review found a queued-cancellation display issue; f34b1b7 fixes it and proves terminal polling stops. [Walkthrough and startup](demo.md).
- Independent reviewer confirmed that cancellation finding closed with no remaining findings from its bounded release review.
- Final binary/browser gate also passed after security and UI fixes: request `work-79f19720-75c9-4afe-81b7-47579e2484cc`, worker `agenova-pool-reference-engineer-pool-8mh65`, invocation `inv-1ff2f531369d22dc6e24df065b90a14d`, 23 input / 26 output tokens, 14 facts and cleanup. The local server currently retains this real result.
- Native npm credential fields fixed at 504fd3e; shared boundary focused/full tests passed. Compound registry-qualified keys require an owner decision in [#145](https://github.com/wunderforge/agenova/issues/145), requested before changing the existing exact-key grammar. #142/#144 remain unmerged; actual checkpoint behavior is verified, release is not complete.
