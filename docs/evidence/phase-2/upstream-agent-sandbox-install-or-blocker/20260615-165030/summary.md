# Evidence Summary

- Phase: phase-2
- Gate: upstream-agent-sandbox-install-or-blocker
- Date: 2026-06-15T16:50:31.4639854+10:00
- Branch: worker/phase2-upstream-install
- Commit: 4ab2f5890eca3d695cbc99bb0d843b071c04ac36
- Command: `kubectl --context kind-agenova-k8s-lab -n agent-sandbox-system get pods -o wide; kubectl --context kind-agenova-k8s-lab get crds`

Raw output: `output.txt`

## Install Details

- Upstream project: Kubernetes SIG Apps Agent Sandbox (formal subproject)
- Go module: `sigs.k8s.io/agent-sandbox` (published May 14, 2026)
- Version installed: v0.4.6 (latest stable as of 2026-06-15)
- Official repo: https://github.com/kubernetes-sigs/agent-sandbox
- Official docs: https://agent-sandbox.sigs.k8s.io/docs/getting_started/
- Go packages: https://pkg.go.dev/sigs.k8s.io/agent-sandbox

### Install commands actually executed

```
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v0.4.6/manifest.yaml --context kind-agenova-k8s-lab
kubectl apply -f https://github.com/kubernetes-sigs/agent-sandbox/releases/download/v0.4.6/extensions.yaml --context kind-agenova-k8s-lab
```

### Resources created

- namespace/agent-sandbox-system
- deployment.apps/agent-sandbox-controller (1/1 Running)
- customresourcedefinition.apiextensions.k8s.io/sandboxes.agents.x-k8s.io
- customresourcedefinition.apiextensions.k8s.io/sandboxclaims.extensions.agents.x-k8s.io
- customresourcedefinition.apiextensions.k8s.io/sandboxtemplates.extensions.agents.x-k8s.io
- customresourcedefinition.apiextensions.k8s.io/sandboxwarmpools.extensions.agents.x-k8s.io

### RuntimeBackend boundary check

The upstream CRD group is `agents.x-k8s.io` and `extensions.agents.x-k8s.io`. These types must only appear inside `internal/runtime/agentsandbox/` (the allowed adapter path). Application-facing Agenova APIs remain on `SandboxClaim` (Agenova type), not `sandboxclaims.extensions.agents.x-k8s.io`. The runtime boundary check in `check.ps1 -All` confirms no leakage.

Result: pending

Result: pass
