# #199 complete reference-kind fault campaign

Measured 19 October 2026 against main `22355d5` (checkout merge `90b773d`). This supersedes the pre-Demo-only scope of the parent report; the earlier report remains historical evidence for `ae24a67`. The user explicitly brought the full #199 matrix forward. Tests were executed locally; no EKS action, product logic change or public publication was performed.

## Tested configuration and provenance

A dedicated disposable cluster `agenova-e15-199-faults` used the **separate** `.tmp/faults.kubeconfig` and explicit context `kind-agenova-e15-199-faults`. The shared `kind-agenova-k8s-lab` controller was never stopped. The cluster node ID, kube-system UID and dedicated Provider ID are recorded in owner.json. Namespace UIDs are in the create receipts.

Both control-plane and compatible worker were freshly built from the current main tree using repository Dockerfiles, then loaded only into the disposable cluster. [Source provenance](current/source-provenance.json), current/current-images.stdout and actual Pod imageIDs in the raw snapshots identify the tested code and images. `git diff --quiet 22355d5 -- api internal cmd deploy/reference harness/integration/agentsandbox go.mod go.sum` confirmed the product tree matched main. Local Docker cache tags were rebuilt; the original installation's running Pod UIDs were unchanged.

The real dedicated Ollama 0.13.5 container used the existing read-only llama3.1 model mount and host loopback port 11439. All task inputs were synthetic. No Secret values, real external API credentials or host environment contents were collected.

Current main includes optional durable authenticated member-mode recovery. **This campaign uses the reference configuration without member config or PostgreSQL Work storage.** Its restart-loss result cannot be applied to member mode, production, or EKS. The #180 Epic is now closed; this report does not assert that its delivered optional storage is absent or broken.

## Results

[Matrix results](matrix-results.json), [Provider results](provider-results.json), [independent Provider response windows](provider-completion-windows.json) and the raw archive contain the actual outputs.

| Fault / control | Observed outcome | Independent evidence / interpretation |
| --- | --- | --- |
| Healthy control | Succeeded | Real agent-sandbox worker; Provider completed 21 → 23. |
| Provider process frozen | Expired, no answer | `docker pause` freezes the actual dedicated process, not a synthetic response. One model attempt ended Cancelled; Provider completions 23 → 23; termination and cleanup succeeded. This models extreme process slowness, not a measured network latency distribution. |
| Queue deadline during freeze | Expired before allocation | No backend identity, model attempt or completion. Queue deadline was 8s. |
| Queued Work after short freeze | Both blocker and queued Work Succeeded | Actual Pending snapshot before resume. Shared isolated window has four Provider completions for exactly four successful model outcomes, 23 → 27. |
| Delete Running worker | Failed, no answer | Pod deleted using observed UID while real model request was in flight. That one already-started Provider request completed (27 → 28); Work still Failed. No replayed extra request. Termination and cleanup succeeded; a new Work succeeded (28 → 30). Deleting a worker does not prove cancellation of an already accepted Provider request. |
| Controller absent during cold allocation + queue | Pending while absent; both succeed after restore | Real controller Deployment scaled to zero and all controller Pods confirmed absent. Shared Provider window 30 → 34 for four successful model outcomes after restore. |
| Controller absent while Work already Running | Succeeded with real Provider result | Worker/control-plane/Gateway can continue the established execution path without a controller. Provider 34 → 36. Recorded cleanup succeeded; snapshots distinguish asynchronous Pod deletion from claim outcome. Restored controller was observed separately. |
| Sustained cold-controller outage | Cold allocation and queued Work both Expired | No backend identities or model attempts. The 15s allocation deadline is observed after the adapter's bounded synchronous allocation wait (about 60s); it is not immediate cancellation of that backend operation. **Unresolved cleanup gap**, detailed below. |
| Restart application control-plane Pod with Running and queued Work | Both old records return HTTP 404 | Pre-restart Running/Pending evidence retained. CLI query failure retained. Outcome is **unknown / lost current-session record**, not Deny, Failed or Succeeded. This is the configured reference mode's storage limit. The current-main post-restart resource snapshot contains no old SandboxClaim; a warm pool remains. New Work succeeds (36 → 38). This tests Pod interruption, not Kubernetes API-server failure, safe revision rollout or PostgreSQL crash recovery. |
| Post-terminal Model + Tool calls | Rejected for Succeeded, Failed and Expired | Supplementary real-backend composition probe captures the production Service's legitimate callback while real worker execution runs. Replaying with a fresh context after terminal status returns a non-running-claim error. Six replays total; facts unchanged and independent Provider counter stays 42 → 42 in all three windows. These are rejection errors before dispatch, not fabricated typed Deny facts. |
| Actual Provider stopped and restarted | Succeeded → Failed → Succeeded | Container state plus kind-node connectivity/refusal probes. Final completed counts: 42 → 44, 44 → 44, 44 → 46. Same Work spec, unique metadata names; failure has no answer; all three terminate and clean up. |

