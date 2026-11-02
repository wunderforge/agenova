# #198 supplementary evidence — 19 October 2026

Baseline: `ae24a67`, same uncommitted #198 harness as the primary campaign. Context `kind-agenova-k8s-lab`; separate disposable installation `e15-198-supplement` in namespace `agenova-e15-198-supplement`. The original `agenova-system` installation was not reconfigured. The image digest and controlled-v1 declaration are captured in platform.yaml and engineer.yaml.

## Observed results

| Item | Result | Evidence |
|---|---|---|
| No allocation on denied submission | PASS within the monitored namespace/window: no Pod or SandboxClaim watch events, no claim/backend identity in Work, no external chat calls | [denial-window.json](denial-window.json), pods.denial-window.events.json, sandboxclaims.denial-window.events.json, denied.cli.stdout, denied.api.json |
| Resource-watch positive control | PASS: subsequent allowed work produced ADDED events for both Pod and SandboxClaim | watch-status.json and both `.watch.jsonstream` files |
| Credential canary refusal | PASS: private API returned HTTP 400, external count unchanged | [canary.api.json](canary.api.json) |
| Canary scanner positive control | PASS: exact marker was detected in a test ConfigMap; that object was then deleted using its returned UID | [canary-scan-positive.json](canary-scan-positive.json) |
| Canary residual scan | PASS in the specified scope: no marker in collected object fields, current control-plane/agent logs or warm-agent environment; every recorded read succeeded | [canary-scan-final.json](canary-scan-final.json) |
| CLI/API/UI consistency | PASS for real Succeeded, Failed and admission Denied Work; CLI and API JSON exactly equal; page status, identifiers, success answer/failure reason match | [ui/result.json](ui/result.json), per-case API JSON/CLI stdout, screenshots below |
| Template ceiling narrowing | PASS: requested git.read + git.write becomes git.read; requested 10 minutes becomes 5 minutes; CLI/API and rendered comparison agree | profile.api.json and [template-narrowing.png](ui/template-narrowing.png) |
| Policy constraints narrowing | **Not implemented in this baseline.** An attempted rule-level constraints field is rejected before registration, active Policy unchanged | [policy-result.json](policy-result.json), policy-constraints-probe.stderr |

The control made two real model calls and Succeeded. The profile scenario made its one allowed control call, then failed on the forged profile without an extra external call. A supported allowed request therefore demonstrates both provider-counter and Kubernetes-watch sensitivity. No unrelated chat traffic was deliberately generated; these are bounded host-log measurements, not a cluster-wide audit.

## Continuous resource watch

For each resource, the harness first reads the raw Kubernetes list and saves its nonempty resourceVersion. It then watches from that exact version using the raw API. This covers events between the initial list and the watch connection. A resourceVersion error, unexpected watch exit, event during the denial window or failed positive control prevents a PASS.

The interval includes submission and a three-second observation period after CLI completion, followed by query/snapshots. The initial state contains only the control-plane Pod and no SandboxClaim. No resource event was observed during this interval. The same watches remain active through later successful allocation and detect creation. Both were explicitly terminated by the harness after capture; exit -15 is intentional. Their stderr is empty, and the captured streams contain no ERROR event.

Development attempts exposed unsupported kubectl watch flags, loss of list metadata in kubectl's generic output and a CRD API-group distinction. Those attempts failed and were retained under `.tmp/e15-198/supplement/`; none was counted as successful evidence. The captured successful run uses raw lists/watches and the actual API groups.

## Canary scope and limits

Only the unique synthetic marker from canary.request.json was searched. The positive-control ConfigMap used that exact marker and was UID-deleted before the rejected request was submitted.

The final scan covers Deployments, ConfigMaps, Pods (including their declared env/volumes), SandboxClaims, Sandboxes, SandboxTemplates, SandboxWarmPools and ServiceAccounts in this disposable namespace. It also reads logs for the current control-plane/agent containers and `printenv` in the warm agent. Reports include object names/readability/match flags, not environment values or log contents.

It excludes Secret values, other namespaces, host files, historical/deleted-container logs and Kubernetes audit logs. A pass means no observed residual in these measured surfaces; it is not proof of absence throughout the cluster. The synthetic positive-control creation itself can legitimately appear in excluded audit history.

## Policy finding

`internal/policy/document.go` exposes only rules; each `policy.Rule` in this baseline contains team, action, project and templateRef. Strict YAML decoding rejects `constraints`. `internal/authority/resolver.go` intersects request and AgentTemplate ceiling; no Policy capability constraint is applied there. This confirms the missing feature rather than proving Policy narrowing.

The malformed/unsupported document probe runs through the operator CLI parser; it is **not** a server-side Policy API request. The installed private API was read before/after to verify the active Policy remained unchanged. A real runtime test of Policy narrowing remains dependent on E14 [#204](https://github.com/wunderforge/agenova/issues/204). Template narrowing must not be reported as that result.

## Rendered proof

These pages were loaded in Connected mode from the real installed service through the loopback Vite proxy, without response mocking. Full JSON equality was checked against CLI for the same request references. Browser page-error collection was empty. All four saved images were visually inspected after the assertions passed.

- [Succeeded Work](ui/control.png): successful result and model data, matching claim/worker IDs.
- [Failed Work](ui/profile.png): generic execution-failed reason shown; successful model control call remains green; no fabricated final Result.
- [Denied Work](ui/denied.png): admission refusal, no access issued, no invented claim or worker ID.
- [Requested versus granted](ui/template-narrowing.png): template removes git.write and caps timeout; evidence explains the source of the narrowing.

These screenshots confirm honest presentation of existing evidence; they do not repair the missing detailed transport-denial records.

## Replay and cleanup

Prepare the digest-pinned test image as in the parent [report](../summary.md#replay). Apply the captured platform and register the captured template with a dedicated state directory. Refuse reuse of any pre-existing namespace. Start explicit loopback services:

```bash
.tmp/agenova platform apply -f docs/evidence/198/supplement/platform.yaml --state-dir .tmp/e15-198/state-supplement --yes --json
.tmp/agenova agent-template apply -f docs/evidence/198/supplement/engineer.yaml --state-dir .tmp/e15-198/state-supplement --json
.tmp/agenova api connect --state-dir .tmp/e15-198/state-supplement --port 18090
# In a second terminal:
AGENOVA_API_URL=http://127.0.0.1:18090 npm --prefix ui run dev -- --port 5178 --strictPort
# Run the observations in a third terminal:
python3 harness/security/supplement-evidence.py --context kind-agenova-k8s-lab --namespace agenova-e15-198-supplement --state-dir .tmp/e15-198/state-supplement --api-url http://127.0.0.1:18090 --output .tmp/e15-198/replay-supplement
node harness/security/verify-198-ui.mjs --base-url http://127.0.0.1:5178 --evidence-dir .tmp/e15-198/replay-supplement --state-dir .tmp/e15-198/state-supplement --cli .tmp/agenova
```

After evidence capture, stop the local services and delete only the namespace created by this replay, using its recorded UID precondition. This run's namespace deletion and confirmation are saved as namespace-cleanup.json and cleanup-confirmed.txt. Reference status still reports changes=[] and installationReady=true. No product code changed, no registry image publication occurred, and no GitHub comment/issue was published.
