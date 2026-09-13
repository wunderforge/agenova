# Task: Spike the provisional RuntimeBackend-to-Agent-Sandbox v0.4.6 mapping

- Ticket: [#66](https://github.com/wunderforge/agenova/issues/66)
- Mission: Produce a time-boxed, evidence-backed provisional mapping that classifies each Agenova runtime semantic (`allocation`, `readiness`, `backend identity`, `start`, `termination`, `cleanup`, `durability`, `isolation`) as `native` / `translated` / `adapter-held` / `unsupported` / `unknown` against the pinned upstream Agent Sandbox `v0.4.6`, with at least one unsupported or ambiguous case demonstrated rather than assumed.
- Target: `harness/spike/agent-sandbox-mapping/` (experimental `v1alpha1` manifests + exact reproduction commands), `docs/backends/agent-sandbox-v0.4.6-mapping.md` (the provisional mapping + gap report, kept separate from the `#48`-owned note), and `docs/evidence/E8-S1/agent-sandbox-mapping/`.
- User value: `#48` (E8-T1) can freeze the *supported* Agent Sandbox mapping and gap list from a reproducible baseline instead of re-deriving it, and `#49`/`#51` inherit a known set of adapter-held vs unsupported semantics.
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution) — this spike characterises how far one real backend can carry the shared `RuntimeBackend` semantics before `#48` commits the supported subset.

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [GitHub Ticket #66](https://github.com/wunderforge/agenova/issues/66) — research question, acceptance criteria, contract guardrails, time box
- [Architecture contract](../../docs/product/architecture-contract.md) — "Backend Neutrality", "Claim Lifecycle", "Scope Discipline"
- [Add or Change a Runtime Backend playbook](../../docs/harness/playbooks.md#add-or-change-a-runtime-backend) — "do not mark static manifests as real backend verification"
- [`internal/runtime/backend.go`](../../internal/runtime/backend.go) — the current `RuntimeBackend` interface being mapped
- [`internal/runtime/contracttest/run.go`](../../internal/runtime/contracttest/run.go) — the lifecycle behaviours the in-memory oracle guarantees
- [`internal/runtime/agentsandbox/`](../../internal/runtime/agentsandbox/) — the existing `v1alpha1` SpikeAdapter, its `doc.go` gap summary, and `types.go`
- [`docs/backends/agent-sandbox.md`](../../docs/backends/agent-sandbox.md) — the current (`v0.4.6`) verified-spike behaviour and Known Gaps that `#48` owns
- [`harness/fixtures/contract/v0/manifest.json`](../../harness/fixtures/contract/v0/manifest.json) — the E1 fixtures that stay authoritative (`#22`, merged)
- [E8-T3 substrate](https://github.com/wunderforge/agenova/pull/99) — `harness/spike/agent-sandbox-substrate/reproduce.sh` stands up the pinned `v0.4.6` cluster this spike runs against
- [Filesystem handoff from #89](../0089-filesystem-boundary/handoff-0048-0051.md) — source-based layout checklist and evidence levels; #51 owns real worker isolation proof.
- [`#30` E3-T1](https://github.com/wunderforge/agenova/issues/30) and [`#48` E8-T1](https://github.com/wunderforge/agenova/issues/48) — merged contract producer and downstream consumer; #48 reviews this report against the accepted RuntimeBackend v0

## Scope

In scope:

- A written **provisional mapping table** covering every Agenova runtime semantic named in the mission, each classified `native` / `translated` / `adapter-held` / `unsupported` / `unknown`, with the upstream `v0.4.6` API field, condition, or reproducible behaviour cited as evidence for each row.
- A **gap report** listing every `translated` / `adapter-held` / `unsupported` / `unknown` row as an explicit adapter gap or supported-gap candidate, with verified facts kept separate from open questions.
- Experimental `extensions.agents.x-k8s.io/v1alpha1` manifests and the exact `kubectl` / `reproduce.sh` commands used, kept under `harness/spike/agent-sandbox-mapping/`.
- At least one **demonstrated** unsupported or ambiguous case (e.g. no upstream `Succeeded`/`Failed`/`Expired` phase; claim-only deletion vs warm-pool recycle; per-pool status breakdown; original-spec retrieval; restart durability of terminal state), captured as reproducible evidence, not prose assertion.
- A reconciliation with merged #30's five-operation RuntimeBackend and #89's filesystem evidence boundary, so #48 consumes the current contract rather than legacy lifecycle helpers.
- Evidence under `docs/evidence/E8-S1/agent-sandbox-mapping/` (`summary.md` + `output.txt` / captured `kubectl` output) and pinned-source references (upstream version tag, CRD schema, doc links).

Out of scope:

- Implementing or rewriting the production adapter in `internal/runtime/agentsandbox/` (that is adapter work for a later E8 ticket). This spike maps the same `extensions.agents.x-k8s.io/v1alpha1` group the existing SpikeAdapter already uses.
- Changing shared API types, the `RuntimeBackend` interface, claim lifecycle, authority semantics, or gateway contracts.
- Freezing `RuntimeBackend v0` or any "final" adapter contract — this report is explicitly provisional and advisory to `#48`.
- Editing `docs/backends/agent-sandbox.md` (owned by `#48`) or `docs/project-status.md` beyond what a new spike doc requires.
- Any claim-governance, gateway, identity, or durable-fact integration proof.
- Running against any Agent Sandbox version other than the one pinned by the E8-T3 substrate.

## Acceptance Criteria

- The report names the exact upstream version (`v0.4.6`) and, for every row, the authoritative API/CRD field, status condition, or reproducible behaviour used as evidence.
- Every Agenova runtime semantic in the mission is classified as exactly one of `native` / `translated` / `adapter-held` / `unsupported` / `unknown`.
- At least one unsupported or ambiguous case is demonstrated with reproducible commands + captured output, not assumed parity.
- All provider-specific types, API group strings, and manifests introduced by this task stay under `harness/spike/agent-sandbox-mapping/` (or the existing `internal/runtime/agentsandbox/` package); `./scripts/check.ps1 -Docs` still reports the Agent Sandbox shape staying inside its adapter.
- The output reads as a provisional mapping + gap report for `#48` to review against merged `#30`; it does not present itself as the final adapter contract, and no shared contract or API is changed.
- Open questions are recorded separately from verified facts.

## Negative Case

- A row left as `native` or `translated` without a cited upstream field / condition / reproduced behaviour is rejected in review as an unsupported assumption.
- If the spike hits its time box before every semantic is verified, the remaining ones are recorded as `unknown` with the specific open question — a partial result is delivered, not a guessed one.
- If any provider type or API group string is introduced outside the adapter/spike boundary, `Test-RuntimeBoundary` fails and the change is rejected.

## Execution Todo

- [x] Stand up the pinned `v0.4.6` substrate (`harness/spike/agent-sandbox-substrate/reproduce.sh up`) and capture the upstream CRD schemas for `sandbox{,claim,template,warmpool}`.
- [x] For each Agenova semantic, locate the upstream `v1alpha1` field/condition/behaviour, classify it, cite the source evidence, and reconcile the cluster-dependent rows with the bounded real run.
- [x] Run the adapter-created experimental resources against the real substrate and demonstrate the Ready-worker Start/Terminate unsupported case; capture the output.
- [x] Update `docs/backends/agent-sandbox-v0.4.6-mapping.md` with the mapping table, gap report, remaining questions, merged #30/#89 reconciliation, and bounded real-cluster result.
- [x] Add `harness/spike/agent-sandbox-mapping/` (adapter-backed reproduction + exact commands) and `docs/evidence/E8-S1/agent-sandbox-mapping/`.
- [x] Run `./scripts/check.ps1 -Docs` and `./scripts/check.ps1 -All` on 2026-09-13; review the final diff for scope, boundary, evidence, and source-of-truth updates.
- [x] Stop at the time box with Start/Terminate unsupported, durability unsupported, and general isolation unknown; leave full restart/isolation proof to #48/#51.

## Quality Gates

- `./scripts/check.ps1 -Docs` — Markdown links, docs structure, and `Test-RuntimeBoundary` (provider shape stays inside the adapter)
- `./harness/spike/agent-sandbox-substrate/reproduce.sh up` + the mapping spike's own `kubectl` commands — real `v0.4.6` cluster evidence for the demonstrated case
- `./scripts/check.ps1 -All` — repository baseline; confirms the evidence-only test logging and documentation changes do not regress behavior

## Evidence Required

- `docs/backends/agent-sandbox-v0.4.6-mapping.md`: the classified mapping table, per-row evidence citations, gap report, verified-facts-vs-open-questions split, and `#30`/`#89` reconciliation.
- `docs/evidence/E8-S1/agent-sandbox-mapping/summary.md` + captured output: pinned version, exact commands/manifests, and the reproduced unsupported/ambiguous case.
- Pinned-source references: upstream `v0.4.6` tag and the CRD schema / doc used per row.
- Passing `./scripts/check.ps1 -Docs`. Prose-only confirmation is not evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`; Agenova's contract and the accepted E1 fixtures are authoritative — upstream convenience cannot redefine them.
- This ticket must not change shared API types, `RuntimeBackend`, claim lifecycle, authority semantics, or gateway contracts.
- A mismatch becomes an explicit adapter gap or supported-gap candidate, never an automatic core-contract change; any proposed core-contract change needs a separate `decision-required` ticket with alternatives, evidence, impact, and product-owner approval.
- Provider-specific types, API group strings, and experimental manifests stay under `harness/spike/agent-sandbox-mapping/` or `internal/runtime/agentsandbox/`.
- Do not treat static manifests or a single local run as production backend verification; the report stays provisional and advisory to `#48`.
- Honour the 2-3 working-day time box; deliver partial `unknown` rows rather than guesses.

## Decisions and Blockers

- Planning depth: Task only. A time-boxed research spike whose deliverable is a provisional report; it freezes no contract and needs no `spec.md`/`design.md`.
- Owner authorization (2026-09-02): the project lead assigned `@yanyang15037755` as Owner, with `@Leo1piece` contributing commits and pairing as agreed between them. The final mapping report is the spike deliverable; independent review remains a PR gate.
- Collaboration decision: this work branch starts from the latest `main` and preserves Leo's three task-packet commits by cherry-pick. The Owner integrates the report, evidence, and quality gates in one #66 delivery lane.
- Decision (updated 2026-09-12): map against the merged five-operation RuntimeBackend in `internal/runtime/backend.go`: Allocate, Observe, Start, Terminate, Cleanup. Include #89 filesystem evidence fields without adding an operation. Legacy phase helpers are compatibility context only.
- Decision: the report lives in a new `docs/backends/agent-sandbox-v0.4.6-mapping.md`, not by editing the `#48`-owned `docs/backends/agent-sandbox.md`.
- Decision: reuse the E8-T3 substrate (`harness/spike/agent-sandbox-substrate/reproduce.sh`) to provision the pinned `v0.4.6` cluster rather than adding a second cluster bootstrapper.
- Decision (2026-09-02): map against `v0.4.6` / `extensions.agents.x-k8s.io/v1alpha1` — the version the shipped `internal/runtime/agentsandbox` adapter, `docs/backends/agent-sandbox.md`, and the E8-T3 substrate are all on. An earlier draft of this packet targeted `v1.0.0` / `v1beta1`; that was dropped so the mapping baseline matches the code `#48` will freeze against. Looking ahead to a newer upstream release is a separate follow-up.
- Decision (completed 2026-09-13): real-cluster work began only after #99 merged into `main`; #66 reused that exact E8-T3 substrate and did not stack on its former branch.
- Blockers / sequencing:
  - GitHub `blocked-by`: `#22` (E1-T1) — **merged**, so this ticket is unblocked.
  - Validation blocker resolved: PR #99 merged on 2026-09-13 as `6128855aa904f6d0a05aa4de1bccd83223cddd7e`; its owned substrate provisioned Agent Sandbox v0.4.6 for this experiment.
  - `#30`, `#89`, #99 and #124 are merged. Their contract and substrate dependencies are resolved; #48 can consume the updated report and bounded real-cluster evidence.
  - Environment resolved on 2026-09-13: Docker Engine 29.7.2, kind v0.33.0, kubectl v1.36.1 and Kubernetes v1.37.0 completed the experiment.

## Source Refresh — 2026-09-12

- Synced this branch with main at `0ee9042c2f6a11b722d0b5875e28f652dcfd2caf` by merge, preserving the original contributor commits.
- Project lead authorization on #66 dated 2026-09-10 explicitly permits source-based mapping, unsupported/unknown classification and the pending-verification checklist now that #30/#89 are merged.
- Reclassified allocation, identity and cleanup as translated; Start and Terminate as unsupported. Durability remains unsupported for reduced allocation/release correlation, separate from application outcome storage.
- Kept general isolation unknown and the adapter's explicit filesystem evidence Unsupported. Reference Simulated and local Git/Go compatibility do not establish BackendVerified.
- The later 2026-09-13 experiment supersedes this former blocker and closes PR #105's reproduced-negative-case finding.
- Validation results are recorded in [source refresh evidence](../../docs/evidence/E8-S1/source-refresh/summary.md).

## Real-Cluster Experiment — 2026-09-13

- Merged main through #99, then ran the final committed mapping script at `3ce542bf951a2cbab1533cda3befabbcf922415b` on the owned `kind-agenova-k8s-lab` context.
- Captured all four installed v0.4.6 CRD schemas and a unique Allocate/Observe/Cleanup lifecycle.
- Reproduced the bounded negative: after the assigned worker reported Ready, Start and Terminate each returned `runtime.ErrUnsupported` with their missing evidence channels stated explicitly.
- Confirmed cleanup independently: the claim and assigned Sandbox were absent, `Released=true`, and `Replaced=false`.
- Kept durability unsupported and general isolation unknown. The experiment observed filesystem evidence level `Unsupported` and makes no #51 isolation claim.
- Evidence: [summary](../../docs/evidence/E8-S1/agent-sandbox-mapping/summary.md) and [raw output](../../docs/evidence/E8-S1/agent-sandbox-mapping/output.txt).
