# Task: Reproduce the upstream Agent Sandbox test substrate on kind

- Ticket: [#50](https://github.com/wunderforge/agenova/issues/50)
- Mission: Provide one self-contained script that ensures `kind`/`kubectl` are available (using an existing install or a pinned binary), brings up a disposable local cluster, installs one pinned upstream Agent Sandbox version, and proves a single upstream-native sandbox lifecycle — with no Agenova claim, adapter, or contract involved.
- Target: `harness/spike/agent-sandbox-substrate/` (the reproduce script, minimal upstream manifests, and runbook) plus `docs/evidence/E8-T3/agent-sandbox-substrate/` and one pointer line in `docs/backends/agent-sandbox.md`.
- User value: A teammate can reproduce the pinned upstream Agent Sandbox lifecycle from a clean machine and see exactly which versions, readiness, and cleanup behavior hold, so E8-T4 (#51) can build the Agenova adapter proof on a known substrate.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution) — E8-T3 builds the substrate that #51 uses to demonstrate the supported allocation/readiness/cleanup path. This is **not** the [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap) path; that outcome targets an existing cluster and explicitly does not create one.

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [GitHub Ticket #50](https://github.com/wunderforge/agenova/issues/50) — acceptance criteria, contract guardrails, and Definition of Done
- [Add or Change a Runtime Backend playbook](../../docs/harness/playbooks.md#add-or-change-a-runtime-backend) — for the "do not mark static manifests as real backend verification" rule
- [Quality gates](../../docs/harness/quality-gates.md#agent-sandbox-integration)
- [Kubernetes Agent Sandbox Adapter note](../../docs/backends/agent-sandbox.md) — existing spike scope and known gaps
- [Evidence convention](../../docs/evidence/README.md)
- [PR #99 repair handoff](pr99-handoff.md) — remaining Owner steps and current verification limits
- Upstream: `kubernetes-sigs/agent-sandbox` pinned release manifests and CRDs
- Prior exploratory work on `origin/neo/e8-t3-agent-sandbox-substrate` (commit `2e3dab7`) assumed `kind`/`kubectl` were pre-installed; this packet supersedes it and adds `kind` installation to scope.

## Scope

In scope:

- One executable script under `harness/spike/agent-sandbox-substrate/` that:
  - resolves and verifies the kube context before any mutation or cleanup;
  - resolves `kind` and `kubectl`: **if already on `PATH`, use them as-is and record their versions (skip install)**; `kind` is never downloaded — a missing `kind` fails loudly with the supported package-manager commands (Owner decision 2026-09-10); only when `kubectl` is absent, download the pinned official release binary for the host OS/arch, verify its published SHA256, cache it under the gitignored `.tmp/`, and use that copy for the run;
  - checks for the Docker daemon and `curl` and fails loudly with a specific message if either is missing (it does not install a container engine or `kind`);
  - creates a disposable, uniquely named local cluster (or reuses an existing one it created);
  - installs one documented, pinned upstream Agent Sandbox version and its prerequisites, waiting for controller readiness;
  - records the resolved `kind`, Kubernetes, CRD, controller, and Agent Sandbox versions and readiness;
  - creates, observes, terminates, and cleans up one minimal upstream-native sandbox using plain `kubectl` only;
  - supports deterministic reruns (idempotent create, safe re-entry) and phase subcommands for debugging;
  - on missing prerequisites (Docker daemon, network) or unsupported upstream behavior (never Ready, pod never terminates), exits non-zero with a specific message instead of a silent pass.
- Minimal upstream-native manifests (`SandboxTemplate` / `SandboxWarmPool` / `SandboxClaim` or the smallest equivalent for the pinned version) kept beside the script.
- A runbook documenting prerequisites, the exact up/smoke/teardown/down commands, what is created, what cleanup removes, and observed upstream behavior.
- Evidence under `docs/evidence/E8-T3/agent-sandbox-substrate/` (`summary.md` + `output.txt`) capturing a real end-to-end run.
- One pointer line from `docs/backends/agent-sandbox.md` to the runbook.
- Any real upstream finding recorded for #48 (E8-T1) and #51 (E8-T4) as prose notes, not contract changes.

Out of scope:

- Any Agenova API type, `RuntimeBackend`, claim/authority/gateway semantics, or `internal/runtime/agentsandbox` adapter code (that is #51).
- A customer-cluster or production install feature, cluster lifecycle management, HA, or upgrades.
- A claim-governance proof or any use of an Agenova `ClaimRequest`/`SandboxClaim`.
- Changing the PRD, architecture contract, or shared checks beyond what a new script/doc requires.
- Managing the Docker daemon; the script only checks for it and fails loudly if it is not running.
- Replacing a `kind`/`kubectl` that is already installed — an existing one on `PATH` is used as-is. `kind` is never fetched by the script (install it with a package manager); a pinned `kubectl` binary is fetched only when `kubectl` is absent.
- Making this substrate part of the default `./scripts/check.ps1 -All` gate.

## Acceptance Criteria

- On a machine that already has `kind` and `kubectl`, the script skips installing them and records the existing versions; without `kind` it exits non-zero naming the supported package managers (`brew`/`port`/`choco`) and the upstream quick-start URL; without `kubectl` it fetches the pinned binary into `.tmp/`, verifies its checksum, and reports the resolved path and version.
- The script verifies the resolved kube context matches the cluster it manages before any mutation or cleanup, and aborts every mutating subcommand (including teardown) on a mismatch.
- It installs the documented pinned upstream Agent Sandbox version and prerequisites, then records `kind`, Kubernetes, CRD, controller image, and Agent Sandbox version plus controller readiness.
- One minimal upstream-native sandbox is created, observed reaching Ready, terminated, and cleaned up — using only `kubectl`, with no Agenova claim or adapter.
- A second consecutive run is deterministic: cluster and namespace creation are idempotent, and teardown removes only resources or clusters the script created.
- Evidence files record the exact commands, pinned versions, controller readiness, sandbox lifecycle output, and safe-cleanup proof from a real run.

## Negative Case

- With the Docker daemon stopped, `kind` absent from `PATH`, or `kubectl`/`curl` unavailable and uninstallable, the script exits non-zero with a specific prerequisite message (the `kind` message names `brew`/`port`/`choco`) and creates nothing.
- If the sandbox never reaches Ready within the timeout, or the sandbox pod never terminates after deletion, the script prints an explicit failure with `kubectl get` / events dumps and exits non-zero — a timeout is never reported as a pass.
- If the active kube context is not the script's managed context, every mutating subcommand (including teardown) refuses to run.

## Execution Todo

- [x] Scout the pinned upstream Agent Sandbox release: `v0.4.6` assets are `manifest.yaml` (core CRDs + `agent-sandbox-controller` in `agent-sandbox-system`) and `extensions.yaml` (Template/WarmPool/Claim CRDs, group `extensions.agents.x-k8s.io/v1alpha1`); minimal path is `SandboxTemplate` -> `SandboxWarmPool(replicas:1)` -> `SandboxClaim(sandboxTemplateRef.name + warmpool: <string>)`.
- [x] Confirm this packet with the Owner (authorized 2026-08-31); record independent Reviewer approval on Ticket #50 as a PR gate.
- [x] Implement tool resolution: use existing `kind`/`kubectl` when present; a missing `kind` fails loudly with the package-manager commands (never downloaded, Owner decision 2026-09-10); a missing `kubectl` is fetched as a checksum-verified pinned binary in `.tmp/`; verify the kube context before any mutation.
- [x] Implement cluster up + pinned Agent Sandbox install + readiness wait + version/readiness recording.
- [x] Implement the create / observe / terminate / cleanup smoke path with explicit failure reporting.
- [x] Split the lifecycle into `smoke` (create + observe Ready, fixtures left in place for inspection) and `teardown` (delete claim/pool/template, assert pods -> 0, drop the namespace); `all` runs `up -> smoke -> teardown -> down`.
- [x] Remove the compare command/report from #50 per the independent review; retain the prior implementation in Git history for a follow-up.
- [x] Add the minimal manifests and the runbook; add the pointer line in `docs/backends/agent-sandbox.md`.
- [x] Re-run the final committed script twice on real Docker/kind and replace the evidence blocker (2026-09-10, commit `0072e38`, script SHA `6a307daf`: two consecutive `reproduce.sh all --capture` runs pass — cluster create, context guard, ownership receipt, v0.4.6 CRD/controller install, `SandboxClaim` `Ready=True`, teardown to zero pods + namespace, scoped `down`. One-time per-cluster node CA-trust step for the corporate TLS-inspection root; `reproduce.sh` unmodified. See `docs/evidence/E8-T3/agent-sandbox-substrate/`).
- [x] Run the isolated shell regression gate and repository baseline on the corrected script (2026-09-10: 15 command-double scenarios incl. the `kind` package-manager prerequisite; baseline exit 0).

Verification status (2026-09-10):

- The 2026-09-09 collaboration on PR #99 landed via #114; Leo's earlier Darwin run at `adf4d76` predates the final script and is superseded.
- Two real `reproduce.sh all --capture` runs pass against `0072e38` (see the Execution Todo line above and `docs/evidence/E8-T3/agent-sandbox-substrate/summary.md`).
- The kind node needs the corporate TLS-inspection root CA trusted before it can pull `registry.k8s.io` / `docker.io`; done per-cluster with `update-ca-certificates` on the throwaway node. A run from a network without interception needs no such step.
- Warm/cold comparison is deferred to #48 or a follow-up; no benchmark numbers are accepted by this task.

## Quality Gates

- `./harness/spike/agent-sandbox-substrate/reproduce.sh all` — real end-to-end run (twice, for determinism), evidence captured
- `bash harness/spike/agent-sandbox-substrate/test-reproduce.sh` — isolated command-double regression gate; not real-backend evidence
- `./scripts/check.ps1 -Docs` — Markdown links and docs structure for the new runbook/pointer
- `shellcheck harness/spike/agent-sandbox-substrate/reproduce.sh` — if available in the environment

## Evidence Required

- `docs/evidence/E8-T3/agent-sandbox-substrate/summary.md`: ticket, gate, date, branch/commit, exact command, pinned Agent Sandbox version, resolved `kind`/`kubectl`/server versions, kube context, and pass/fail result.
- `docs/evidence/E8-T3/agent-sandbox-substrate/output.txt`: raw output of a real run showing the resolved tool versions, CRD served/storage and controller records, the sandbox reaching Ready, termination, and scoped cleanup.
- A short prose note of any upstream behavior finding, linked to #48 and #51, with no contract change.
- Passing `./scripts/check.ps1 -Docs`. Prose-only confirmation is not evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- This ticket must not add or change Agenova API types, `RuntimeBackend`, claim semantics, authority semantics, or gateway contracts.
- Provider types and experimental manifests stay inside the `harness/spike/agent-sandbox-substrate/` boundary; nothing leaks into `api/`, `internal/`, or shared docs beyond one pointer line.
- Upstream convenience is not evidence that the Agenova contract should change; any proposed core-contract change needs a separate `decision-required` ticket and product-owner approval.
- The disposable cluster must use a script-specific name/context; teardown must only ever delete that cluster and the script's own namespace.
- Do not treat static manifests or a single local run as production backend verification.

## Decisions and Blockers

- Planning depth: Task only. One bounded S-size spike inside the E8 spike/test boundary; no shared contract, schema, or multi-approach decision is involved, so no `spec.md` or `design.md`.
- Decision (Owner-confirmed 2026-08-31, revised 2026-09-10): the script prefers an existing `kind`/`kubectl` on `PATH` and skips installing them. A missing `kubectl` is fetched as a pinned, checksum-verified official release binary into `.tmp/`. A missing `kind` is **not** fetched — see the 2026-09-10 decision below. It only *checks* for the Docker daemon and `curl`, failing loudly when they are missing — installing a container engine is out of scope.
- Decision (Owner, 2026-09-10): the harness no longer downloads `kind`. `kind` must already be on `PATH`, installed with a supported package manager per the upstream quick-start (`https://kind.sigs.k8s.io/docs/user/quick-start/#installing-with-a-package-manager`): macOS `brew install kind` or `sudo port selfupdate && sudo port install kind`; Windows `choco install kind`. A missing `kind` exits non-zero printing those commands. Rationale: the pinned-binary download for `kind` added the Windows-asset-name special-casing flagged in the #99 review and a checksum/cache code path that a one-line package-manager install replaces; `kubectl`'s official `dl.k8s.io` download is simpler and stays. This supersedes the 2026-08-31 "install `kind` via its pinned official release binary" decision.
- Decision: reuse the kube context name `kind-agenova-k8s-lab` that `./scripts/check.ps1 -Integration -KubeContext kind-agenova-k8s-lab` already expects, so #51 can reuse this substrate.
- Decision (Owner-confirmed 2026-08-31): pin upstream Agent Sandbox **`v0.4.6`** (`extensions.agents.x-k8s.io/v1alpha1`), matching the rest of the codebase — the `internal/runtime/agentsandbox` adapter, `docs/backends/agent-sandbox.md`, `THIRD_PARTY_NOTICES.md`, and the `scripts/checks/repository.ps1` check are all on `v0.4.6`. This keeps the substrate consistent with the shipped adapter. Assets for `v0.4.6` are `manifest.yaml` (core) + `extensions.yaml`. Adopting a newer upstream release across the whole E8 surface is deferred and tracked by the #66 mapping spike.
- Decision (Owner-confirmed 2026-08-31, **superseded 2026-09-10**): originally the script downloaded a pinned `kind` release binary into `.tmp/` when `kind` was absent. Replaced by the 2026-09-10 decision above — `kind` is now a package-manager prerequisite and is never downloaded.
- Owner authorization: the Owner explicitly approved this packet and requested execution and a local test on 2026-08-31, and will review the resulting PR. Independent Reviewer approval on Ticket #50 remains a PR gate.
- Owner machine baseline (2026-08-31, Darwin arm64): `kind v0.32.0` and `kubectl v1.36.2` are already installed via Homebrew, so the pinned-binary install path will not be exercised there (the skip path will). The recorded early prerequisite failure was later followed by a successful legacy run; it is historical, not the current blocker.
- Decision: the smoke fixtures use a bare `busybox:1.36` `sleep` pod with no `volumeClaimTemplates`, `runtimeClassName`, or `NetworkPolicy` — kind has no pre-provisioned RWO storage class and the ticket only needs one observable lifecycle, not a hardened template.
- Decision: keep one `bash` script (no PowerShell port) for cross-platform use. macOS/Linux run it directly; Windows runs it under WSL2 (recommended — Docker Desktop's `kind` support uses the WSL2 backend) or Git Bash. The script normalises `mingw/msys/cygwin` to a `windows` OS and uses `.exe` for the local `kubectl` destination. Rationale: the heavy lifting (`kind`, `kubectl`, `docker`) is identical across platforms and only the ~250-line wrapper differs; a `bash` + WSL2/Git Bash story is standard for kind harnesses and far less code than maintaining a parallel `.ps1`. Revisit only if a contributor genuinely cannot use WSL2 or Git Bash.
- Decision: the `kubectl` pinned fallback is `v1.34.0` (only used when `kubectl` is absent); `kind` has no pin here (install via package manager). The Owner machine has both via Homebrew so its recorded run shows `existing`.
- Blocker (2026-09-09, **cleared 2026-09-10**): the repair machine had no Docker. Two real `reproduce.sh all --capture` runs now pass on Darwin arm64 / Docker Desktop against `0072e38`; evidence regenerated. Remaining gates are the independent re-review of #99 and the Owner/Reviewer task-packet approval on this issue.
- Environment note (2026-09-10): on a network that TLS-inspects `registry.k8s.io` / `docker.io` (e.g. Zscaler), the disposable kind node needs the inspection root CA added to its trust store (`docker cp` the root into `/usr/local/share/ca-certificates/`, `update-ca-certificates`, restart containerd) before the controller image can pull. `reproduce.sh` is unmodified and fails non-zero without it. No such step is needed off the corporate network.

- Review remediation: use a checkout-local cluster fingerprint (Docker container ID + kube-system UID), a namespace ownership label, and explicit context checks before mutation/deletion. Unowned existing labs must be inspected manually, never adopted.
- Review remediation: diagnostics are read-only; mutating phases capture only with explicit `--capture` into unique gitignored directories. Record actual CRD served/storage versions, commit and script hash.
- Review remediation: cleanup lookup failures and timeouts fail non-zero; compare/report are removed from #50. The existing upstream version remains v0.4.6.
