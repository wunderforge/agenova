# Task: Deliver scoped cross-Work memory with PostgreSQL

- Ticket: [#179](https://github.com/wunderforge/agenova/issues/179)
- Mission: Let independently authorized worker claims write and retrieve persistent Memory through one governed interface, with zero backend calls for denied access and metadata-only public evidence.
- Target: Shared access contracts and resolution, a Memory interface and adapter, Platform configuration, controlled-worker protocol, fact journal, connected CLI/API/Portal, and task-owned reference evidence.
- User value: Work B and Work C can use authorized knowledge from Work A without inheriting its authority, confusing Memory with audit history, or exposing stored text by default.
- PRD outcome: Elaborates [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution) under the accepted bounded E17 extension. It does not add a complete Memory platform to the committed MVP.

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md) and [technical design](design.md)
- [Architecture contract](../../docs/product/architecture-contract.md): Product Center, Backend Neutrality, Authority and Credentials, Evidence Surfaces, Scope Discipline
- [AIDLC](../../docs/development/AIDLC.md): Sources of Truth, Adaptive Planning Depth, PR Review Context
- [Playbooks](../../docs/harness/playbooks.md): Start a GitHub Ticket, Change a Core Contract, Add a User-Facing Demo Slice
- [Access request](../../api/v1alpha1/claim_request.go), [template](../../api/v1alpha1/agent_template.go), [issued state](../../api/v1alpha1/sandbox_claim.go), and [authority resolution](../../internal/authority/resolver.go)
- [Trusted application preparation](../../internal/app/prepare.go), [run service](../../internal/app/run_service.go), [worker invocation composition](../../internal/console/service.go), and [worker protocol](../../internal/workerprotocol/protocol.go)
- [Platform contract](../../api/v1alpha1/platform.go), [Platform resolution](../../internal/platform/resolve.go), [adapter registry](../../internal/adapters/bundled/registry.go), and [installed service](../../cmd/agenova-control-plane/main.go)
- [Fact journal](../../internal/facts/journal.go), [evidence view](../../internal/evidence/view.go), and [connected client](../../internal/connectedclient/)
- [Governed Memory contract](../../internal/memory/contract.go), [trusted session service](../../internal/memory/service.go), [boundary tests](../../internal/memory/service_test.go), and [metadata validation](../../internal/facts/memory.go)
- [Trusted retry tests](../../internal/memory/retry_test.go) and [composed PostgreSQL retry test](../../internal/memory/postgres/retry_test.go)
- [PostgreSQL adapter core](../../internal/memory/postgres/backend.go), [version-one migration](../../internal/memory/postgres/migrations/001_memory.sql), and [SQL-driver contract tests](../../internal/memory/postgres/backend_test.go)
- [Projection tests](../../internal/evidence/privacy_test.go), [HTTP boundary](../../internal/console/http.go), [CLI output](../../internal/cli/cli.go), [Portal reader](../../ui/src/connected-source.ts), [Portal view](../../ui/src/ConnectedPortal.tsx), and [connected browser smoke](../../ui/smoke/connected.spec.ts)
- [Credential resolver #155](https://github.com/wunderforge/agenova/issues/155), [Policy kernel PR #207](https://github.com/wunderforge/agenova/pull/207), [organizational constraints #204](https://github.com/wunderforge/agenova/issues/204), [worker identity #205](https://github.com/wunderforge/agenova/issues/205), and [MCP integration PR #192](https://github.com/wunderforge/agenova/pull/192)
- [Kind startup network gap #208](https://github.com/wunderforge/agenova/issues/208), [public egress gap #202](https://github.com/wunderforge/agenova/issues/202), and [current reference runbook](../../docs/reference-cli-kind-ollama.md)

## Scope

In scope:

- Optional backend-neutral read/write operation grants alongside logical Memory scopes.
- A trusted, claim-bound Memory interface, explicit scope ownership, and append-only Write plus bounded literal-keyword Search.
- One PostgreSQL adapter, forced row-level security, transaction-local ownership context, durable invocation receipts, explicit failure categories, and Pod-restart persistence.
- Optional Platform Memory backend/routes, host-side credentials through the E14-owned boundary, and the existing controlled-worker execution path.
- Shared metadata-only invocation evidence and an explicit content-redacted query projection for Memory-requesting Work.
- Real kind/PostgreSQL acceptance, independent zero-call and data-use oracles, browser proof, cleanup instructions, and independent teammate reproduction.

Out of scope:

- Automatic extraction, embeddings, semantic search, knowledge graphs, update/delete-by-object-ID, general retention administration, or a second production adapter.
- A second Policy engine, provider credential resolver contract, identity system, or workflow orchestrator.
- Durable Work history, production tenancy/HA, cluster creation, database operator, upgrades/restore service, and survival after cluster/PVC deletion.
- Claims of authenticated hostile-worker isolation before #205 or network claims without real probes.

## Acceptance Criteria

1. An operator can install the optional backend and bind one logical scope to one trusted Team/Project. Identical reapply preserves named resources and stored Memory; missing installation remains explicitly unsupported.
2. Work A writes a per-campaign synthetic fact using a Running claim. Independent Work B obtains its own authority, searches the same allowed scope, and derives a result from the returned content. B's input contains no expected answer.
3. After separately restarting the Control Plane and PostgreSQL Pod without deleting the PVC, new Work C retrieves the fact. Restored Memory does not restore prior claims or imply durable Work history.
4. Each invocation checks its trusted worker/claim context, Running lifecycle, operation grant, logical scope grant, and trusted scope ownership before calling the backend.
5. Cross-team/project, no-grant, read-only-write, unknown/non-Running, and claim-A-context/claim-B-target attempts produce inspectable denial metadata and zero per-invocation backend calls. No victim-claim fact is fabricated.
6. Success, empty search, denial, unavailable/unsupported backend, timeout, cancellation, backend failure, and uncertain write acknowledgement remain distinguishable without mock fallback.
7. CLI/API/Portal expose the same authority and invocation metadata, redact Memory-requesting task input and free-text outcome, and never automatically serialize Memory body, query text, SQL parameters, or credentials.
8. Database ingress permits the Control Plane and blocks worker Pods, including newly started workers, with positive controls and an independent network oracle.
9. Focused behavioral gates, the complete repository gate, rendered UI evidence, and independent teammate reproduction are captured with versions, commands, and sanitized output.

## Negative Case

- A worker bound to claim A nominates running claim B or an ungranted scope: deny before backend access; record only trusted-context attribution.
- Requested scopes without requested/granted operations never gain implicit read/write. Ownership denial is not disguised as an empty database search.
- A lost commit acknowledgement is not reported as certain rollback and is not automatically retried under a fresh invocation ID.
- A committed write followed by evidence failure fails the Work explicitly; it is not replayed to conceal incomplete facts.
- Sentinel content in request input, body, query, provider error, and credentials must not appear in default evidence or worker configuration.
- Static manifests, fake adapters, successful database authentication, and a PVC declaration alone cannot prove the real backend or network gates.

## Execution Todo

- [x] Scout current main, public access types, worker binding, journal/evidence surfaces, and accepted E17 outcome.
- [x] Create Task + Spec + Design from the canonical templates and record user-selected defaults.
- [x] Assignee self-review: check scope, acceptance, negative cases, constraints, and named gates against #179, PRD, and the architecture contract; no additional architecture exception is requested.
- [x] Record user authorization to follow latest main's assignee-self-review workflow before implementation (2026-10-08).
- [ ] Resolve the host-credential contract dependency and coordinate shared-module integration with #192/#204/#207.
- [ ] Preflight the explicit kind context, CNI enforcement, existing reference install, PVC provisioner, PostgreSQL image digest, and Go-compatible driver version.
- [x] Slice 1: add operation grants, strict validation, resolution/copy/provenance, representative fixture-based cases, and generated frontend contracts.
- [x] Slice 1: implement the governed Memory boundary and deterministic fake-adapter tests with complete zero-call negatives and fact correlation.
- [ ] Slice 2: implement PostgreSQL schema, migrations, forced RLS, bounded transactions, atomic row/receipt write, replay dedup, and explicit uncertain/fault outcomes.
- [x] Slice 2 core: implement driver-independent PostgreSQL SQL/transactions through a host-owned standard DB handle; add SQL-driver tests without installing a live database or claiming live persistence.
- [ ] Slice 2 connection: select a security-reviewed, Go-compatible driver through accepted host credential composition; no implicit DSN/env lookup or competing credential resolver.
- [ ] Slice 2: capture real database evidence including scope isolation, connection-pool reuse, restart, duplicates, cancellation, and lost acknowledgement.
- [ ] Slice 3: wire optional Platform configuration, host-side credentials, PVC/network prerequisites, worker Write/Search, and installed capability validation.
- [ ] Slice 4: add Memory facts, query redaction, CLI/API/Portal parity, read-side integrity negatives, and rendered browser proof.
- [x] Slice 4 privacy core: project task input/outcome at public submission/query/list/CLI boundaries, validate explicit redaction and add rendered desktop/mobile fixture proof. Live Memory invocation reader/UI parity remains pending.
- [x] Review P1: preserve the original uncertain-write receipt through a trusted retry continuation, rerun authority checks, retain unique audit attempts, and prove zero duplicate writes and evidence-failure closure. Independent re-review remains required.
- [ ] Slice 5: execute the real A/B/restart/C campaign, hostile-request negatives, worker network controls, failure cases, and sentinel scans.
- [ ] Run focused gates followed by `./scripts/check.ps1 -All` after each implementation slice; stop expansion on failure.
- [ ] Review scope, regression risk, and source ownership; obtain independent PR review and teammate reproduction before reporting E17 complete.

## Quality Gates

- Preparation: `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` and `git diff --check`.
- Shared-contract slice: `go test ./api/v1alpha1 ./internal/authority ./internal/authorization ./internal/issuance ./internal/app` plus generated UI contract checks.
- New Memory packages: focused Go tests and race tests; package commands are recorded when the packages exist.
- Database slice: opt-in real PostgreSQL contract/integration tests. Missing explicit backend configuration must fail a selected real gate, not silently skip it.
- Installed slice: a task-owned campaign requiring explicit cluster context, namespace, installed revision, backend version, and initialized dataset. No unscoped teardown.
- Privacy/UI slice: connected CLI/API/browser parity, payload/credential scans, and screenshots from real records. Fixture browser smoke is a separate gate.
- Baseline: `pwsh -NoProfile -File ./scripts/check.ps1 -All`. This is not a substitute for the live database/cluster campaign.

## Evidence Required

- Task-owned evidence under `docs/evidence/179/`: baseline revision, focused/full gate output, real database tests, kind campaign summary, sanitized raw output, and screenshots.
- Independently observed backend call counts for every denied case, with control-plane health traffic excluded from the per-invocation measurement.
- Per-campaign random dataset plus an independent oracle proving B/C used retrieved data; body material remains in private test input, not the public export.
- SQL-role/RLS checks, immutable ownership binding tests, persistent row/receipt checks, and real restart output.
- Worker-at-startup and steady-state database probes, successful control-plane control, CNI identity, PVC identity, and bounded cleanup inventory.
- Evidence-loss and commit-uncertainty results, default evidence/worker secret scans, and explicitly named residual gaps.
- A second teammate's successful clean-environment reproduction, or an explicit outstanding acceptance blocker.

## Constraints

- Preserve the architecture contract. The accepted Epic is a bounded extension, not a change to the committed MVP completion requirements.
- Requested access remains intent. Policy may narrow authority but cannot add Memory capabilities; provider shapes stay in adapter configuration.
- Scope lifetime exceeds one claim; each subsequent Work needs fresh authority. Memory content is untrusted data, not instructions or Policy.
- Configuration/route changes affect later Work. Active claims retain their issued authority and captured scope-ownership binding.
- No credential values in Platform YAML, requests, templates, claims, worker configuration, status, or evidence.
- Restrict ordinary restart/reapply to named task resources; PVC deletion is a separate explicit cleanup action.
- Keep documentation in English and source/code changes scoped to the active slice.

## Decisions and Blockers

- Planning baseline: main `ebb4c04f7b8cc76d63036c820e04c62af68fad3d`, checked on 2026-10-08.
- Owner: `yanyang15037755`, assigned on #179. An independent Reviewer has not been named in the issue.
- User decisions: governance/persistence first; local kind; PostgreSQL first adapter; reference trusted context before E14 identity; approximately 12-15 engineering days excluding dependency/review wait.
- User workflow update (2026-10-08): explicitly authorized latest main's self-review process; this supersedes the earlier session packet-approval stop. Independent review remains a PR gate.
- Packet status: self-reviewed and authorized for implementation. Live-backend acceptance remains pending executable evidence.
- Credential blocker: #155 is open and unassigned with no approval/implementation evidence observed. Coordinate its smallest accepted host-side resolver slice; do not copy #192's provisional token configuration as a new Memory secret contract.
- Integration coordination: #192/#207 are open PRs; #204/#205 are open issues. Rebase on accepted producers and preserve their contracts, without stacking unrelated implementation branches.
- #205 is not a blocker for the approved controlled reference path. Authenticated workload claims require a later integration rerun using its accepted verifier.
- Environment: Windows PATH contains Go and PowerShell but no Docker/kind/kubectl. WSL Ubuntu exists; `wsl -d Ubuntu -- which docker kind kubectl go` returned no paths (exit 1). Reference tools, cluster, CNI, and storage remain unverified; no cluster was changed.
- General PostgreSQL, inference, and cluster readiness results remain pending executable evidence.

## Preparation Verification - 2026-10-08

- Created the dedicated `codex/0179-scoped-memory` branch from current main; retained pre-existing untracked `deliverables/` without editing it.
- Canonical generator: `pwsh -NoProfile -File ./scripts/new-task.ps1 -Issue 179 -Slug scoped-memory -Title 'Deliver scoped cross-Work memory with PostgreSQL' -WithSpec -WithDesign`.
- Completed task/spec/design and checked for unfilled template markers. Only preparation documents have been added; shared contracts, implementation, and authoritative product documents are unchanged.
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`: passed after retry outside the sandbox. The first attempt could not enumerate an existing `.tmp/montage_convert_oq6q8l0b` directory; the retry passed metadata, architecture, Markdown links, backend boundaries, delivery contracts, and explicit-context cases.
- [Captured documentation gate output](preparation-docs.log) records the final preparation run.
- `git diff --check` reported no errors. Individual `git diff --no-index --check -- NUL <packet-file>` checks produced no whitespace diagnostics (exit 1 denotes added content).
- Product focused tests, `-All`, live database/kind tests, and browser acceptance have not run. There is no product implementation to verify yet.
- The original preparation stopped for packet approval. The later explicit user authorization above supersedes that stop.

## Implementation Verification - 2026-10-08

- Operation-grant contracts and focused API/authority/issuance/application/connected-client tests passed on Windows. No legacy fixture was given implicit operations.
- Native Windows `-All` reached an unchanged operator symlink test and failed because the account lacks symlink creation privilege. An elevated retry of that test had the same result; the operator source was not changed.
- Prepared workspace-local, ignored Linux tools: Go 1.27.1, Node 24.12.0, PowerShell 7.6.6, and Playwright Chromium 153. Tool archives were verified against published checksums. Source `go.mod` remains unchanged at its existing Go requirement.
- The first Linux full gate passed Go, contracts, 124 frontend tests and build, but browser launch failed on missing WSL libraries. Extracted Ubuntu libnspr4/libnss3/libasound2t64 into ignored workspace storage; did not modify system packages or project dependencies.
- The [Linux retry](slice1-contracts-linux-retry.log) passed the full repository gate, including all Go tests, integration compile checks, 124 frontend tests and all 52 browser smoke tests. These are existing fixture/connected smoke gates, not installed Memory acceptance.
- The Memory boundary now exists independently of installed wiring. It binds issued identity/authority and worker state, captures immutable ownership, limits calls by run/caller/five-second deadlines, sanitizes replies and records correlated metadata. There is no persistence fallback or public Memory endpoint.
- Focused `go test -count=1 ./internal/memory ./internal/facts ./internal/app` passed. Tests cover zero-call denial, cancellation, typed faults, evidence failure before/after dispatch, fake-adapter acknowledgements of known committed writes, namespace misrouting, encoded reply limits, concurrent invocation IDs and fact correlation.
- The first Linux boundary run stopped at the race detector because cgo had no C compiler. Installed Ubuntu GCC 13.3 and libc development headers through the reviewed environment setup. This changed WSL tool/runtime packages, not repository dependencies; no cluster or database was installed.
- The [boundary retry](slice1-boundary-linux-retry.log) passed focused tests, `go test -race -count=1 ./internal/memory ./internal/facts ./internal/app`, contract generation, and the post-boundary full repository gate, including all Go tests, 124 frontend tests, production build and 52 browser tests.
- Final diff review added fail-closed CLI/Portal safeguards for Memory metadata on legacy records and unsupported Memory decisions. Native connected-client regression tests passed. The [final Linux campaign](slice1-final-linux.log) passed focused tests, the same race gate and `-All`, now with 125 frontend tests and 52 browser cases. No source changes followed that campaign; the only later edits are these evidence summaries.
- [Contract and boundary evidence](../../docs/evidence/179/contract-boundary.md) distinguishes these checks from pending real-backend acceptance. Diff self-review found no backend/provider types in the shared grant or Memory interfaces. `git diff --check` passed; pre-existing `deliverables/` and operator source remain untouched.
- `gh issue view 155 --json number,title,state,assignees,updatedAt,body` reconfirmed the shared host-credential Ticket is open/unassigned, last updated 2026-09-21. No accepted resolver implementation was found; installed credentialed PostgreSQL remains blocked on its producer.
- PostgreSQL schema/adapter, Platform/worker integration, public content projection, live database/network campaign, rendered Memory records and independent reproduction are not implemented or proven by these gates. E17 is not complete.
- Requested user decisions at the end of this implementation campaign: authorization to take the E14-owned #155 in a separate task packet, versus waiting for its producer; and the name of the existing kind context for future explicit-context acceptance. Neither decision has been supplied. No unrelated Ticket implementation or GitHub mutation had occurred during the campaign.

## Draft PR Publication

- The user explicitly requested a draft PR for the verified foundation. Publish the existing `codex/0179-scoped-memory` branch without pre-existing `deliverables/`, ignored tool/runtime files or unrelated changes.
- At initial publication this draft contained Slice 1 only. The continuation below adds PostgreSQL core and public content projection. Installed/worker wiring, live acceptance and independent review remain outstanding; do not mark ready or merge while #179 acceptance is incomplete.
- The repository's PR delivery validator requires a closing Ticket reference matching this task packet. The draft therefore targets eventual closure of #179 on a future accepted merge, rather than bypassing that check with `Refs`. Opening a draft does not close the Ticket.
- No product source changed after the successful final campaign. Documentation and PR-body checks are rerun for publication; GitHub CI status is separate from captured local passes.

## Adapter and Privacy Continuation

- The user authorized continued PostgreSQL-adapter and redaction development after draft PR #212. This is not authorization to take #155 or provision an unspecified cluster.
- The adapter core consumes a trusted host-created `database/sql.DB`, not a new credential API. Driver locking is deferred after the compatibility/security preflight described in the design; `go.mod` is unchanged.
- Initial adapter unit gate: `go test -count=1 ./internal/memory/postgres ./internal/memory` passed. The first run exposed an overly broad assertion in the fake commit-error test; narrowing it to distinguish server-declared rollback from unknown acknowledgement resolved the failure. No gate was weakened.
- SQL-driver tests exercise parameter binding, transaction-local ownership, read-only Search, serialized receipts, replay/digest mismatch, atomic row/receipt sequencing, safe role/schema readiness and sanitized fault/commit outcomes. They do not execute PostgreSQL, prove RLS enforcement or storage restart durability. Those gates remain blocked on accepted host composition and explicit real-backend configuration.
- Adapter core focused/race and full Linux gate passed; [captured output](slice2-adapter-linux.log) includes 125 frontend tests and 52 browser cases.
- Public content projection now exists at reference/HTTP submissions, query/list/Claim and CLI boundaries. Private request/result data stays available for execution; scope-only and denied Memory intent are deliberately projected too. Readers reject absent/unknown/duplicate markers, missing empty input and hidden content. Ordinary non-Memory records retain their existing display.
- Privacy focused Go tests and race checks passed for evidence/console/connectedclient/CLI/app/Memory packages. The two focused desktop/mobile browser cases passed and their screenshots were inspected for withheld content and overflow. The complete post-review Linux gate passed all Go tests, generated contracts, 126 frontend tests, production build and 54 browser cases; [final output](slice2-final-linux.log) and [adapter and privacy evidence](../../docs/evidence/179/adapter-privacy.md) record exact gates and residual gaps. Only documentation/publication edits followed this passing source campaign.
- A custom View serializer failed the unchanged contract generator, so public outputs use explicit shared projection/serialization without changing canonical input tags. Further early failures were fixture/import assertions and browser-test registration; fixes preserve all privacy gates. Failed logs remain local; passing output is selected for review.
- No driver or live PostgreSQL, installed Memory wiring, complete Memory invocation reader, network/data-use campaign, live record screenshots or independent reproduction has been delivered by these checks. Keep #212 draft and #179 open.

## P1 Review Correction - 2026-10-09

- The user authorized fixing the [uncertain-write retry finding](https://github.com/wunderforge/agenova/pull/212#discussion_r4216500678), not expanding into credential or installed-backend work.
- Retain the original receipt ID through an opaque host-only retry continuation bound to the original Session/request. Recheck admission on every retry and use fresh journal IDs for separate attempts rather than weakening the journal's one-decision/attempt/outcome rules.
- Copies share disposition: known completion/evidence failure disables recovery; transient faults retain uncertainty. Cancellable serialization stops waiting before admission with no invocation/backend call. Worker JSON, provider credentials, claim/resource schemas, journal rules and canonical fixtures remain unchanged.
- Native focused tests and 20 repeated retry cases passed. The [final Linux source campaign](review-retry-final-linux.log) passed focused tests, race checks, 20 repeated retry/cancellation/concurrency cases, and the complete repository gate: all Go tests, generated contracts, 126 frontend tests, production build and 54 browser cases. It covers the cancellable queue and already-waiting regression; only documentation/publication edits followed. [Review evidence](../../docs/evidence/179/retry-review.md) separates scripted SQL-driver proof from still-blocked real-backend acceptance. Request independent re-review after publication; keep #212 draft and #179 open.
