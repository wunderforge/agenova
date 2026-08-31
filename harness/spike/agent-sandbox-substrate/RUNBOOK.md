# Agent Sandbox Test Substrate — Runbook (E8-T3 / #50)

> **Where to run this**
> - **macOS / Linux:** run `reproduce.sh` directly.
> - **Windows:** run it inside **WSL2** (recommended) — or Git Bash. Not
>   PowerShell or `cmd`. See [Platforms](#platforms) for details.

Reproduces the upstream [Agent Sandbox](https://agent-sandbox.sigs.k8s.io/) test
substrate on a disposable local `kind` cluster: a pinned upstream install, and one
minimal `SandboxTemplate` → `SandboxWarmPool` → `SandboxClaim` lifecycle observed
and cleaned up with plain `kubectl`.

**This is upstream-native only.** It never touches Agenova's `RuntimeBackend`,
claim types, or `internal/runtime/agentsandbox` adapter — that proof is E8-T4 (#51).

## Prerequisites

You must install and start these yourself before running the script:

| Requirement | Install | Check | Why the script needs it |
| --- | --- | --- | --- |
| **Docker Desktop** (or a compatible engine) — **must be running** | <https://docs.docker.com/desktop/> (macOS/Windows) or Docker Engine / Podman on Linux | `docker info` | `kind` runs the Kubernetes node as a container. The script calls `docker info` first and exits non-zero with `docker daemon is not reachable` if it is stopped. |
| `curl` | preinstalled on macOS and most Linux | `curl --version` | Fetches the pinned upstream manifests and, when needed, the pinned `kind`/`kubectl` binaries. |
| Network access | — | — | Needs `github.com`, `dl.k8s.io`, and `registry.k8s.io`. |

The script installs these for you when missing (an existing one on `PATH` is
used as-is), so they are **not** manual prerequisites:

| Tool | If already on `PATH` | If absent |
| --- | --- | --- |
| `kind` | used as-is, version recorded | pinned `v0.33.0` downloaded, SHA256-verified, cached under `.tmp/agenova-k8s-lab-tools/` |
| `kubectl` | used as-is, version recorded | pinned `v1.34.0` downloaded, SHA256-verified, cached under `.tmp/agenova-k8s-lab-tools/` |

The script never writes into your system `PATH`; downloaded binaries live only
under the gitignored `.tmp/` directory.

## Pinned versions

`reproduce.sh` holds the pins; change them there.

| Pin | Value | Meaning | Bump when |
| --- | --- | --- | --- |
| `AGENT_SANDBOX_VERSION` | `v0.4.6` (`v1alpha1` APIs) | installed on every run | the team adopts a newer upstream Agent Sandbox release |
| `KIND_FALLBACK_VERSION` | `v0.33.0` | used only to bootstrap `kind` on a machine that lacks it | a clean machine should bootstrap a newer `kind` |
| `KUBECTL_FALLBACK_VERSION` | `v1.34.0` | used only to bootstrap `kubectl` on a machine that lacks it | the `kind` node Kubernetes version moves a minor |

Values verified working on Darwin arm64 on 2026-08-31.

## Platforms

`reproduce.sh` is a `bash` script (it uses `bash` process substitution, not just
POSIX `sh`). It runs on:

- **macOS / Linux** — run it directly.
- **Windows** — run it from a `bash` shell, not PowerShell or `cmd`:
  - **WSL2 (recommended).** Docker Desktop's Kubernetes/`kind` support on Windows
    uses the WSL2 backend anyway, so run the whole flow inside your WSL2 distro
    with Docker Desktop's WSL integration enabled.
  - **Git Bash** (ships with Git for Windows) also works. The script detects the
    MSYS/MinGW environment and fetches the `windows/amd64` `kind`/`kubectl`
    binaries (with `.exe`) when they are not already on `PATH`.

There is no native PowerShell port; on Windows without WSL2 or Git Bash, install
one of them (or run `kind`/`kubectl` by hand following this runbook).

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
- Namespace `agent-sandbox-system` (from the upstream `manifest.yaml`), removed with
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

## Upstream notes (v0.4.6)

- Pinned to `v0.4.6` on purpose: the `internal/runtime/agentsandbox` adapter,
  `docs/backends/agent-sandbox.md`, `THIRD_PARTY_NOTICES.md`, and the
  `scripts/checks/repository.ps1` check are all on `v0.4.6` /
  `extensions.agents.x-k8s.io/v1alpha1`. Adopting a newer upstream Agent Sandbox
  release across E8 is a separate change tracked by the #66 mapping spike.
- Release assets for `v0.4.6` are `manifest.yaml` (core CRDs + controller) and
  `extensions.yaml` (Template/WarmPool/Claim CRDs).
- The `v1alpha1` `SandboxClaim` names its warm pool with the string field
  `spec.warmpool` and points at the template via `spec.sandboxTemplateRef`.
- Deleting a warm-pool-backed `SandboxClaim` on `v0.4.6` recycles its `Sandbox`
  back into the pool; the pod is only removed once the pool and template are
  deleted too. `smoke` tears down all three before asserting pod count zero.

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
