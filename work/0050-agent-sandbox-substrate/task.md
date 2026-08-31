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
- Upstream: `kubernetes-sigs/agent-sandbox` pinned release manifests and CRDs
- Prior exploratory work on `origin/neo/e8-t3-agent-sandbox-substrate` (commit `2e3dab7`) assumed `kind`/`kubectl` were pre-installed; this packet supersedes it and adds `kind` installation to scope.

## Scope

In scope:

- One executable script under `harness/spike/agent-sandbox-substrate/` that:
  - resolves and verifies the kube context before any mutation or cleanup;
  - resolves `kind` and `kubectl`: **if already on `PATH`, use them as-is and record their versions (skip install)**; only when a tool is absent, download the pinned official release binary for the host OS/arch, verify its published SHA256, cache it under the gitignored `.tmp/`, and use that copy for the run;
  - checks for the Docker daemon and `curl` and fails loudly with a specific message if either is missing (it does not install a container engine);
  - creates a disposable, uniquely named local cluster (or reuses an existing one it created);
  - installs one documented, pinned upstream Agent Sandbox version and its prerequisites, waiting for controller readiness;
  - records the resolved `kind`, Kubernetes, CRD, controller, and Agent Sandbox versions and readiness;
  - creates, observes, terminates, and cleans up one minimal upstream-native sandbox using plain `kubectl` only;
  - supports deterministic reruns (idempotent create, safe re-entry) and phase subcommands for debugging;
  - on missing prerequisites (Docker daemon, network) or unsupported upstream behavior (never Ready, pod never terminates), exits non-zero with a specific message instead of a silent pass.
- Minimal upstream-native manifests (`SandboxTemplate` / `SandboxWarmPool` / `SandboxClaim` or the smallest equivalent for the pinned version) kept beside the script.
- A runbook documenting prerequisites, the exact setup/smoke/cleanup commands, what is created, what cleanup removes, and observed upstream behavior.
- Evidence under `docs/evidence/E8-T3/agent-sandbox-substrate/` (`summary.md` + `output.txt`) capturing a real end-to-end run.
- One pointer line from `docs/backends/agent-sandbox.md` to the runbook.
- Any real upstream finding recorded for #48 (E8-T1) and #51 (E8-T4) as prose notes, not contract changes.

Out of scope:

