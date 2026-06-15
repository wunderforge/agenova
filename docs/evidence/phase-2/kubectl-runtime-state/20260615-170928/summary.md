# Evidence Summary

- Phase: phase-2
- Gate: kubectl-runtime-state
- Date: 2026-06-15T17:09:29.4590359+10:00
- Branch: worker/phase2-agent-sandbox-adapter
- Commit: 0844fb5167b19f7738e0f1056ea2224128e4b2fd
- Command: kubectl get crds; kubectl -n agent-sandbox-system get pods; kubectl get nodes

Raw output: `output.txt`

## Details

- Cluster context: kind-agenova-k8s-lab (kind, k8s v1.36.1)
- Upstream Agent Sandbox controller: agent-sandbox-controller-6447bf8dc4-8r29x — 1/1 Running
- CRDs confirmed present:
  - sandboxclaims.extensions.agents.x-k8s.io
  - sandboxes.agents.x-k8s.io
  - sandboxtemplates.extensions.agents.x-k8s.io
  - sandboxwarmpools.extensions.agents.x-k8s.io

## E2E Observed Kubernetes Events (from claim-lifecycle-e2e test run)

During the e2e test (see claim-lifecycle-e2e evidence), the following Kubernetes
events confirmed real claim lifecycle behavior:

- SandboxAdopted: controller adopted warm pool sandbox into claim
- Pod Scheduled → Pulled → Created → Started (Pending→Running)
- Killing / Stopping container: triggered by SucceedClaim (adapter delete)
- New pod created immediately after deletion: OnReplenish strategy confirmed

## RuntimeBackend Boundary

Test resources were created and deleted only by the SpikeAdapter via kubectl.
No upstream CRD type names or API group strings appear outside
internal/runtime/agentsandbox/. Confirmed by check.ps1 -All Test-RuntimeBoundary.

Result: pass
