# Provider outage evidence — #199 pre-Demo RI-3

Historical first slice. The user subsequently authorized the complete matrix; see the [current-main full campaign](full/summary.md).

Measured on 19 October 2026. This is a partial delivery of [#199](https://github.com/wunderforge/agenova/issues/199), following [#177](https://github.com/wunderforge/agenova/issues/177). It does not complete the post-Demo fault matrix or close either issue. No product behavior was changed.

## Environment and fault

Checkout baseline `ae24a67261dff289a7063f7435c7c3616fd36dc0`, branch `codex/199-provider-failure`. Explicit context `kind-agenova-k8s-lab`, Kubernetes v1.37.0, Docker 29.8.0. Dedicated namespace `agenova-e15-199-provider`, UID `6ca1843e-7637-4e40-97d8-c891721851c3`. The original `agenova-system` installation was not reconfigured.

The unmodified reference control-plane and worker images already loaded locally were reused. Exact Docker identities are in images.stdout; actual Pod imageIDs are in pods.before-cleanup.stdout. These are runtime identity evidence, not a fresh reproducible-build attestation from this checkout.

The dedicated real Ollama 0.13.5 container used image digest `sha256:2c9595c555fd70a28363489ac03bd5bf9e7c5bdf2890373c3a830ffd7252ce6d`, ID `802b940856c53358d27306333bd14bc5faac621e348d7357f196befd0e8f300e`. It exposed only host loopback port 11439 and mounted only `/Users/yue/.ollama/models` read-only. `llama3.1:latest` model digest was `46e0c10c039e019119339687c3c1757cc81b9da49709a3b3924863ba87ca666e`.

Fault injection was **actual stop/start of this dedicated provider container**, with the Platform endpoint fixed at `http://host.docker.internal:11439/v1`. No synthetic error response, configuration swap, network proxy, product fault hook or global Ollama shutdown was used. This models provider process unavailability; it does not establish slow-provider, network-partition, GPU-provider or host Ollama service behavior.

## Results

The final campaign is in [verified/results.json](verified/results.json). An earlier successful campaign remains in campaign/ for provenance. Requests share exactly the same `spec`; only `metadata.name` changes to avoid request identity reuse.

| Window | Independent provider state/probe | CLI / claim / Work | Provider completed count | Model attempt/outcome | Cleanup |
| --- | --- | --- | --- | --- | --- |
| Healthy | Running; kind-node HTTP probe succeeds | exit 0 / Succeeded / Succeeded | 4 → 6 | 2 / 2 succeeded | TerminateSucceeded + CleanupSucceeded |
| Outage | Stopped; kind-node HTTP probe fails | exit 1 / Failed / Failed; no result text | 6 → 6 | 1 / 1 failed | TerminateSucceeded + CleanupSucceeded |
| Recovery | Same container restarted; same probe succeeds | exit 0 / Succeeded / Succeeded | 6 → 8 | 2 / 2 succeeded | TerminateSucceeded + CleanupSucceeded |

All were admitted `Allow` and obtained real `agent-sandbox` backend identities. The outage is an execution failure, **not** a permission Deny. No missing evidence was interpreted as Deny or success. Attempt and outcome invocation IDs match one-to-one. Each success has corresponding independent real-provider HTTP 200 completion records. A three-second quiet period followed by terminal re-query found all three complete evidence representations unchanged.

Provider counts use the dedicated container's Ollama `[GIN]` records for `/v1/chat/completions`; HTTP 200 records count completions. Requests that cannot reach a stopped process cannot appear in that process's log. Thus the outage has one Agenova-side failed attempt and zero provider-side arrivals/completions, not zero governed attempts. Healthy/recovery controls establish log sensitivity. There was no unrelated client traffic to this dedicated endpoint during either campaign.

## Claim matrix and limits

| Claim | Conclusion and basis |
| --- | --- |
| RI-3: stopped provider cannot produce successful Work | Observed PASS for this configuration: stopped-state receipt, kind-node refusal, failed provider outcome, terminal Failed, no fabricated answer, zero completion delta. |
| Recovery with unchanged Work spec succeeds | Observed PASS, backed by independent provider completions. |
| Mock/memory fallback | Zero observed fallback results or recorded attempts; both runs use a real agent-sandbox backend. Installed composition (`cmd/agenova-control-plane/main.go`, `internal/adapters/bundled/bedrock.go`) selects one real model client; `internal/modelprovider/adapter.go` returns the transport failure rather than selecting another client. The runtime journal has no dedicated fallback-attempt counter, so an independent universal count of all hypothetical fallback attempts is **not available**. Do not present journal absence alone as that proof. |
| Work termination / cleanup | Both recorded successful in each phase; Kubernetes snapshots retain actual resource state. Pods may remain briefly during asynchronous deletion and warm-pool replenishment; this is not a new claim phase. Final namespace deletion is separately confirmed. |
| Post-terminal governed access denied | **Not independently probed in this installed campaign.** Terminal phase, cleanup and stable evidence are observed; they do not prove a replayed call is denied. The private installed API exposes Work submission/query, not arbitrary trusted invocation-context replay. Shared gateway tests passing in the full gate remain code evidence only. |
| Other component faults / lost history / side-effect replay | Deferred to the post-Demo matrix. No conclusion. |

No broken security invariant was established by this bounded campaign. No follow-up issue or advisory was published. It would be incorrect to mark the entire #199 complete or claim every fault invariant passed.

## Replay

Prerequisites match the [kind + Ollama guide](../../reference-cli-kind-ollama.md). Use a fresh checkout state directory and refuse an existing namespace or provider name. Record the namespace create response and full container ID before running. The harness requires an existing installed environment and does not install or delete it.

```bash
go build -o .tmp/agenova ./cmd/agenova
# Refuse existing namespace/container names; create and save namespace UID.
kubectl --context kind-agenova-k8s-lab create namespace agenova-e15-199-provider -o json
docker run -d --name agenova-e15-199-provider --label agenova.ticket=199 \
  -p 127.0.0.1:11439:11434 \
  -v /Users/yue/.ollama/models:/root/.ollama/models:ro ollama/ollama:0.13.5
.tmp/agenova platform apply -f docs/evidence/199/platform.yaml --state-dir .tmp/state-199 --yes --json
.tmp/agenova policy apply -f deploy/reference/demo/policy.yaml --state-dir .tmp/state-199 --json
.tmp/agenova agent-template apply -f deploy/reference/demo/engineer.yaml --state-dir .tmp/state-199 --json
python3 harness/security/provider-outage.py --container-id <recorded-full-container-id> \
  --output .tmp/e15-199-replay --state-dir .tmp/state-199
```

Each captured `.command.json` supplies exact executed argv and exit code, with paired stdout/stderr. Inspection receipts contain no configured external credentials; only synthetic task inputs were used. No Secret values were read. The model mount stays read-only; Ollama generated only its dedicated container's disposable default files. Do not bulk-read real host logs or environment values for replay.

After capture, stop/remove only the recorded container ID after verifying its ticket label. Delete the namespace through Kubernetes DeleteOptions with the **recorded UID precondition**, then wait for actual deletion. See namespace.delete.stdout, namespace.deleted.stdout, provider.cleanup-remove.stdout and provider.absent.stderr. The initial absence assertion had a case-sensitive text mismatch after successful deletion; cleanup-confirmed.json records an independent successful recheck. Keep the reusable image and host model files. The original platform plan still has `changes=[]`; original Pod identities are recorded after cleanup.

## Gates

- Two real installed campaigns passed their executable behavioral assertions; final campaign includes independent kind-node availability probes.
- Docs gate passed (docs-check.log).
- Initial `-All` stopped after Go checks because the new worktree lacked frontend dependencies (check-all.log). This was an environment prerequisite failure, not a behavioral pass.
- Installed lockfile dependencies and Chromium; final `pwsh -NoProfile -File ./scripts/check.ps1 -All` exited 0 (check-all-final.log), including 52 browser smoke tests. Those browser tests are repository fixture/smoke coverage, not rendered proof of this real outage campaign.
- No UI behavior was changed or claimed; no live UI screenshot was substituted for backend evidence.

Raw Ollama stdout/stderr retain their original trailing spaces. An unrestricted `git diff --cached --check` flags those raw capture lines; the source/Markdown/YAML/JSON-only whitespace check passes. Evidence was not rewritten to silence formatting diagnostics.
