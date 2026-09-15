# #136 governed application run on kind

- Date: 2026-09-15
- Context: `kind-agenova-k8s-lab`
- Disposable namespace: `agenova-run136-629fa8c` (deleted after the run)
- Docker Server: 28.1.1
- kind: v0.32.0
- kubectl client: v1.32.2
- Worker image: `agenova-testworker:kind` (`sha256:798e4b9c4b798dc70a9165f7e388783c03f98f7feff7e752a82dd1afe4c8f249`)

## Command

```powershell
kubectl --context kind-agenova-k8s-lab create namespace agenova-run136-629fa8c

go test -count=1 -v -tags 'integration controlled' -timeout 5m `
  ./harness/integration/agentsandbox/ `
  -run '^TestGovernedApplicationRun_Kind$' -args `
  -kube-context kind-agenova-k8s-lab `
  -namespace agenova-run136-629fa8c
```

## Observed output

```text
=== RUN   TestGovernedApplicationRun_Kind
denied before allocation: request=fix-payment-timeout principal=user:team-b-engineer decision=Deny backendCalls=0
governed kind run complete: request=fix-payment-timeout decision=Allow claim=claim:fix-payment-timeout:issuance:cb71a09da1c3a0f4ff16dd25bfb758fa worker=agenova-pool-reference-engineer-pool-kfdnc phase=Succeeded
controlled worker result: state=running claim=sha256:aded0582e0c1ed5a214740384dc7a9cbaf27009e022bb658de0f992eef12facb result=probe-7fbc974b060d89bf
backend calls: Allocate -> Observe -> Start -> Terminate -> Cleanup
--- PASS: TestGovernedApplicationRun_Kind (26.94s)
PASS
```

The test independently listed the namespace after the denied request and after application cleanup. No `SandboxClaim` remained, and both the released Sandbox and Pod identities were absent. The integration-owned template and warm pool were deleted by test cleanup; the empty namespace was then deleted explicitly.

## What this proves

- The denied path stops before any backend method or Kubernetes claim workload.
- The allowed path reuses the same ClaimRequest, authorization, authority-resolution, issuance, and `RunService` code as the CLI reference path.
- Only the RuntimeBackend composition changes for Kubernetes.
- Ready is followed by an independently acknowledged controlled worker result; success is followed by confirmed stop and cleanup.

## Remaining boundary

This does not complete #51 filesystem/mount hardening, gateway invocations, durable evidence assembly, the live API/UI, or support arbitrary agent images. The controlled protocol is the opt-in demo bridge documented by #135/PR #134.
