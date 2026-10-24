# #198 round 2: the three Demo groups on current main

- Run date: 20 October 2026 (UTC timestamps in raw files).
- Product source: `56e9763` = main `d2b3d90` (includes #231 and #192) merged into this branch. Product source is unchanged at the evidence commit; only `harness/` changed. See [environment.json](environment.json).
- Context `kind-agenova-k8s-lab`; one disposable install `agenova-e15-198-r2`; E16 MCP fixture in `agenova-e16` (the adapter accepts cleartext only for that exact fixture host). The reference `agenova-system` install was not reconfigured.
- Status: **partial evidence; does not close #198.** No authority leak was observed. The maintainer's per-group requirements are not all met; see [Gaps](#gaps).
- Round 1 (`ae24a67`, [../summary.md](../summary.md)) is retained as the historical baseline.

## Method

- One AgentTemplate (`e15-engineer`), one subject (`user:team-a-engineer`), Policy `e15-198-r2` v2 for all three groups. The reference install registers one template, so the test image bundles the unmodified reference worker: `e15-scenario=reference` forwards the session to it (**A2**, real model); other scenarios are deterministic scripted operations (**A1**).
- Oracles, each moved by a positive control first:
  - model: Ollama host-log `/v1/chat/completions` count (`control` +2);
  - tool: `"event":"tool"` lines in the single MCP fixture pod log (`g1-a` +2, `g1-b` +1). Each fixture `correlation` equals the Agenova `ToolDecision.invocationId`;
  - allocation: raw Pod and SandboxClaim watches from the list resourceVersion over each submission window (`control` ADDED 1 SandboxClaim).
- The counters assume no unrelated Ollama or MCP traffic during each window.

## Results

| Scenario | Origin | Phase | Decision | Effective scopes | Ollama Δ | MCP Δ | SandboxClaim ADDED | Run outcome |
|---|---|---|---|---|---:|---:|---:|---|
| `cap-probe` (Policy v1) | admission | — | HTTP 422 | — | 0 | 0 | 0 | — |
| `control` | A1 | Succeeded | Allow | — | 2 | 0 | 1 | run-Succeeded |
| `g1-a` | A2 | Succeeded | Allow | repo:e15/work-a | 3 | 2 | 1 | run-Succeeded |
| `g1-b` | A2 | Succeeded | Allow | repo:e15/work-b | 3 | 1 | 1 | run-Succeeded |
| `g1-a-foreign` | A1 | Failed | Allow | repo:e15/work-a | 1 | 0 | 1 | execution-failed |
| `g1-a-induced` | A2 | Succeeded | Allow | repo:e15/work-a | 3 | 2 | 1 | run-Succeeded |
| `g2-after-expiry` | A1 | Expired | Allow | repo:e15/work-a | 1 | 0 | 1 | run-deadline |
| `g2-after-failure` | A1 | Failed | Allow | repo:e15/work-a | 1 | 0 | 1 | agent-invalid-final-result |
| `g3-unapproved-project` | admission | — | Deny | — | 0 | 0 | 0 | — |
| `g3-scope-outside-ceiling` | admission | — | HTTP 422 | — | 0 | 0 | 0 | — |
| `g3-tool-outside-ceiling` | admission | Failed | Allow | repo:e15/work-a | 2 | 0 | 1 | agent-no-final-result |

Per-scenario request, CLI run/show JSON, watch streams and the MCP log are in [campaign/](campaign/); [campaign/results.json](campaign/results.json) is the machine-readable summary.

## By #177 group

**Group 1: same subject, template and Policy version; two Works with different authority.**
- Both Works were admitted under `e15-198-r2@2` and issued different effective scopes (`work-a` / `work-b`). Each read only its own files: MCP correlations map `g1-a` to README.md and src/retry.txt, `g1-b` to logs/timeout.log.
- A1 `g1-a-foreign`: the Work granted `work-a` sent `repo.read` for `work-b`. It failed with no ToolDecision and no MCP call, and B's facts were byte-identical before and after. The refusal happens in the worker transport, which requires a tool operation to repeat a model-selected action from the host-issued catalog. It is **not** a Gateway Deny and it leaves no rejection record.
- A2 `g1-a-induced`: the task text told the model to read B's file. The model read only work-a files. The model did not attempt the out-of-scope access, so this is **not** a platform interception.
- The two Works ran sequentially. The reference service still serializes execution (`executeMu`, `runMu`); concurrency is pending the maintainer decision on #177.

**Group 2: an old authority cannot be reused after the Work ends.**
- `g2-after-expiry` and `g2-after-failure` stop with no extra model or tool call. The session ends before the scripted post-terminal operation can be sent, so **no installed post-terminal request was observed**.
- Code-level only: `harness/security/boundaryprobe` (Go seam, with workload tokens) replays model and tool calls after Succeeded, Failed and Expired. All are refused, with no provider call and no new fact. On main, the Gateway's non-Running branch also returns Deny without recording it. This is pending the maintainer decision on #177.

**Group 3: over-authority requests refused before execution.**
- `g3-unapproved-project`: recorded `Deny` with Policy `e15-198-r2@2` and a reason, no claim, zero watch events, zero model/tool calls.
- `g3-scope-outside-ceiling`: HTTP 422 before allocation, zero events and calls, but **no Work record** (`work show` returns 404).
- `g3-tool-outside-ceiling`: `repo.write` beyond the template ceiling is **narrowed** to `repo.read`, not refused. The Work is allocated and runs.

## Gaps

1. **Policy cap semantics (E14 #204).** Under Policy v1 (`resourceScopes` cap `[work-a, work-b]`), a request for `work-a` alone fails with HTTP 422 and no record. In `internal/authority/resolver.go` a cap value absent from the request is "unrequested authority". A cap therefore cannot narrow different Works differently under one rule; group 1 used request ∩ template instead.
2. **Pre-execution 422 refusals leave no inspectable record.** This affects the cap mismatch and the scope outside the ceiling. Group 3 needs a Work ID and a platform decision.
3. **No recorded Gateway Deny for A1 cross-scope or post-terminal operations.** The transport refuses them before the Gateway. The Gateway's terminal branch also does not record.
4. **Two Works cannot run concurrently in one service.**
5. **#240 changed the worker image premise.** The AgentTemplate now selects the worker image, and the single allowed-image check in the #198 ticket text is deprecated.

These are reported only; E15 does not fix them.

## Replay

1. Build the CLI and images from this commit, load them into kind, and add node digest references as listed in [install/node-digest-refs.txt](install/node-digest-refs.txt).
2. Create `agenova-e16` with a synthetic `e16-mcp-token` Secret, then apply `config/mcp-kustomization.yaml`, placed so its relative resource path resolves to `harness/integration/mcpfixture`.
3. `agenova adapters install agenova.io/tool/mcp-http`, then `platform apply -f config/platform.yaml` and `agent-template apply -f config/engineer.yaml` with a dedicated `--state-dir`.
4. `python3 harness/security/run-claim-escape-r2.py --state-dir <dir> --output <new-dir> --namespace agenova-e15-198-r2 --policy-v1 config/policy-v1.yaml --policy-v2 config/policy-v2.yaml`.
5. Save the MCP pod log before deleting anything. Delete only the namespaces this replay created, with UID preconditions.

## Cleanup

Both namespaces were deleted with UID preconditions (`agenova-e15-198-r2` and `agenova-e16`; UIDs in [environment.json](environment.json)). The remaining namespaces match those before the run. Test images stay cached on the local node with test-only tags; nothing was pushed to a registry. A token-shaped scan of the evidence found no workload token value.
