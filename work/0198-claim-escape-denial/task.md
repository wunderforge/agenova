# Task: Deny claim-escaping worker and model-originated operations

- Ticket: [#198](https://github.com/wunderforge/agenova/issues/198)
- Mission: Record runtime evidence on the reference kind install that operations outside the bound claim's authority are refused, attributed and never reach the provider.
- Target: `harness/security/` (test-only scripted worker image, run scripts), `docs/evidence/198/`; no product code changes.
- User value: The demo can state which claim-scope denials are proven on kind, and which are refused without a recorded decision.
- PRD outcome: [4. Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [5. Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md) and [`design.md`](design.md)
- [#194 matrix](../0194-security-claim-matrix/spec.md): rows SR-3, SR-4, AC-1, AC-2 (re-test), AC-3, AC-4 (terminal), AC-5, FL-1, ES-1
- [Architecture contract: Authority and Credentials, Facts](../../docs/product/architecture-contract.md#authority-and-credentials)
- Worker channel: `internal/runtime/agentsandbox/execution.go` (`Execute`, `exchangeWorker`)
- Operation handler and gateways: `internal/console/service.go` (`run`), `internal/toolgateway`, `internal/modelgateway`
- Prior method and oracle: #52 evidence and `harness/security/ollama-count.sh` in [PR #209](https://github.com/wunderforge/agenova/pull/209) (not yet on `main`; this branch reuses the counter once it merges)

## Scope

In scope:

- Out-of-claim operations sent over the existing worker channel: another claim's ID, malformed or unsupported operations, operations after a terminal phase, exceeding the turn limit.
- Model-originated requests for an ungranted tool, resource scope or model profile.
- Provider-side oracle (Ollama request log) compared with `agenova work show --json`, with a positive control.
- Classification of each result as Gateway Deny, composition-edge rejection or run failure.
- A follow-up issue for each gap, mapped to its matrix row.

Out of scope:

- Fixes (#177 decision, 2 October 2026: report only).
- Network bypass, credential and mount exposure (#52); filesystem isolation (#51).
- Worker→Gateway network authentication (#121) and caller identity (#172).
- Real MCP tools (E16); EKS.

## Acceptance Criteria

- Two Running claims A and B: an operation from A naming B is refused, B receives no invocation fact, and the attempted operation causes zero provider calls.
- A model/task-induced out-of-scope resource or ungranted profile reaches the Gateway and records Deny with policy reference and reason, with zero provider calls for that attempt.
- Operations after Succeeded, Failed or Expired are refused and recorded.
- Evidence distinguishes Gateway Deny, demo-edge rejection and run failure.
- Each finding is mapped to a matrix row; high-severity findings follow private disclosure.
- These criteria are not waived by reference-environment limits. Current partial coverage and open items are in [the evidence report](../../docs/evidence/198/summary.md).

## Negative Case

- Positive control: one allowed in-scope model call raises the provider count by the expected amount. Without it, a zero count is "inconclusive", not "pass".

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [x] Build the test-only scripted worker image and its fake-input unit tests.
- [x] Bring up the reference kind install; run the positive control.
- [x] Run each scenario in one control plane; record CLI JSON and provider counts.
- [x] Install a second, isolated reference instance; run the cross-claim case with B Running there.
- [x] Delete both isolated test installations; confirm unchanged reference platform has no drift.
- [x] Write `docs/evidence/198/`; draft follow-up issues for owner approval.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./harness/security/...` (scripted worker scenario parsing)
- `bash -n harness/security/*.sh`
- Real kind run; output committed as evidence
- `.\scripts\check.ps1 -Docs`
- `.\scripts\check.ps1 -All` (known macOS-only failure `ui/smoke/console.spec.ts:74`, passes on CI)

## Evidence Required

- `docs/evidence/198/`: replayable commands, kind context, image IDs, `work show --json` per scenario, provider counts before/after for positive and negative runs, classification table, residual risk.
- Synthetic data and canaries only.
- High-severity findings follow the #177 disclosure rule; reproduction detail stays out of public evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Report only; no product code changes.
- Disposable, explicitly selected cluster: every command passes `--context kind-agenova-k8s-lab`.
- The single-image check (`AGENOVA_ALLOWED_WORKER_IMAGE`) stays enforced; only the configured image reference changes for the test run, and it is restored afterwards.

## Decisions and Blockers

- Planning depth: Task + Spec + Design, as the Ticket requires.
- Worker method (assignee, 17 Oct): a test-only image built from the reference worker Dockerfile whose worker entrypoint sends scripted operations. The platform's compatible worker image points to it for the run and is restored afterwards. This substitutes the agent code, which A1 controls by definition; it does not bypass the image check.
- Concurrency (17 Oct): the reference control plane runs one claim at a time (`executeMu` in `internal/console/service.go`), so two Running claims cannot coexist in one install. Cross-claim runtime coverage was collected with B terminal in the same service and B Running in a second install. A separate composition test confirms B queues while A is Running; the queued-target cross-claim runtime attempt was not performed. These are supplemental cases, not substitutes for two known Running claims.
- Initial scouting result (17 Oct; see measured qualifications below): `exchangeWorker` rejects claim-ID, profile, tool and scope mismatches before the handler and gateways, so these are expected to be refused without a decision fact and reported as generic `execution-failed`. If confirmed on kind, this is filed as a follow-up (evidence gap, not an authority leak), per assignee decision.
- Dates: evidence target 23 Oct 2026; demo 5 Nov 2026.

## Execution update — 19 October 2026

- Test-only worker, tests and real kind campaign completed using supported digest image + controlled-v1 configuration in two isolated namespaces; original reference platform was preserved.
- Positive control succeeded; forged worker operations caused no provider calls beyond their explicit allowed control. Two-install Running target facts were unchanged.
- Partial evidence only: same-install two Running claims, actual model-originated Gateway Deny, recorded terminal-operation refusal and policy narrowing re-test remain open. Earlier scouting conclusions are hypotheses unless supported by captured output.
- Both test namespaces cleaned up by UID; reference status unchanged. Follow-up drafted locally, not published.

## Blocker follow-up — 19 October 2026

- [Analysis and exact source references](../../docs/evidence/198/blocker-analysis.md): console executeMu and RunService runMu both serialize execution; alternate submission alone cannot resolve concurrency.
- Public console.Executor seam tests confirm Gateway Deny records with policy/reason for profile and scope, but skip the installed transport; they are supporting code evidence only.
- Fresh-context model/tool calls after Succeeded/Failed/Expired are refused by the authoritative phase guard, with zero extra provider calls and no new facts. This reproduces missing rejection records. It does not prove a live installed late worker request.
- [ ] Maintainer assigns concurrency and rejection-record prerequisites and specifies a supported A2 installed acceptance path; retain original criteria.
- [ ] Retest installed path after the prerequisites exist; publish only after assignee approval.

## Supplementary evidence — 19 October 2026

- [x] Monitor the denied submission using raw Pod and SandboxClaim watches from saved resourceVersion; verify zero denial-window events and positive-control ADDED events.
- [x] Reject synthetic canary via private API, verify scanner positive control, then scan the explicit namespace objects, current logs and agent environment.
- [x] Compare actual CLI/API JSON and inspect rendered Succeeded/Failed/Denied pages and requested/granted template limits; save screenshots.
- [x] Probe current Policy schema: constraints are rejected before registration, active Policy unchanged; actual Policy narrowing is unimplemented in this baseline (E14 #204).
- Evidence: [supplement/summary.md](../../docs/evidence/198/supplement/summary.md). New optional runners: `harness/security/supplement-evidence.py`, `harness/security/verify-198-ui.mjs`; UI verification reads existing installed records and makes no submissions.
- Product implementation unchanged. Main acceptance blockers remain open; supplemental checks do not waive them.