- Any Agenova API type, `RuntimeBackend`, claim/authority/gateway semantics, or `internal/runtime/agentsandbox` adapter code (that is #51).
- A customer-cluster or production install feature, cluster lifecycle management, HA, or upgrades.
- A claim-governance proof or any use of an Agenova `ClaimRequest`/`SandboxClaim`.
- Changing the PRD, architecture contract, or shared checks beyond what a new script/doc requires.
- Managing the Docker daemon; the script only checks for it and fails loudly if it is not running.
- Replacing a `kind`/`kubectl` that is already installed — an existing one on `PATH` is used as-is; a pinned binary is fetched only when the tool is absent.
- Making this substrate part of the default `./scripts/check.ps1 -All` gate.

## Acceptance Criteria

- On a machine that already has `kind` and `kubectl`, the script skips installing them and records the existing versions; on a machine without one of them, it fetches the pinned binary into `.tmp/`, verifies its checksum, and reports the resolved path and version.
- The script verifies the resolved kube context matches the cluster it manages before any mutation or cleanup, and aborts every mutating subcommand (including teardown) on a mismatch.
- It installs the documented pinned upstream Agent Sandbox version and prerequisites, then records `kind`, Kubernetes, CRD, controller image, and Agent Sandbox version plus controller readiness.
- One minimal upstream-native sandbox is created, observed reaching Ready, terminated, and cleaned up — using only `kubectl`, with no Agenova claim or adapter.
- A second consecutive run is deterministic: cluster and namespace creation are idempotent, and teardown removes only resources or clusters the script created.
- Evidence files record the exact commands, pinned versions, controller readiness, sandbox lifecycle output, and safe-cleanup proof from a real run.

## Negative Case

- With the Docker daemon stopped (or `kind`/`kubectl`/`curl` unavailable and uninstallable), the script exits non-zero with a specific prerequisite message and creates nothing.
- If the sandbox never reaches Ready within the timeout, or the sandbox pod never terminates after deletion, the script prints an explicit failure with `kubectl get` / events dumps and exits non-zero — a timeout is never reported as a pass.
- If the active kube context is not the script's managed context, every mutating subcommand (including teardown) refuses to run.

## Execution Todo

- [x] Scout the pinned upstream Agent Sandbox release: `v0.4.6` assets are `manifest.yaml` (core CRDs + `agent-sandbox-controller` in `agent-sandbox-system`) and `extensions.yaml` (Template/WarmPool/Claim CRDs, group `extensions.agents.x-k8s.io/v1alpha1`); minimal path is `SandboxTemplate` -> `SandboxWarmPool(replicas:1)` -> `SandboxClaim(sandboxTemplateRef.name + warmpool: <string>)`.
- [x] Confirm this packet with the Owner (authorized 2026-08-31); record independent Reviewer approval on Ticket #50 as a PR gate.
- [x] Implement tool resolution: use existing `kind`/`kubectl` when present, pinned checksum-verified binary in `.tmp/` when absent; verify the kube context before any mutation.
- [x] Implement cluster up + pinned Agent Sandbox install + readiness wait + version/readiness recording.
- [x] Implement the create / observe / terminate / cleanup smoke path with explicit failure reporting.
- [x] Add the minimal manifests and the runbook; add the pointer line in `docs/backends/agent-sandbox.md`.
- [x] Run the script end to end (three consecutive `reproduce.sh all` passes) on a real local machine; capture `summary.md` + `output.txt`.
- [ ] Run `./scripts/check.ps1 -Docs` and review the diff for scope and source-of-truth updates. (`pwsh` is not installed on the Owner machine; run in CI or by the Reviewer. Manual Markdown-link check done.)

Verification (Darwin arm64, 2026-08-31):

- Without Docker: `tools`/`status` reuse the existing Homebrew `kind v0.32.0` / `kubectl v1.36.2` and skip install; `up` with the daemon down exits 1 with `docker daemon is not reachable` and creates nothing; an unknown subcommand prints usage and exits 1. `bash -n` clean; `shellcheck` not installed locally.
- With Docker (29.6.1): `reproduce.sh all` passed end to end against Agent Sandbox `v0.4.6` (`registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.4.6`) on kind cluster `agenova-k8s-lab` — cluster created and context verified, `manifest.yaml` + `extensions.yaml` applied and controller Ready, `SandboxClaim/smoke-claim` reached `Ready=True` with a `Running` sandbox pod, then claim/pool/template torn down (pods -> 0), namespace and cluster deleted. The active context `ais-uat` was restored after each run.
- Finding for #48: on `v0.4.6`, deleting a warm-pool-backed `SandboxClaim` recycles its `Sandbox` back into the pool; the pod is removed only once the pool and template are also deleted. The harness accounts for this by tearing down claim, pool, and template together and asserting the pod count reaches zero.

## Quality Gates

- `./harness/spike/agent-sandbox-substrate/reproduce.sh all` — real end-to-end run (twice, for determinism), evidence captured
- `./scripts/check.ps1 -Docs` — Markdown links and docs structure for the new runbook/pointer
- `shellcheck harness/spike/agent-sandbox-substrate/reproduce.sh` — if available in the environment

## Evidence Required

- `docs/evidence/E8-T3/agent-sandbox-substrate/summary.md`: ticket, gate, date, branch/commit, exact command, pinned Agent Sandbox and `kind` versions, kube context, and pass/fail result.
- `docs/evidence/E8-T3/agent-sandbox-substrate/output.txt`: raw output of a real run showing `kind` install, version/readiness records, the sandbox reaching Ready, termination, and scoped cleanup.
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
- Decision (Owner-confirmed 2026-08-31): the script prefers an existing `kind`/`kubectl` on `PATH` and skips installing them; when a tool is absent it fetches a pinned, checksum-verified official release binary into `.tmp/`. It only *checks* for the Docker daemon and `curl`, failing loudly when they are missing — installing a container engine is out of scope.
- Decision: reuse the kube context name `kind-agenova-k8s-lab` that `./scripts/check.ps1 -Integration -KubeContext kind-agenova-k8s-lab` already expects, so #51 can reuse this substrate.
- Decision (Owner-confirmed 2026-08-31): pin upstream Agent Sandbox **`v0.4.6`** (`extensions.agents.x-k8s.io/v1alpha1`), matching the rest of the codebase — the `internal/runtime/agentsandbox` adapter, `docs/backends/agent-sandbox.md`, `THIRD_PARTY_NOTICES.md`, and the `scripts/checks/repository.ps1` check are all on `v0.4.6`. This keeps the substrate consistent with the shipped adapter. Assets for `v0.4.6` are `manifest.yaml` (core) + `extensions.yaml`. Adopting a newer upstream release across the whole E8 surface is deferred and tracked by the #66 mapping spike.
- Decision (Owner-confirmed 2026-08-31): install `kind` via its pinned official release binary, not `go install` or a package manager. Pin an exact `kind` version in the script; download the release binary for the host OS/arch, verify the published SHA256, cache it under the gitignored `.tmp/`, and reuse a `PATH` `kind` only when its version already matches the pin. This keeps reruns deterministic and never mutates the developer's system `PATH` or existing `kind`.
- Owner authorization: the Owner explicitly approved this packet and requested execution and a local test on 2026-08-31, and will review the resulting PR. Independent Reviewer approval on Ticket #50 remains a PR gate.
- Owner machine baseline (2026-08-31, Darwin arm64): `kind v0.32.0` and `kubectl v1.36.2` are already installed via Homebrew, so the pinned-binary install path will not be exercised there (the skip path will). The Docker daemon is currently **not running** and the active context is `ais-uat`; Docker Desktop must be started before the `up`/`smoke` phases, and the context-mismatch refusal can be verified as-is.
- Decision: the smoke fixtures use a bare `busybox:1.36` `sleep` pod with no `volumeClaimTemplates`, `runtimeClassName`, or `NetworkPolicy` — kind has no pre-provisioned RWO storage class and the ticket only needs one observable lifecycle, not a hardened template.
- Decision: keep one `bash` script (no PowerShell port) for cross-platform use. macOS/Linux run it directly; Windows runs it under WSL2 (recommended — Docker Desktop's `kind` support uses the WSL2 backend) or Git Bash. The script normalises `mingw/msys/cygwin` to a `windows` OS and adds `.exe` for the pinned `kind`/`kubectl` fallback download. Rationale: the heavy lifting (`kind`, `kubectl`, `docker`) is identical across platforms and only the ~250-line wrapper differs; a `bash` + WSL2/Git Bash story is standard for kind harnesses and far less code than maintaining a parallel `.ps1`. Revisit only if a contributor genuinely cannot use WSL2 or Git Bash.
- Decision: `kind`/`kubectl` pinned fallbacks are `v0.33.0` / `v1.34.0` (only used when the tool is absent); the Owner machine already has both via Homebrew so its recorded run will show `existing`.
- Blockers: Docker daemon not running on the Owner machine — blocks the end-to-end `up`/`smoke` run until Docker Desktop is started. No code blocker. Execution also needs network access to `github.com`, `dl.k8s.io`, and `registry.k8s.io`.
