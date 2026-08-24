# Agent Sandbox Test Substrate — Runbook (E8-T3 / #50)

Reproduces the upstream [Agent Sandbox](https://agent-sandbox.sigs.k8s.io/) test substrate on a
disposable local `kind` cluster: a pinned upstream install, and one minimal
`SandboxTemplate` → `SandboxWarmPool` → `SandboxClaim` lifecycle observed and
cleaned up with plain `kubectl`. **This is upstream-native only.** It never
touches Agenova's `RuntimeBackend`, claim types, or
`internal/runtime/agentsandbox` adapter — that proof is E8-T4 (#51).

## Non-goals

- No Agenova claim, adapter, or API type is created or exercised here.
- No production/customer-cluster install flow.
- No claim-governance proof — see `docs/backends/agent-sandbox.md` and #51 for that.

## Prerequisites

| Requirement | Check | Notes |
| --- | --- | --- |
| Docker daemon running | `docker info` | Start Docker Desktop (or your engine) first; the script fails loudly if it can't reach it. |
| `kind` | `kind version` | Any recent release works; the script pins the *Kubernetes workload* (Agent Sandbox), not kind itself. |
| `kubectl` | `kubectl version --client` | |
| `curl` | `curl --version` | Used to fetch the pinned upstream release manifests. |
| Network access | — | Needs `github.com` (release assets) and `registry.k8s.io` (controller image). |

## Run it

```sh
./harness/spike/agent-sandbox-substrate/reproduce.sh all
```

This runs `up` (create cluster + install pinned Agent Sandbox v0.4.6) then
`smoke` (create/observe/terminate/clean up one claim) then `down` (delete the
cluster). It is safe to re-run: cluster creation and namespace creation are
idempotent, and `down` is a no-op if nothing exists.

### Individual phases (for iterative debugging)

```sh
./harness/spike/agent-sandbox-substrate/reproduce.sh up      # cluster + controller only, cluster stays up
./harness/spike/agent-sandbox-substrate/reproduce.sh smoke   # requires `up` to have run first
./harness/spike/agent-sandbox-substrate/reproduce.sh status  # read-only: context, cluster, CRDs, controller readiness
./harness/spike/agent-sandbox-substrate/reproduce.sh down    # delete only the kind cluster this script created
```

## What gets created, and what cleanup removes

- kind cluster `agenova-k8s-lab` (kube context `kind-agenova-k8s-lab`) — the
  same context name `scripts/check.ps1 -Integration -KubeContext
  kind-agenova-k8s-lab` already expects, so this substrate is reusable by
  that gate later.
- Namespace `agent-sandbox-smoke` holding the three smoke fixtures in
  `manifests/`.

`down` deletes **only** the `agenova-k8s-lab` kind cluster. `smoke`'s own
cleanup step deletes **only** the `agent-sandbox-smoke` namespace and its
contents. Nothing else on your machine or kubeconfig is touched. Before any
mutating step (including cleanup), the script re-resolves
`kubectl config current-context` and refuses to proceed if it isn't exactly
`kind-agenova-k8s-lab`.

## Safety / honesty behavior

- Missing prerequisites (docker, kind, kubectl, curl) abort immediately with
  a specific message and non-zero exit — never a silent pass.
- If the `SandboxClaim` never reaches `Ready=True`, or the sandbox pod never
  terminates after claim deletion, the script reports an explicit failure
  (with `kubectl get`/`get events` dumps) rather than treating a timeout as
  success.
- A context mismatch aborts every mutating subcommand, including `down`.

## Evidence

Every invocation writes:

- `docs/evidence/E8-T3/agent-sandbox-substrate/output.txt` — full command output.
- `docs/evidence/E8-T3/agent-sandbox-substrate/summary.md` — ticket, gate, date,
  branch, commit, pinned version, and pass/fail result, matching the shape
  `scripts/evidence.ps1` already uses elsewhere in the repo.

## Observed upstream behavior (v0.4.6, informs #48)

Deleting a warm-pool-backed `SandboxClaim` **recycles** its `Sandbox` back
into the `SandboxWarmPool` for reuse — it does not terminate the pod. The
sandbox is only actually torn down once the pool (and its template) is
deleted too. `smoke` accounts for this: it tears down the claim, pool, and
template before asserting the pod count reaches zero.

## Troubleshooting

- **`docker daemon is not reachable`** — start Docker Desktop and re-run.
- **Controller image pull stuck / `ImagePullBackOff`** — check network access
  to `registry.k8s.io`; `kubectl -n agent-sandbox-system get pods` and
  `describe` the pod for details.
- **Controller `CrashLoopBackOff`** — check
  `kubectl -n agent-sandbox-system logs deploy/agent-sandbox-controller`;
  this usually means the installed CRD version doesn't match the pinned
  controller version (don't hand-edit the downloaded manifests).
- **"resolved kube context is ... expected kind-agenova-k8s-lab"** — another
  cluster/context is currently active; run
  `kubectl config use-context kind-agenova-k8s-lab` (or `down` the stray
  cluster if it isn't needed) and retry.
- **`SandboxClaim never reached Ready=True`** — usually means the warm pool
  hasn't provisioned yet or the extensions controller isn't running; check
  `kubectl -n agent-sandbox-smoke get sandboxwarmpool,pod -o wide` and
  `kubectl -n agent-sandbox-system logs deploy/agent-sandbox-controller`.
