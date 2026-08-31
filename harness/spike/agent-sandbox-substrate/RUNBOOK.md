# Agent Sandbox Test Substrate — Runbook (E8-T3 / #50)

Reproduces the upstream [Agent Sandbox](https://agent-sandbox.sigs.k8s.io/) test
substrate on a disposable local `kind` cluster: a pinned upstream install, and one
minimal `SandboxTemplate` → `SandboxWarmPool` → `SandboxClaim` lifecycle observed
and cleaned up with plain `kubectl`.

**This is upstream-native only.** It never touches Agenova's `RuntimeBackend`,
claim types, or `internal/runtime/agentsandbox` adapter — that proof is E8-T4 (#51).

## Pinned versions

| Thing | Pin | Where |
| --- | --- | --- |
| Agent Sandbox | `v1.0.0` (v1beta1 APIs) | `AGENT_SANDBOX_VERSION` in `reproduce.sh` |
| `kind` (fallback only) | `v0.33.0` | `KIND_FALLBACK_VERSION`; used **only if `kind` is not already on `PATH`** |
| `kubectl` (fallback only) | `v1.34.0` | `KUBECTL_FALLBACK_VERSION`; used **only if `kubectl` is not already on `PATH`** |

An existing `kind` / `kubectl` on `PATH` is used as-is and its version is
recorded. A missing one is downloaded from its official release, checksum-verified
against the published `.sha256sum` / `.sha256`, and cached under the gitignored
`.tmp/agenova-k8s-lab-tools/` — the script never installs into your system `PATH`.

## Prerequisites

| Requirement | Check | Notes |
| --- | --- | --- |
| Docker daemon running | `docker info` | Start Docker Desktop (or your engine) first; the script fails loudly if it can't reach it. |
| `curl` | `curl --version` | Used to fetch pinned release manifests / binaries. |
| `kind` | optional | Installed on demand (pinned, checksum-verified) if absent. |
| `kubectl` | optional | Installed on demand (pinned, checksum-verified) if absent. |
| Network access | — | Needs `github.com`, `dl.k8s.io`, and `registry.k8s.io`. |

## Run it

```sh
./harness/spike/agent-sandbox-substrate/reproduce.sh all
```

`all` = `up` (create cluster + install pinned Agent Sandbox) → `smoke`
(create/observe/terminate/clean up one claim) → `down` (delete the cluster). It is
safe to re-run: cluster and namespace creation are idempotent, and `down` is a
no-op when nothing exists.

### Individual phases

```sh
./harness/spike/agent-sandbox-substrate/reproduce.sh tools    # read-only: resolve kind/kubectl, print versions
./harness/spike/agent-sandbox-substrate/reproduce.sh status   # read-only: context, cluster, CRDs, controller readiness
./harness/spike/agent-sandbox-substrate/reproduce.sh up       # cluster + controller only, cluster stays up
./harness/spike/agent-sandbox-substrate/reproduce.sh smoke    # requires `up` first
./harness/spike/agent-sandbox-substrate/reproduce.sh down     # delete only the kind cluster this script created
```

## What gets created, and what cleanup removes

- kind cluster `agenova-k8s-lab` (kube context `kind-agenova-k8s-lab`) — the same
  context name `./scripts/check.ps1 -Integration -KubeContext kind-agenova-k8s-lab`
  already expects, so this substrate is reusable by that gate later.
- Namespace `agent-sandbox-smoke` holding the three fixtures in `manifests/`.
- Namespace `agent-sandbox-system` (from the upstream `sandbox.yaml`), removed with
  the cluster.

`down` deletes **only** the `agenova-k8s-lab` kind cluster. `smoke`'s own cleanup
deletes **only** the `agent-sandbox-smoke` namespace and its contents. Before any
mutating step — including cleanup — the script re-resolves
`kubectl config current-context` and refuses to proceed unless it is exactly
`kind-agenova-k8s-lab`.

## Safety / honesty behavior

- Missing Docker daemon or `curl` aborts immediately with a specific message and a
  non-zero exit — never a silent pass.
- A tool download whose SHA256 does not match the published checksum aborts.
- If the `SandboxClaim` never reaches `Ready=True`, or a sandbox pod never
  terminates after claim/pool/template teardown, the script prints an explicit
  failure (with `kubectl get` / events / controller-log dumps) and exits non-zero.
- A kube-context mismatch aborts every mutating subcommand, including `down`.

## Evidence

Every invocation writes, under `docs/evidence/E8-T3/agent-sandbox-substrate/`:

- `output.txt` — full command output.
- `summary.md` — ticket, gate, date, branch/commit, command, pinned Agent Sandbox
  version, resolved `kind`/`kubectl` (existing vs pinned), context, and pass/fail.

## Upstream notes (v1.0.0)

- Core + extension APIs are served as `v1beta1`; `v1alpha1` and the conversion
  webhook were removed in v1.0.0. Fresh installs (this script) need no migration.
- Release assets are `sandbox.yaml` (core CRDs + controller), `extensions.yaml`
  (Template/WarmPool/Claim CRDs), and `sandbox-with-extensions.yaml` (combined).
  The old `manifest.yaml` name (≤ v0.4.x) no longer exists.
- `SandboxClaim.status.sandbox.serviceFQDN` and the claim's own
  `Ready`/`observedGeneration` reporting were fixed/added in v1.0.0.

## Troubleshooting

- **`docker daemon is not reachable`** — start Docker Desktop and re-run.
- **Controller image `ImagePullBackOff`** — check network to `registry.k8s.io`;
  `kubectl -n agent-sandbox-system get pods` then `describe` the pod.
- **Controller `CrashLoopBackOff`** — usually a CRD/controller version mismatch;
  don't hand-edit the downloaded manifests. Check
  `kubectl -n agent-sandbox-system logs deploy/agent-sandbox-controller`.
- **`resolved kube context is ... expected kind-agenova-k8s-lab`** — another
  context is active; run `kubectl config use-context kind-agenova-k8s-lab` (or
  `down` the stray cluster) and retry.
- **`SandboxClaim never reached Ready=True`** — the warm pool may not have
  provisioned; check `kubectl -n agent-sandbox-smoke get sandboxwarmpool,pod -o wide`
  and the controller logs.
