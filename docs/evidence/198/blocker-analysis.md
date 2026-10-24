# #198 remaining acceptance: feasibility and evidence

Date: 19 October 2026. Baseline `ae24a67`. This analysis preserves the accepted Ticket criteria. Source references below are checkout-relative. Supplementary tests use public composition seams and an explicitly in-memory runtime; they do not change product code and are not installed-kind or A2 evidence.

## 1. Two known Running claims in one installed service

**Blocked by the reference implementation, not by the cluster or worker image.**

`internal/console/service.go:104,242` admits one executing Work through executeMu. Independently, `internal/app/run_service.go:267` holds runMu across work, terminal transition and cleanup. Calling RunService directly would retain the second lock; allocating two Pods or installing another control plane does not put two Running claims in the same authoritative service.

`TestConsoleQueuesSecondClaimUntilFirstCompletes` configures two idle workers (so capacity is not the explanation), holds A after a real Service handler calls a synthetic provider, submits B, checks A Running / B Pending / B.backendIdentity=nil, then verifies B succeeds only after A is released. This confirms composition behavior without editing private fields.

No public concurrency setting or alternate installed worker invocation endpoint was found in the baseline. The only worker operation path is the pinned exec session and its callback. An artificial aggregate of two service stores, modified locks, or hand-edited claim phases would change the system under test; it cannot satisfy this criterion.

**Maintainer action needed:** assign the prerequisite for same-service concurrent execution, or explicitly approve a narrower acceptance outcome and update the governing requirement where necessary. E15 cannot silently waive this requirement or implement a product fix under report-only scope.

## 2. Model-originated out-of-scope request recorded as Gateway Deny

**Gateway denial works; the required installed model-to-Gateway route is the blocker.**

The reference worker always sets operation.Profile from task.ModelProfile and operation.ResourceScope from task.ResourceScope (`cmd/agenova-demo-worker/main.go:72,103`). Its parsed model action has only action/tool/input/answer (`internal/workerprotocol/react.go:46`). It cannot nominate a model profile or resource scope. A file input remains a parameter to a synthetic adapter that never opens a real repository; changing the filename alone is not a request for another governed resource scope.

A scripted worker can nominate a different profile or scope, but `exchangeWorker` rejects profile mismatches at `internal/runtime/agentsandbox/execution.go:211` and scope mismatches at line 223 before calling the Service handler. A tool request must first match the preceding model action (line 217). Transport-compatible operations use task values taken from system-issued effective authority, so the baseline does not offer a legitimate installed operation that both violates those grants and reaches these Gateway authority checks. The console composes these Gateways with their default invocation policy; no separate operator-configurable invocation-deny policy seam was found there.

`TestGatewayDenyAvailableThroughExecutorSeamOnly` calls the genuine console handler via an injected Executor: ungranted profile and scope yield ModelDecision/ToolDecision Deny with policy ID/version, reason and correct claim attribution. Only the preceding allowed model call reaches the synthetic provider. This proves the downstream function; it **skips exchangeWorker**, so it is not accepted as installed-kind or model-originated injection proof.

**Maintainer action needed:** choose a supported product composition for this acceptance case, or approve reporting the stronger early refusal together with its missing rejection-evidence limitation. A test-only endpoint that calls the Gateway directly would be a different entrypoint; it must not be labelled the installed worker channel.

## 3. Observed requests after Succeeded, Failed and Expired

**We can now prove refusal through the public handler seam; installed reachability and rejection records remain open.**

`TestRetainedHandlerRefusesAllTerminalPhases` keeps the real console callback, waits for canonical terminal outcome, then sends both model and tool operations with a **fresh uncancelled context**. All six attempts are refused and cause zero extra synthetic provider calls. Because the context is fresh, these results exercise the authoritative phase guard (`internal/console/service.go:291`) rather than just detecting the old cancelled context.

For all six attempts, the returned Work facts are unchanged. Thus refusal occurs but there is no new inspectable decision record for the late attempt. This is a reproduced evidence gap, not merely an inability to construct a test. See [the output](boundary-probe.log).

In the installed path, the executor and worker session are cancelled/stopped as the run ends, and cleanup releases the worker binding. The callback used by this test is process-local and is not exposed as a supported installed request endpoint. Keeping a callback in a Go test therefore does not prove a post-terminal operation arrived from a real worker channel. Existing expiry and invalid-result kind runs remain cancellation/result-validation evidence only.

**Maintainer action needed:** decide where rejected terminal attempts should be accepted for inspection and recorded (composition edge versus another governed endpoint). E15 can then retest the installed route without fabricating lifecycle state or weakening eligibility checks.

## Evidence and routing

- All new tests passed with the race detector: `go test -race -count=1 -v ./harness/security/boundaryprobe`.
- Test file: `harness/security/boundaryprobe/boundary_test.go`; exact output: [boundary-probe.log](boundary-probe.log).
- No product implementation changed. No live cluster mutation was needed for this follow-up analysis.
- Findings concerning early/late refusal records map to AC-1, AC-4, AC-5 and ES-1. Same-service concurrency maps to the AC-5 runtime acceptance prerequisite.
- No authority leak was observed. No high-severity reproduction is included here.
- Follow-up drafts and the #198 comment draft are local and unpublished. The owning Epic remains a maintainer decision; proposed destinations are authority/worker execution for the invocation path and diagnosis/evidence for safe decision records.

## Existing work checked

Read-only inspection of open PR #217 (E18 diagnosis fixes) found changes to capacity/reservation handling and notification of denials *inside* the Gateways before claim resolution. Its diff does not change executeMu, RunService runMu, exchangeWorker guards, or the Service terminal-phase guard. It therefore does not by itself clear the three installed-path blockers above. Coordinate rejection-record ownership with #180 rather than assuming #217 already fixes the early/late worker cases.
