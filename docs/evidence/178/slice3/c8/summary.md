# #178: current-branch kind acceptance c8

- Task: [E16 packet](../../../../../work/0178-governed-mcp-tools/task.md), [acceptance plan](../../../../../work/0178-governed-mcp-tools/slice3-kind-acceptance-plan.md).
- Source: clean `codex/e16-slice1` at `c50a6bc9af3caeba8cef815ebb29d7dcad4b1e5d`. Product and harness source were unchanged throughout this run; this export was added afterwards.
- Date: 2026-10-15, Australia/Melbourne (UTC+11). Machine timestamps in the receipts are UTC.
- Target: existing `kind-agenova-k8s-lab`, Kubernetes v1.37.0, with independent namespaces `agenova-e16-system` and `agenova-e16`.
- Model: `qwen2.5:7b`, digest `845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e`, through host Ollama 0.34.4.
- Platform revision: `sha256:d244b22e035ce339d1b72f0abafb2990eb0d3cab014e373dd5538413482afe94`.

All eight Work cases passed on attempt 1, including their CLI/API/Portal parity. The probe produced 30 receipts; all 31 server-log validator rows passed, including the unattributed-traffic check. The completed token scan passed. Execution had two recorded orchestration problems described below; this is not an all-steps-first-pass claim.

## Results

| Check | Result | Evidence |
| --- | --- | --- |
| Current-source builds, production/probe separation and loaded image identities | pass | [build identity](campaign/build-identity.txt), [loaded identities](campaign/load-identity.txt) |
| Real MCP controlled read and header-less token endpoint rejection | pass | [interop output](campaign/controlled-read/interop.log), [401 status](campaign/controlled-read/token-probe-status.txt) |
| Install, configuration, registration, repeated apply and access checks | pass | [install transcript](steps/install.log); raw access-review and Pod records retained locally |
| Positive task: two real reads, three model turns, checked answer and cleanup | pass | [checker](campaign/work/positive/attempt-1/evidence.txt) |
| Admission denial before claim or downstream activity | pass | [checker](campaign/work/admission-deny/attempt-1/evidence.txt) |
| N6 bounded timeout | pass | [checker](campaign/work/n6-timeout/attempt-1/evidence.txt) |
| N7 hard response limit | pass | [checker](campaign/work/n7-oversize/attempt-1/evidence.txt) |
| N8 observation truncation and incomplete-answer marker | pass | [checker](campaign/work/n8-truncation/attempt-1/evidence.txt) |
| Valid token read | pass | [checker](campaign/work/token-valid/attempt-1/evidence.txt) |
| Missing token: `tool-credential-unavailable`, zero server entries | pass | [checker](campaign/work/token-missing/attempt-1/evidence.txt) |
| Wrong token: `tool-credential-rejected`, one rejected initialize, no retry | pass | [checker](campaign/work/token-wrong/attempt-1/evidence.txt) |
| N1-N5 denial categories and server-backed N11 outcome-record failure | pass | [30 receipts](campaign/probe/probes.jsonl), [server-log verdicts](campaign/probe/evidence.jsonl) |
| CLI/API/Portal parity for every Work; 16 browser tests, no skipped/flaky tests | pass | `campaign/work/*/attempt-1/parity/` |
| Original final-scan invocation | fail | [incomplete scan](campaign/scan/scan.txt): an execution-control FIFO was not a regular file |
| Completed scan after moving the FIFO outside evidence roots | pass | [scan](campaign/scan-corrected/scan.txt), [command and exit codes](campaign/scan-corrected/command.txt) |
| Protect/restore of original reference images and replicas | pass | [restore transcript](steps/--yes-restore.log) |
| Full macOS gate on this source, Go 1.22.12 / Node 24.21.0 | pass | [full gate](gates/check-all.txt), including 52/52 browser smoke tests |
| Independent fixture module | pass | [fixture tests](gates/fixture.txt) |
| Acceptance runner: 52 offline groups | pass | [runner tests](gates/test-campaign.txt) |

The host gates ran immediately before c8 on the same clean source, during the preceding c7 preparation. They are not new executions of the historical c6 gates. Positive, denied, truncated and credential-rejected Portal screenshots were visually inspected.

## Execution records and scan completion

c7 passed preflight, build, protect/load, fixture, controlled-read and install. Before its first Work could be submitted, `require_collector` rejected the absent fixture collector: the execution tool had reaped background children when their command session ended. No c7 Work was submitted. Its final Work list was empty; a separate abort scan passed over 99 files. The old reference installation was restored and the two c7 namespaces were removed. Raw c7 records remain under `.tmp/e16-c7/` and `.tmp/e16-c7-steps/` locally.

