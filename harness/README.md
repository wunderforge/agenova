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

## `spike/`

`spike/` holds scaffolding that de-risks a later ticket rather than proving the current contract: it does **not** run in `.\scripts\check.ps1 -All`, may stand up or depend on external substrate (a local `kind` cluster, an upstream controller install), and is expected to be revisited or retired once the ticket it feeds is delivered. `spike/agent-sandbox-substrate/` (E8-T3) reproduces the pinned upstream Agent Sandbox lifecycle on a disposable `kind` cluster so E8-T4 can build the Agenova adapter proof on a known substrate; its runbook is `spike/agent-sandbox-substrate/RUNBOOK.md`.

## Security boundary campaign (#198)

`security/scriptedworker/` supplies a test-only controlled-v1 worker for explicit disposable installs. The Python campaign drivers record CLI JSON and independent Ollama counts; they do not run in the default gate. See [the evidence report](../docs/evidence/198/summary.md) for replay steps, cleanup and open acceptance gaps.

`go test -race ./harness/security/boundaryprobe` runs supplementary in-memory tests for queueing, terminal refusal and downstream Gateway Deny. These use an injected Executor and do not replace installed-worker or prompt-injection evidence.

The optional `security/supplement-evidence.py` and `security/verify-198-ui.mjs` runners add positive-controlled allocation watches, bounded canary scanning and real rendered evidence parity. They require an explicit disposable install; replay steps and limits are in [the supplement](../docs/evidence/198/supplement/summary.md).
