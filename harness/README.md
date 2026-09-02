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

## `spike/`

`spike/` holds scaffolding that de-risks a later ticket rather than proving the current contract: it does **not** run in `.\scripts\check.ps1 -All`, may stand up or depend on external substrate (a local `kind` cluster, an upstream controller install), and is expected to be revisited or retired once the ticket it feeds is delivered. `spike/agent-sandbox-substrate/` (E8-T3) reproduces the pinned upstream Agent Sandbox lifecycle on a disposable `kind` cluster so E8-T4 can build the Agenova adapter proof on a known substrate; its runbook is `spike/agent-sandbox-substrate/RUNBOOK.md`.
