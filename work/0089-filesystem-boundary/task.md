# Task: Define and test the minimum agent filesystem boundary

- Ticket: [#89](https://github.com/wunderforge/agenova/issues/89)
- Mission: Make one claim's writable task directory explicit without turning Agenova into a filesystem proxy or workspace service.
- Target: The merged #30 runtime seam, reference backend, reusable filesystem contract cases, local compatibility fixture, and #48/#51 handoff.
- User value: An agent can prepare a repository, edit, compile, test and produce an output within a documented boundary whose evidence limitations are visible.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Specification](spec.md) and [design](design.md).
- [Architecture contract](../../docs/product/architecture-contract.md): Backend Neutrality, Claim Lifecycle, Authority and Credentials, Scope Discipline.
- [AIDLC](../../docs/development/AIDLC.md), [playbooks](../../docs/harness/playbooks.md): Start a GitHub Ticket, Change a Core Contract, Add or Change a Runtime Backend, Elaborate Parallel Work.
- [Quality gates](../../docs/harness/quality-gates.md) and [gotchas](../../docs/harness/gotchas.md).
- [Product tour](../../docs/project-design.md): worker ownership, lifecycle and backend explanation only; this is not a source of implemented schemas.
- [Runtime contract](../../internal/runtime/backend.go), [reusable cases](../../internal/runtime/contracttest/run.go), [reference runtime](../../internal/operator/runtime.go) and [tests](../../internal/operator/runtime_test.go).
- [Adapter](../../internal/runtime/agentsandbox/adapter.go), [backend note](../../docs/backends/agent-sandbox.md), [canonical template](../../api/v1alpha1/agent_template.go).
- Producer [#30](https://github.com/wunderforge/agenova/issues/30) and draft [#106](https://github.com/wunderforge/agenova/pull/106); consumers [#48](https://github.com/wunderforge/agenova/issues/48), [#51](https://github.com/wunderforge/agenova/issues/51), [#52](https://github.com/wunderforge/agenova/issues/52), [#53](https://github.com/wunderforge/agenova/issues/53).

## Scope

In scope:

- Propose a backend-neutral, claim-scoped writable directory, deterministic outside-boundary rule, lifecycle and bounded output handoff.
- Specify compatibility, positive/negative cases and simulated versus real evidence before implementation.
- After the implementation gate opens, add minimum shared semantics and reference evidence to the accepted #30 seam and document capability gaps for consumers.

Out of scope:

- Production filesystem proxy APIs, persistent storage, or a claim-selectable host path.
- Host home, arbitrary host paths, unrelated workspaces, host credentials, provider types in shared contracts, syscall interception, FUSE and per-file audit.
- Managed or persistent Workspace, Web IDE, Portal editing, interactive human access, orchestration or new gateway authority.
- Real isolation evidence (#51), provider mapping (#48), direct-egress experiment (#52), and the engineer artifact implementation (#53).

## Acceptance Criteria

- One backend-selected working directory is writable for the claim, including task files, scratch and tool caches; callers cannot select a host mount.
- A prepared example repository supports ordinary git, compiler/test and agent-process filesystem operations directly, without per-syscall Tool Gateway calls.
- Outside it, runtime files are read-only and all other filesystem data is unavailable through the supported configuration; unsupported enforcement is explicit.
- Host home, unrelated claims/workspaces and long-lived provider credentials are absent from supported worker configuration.
- Shared semantics and evidence use claim/backend identity and neutral values, with no Kubernetes, container or provider types.
- Reusable reference cases cover directory behavior and an outside-boundary denial with an explicit simulation label; local process compatibility is separately demonstrated.
- Termination ends worker access, cleanup releases the directory without workspace retention, and only outputs exported before termination survive through the supported handoff.
- Tool/Model Gateway authorization and credential boundaries remain unchanged.

## Negative Case

The [spec case matrix](spec.md#negative-cases) covers traversal and alias escape, outside mutation, cross-claim access, forbidden configuration, missing capability, late output export and failed termination/cleanup. Each denial must leave the relevant synthetic sentinel unchanged and must not fabricate success or affect another claim.

## Execution Todo

- [x] Scout current code, requested issues, PR #106 and repository planning requirements.
- [x] Draft the canonical Task + Spec + Design with compatibility and negative cases.
- [x] Record Owner approval and review corrections in #89; apply the Owner's documented one-day review fallback for final delivery.
- [x] Verify #30's final RuntimeBackend implementation is merged and reconcile this packet with its accepted types.
- [x] Implement the smallest filesystem semantic slice against that seam, with reference cases and explicit simulation labels.
- [x] Add a controlled local repository/compile/test/output compatibility fixture, separate from security evidence.
- [x] Complete the capability/gap handoff, focused and repository gates, final review, and merge decision.
- [x] Reopen after post-merge exhaustive review and harden pre-Start, runtime-read-only, export-cutoff, failed-termination ordering, credential scrubbing, hard-link, and evidence-provenance cases.

## Quality Gates

Planning:

- `pwsh -NoProfile -File scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File scripts/check-pr-body.ps1 -BodyPath .tmp/0089-pr-body.md`
- `git diff --cached --check`
- `pwsh -NoProfile -File scripts/check.ps1 -All`

After implementation is unblocked:

- `go test -count=1 -v ./internal/operator/... ./internal/runtime/...`
- `go test -count=1 -v ./internal/toolgateway/... ./internal/modelgateway/... ./harness/e2e/...`
- `pwsh -NoProfile -File scripts/check.ps1 -All`
- G2 reference contract evidence is the strongest #89 gate; #51 owns G5 real-backend evidence. Planning checks prove neither.

## Evidence Required

- PR records base/commit, exact commands, exit results and limitations; no new behavior is claimed by this packet.
- Implementation records case IDs from the spec, directory identity, command cwd/exit/output, synthetic sentinel comparisons, cleanup/error observations and output digest/size.
- Reference evidence labels policy simulation, real local command execution and unverified isolation separately. Existing test commands without the new named cases cannot count as #89 completion.
- #48 receives the accepted semantics and capability/gap table; #51 supplies real mount/layout, worker negative access and cleanup evidence using the same fixture; #52 retains the egress boundary.

## Constraints

- Preserve the [architecture contract](../../docs/product/architecture-contract.md); do not broaden the PRD.
- Do not modify or freeze shared RuntimeBackend/filesystem Go contracts before final #30 implementation merge and recorded Owner/independent planning approval.
- Do not reshape #30 or Agent Sandbox semantics for convenience. If the merged producer cannot carry an accepted requirement, record the conflict and request a maintainer decision.
- No real secrets or host credential locations may be read by fixtures; use synthetic data only.
- GitHub owns mutable assignment and delivery status; this packet records execution gates and provenance.

## Decisions and Blockers

- PR #106 / #30 merged on 2026-09-10. This branch was reconciled with its final five-operation RuntimeBackend before implementation.
- The Owner approved the outside-boundary rule, pre-termination output/no-retention behavior, and evidence split in #89. The Owner subsequently established a one-day final-review fallback: fix actionable findings directly; if the final change and CI are clean, bypass an unavailable approval and merge.
- Shared allocation/observation now report neutral filesystem boundary and evidence values. The reference model reports `Simulated`; Agent Sandbox reports `Unsupported` until #48 maps and #51 proves the real substrate.
- Planning review correction: use independent-claim isolation rather than parent/child lineage, and do not imply that Agenova already has a generic artifact-output API. The integration must select an approved governed operation; the collector remains test-only.
- [Capability and evidence handoff to #48/#51](handoff-0048-0051.md).
- Post-merge exhaustive review of #112 identified evidence gaps. The follow-up requires the critical worker-observable cases explicitly in #51 and strengthens the reference/local fixtures without adding a production filesystem API.
- The requested `docs/aidlc.md` does not exist at this baseline; `docs/development/AIDLC.md` is the canonical workflow linked by AGENTS.md.
