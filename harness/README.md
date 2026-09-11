# Executable Harness

The harness contains behavior-level proof for current Agenova contracts and
clearly labelled experimental regressions.

## Reference E2E

Runs locally against the in-memory backend. The committed MVP evidence covers
single-claim governed calls, fact attribution, and authority ending with the
claim. The same test tree retains experimental parent/child lineage regressions;
those cases are not committed MVP proof.

```powershell
go test -v ./harness/e2e/
```

## Agent Sandbox Integration

Runs only when a compatible Kubernetes cluster and Agent Sandbox installation are available.

```powershell
.\scripts\check.ps1 -Integration -KubeContext kind-agenova-k8s-lab
```

Static fixture files do not count as runtime evidence. Add a harness scenario only when it exercises an acceptance criterion that unit or contract tests cannot prove clearly.