c8 used one persistent controller session for all campaign commands, so both collectors remained alive through the Work, probe and final-log checks. The FIFO used to send commands to that session was initially inside the extra steps directory. The canonical final scan exited 2 when it encountered that non-regular file. The failure record remains intact; no token leak was reported, and this incomplete invocation is not called a pass.

The FIFO was moved outside all evidence roots. The completion procedure repeated the same three Secret identity checks, captured complete control-plane and fixture logs, checked their continuity and fixture identity, and invoked the same built `e16-evidence scan` binary on the raw campaign, CLI state, sanitised export and step records. Both the token stream and checker exited 0. It scanned 782 files / 120,617,848 bytes with 12 patterns across four roots and found no tested token value. [The completion record](campaign/scan-corrected/reason.txt) explains this deviation. No product code, token, assertion, fixture, model setting or Work input was changed to obtain the result.

## Reproduction

Use a fresh campaign output and CLI state directory with both acceptance namespaces absent. Keep the command session alive until logs, evidence and scan are complete. The execution-control FIFO, if used, must be outside scanned artifact roots. The local [step wrapper](steps/run-step.sh.txt) and [controller](steps/controller.sh.txt) are retained as sanitised text; replace `<repo>` with the checkout path.

```sh
export PATH="$(brew --prefix node@24)/bin:$PATH"
A=(--context kind-agenova-k8s-lab --install-namespace agenova-e16-system \
   --state-dir .tmp/e16-new-state --output .tmp/e16-new --model-profile coding-standard)
bash harness/integration/e16/campaign.sh "${A[@]}" preflight
bash harness/integration/e16/campaign.sh "${A[@]}" build
bash harness/integration/e16/campaign.sh "${A[@]}" --yes protect
bash harness/integration/e16/campaign.sh "${A[@]}" --yes load
bash harness/integration/e16/campaign.sh "${A[@]}" fixture
bash harness/integration/e16/campaign.sh "${A[@]}" controlled-read
bash harness/integration/e16/campaign.sh "${A[@]}" install
for case_name in positive admission-deny n6-timeout n7-oversize n8-truncation token-valid token-missing token-wrong; do
  bash harness/integration/e16/campaign.sh "${A[@]}" work "$case_name" || break
done
bash harness/integration/e16/campaign.sh "${A[@]}" probe
# Export evidence, then scan with any additional roots given as absolute paths.
bash harness/integration/e16/campaign.sh "${A[@]}" scan
bash harness/integration/e16/campaign.sh "${A[@]}" --yes restore
```

Do not continue to the next case after a failed check without assessing its evidence. The existing maximum-five-attempt/first-pass rule remains unchanged; c8 needed no retry of a Work.

## Evidence limits

- This run proves the recorded cases at `c50a6bc`, including the post-main-merge installed composition. c6 remains separate historical evidence at `fd08ac0`.
- The fixture does not reflect authorization headers. Passing c8 does not resolve the maintainer's private R6 / AC-7 finding or replace independent human approval. Token reflection has deterministic regression coverage in the full gate.
- N3 proves the service-side correlation guard, not authenticated worker isolation. Worker authentication and the authenticated rerun belong to #205.
- The additional unreachable-endpoint case on kind, native Windows execution, deferred first-turn prompt-budget work, production upgrades and durable Work history are outside this run.
- All eight Works passing once is not a measured repeatability rate. The answer checker validates the specified facts and required markers, not every natural-language sentence.

The subsequent permanent update of `agenova-system` used the same production image configs. [Final status](reference-update/platform-status-final.txt) is ready; [identical reapply](reference-update/platform-reapply.txt) reports `changed: false`. The real Ollama allowed Work succeeded, the denied Work allocated no claim, and [both installed CLI/API/Portal tests passed](reference-update/accept.log). Final Pod mappings prove the [control plane](reference-update/control-plane-final.txt.mapping) and [worker](reference-update/worker-final.txt.mapping) use the recorded configs. Both Pods were ready with zero restarts. The two temporary acceptance namespaces were removed after archive and restore.

The default reference Platform retains `llama3.1:latest` and its explicitly synthetic tool configuration. The worker's config digest matches c6 because that Docker build reused unchanged worker content; its old `import-2026-10-05` repository-digest spelling is resolved through the node image inventory, not treated as proof of source age.
