# #51 controlled Agent Sandbox kind run

- Date: 2026-09-14 (Sydney)
- Context/cluster: `kind-agenova-k8s-lab` / `agenova-k8s-lab` (pre-existing; this test did not create or delete it)
- Namespace: `default`; all test resources use a unique `run51-...` prefix
- Upstream controller: `registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.4.6`, Ready 1/1
- SandboxClaim CRD: served/stored `v1alpha1`
- kind: v0.32.0; kubectl client v1.32.2; Kubernetes server v1.36.1
- Test image ID: `sha256:b53b65c456e458fcf2db233a2d5c03bdfcffb36cf3e2a6f0feb93c300eecec15`
- Result: pass after independent review fixes for Cleanup/Start and concurrent-Terminate races plus uncertain-start cancellation

## Reproduce

From the repository root with Docker Desktop running and an explicit pre-verified kind context:

```powershell
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
go test -count=1 -v -tags 'integration controlled' -timeout 5m ./harness/integration/agentsandbox/ -run '^TestControlledRuntimeBackend_Kind$' -args -kube-context kind-agenova-k8s-lab -namespace default
.\scripts\check.ps1 -All
```

The test uses a separate `controlled` build tag so the ordinary `check.ps1 -Integration` gate does not require its disposable image. It creates unique SandboxTemplate, warm pool and claim through the adapter. It uses the upstream-assigned worker name for every subsequent operation. Cleanup checks claim, Sandbox and Pod absence with independent kubectl queries and removes only its own pool/template; it does not invoke cluster teardown. The full repository gate passed, including frontend browser smoke (7/7).

## Observed flow

```text
allocated claim=run51-18d52cc44b36a9f0-claim
  worker=agenova-pool-run51-18d52cc44b36a9f0-pool-wtwvn backend=agent-sandbox
Ready is infrastructure-only for that claim/worker
actual child-task result:
  state=running claim=run51-18d52cc44b36a9f0-claim result=probe-b680d195c808ea30
task process stopped before resource deletion:
  state=stopped claim=run51-18d52cc44b36a9f0-claim result=probe-b680d195c808ea30
confirmed cleanup: same claim/worker, released=true
TestControlledRuntimeBackend_Kind PASS (6.07s)
```

The result is derived from the claim ID by the real test child process; Start waited for its acknowledgement and then checked status. Terminate waited for child exit and checked stopped status while the Pod still existed. Cleanup was a later, separate operation. Unit tests cover wrong/stale identities, not-Ready, duplicate Start, failed/uncertain stop, uncertain Start cancellation, Cleanup/Start serialization, failed-Cleanup no-restart, and concurrent-Terminate serialization.

## Limits and next integration

This proves a bounded opt-in worker protocol using `kubectl exec`, not native upstream Start/Terminate or arbitrary coding-agent images. `SpikeAdapter` retains `ErrUnsupported` for ordinary images. The control channel and allocation map are process-local; no production workload identity, atomic upstream binding guard, or adapter-restart reconstruction is established. `FilesystemEvidenceUnsupported` remains correct: this run did not probe inside/outside workspace, cross-claim access or credential/network isolation. #51 therefore remains open for its full filesystem and robustness acceptance. #31 will connect the run service after its separate delivery.

The kubectl client/server minor version difference emits a skew warning; commands succeeded, but a supported client version should be used for repeatable production evidence.
