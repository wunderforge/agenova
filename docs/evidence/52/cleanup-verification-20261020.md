# #52 cleanup verification — 20 October 2026

## Scope

Follow-up validation of the cleanup fix in `cab7e4b`, after integrating `origin/main` (`d2b3d90`). This does not replace the 5 October boundary campaign or claim current-main runtime isolation: the existing local deployment was not upgraded.

## Focused checks

- `bash harness/security/test-startup-window.sh`: PASS, four cases (invalid mode, invalid attempts, unrelated marker preserved, existing claim refused and earlier owned claim cleaned).
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`: PASS.

## Real kind cleanup

Explicit context `kind-agenova-k8s-lab`, namespace `agenova-system`; existing disposable local cluster, Kubernetes v1.37.0, Docker 29.8.0. The control-plane and warm worker existed before the test; no deployment or network policy was changed.

```bash
bash harness/security/startup-window.sh --context kind-agenova-k8s-lab --namespace agenova-system --mode pool --marker cleanup-20261020-52 --attempts 2
kubectl --context kind-agenova-k8s-lab -n agenova-system get sandboxclaims -o name
kubectl --context kind-agenova-k8s-lab -n agenova-system get pods -o name
```

Probe exited 0. Raw probe output:

```text
# mode=pool pod=e15-startup-cleanup-20261020-52 created=2026-10-20T11:56:24Z containerStarted=2026-10-20T11:56:25Z target=192.168.65.254:11434
attempt	epoch	rc	detail
1	1792497385	1	wget: server returned error: HTTP/1.1 404 Not Found
2	1792497386	1	wget: download timed out
# ollama log lines with marker cleanup-20261020-52: before=0 after=1

```

After the script exited, `get sandboxclaims -o name` returned no objects: both this run's consume claim and probe claim were removed. Pods remaining:

```text
pod/agenova-control-plane-56bf58ffcb-w4nwm
pod/agenova-pool-pool-engineer-t77cc
```

The real test verifies successful cleanup against the API server; preservation of an unrelated claim and name collision handling are covered by the fake-kubectl regression, not a concurrent real-cluster campaign. No independent unrelated claim was created in this live run.

The first direct probe reached Ollama (HTTP 404; unique-marker host-log count 0 → 1); the second timed out. This remains an observed startup reachability gap on this existing local deployment. It is not evidence of remediation or EKS behavior.

## Repository gate

`pwsh -NoProfile -File ./scripts/check.ps1 -All`: PASS (exit 0), including Go vet/tests, 22 UI contract tests, 135 UI unit tests and 69 Playwright smoke tests. [Full gate log](cleanup-full-gate-20261020.log). The first sandboxed attempt stopped at dependency DNS resolution; the permitted rerun completed successfully.