The terminal probe is supplementary **real-backend** evidence, not a mock unit test and not a replay through the installed HTTP API. No product endpoint or fault hook was added. The installed API does not expose arbitrary trusted invocation-context replay. See terminal-succeeded.json, terminal-failed.json and terminal-expired.json.

## Invariant conclusions and residual gaps

- **No fabricated success:** proven in the measured windows. Successful model outcomes match the dedicated Provider's HTTP 200 completion counts exactly, grouping the two queue windows explicitly. Every non-success has no final answer.
- **No authority expansion:** executable verifier compares issued effective authority before/after each installed run; unchanged. This is fault-path evidence, not a new multi-user authorization certification.
- **No mock/memory model fallback observed:** outage gets one failed attempt, real backend identity and no fabricated result. No dedicated universal fallback-attempt counter exists. Source composition selects one real model client and returns its failure; no independent all-target network capture was performed. Do not turn missing instrumentation into a universal zero-attempt claim.
- **No hidden model retries:** exact response counts match successful model outcomes in isolated observed windows. The supported reference tools are synthetic read-only git.read. A real mutating external-tool side-effect replay invariant is **not applicable/proven here**, because that capability is not configured; no claim about an untested side-effect adapter is made.
- **Post-terminal access:** real Service/real runtime replay rejected for all three terminal phases, with zero external model-call delta. This does not claim replay of lost historical context inside the restarted installed process.
- **Reference restart evidence:** unknown/lost is retained explicitly; optional member-mode durability is outside this configuration. No old Work is automatically resubmitted by the harness after restart.
- **Cleanup:** passed for identified normal, slow and worker-deletion runs. Sustained allocation failure is unresolved; it must not be labelled CleanupSucceeded.

### Confirmed cleanup gap: E8 follow-up proposed

With the controller stopped before first Work, the cold-allocation Work reaches Expired without an observed backend identity. Its upstream SandboxClaim is retained. After controller recovery, [the late resource snapshot](current/cold-late.stdout) shows that same claim assigned a real sandbox/Pod, although the Agenova Work remains Expired. It has no execution authority or Provider invocation in this evidence, but release is not confirmed and the idle allocation remains until operator cleanup.

Classification: **P2 operational resource-cleanup gap**, not a demonstrated credential disclosure or authority bypass. The adapter deliberately retains an unobserved allocation rather than deleting an unknown worker; the missing result is a reconciled, visible cleanup outcome. Suggested owner: E8 [#12](https://github.com/wunderforge/agenova/issues/12), related existing cleanup-proof ticket [#51](https://github.com/wunderforge/agenova/issues/51). A narrow follow-up draft is in publication-draft.md; no new issue has been published pending user review. E15 did not repair this behavior.

The older `ae24a67` campaign also left a Running allocation after control-plane interruption. The rebuilt current-main snapshot did **not** retain that old SandboxClaim. Do not carry the historical orphan conclusion forward to current main. Reference-session evidence loss remains observable in both.

## Raw evidence and reproducibility

[raw-current.tar.gz](raw-current.tar.gz) contains the current installed campaign, sustained-controller campaign, three real-backend terminal probes and actual Provider outage run. [raw-manifest.json](raw-manifest.json) lists every member's SHA-256 and size; raw-current.tar.gz.sha256 hashes the archive. It deliberately excludes kubeconfig, client private keys and host files. Original stdout/stderr, API JSON, CLI JSON, object snapshots and command argv/exit/time receipts are retained without trimming raw output.

Extract into a fresh directory, then run the retained-evidence gate:

```bash
mkdir -p .tmp/e15-199-verify
tar -xzf docs/evidence/199/full/raw-current.tar.gz -C .tmp/e15-199-verify
python3 harness/security/verify-failures.py --campaign .tmp/e15-199-verify/campaign \
  --cold .tmp/e15-199-verify/cold --terminal .tmp/e15-199-verify/terminal
```

See [replay.md](replay.md) for environment setup and live commands. Every cluster mutation is local and explicitly selected; no default-context write is used.

## Gates and delivery status

- Current-main installed matrix, sustained controller failure, Provider stop/recovery and all three supplementary real-backend terminal probes completed successfully as experiments. The resource-cleanup finding and unknown/not-configured cells remain findings, not test passes.
- `python3 harness/security/verify-failures.py ...` passed (focused-current.log).
- `go test ./harness/security/terminalprobe` compiles the added command; its actual behavioral proof is the real executions, not this compile check (focused-compile.log).
- `pwsh -NoProfile -File ./scripts/check.ps1 -All` exited 0 on current main, including **56** browser smoke tests (check-all-current.log). Browser smoke is repository coverage, not rendered proof of a live fault UI.
- Final Docs check and exact-ID cleanup receipts accompany delivery. The original kind Pod UID comparison and host Ollama health are checked after cleanup; EKS was untouched.
- Test execution now covers the #199 requested component classes. Ticket/PR publication and the proposed follow-up still require user approval; #199 is not closed and universal unsupported claims are not accepted.
