# Evidence Summary

- Phase: phase-2
- Gate: kubectl-runtime-state
- Date: 2026-06-15T17:08:54.8157446+10:00
- Branch: worker/phase2-agent-sandbox-adapter
- Commit: 0844fb5167b19f7738e0f1056ea2224128e4b2fd
- Command: `kubectl --context kind-agenova-k8s-lab get sandboxtemplates,sandboxwarmpools,sandboxclaims -A -o wide; kubectl --context kind-agenova-k8s-lab get pods -A -o wide; kubectl --context kind-agenova-k8s-lab -n agent-sandbox-system get pods; kubectl --context kind-agenova-k8s-lab get events --sort-by=.lastTimestamp`

Raw output: `output.txt`

Result: pending
