# #178 Slice 3 and Slice 4: kind campaign c6

- Task: [Epic #178](https://github.com/wunderforge/agenova/issues/178), governed MCP tools. Packet: [task.md](../../../../../work/0178-governed-mcp-tools/task.md); plan: [Slice 3 kind acceptance plan](../../../../../work/0178-governed-mcp-tools/slice3-kind-acceptance-plan.md).
- Gate: the formal kind campaign for Slice 3 (installed MCP path, failure cases, zero-call probes, CLI/API/Portal parity) and Slice 4 (token-required path).
- Date: 2026-10-04 to 2026-10-05 UTC. Preflight and build ran at 19:51 UTC on 10-04; protect to probe ran 01:40–01:45 UTC on 10-05.
- Branch and commit: `codex/e16-slice1` at `fd08ac06eed3e702109c4b55276f2ad110272ff0` (clean tree).
- Cluster: `kind-agenova-k8s-lab`. Install namespace `agenova-e16-system`, fixture namespace `agenova-e16`, model profile `coding-standard` on `qwen2.5:7b` (digest `845dbda0…`, recorded by preflight), host Ollama 0.34.4.

## Command

```sh
A=(--context kind-agenova-k8s-lab --install-namespace agenova-e16-system --state-dir .tmp/e16-c6-state --output .tmp/e16-c6 --model-profile coding-standard)
bash harness/integration/e16/campaign.sh "${A[@]}" preflight
bash harness/integration/e16/campaign.sh "${A[@]}" build
bash harness/integration/e16/campaign.sh "${A[@]}" --yes protect
bash harness/integration/e16/campaign.sh "${A[@]}" --yes load
bash harness/integration/e16/campaign.sh "${A[@]}" fixture
bash harness/integration/e16/campaign.sh "${A[@]}" controlled-read
bash harness/integration/e16/campaign.sh "${A[@]}" install
bash harness/integration/e16/campaign.sh "${A[@]}" work <case>   # positive, admission-deny, n6-timeout, n7-oversize, n8-truncation, token-valid, token-missing, token-wrong
bash harness/integration/e16/campaign.sh "${A[@]}" probe
```

## Result

Every step passed on its first run, with no workaround and no deviation from the runner's procedure. Every Work passed on attempt 1; the five-attempt rule (plan decision 18) was never needed. Durations are in `run.json` (`steps`).

| Case | Result | Evidence |
| --- | --- | --- |
| Phase 2: fixture on its recorded image, controlled read (three sessions, header-less `/mcp-token` initialize answered 401) | pass | `campaign/fixture-identity-start.txt*`, `campaign/controlled-read/` |
| install: 13 commands exit 0, reapply `changed: false`, Deployment revision 1, one ReplicaSet, restarts 0 | pass | `campaign/install/`, `campaign/control-plane-identity.txt*` |
| positive: real MCP reads of `logs/timeout.log` and `src/retry.txt`, every fact right, `qwen2.5:7b` | pass | `campaign/work/positive/attempt-1/` |
| admission-deny: Deny, no claim, no tool or model activity | pass | `campaign/work/admission-deny/attempt-1/` |
| N6: `tool-timeout` on `logs/slow.log`, slow handler settled | pass | `campaign/work/n6-timeout/attempt-1/` |
| N7: `tool-response-too-large` on `logs/full-trace.log` | pass | `campaign/work/n7-oversize/attempt-1/` |
| N8: one truncated successful read of the timeline (`truncated: true` in CLI, API and Portal), answer ends `timeline_complete: no` | pass | `campaign/work/n8-truncation/attempt-1/` |
| token-valid: read through `/mcp-token`, every receipt `auth=ok`. The claimed worker has automount off, no volumes or mounts, no env from a Secret, and nothing mounted under `/var/run/secrets`. The token scan searched the local worker captures | pass | `campaign/work/token-valid/attempt-1/` |
| N13 token-missing: `tool-credential-unavailable`, no server entry | pass | `campaign/work/token-missing/attempt-1/` |
| N14 token-wrong: one `initialize` with `auth=invalid` answered 401, no retry, `tool-credential-rejected` | pass | `campaign/work/token-wrong/attempt-1/` |
| probe: N1, N2a, N2b, N3, N4a/b/c, N5a/b with controls, server-backed N11; every step matches the server log | pass | `campaign/probe/` |
| CLI/API/Portal parity, archived per Work | pass (8 of 8) | `campaign/work/*/attempt-1/parity/` |

`gates/` holds the gates rerun on the clean tree at `fd08ac0` after the campaign (`gates/gates-source.txt`). The focused Go suite, fixture module, runner tests (52 groups), UI unit tests, typecheck and docs check pass. `check.ps1 -All` fails only on the known macOS keyboard smoke `ui/smoke/console.spec.ts:74` (51 passed). Step start and end times are in `steps/timings.txt`.

## Limitations

- This export is reduced and sanitised (`MANIFEST.md`). The worker Pod JSON, environment and mount captures, the access reviews, the Secret records and the reference install's protect records stay in the local campaign directory and are not reproduced here. During the campaign, the runner checked the worker Pod spec and mount table and the access-review answers. The token scan searched every one of these files for the token values.
- The token scan and `restore` ran after this export was written. Their results are recorded in plan section 10, not here.
- N10 (unreachable endpoint on kind) is an Additional-tier case and was not run.
- N3 proves only the service-side correlation guard, not authenticated worker isolation, which belongs to #197.
- The answer rates behind the five-attempt rule were measured off-cluster. On kind, every model-driven Work passed first time, which is one run each, not a rate.
