# Agent Sandbox Test Substrate — Runbook (E8-T3 / #50)

Reproduce upstream Agent Sandbox **v0.4.6** on disposable Docker/kind resources.
This harness does not exercise Agenova claims, governance or its runtime adapter.

## Prerequisites and platforms

Use Bash on macOS/Linux, WSL2 with Docker integration, or Git Bash on Windows.
Install and start Docker yourself; the harness never installs or starts an engine.
It also needs curl, a SHA256 utility, Git and access to GitHub, dl.k8s.io and
registry.k8s.io. Git Bash regression tests cover command construction and failure
handling; they do not prove a real Windows Kubernetes run.

**Install `kind` yourself with a supported package manager** — the harness does
not download it (Owner decision 2026-09-10). Per the upstream
[quick-start](https://kind.sigs.k8s.io/docs/user/quick-start/#installing-with-a-package-manager):
macOS `brew install kind` or `sudo port selfupdate && sudo port install kind`;
Windows `choco install kind`. A missing `kind` is a loud prerequisite failure
that prints these commands, never a download. An existing `kind` on PATH is used
as-is and its version recorded.

`kubectl` on PATH is likewise used as-is; only when it is absent does a mutating
phase download a checksum-verified official binary to gitignored
`.tmp/agenova-k8s-lab-tools/` (filename qualified by version, OS and arch).
Diagnostics never download anything.

| Pin | Value | Meaning |
| --- | --- | --- |
| Agent Sandbox | v0.4.6 | Core `manifest.yaml` + `extensions.yaml`, v1alpha1 APIs |
| kind | not pinned here | Install via package manager; existing version used as-is and recorded |
| kubectl fallback | v1.34.0 | Only used when kubectl is absent; not a server-version claim |

Pins live in `reproduce.sh`. The kubectl fallback pin does not constrain an
existing installation or prove which Kubernetes node image it uses; the actual
server version is recorded. On Windows the upstream kubectl asset includes an
`.exe` suffix.

## Run it

From the repository root:

```sh
bash harness/spike/agent-sandbox-substrate/reproduce.sh tools
bash harness/spike/agent-sandbox-substrate/reproduce.sh status
bash harness/spike/agent-sandbox-substrate/reproduce.sh all --capture
```

`all` runs `up -> smoke -> teardown -> down`. It creates the kind cluster,
installs upstream CRDs/controller, waits for one claim to become Ready, removes
claim/pool/template, verifies zero pods and namespace removal, then deletes the
owned cluster. Run `all --capture` twice from the final committed script to
demonstrate deterministic reruns.

| Phase | Behavior |
| --- | --- |
| tools / status | Read-only diagnostics; no downloads, evidence writes or context switches |
| up | Create a fresh cluster or reuse a cluster with a matching ownership receipt |
| smoke | Create/observe fixtures; leave them available for inspection |
| teardown | Remove only the owned smoke namespace and its fixtures; fail on timeout |
| down | Delete only the cluster whose current fingerprint matches the saved receipt |
| all | Execute the complete lifecycle |

Warm/cold performance comparison and its HTML report are deferred to #48 or a
separate follow-up, per the #99 review. `compare` is no longer a subcommand.
The previous implementation remains available in PR #99's Git history.

## Ownership and cleanup

The cluster is `agenova-k8s-lab`, with explicit context
`kind-agenova-k8s-lab`. Successful creation records the Docker control-plane
container ID and Kubernetes `kube-system` namespace UID in this checkout's
`.tmp/agenova-k8s-lab-owner/identity`. Re-entry, mutation and deletion verify
both identities and the active context. An existing same-name cluster without
this receipt is refused. Replacing the cluster invalidates the receipt.

The namespace `agent-sandbox-smoke` is created atomically with label
`agenova.io/substrate-owner` equal to the recorded cluster UID. An existing
namespace must match before fixtures can be applied or deleted. Upstream
`agent-sandbox-system` resources are installed only inside the owned cluster
and removed with it. Kubernetes mutation commands explicitly select the context.

This is an accidental-deletion guard for a trusted local lab, not protection
against another local administrator forging receipts or changing Docker/Kubernetes
state concurrently. Treat the cluster as disposable and exclusive to this harness;
do not put unrelated workloads in it. Avoid concurrent harness invocations.

A cluster created by the old script has no receipt: inspect and remove it manually
only after confirming it is disposable, then run `up` to create a new one.
Do not copy or fabricate receipts. A partial cluster-creation failure or loss of
the receipt requires manual inspection; the harness will not adopt that cluster.
Changing checkout/worktree also changes the receipt location.

Missing prerequisites, context/identity mismatch, API lookup errors, readiness
timeouts, pod cleanup failures and namespace deletion timeouts all fail non-zero.
Failure leaves resources for inspection; no automatic destructive EXIT cleanup runs.
After resolving the cause, use `teardown` / `down` from the owning checkout.

## Verification and evidence

```sh
bash -n harness/spike/agent-sandbox-substrate/reproduce.sh
bash harness/spike/agent-sandbox-substrate/test-reproduce.sh
```

The regression gate uses command doubles only and never contacts Docker or
Kubernetes. It tests foreign/replaced clusters, wrong contexts, namespace ownership,
lookup failures, timeout failures, idempotent owned reruns, the `kind` package-manager
prerequisite failure and diagnostic evidence preservation. This does not prove
upstream behavior.

Run the repository baseline separately:

```powershell
./scripts/check.ps1 -All
```

Ordinary phases print to the terminal. Explicit `--capture` on a mutating phase
creates a unique directory under `.tmp/agent-sandbox-evidence.*` containing
`summary.md` and `output.txt`; committed evidence is never automatically touched.
The summary records commit, dirty state, script SHA256, exact phase and result.
Only a successful complete `all` run counts as lifecycle evidence.

Once two real runs pass on the final committed script, review the capture and
replace the explicit blocker in
`docs/evidence/E8-T3/agent-sandbox-substrate/` with one current accepted run.
Do not reuse the old pre-fix evidence. Record kind/Kubernetes versions, each CRD's
served/storage versions, controller image/readiness, claim readiness, zero pods,
namespace deletion and owned cluster deletion. A green baseline is not a substitute.

## Upstream boundary

The fixtures use `extensions.agents.x-k8s.io/v1alpha1` with
`sandboxTemplateRef` and a string `warmpool`. Cleanup removes the claim, pool
and template so warm-pool replenishment cannot keep pods alive. Claim-only cleanup
and performance semantics need separate evidence under #48/#51; this harness
does not establish those findings or any hostile-agent isolation guarantee.
