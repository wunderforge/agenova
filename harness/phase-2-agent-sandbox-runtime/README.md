# Phase 2: Agent Sandbox Runtime Harness

This harness verifies that the `internal/runtime/agentsandbox` SpikeAdapter can
exercise a real Kubernetes claim lifecycle using the upstream Agent Sandbox
controller.

## Prerequisites

- `kind-agenova-k8s-lab` cluster running with Agent Sandbox v0.4.6 installed.
- `kubectl` in PATH and configured for the `kind-agenova-k8s-lab` context.
- Go 1.22+.

## Cluster Verification

```powershell
kubectl --context kind-agenova-k8s-lab cluster-info
kubectl --context kind-agenova-k8s-lab get crds | Select-String agents.x-k8s.io
kubectl --context kind-agenova-k8s-lab -n agent-sandbox-system get pods
```

## Static Scenarios

`scenarios/smoke-claim-lifecycle/` documents the expected lifecycle sequence for
a minimal create -> bind -> start -> succeed claim flow.

## E2E Test

The integration test is in `e2e/claim_lifecycle_test.go` and requires the
`integration` build tag plus the cluster context:

```powershell
Set-Location D:\Projects\2026bigdream\...\agenova
go test -v -tags integration -timeout 5m ./harness/phase-2-agent-sandbox-runtime/e2e/ `
  -kube-context kind-agenova-k8s-lab -namespace default
```

The test records its own evidence under `docs/evidence/phase-2/`.

## Semantic Gap Summary

See `internal/runtime/agentsandbox/doc.go` for the three confirmed gaps:

1. No upstream phase field - adapter maps conditions to Agenova phases.
2. No SucceedClaim/FailClaim primitives - terminal state is adapter-local only.
3. Pool status granularity - upstream does not break down idle/bound/running counts.
