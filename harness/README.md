# Executable Harness

The harness contains behavior-level proof for the current Agenova contract.

## Reference E2E

Runs locally against the in-memory backend and proves claim lineage, governed calls, fact attribution, and authority ending with parent scope.

```powershell
go test -v ./harness/e2e/
```

## Agent Sandbox Integration

Runs only when a compatible Kubernetes cluster and Agent Sandbox installation are available.

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

Static fixture files do not count as runtime evidence. Add a harness scenario only when it exercises an acceptance criterion that unit or contract tests cannot prove clearly.
