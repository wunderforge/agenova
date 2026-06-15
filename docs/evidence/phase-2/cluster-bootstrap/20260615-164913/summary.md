# Evidence Summary

- Phase: phase-2
- Gate: cluster-bootstrap
- Date: 2026-06-15T16:49:13.6192551+10:00
- Branch: worker/phase2-upstream-install
- Commit: 4ab2f5890eca3d695cbc99bb0d843b071c04ac36
- Command: `kubectl --context kind-agenova-k8s-lab cluster-info; kubectl --context kind-agenova-k8s-lab get nodes -o wide`

Raw output: `output.txt`

## Details

- Cluster type: kind (Kubernetes in Docker)
- Cluster name: `agenova-k8s-lab`
- Kubernetes version: v1.36.1
- Node: `agenova-k8s-lab-control-plane` — Ready, control-plane role
- Container runtime: containerd 2.3.1
- Tool versions confirmed: docker 28.1.1, kind v0.32.0, kubectl v1.32.2

Result: pending

Result: pass
