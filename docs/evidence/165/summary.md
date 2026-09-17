# #165 kind + Ollama CLI control-flow verification

Date: 2026-09-17. Target: existing `kind-agenova-k8s-lab/agenova-system`; Agent Sandbox v0.4.6; local Ollama `llama3.1:latest`. No React or separate console process was started.

## Commands and observed result

| Command | Observed |
| --- | --- |
| `agenova platform validate -f deploy/reference/platform.kind.yaml` | valid, revision `sha256:4d2e0047c254463818db3623a1d47576211ee30a6116f78d38decd72b456a4df` |
| `agenova platform plan -f deploy/reference/platform.kind.yaml` | selected `kind-agenova-k8s-lab/agenova-system`, declared adapter and installation changes |
| `agenova platform apply -f deploy/reference/platform.kind.yaml` | ready after reconciliation; installation-components scope; repeat was unchanged |
| `agenova platform status` | same revision and target, zero remaining changes |
| `agenova policy apply -f deploy/reference/demo/policy.yaml` | `reference-default-deny@1` active; identical reapply: already registered |
| `agenova agent-template apply -f deploy/reference/demo/engineer.yaml` | `engineer` registered; identical reapply: already registered |
| `agenova run -f deploy/reference/demo/work.yaml` | Allow, Agent Sandbox worker bound, Succeeded, task-specific answer; post-review final-image rerun used 362 input and 71 output tokens |

The positive request's evidence had `Bound, BackendReady, Running, Succeeded, TerminateSucceeded, CleanupSucceeded`, four allowed/succeeded model invocations, three allowed mock `git.read` invocations, and worker ID `agenova-pool-pool-engineer-9bk5d`. A subsequent namespace query found no remaining SandboxClaim. The model's answer identified a synthetic retry loop resetting a fresh 5-second deadline instead of using the remaining request time.

Negative checks: `denied-work.yaml` received Deny before Claim/worker/model/tool facts; `policy-conflict.yaml` was rejected as same identity with different content. Both registrations were idempotent on identical reapply.

Final-image verification rebuilt `agenova-control-plane:0.1.0`, loaded it into kind, reconciled the Deployment, and observed a newly created ready Pod. `platform status` then reported zero drift. The subsequent Work completed through the installed service and local Ollama; the denied Work had no Claim or runtime/model/tool invocations; querying SandboxClaims again returned none. The full `scripts/check.ps1 -All` gate passed, including Go checks and 48 Playwright browser smoke tests.

## Scope and caveats

The tool observations are mock fixtures; the model call and Sandbox worker were real. The reference identity is fixed Team A, not production authentication. Work/evidence are in-memory and lost on service restart, while registered Policy and AgentTemplate ConfigMaps persist. Do not roll out a new Platform revision while Work is active; drain and revision-stable routing/durable handoff are not implemented in this reference slice. `platform status` assesses installation drift, not provider health or an E2E run. The first attempt failed immediately after Running because the in-pod execution seam required an external kube context; it was fixed to accept an explicit in-cluster kubeconfig, then the next run succeeded. This is recorded to prevent presenting a first-pass-only narrative.
