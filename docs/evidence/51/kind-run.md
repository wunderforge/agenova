# #51 controlled Agent Sandbox kind run

- Date: 2026-09-15 (Sydney)
- Context/cluster: `kind-agenova-k8s-lab` / `agenova-k8s-lab` (pre-existing; this test did not create or delete it)
- Namespace: `default`; all test resources use a unique `run51-...` prefix
- Upstream controller: `registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.4.6`, Ready 1/1
- SandboxClaim CRD: served/stored `v1alpha1`
- kind: v0.32.0; kubectl client v1.32.2; Kubernetes server v1.36.1
- Test image ID: `sha256:798e4b9c4b798dc70a9165f7e388783c03f98f7feff7e752a82dd1afe4c8f249`
- Result: pass against the code committed as `8d4eee7`, after both Codex review rounds were addressed

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
allocated claim=run51-18d55912c415cd90-claim
  worker=agenova-pool-run51-18d55912c415cd90-pool-sbj86 backend=agent-sandbox
Ready is infrastructure-only for that claim/worker
actual child-task result:
  state=running claim=sha256:a5a1e9d204669944be4b3133668189a35e0c74d0315e53996b2aae22f8c9f54e
  result=probe-9af335af96434d0e
task process stopped before resource deletion:
  state=stopped claim=sha256:a5a1e9d204669944be4b3133668189a35e0c74d0315e53996b2aae22f8c9f54e
  result=probe-9af335af96434d0e
confirmed cleanup: same claim/worker, released=true
TestControlledRuntimeBackend_Kind PASS (7.43s)
```

The adapter derives the fixed control token from the system-issued claim ID, and the real child process derives its result from that token. Start first waited for the non-mutating control-server status, then required the start and running acknowledgements to preserve the same token-bound result. Terminate waited for child exit and checked stopped status while the Pod still existed. Cleanup was a later, separate operation. Unit tests cover arbitrary claim IDs, wrong/stale identities, not-Ready, control-server startup, inconsistent results, duplicate Start, failed/uncertain stop, pre-start cancellation, Start/Terminate and Cleanup/Start serialization, shared concurrent operation outcomes, failed-Cleanup no-restart, and idempotent release.

## Limits and next integration

This proves a bounded opt-in worker protocol using `kubectl exec`, not native upstream Start/Terminate or arbitrary coding-agent images. `SpikeAdapter` retains `ErrUnsupported` for ordinary images. The control channel and allocation map are process-local; no production workload identity, atomic upstream binding guard, or adapter-restart reconstruction is established. `FilesystemEvidenceUnsupported` remains correct: this run did not probe inside/outside workspace, cross-claim access or credential/network isolation. #51 therefore remains open for its full filesystem and robustness acceptance. #31 will connect the run service after its separate delivery.

The kubectl client/server minor version difference emits a skew warning; commands succeeded, but a supported client version should be used for repeatable production evidence.
