# Task: Verify fail-closed component failure behavior

- Ticket: [#199](https://github.com/wunderforge/agenova/issues/199)
- Mission: Measure the full #199 failure matrix and RI-3 on an isolated real kind installation without changing product behavior.
- Target: harness/security, docs/evidence/199.
- User value: Reviewers can distinguish provider failure from successful Work and verify recovery.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet.
- `docs/product/architecture-contract.md`: Claim Lifecycle, Authority and Credentials, Reference Installation and Bootstrap.
- `docs/reference-cli-kind-ollama.md`: supported installation and connected Work path.
- `deploy/reference/platform.kind.yaml`, `deploy/reference/demo/`: baseline fixtures.
- `internal/console/service.go`, `internal/modelgateway/`: outcome and gateway evidence.
- #177 latest maintainer decisions; #194 RI-3.
- `internal/app/run_service.go`, `internal/runtime/agentsandbox/allocation.go`, `controlled.go`, `execution.go`: bounded operations, lifecycle and resource cleanup.
- `cmd/agenova-control-plane/main.go`, `internal/console/recovery.go`: reference versus optional durable member composition.
- `harness/security/component-failures.py`, `controller-timeout.py`, `provider-outage.py`, `terminalprobe/main.go`, `verify-failures.py`.
- `docs/evidence/199/full/summary.md`: complete current-baseline evidence and proof limits.
- #51 (E8 cleanup evidence), #169 (safe rollout), #180 (closed; optional durability shipped via #215): follow-up routing only.

## Scope

In scope: full #199 matrix, brought forward by the user on 2026-10-19: slow/unavailable provider, worker deletion, controller outage, control-plane restart and queued Work; healthy control, actual dedicated Ollama provider stop, unchanged-spec Work during outage, provider restart and recovery; independent provider completion counts, CLI JSON, cleanup evidence and limitations.

Out of scope: product fixes; persistence; production or EKS claims; global service interruption; public publication without user review.

## Acceptance Criteria

- Provider-down Work never reports Succeeded or returns a fabricated model answer.
- No observed mock/memory fallback, with the measured evidence scope stated explicitly.
- Provider recovery permits identical Work spec (unique metadata names) to succeed.
- Record real backend/image identities, provider state/counts, claim outcomes, cleanup and residual uncertainty.
- Cover Provider, Running worker deletion, controller unavailability, control-plane Pod interruption and fault-time queued Work with before/after controls.
- Check no authority expansion or unsupported success; distinguish Failed, Expired and unknown/lost history. Test terminal Model/Tool rejection on a real backend and state its supplementary composition scope.
- Classify cleanup/replay limits and prepare an owning-Epic follow-up for a confirmed gap; no product remediation.
- Each matrix row records proven, broken or unknown with reproducible evidence; any broken invariant gets a severity and reviewable follow-up draft. Publication requires user approval.

## Negative Case

A real installed Work submitted while its unchanged configured provider is stopped must fail rather than synthesize success. Failed invocation is not admission Deny; missing evidence is unknown.

## Execution Todo

- [x] Scout relevant implementation, tests, risks and dependencies.
- [x] Self-review packet against Ticket, PRD and architecture contract.
- [x] Prepare owned namespace and dedicated real provider; record UIDs and image identities.
- [x] Execute healthy/outage/recovery campaign and focused assertions.
- [x] Record terminal access and cleanup evidence, or explicit proof limitations.
- [x] Run focused gate, Docs and `./scripts/check.ps1 -All`.
- [x] Review diff and prepare reviewable public drafts.

## Quality Gates

- Executable campaign assertions over real installed CLI JSON and provider logs.
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

`docs/evidence/199/`: exact commands, baseline commit, cluster and image identity, fault state, provider request/completion counts, CLI JSON before/during/after, owned-resource cleanup receipts, matrix and limitations. Rendered proof if UI assertions are added.

## Constraints

Preserve the architecture contract and backend neutrality. E15 reports only; do not modify product logic or weaken checks. Synthetic inputs only; no real credentials. Refuse pre-existing namespace/container names and delete only recorded owned resources. No shared provider shutdown. High-risk new findings remain private.

## Decisions and Blockers

- 2026-10-19: self-review found no product/architecture conflict. Task depth only.
- Main baseline ae24a67261dff289a7063f7435c7c3616fd36dc0; separate worktree avoids disturbing #198.
- Use a dedicated real Ollama container with read-only model files. Stop/start that container while Platform and Work spec remain unchanged. This simulates provider process unavailability, not global host Ollama failure or slow-provider behavior.
- Docker 29.8.0, kind node Ready (Kubernetes v1.37.0), existing local llama3.1:latest confirmed. Resource preparation and execution pending.

- 2026-10-19: two installed campaigns passed; final evidence includes kind-node probe and one-to-one invocation correlation. Full gate passed after installing missing frontend prerequisites. See `docs/evidence/199/summary.md` for proof limits and partial-delivery status. Public drafts require user approval; no automatic-closing keyword is used for incomplete #199.

## Full-matrix execution (user authorized 2026-10-19)

- [x] Prepare a dedicated disposable kind cluster and kubeconfig; preserve original kind/EKS and default context.
- [x] Confirm healthy real provider control on the isolated installation.
- [x] Slow the dedicated real provider process and test bounded timeout and queued Work; restore and compare.
- [x] Delete the owned Running worker by UID; capture provider/claim/cleanup behavior and recovery.
- [x] Stop the dedicated cluster's controller, submit Work, observe pending/timeout behavior, restore and compare.
- [x] Restart the owned application control-plane Pod during Running Work; retain pre-restart evidence and query afterward for explicit unknown/lost history.
- [x] Probe observable post-terminal behavior without adding product hooks; report unavailable trusted-context replay as unknown.
- [x] Record matrix, follow-up drafts where required, gates and exact-ID cleanup.

Dedicated cluster decision: the original controller is shared cluster-wide. Use cluster `agenova-e15-199-faults` and separate `.tmp/faults.kubeconfig`; all test kubectl calls pass that file and `kind-agenova-e15-199-faults` explicitly. Cluster creation is harness substrate preparation, not an Agenova product capability. Product logic remains unchanged. Application control plane means Agenova's Pod, not the Kubernetes API server. Use SIGSTOP/SIGCONT on only the dedicated real Ollama process for slowness; do not fabricate provider responses. Queue tests distinguish admitted Pending from Running and test expiry without allocation. Terminal replay through a missing private interface remains unproven, not assumed.

- Baseline refresh: origin/main advanced from ae24a67 to 22355d5 during this session. Historical matrix is retained under its original baseline; current-main control-plane and worker images are rebuilt before repeating. Main now includes optional durable member-mode recovery (#215); the #199 reference configuration does not enable member mode or its durable store. No EKS/member-production claims are made. The terminal probe follows the current per-Work schema API and uses the same FinishSchema fallback as installed composition.

- Final current-main outcome: full installed matrix, sustained cold-controller outage, three supplementary real-backend terminal modes and actual Provider stop/recovery executed. Exact Provider windows and immutable authority checks passed. Current full gate exited 0 (56 browser smoke tests); final Docs and raw archive validation recorded in full evidence.
- Confirmed P2 cleanup gap: expired unobserved allocation retains upstream SandboxClaim; controller recovery allocates a worker afterward without an Agenova Running authority. Owner routing is E8 #12 / #51; narrow follow-up draft awaits user publication approval. Do not represent this as a passed cleanup invariant or silently fix it.
- Reference restart records return 404/unknown, while current-main cleanup removes the interrupted observed claim. Optional member-mode durability was not configured and is not assessed. Real mutating-tool side effects are unconfigured; universal fallback-attempt instrumentation is unavailable. These limits remain explicit.
- Exact-ID cleanup completed for four namespaces, dedicated kind and Provider. Original reference Pod UIDs unchanged; host Ollama reachable; zero EKS actions. Public publication still awaits user approval. Full test execution does not mean every invariant passed or that #199 has been closed.
