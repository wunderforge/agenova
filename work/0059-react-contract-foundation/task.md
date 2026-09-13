# Task: React contract bindings and fixture evidence source

- Ticket: [#59](https://github.com/wunderforge/agenova/issues/59)
- Mission: Establish a minimal React and TypeScript fixture storyboard using mechanically checked canonical contracts and one replaceable evidence source.
- Target: New `ui/` foundation, contract tooling, focused frontend tests, and the shared repository quality gate.
- User value: A contributor can inspect the available canonical request, running/allowed claim, and pre-claim denial without mistaking requested access for authority.
- PRD outcome: [Read-only claim console](../../docs/product/prd.md#7-read-only-claim-console) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), limited here to Fixture / Mock delivery.

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Consumer behavior specification](spec.md).
- [AIDLC planning approval and source ownership](../../docs/development/AIDLC.md#from-existing-ticket-to-task-packet).
- [Start a GitHub Ticket and Add a User-Facing Demo Slice](../../docs/harness/playbooks.md).
- [Architecture: Evidence Surfaces](../../docs/product/architecture-contract.md#evidence-surfaces), Submission and Resolution, and Authority and Credentials.
- [ClaimRequest specification](../0024-claim-request-v0/spec.md) and [issued-state specification](../0025-sandbox-claim-v0/spec.md).
- `api/v1alpha1/claim_request.go`, `sandbox_claim.go`, `types.go`, `validation.go`, and their focused tests; `agent_template.go` only for shared Duration serialization and validation primitives.
- [Canonical fixture manifest](../../harness/fixtures/contract/v0/manifest.json), its referenced inputs and `inventory_test.go`.
- [Quality gates](../../docs/harness/quality-gates.md), `scripts/check.ps1`, `scripts/checks/go.ps1`, `.github/workflows/ci.yml`, and `.github/pull_request_template.md`.

## Scope

In scope:

- Minimal React + TypeScript development/build foundation under `ui/` with a locked dependency graph.
- Generate or mechanically verify the full transitive JSON shapes of ClaimRequest, SandboxClaim, Decision, EffectiveAuthority, Evidence, and their IssuedState envelope against `api/v1alpha1`; include enums, optionality, nulls and custom Duration serialization.
- A single typed fixture EvidenceSource reads manifest-referenced canonical inputs directly. Components receive the interface through injection and never import fixtures.
- A read-only executable storyboard for available fixture states and visible malformed/missing/unknown data diagnostics.
- Repeatable drift, adapter/component, browser smoke/render, type, build and shared baseline checks.

Out of scope:

- HTTP transport and live polling (#68/#60), final console/dashboard, search, mutation, policy editing, operational controls, or runtime integration.
- New governance models, redefinition of canonical contracts, copied UI fixture sets, or richer invocation/lineage schemas than upstream currently provides.

## Acceptance Criteria

- All five requested bindings and reachable types are generated from or structurally checked against canonical Go JSON contracts; stale output and unsupported types fail rather than falling back to any.
- One manifest-driven fixture source consumes all seven ClaimRequest and five IssuedState cases directly; caller-invalid issued states use the caller trust boundary, never the trusted system boundary.
- Components display request-only intent, Team A Allow/Running, Team B pre-claim Deny, and explicit invalid/missing/unknown states without synthesizing a claim, grant, backend identity or decision.
- A substitute in-memory EvidenceSource passes the same component tests without changing rendering semantics.
- Contract drift fails a documented frontend command, including a demonstrated mutation of a generated binding or source shape.
- Focused frontend checks, executable rendered proof, build and `./scripts/check.ps1 -All` pass after implementation; CI PR/Main reuse the same frontend gate.

## Negative Case

- Every invalid ClaimRequest and IssuedState manifest case produces the expected categorized failure; never echo credential-like fixture values in the UI.
- Delete a required field and add an unknown field to an in-memory canonical fixture derivative: show the field path and failure visibly, with no authority fallback. Test an unknown decision/phase value too.
- Denial must remain claimless and authorityless; missing invocation arrays must not silently become evidence of zero invocations. Distinguish schema-permitted normalization from a source-data diagnostic.
- ApprovalRequired is never interpreted as Allow; if tested through a derived snapshot, label it as derived, not a canonical fixture state.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Packet at 3504988 approved by the user in the task; approval recorded on #59 before resuming implementation.
- [x] Add minimal React/TypeScript foundation and deterministic canonical binding generation/check.
- [x] Implement one typed fixture source and fixture/canonical-validator parity evidence.
- [x] Implement the small read-only storyboard with explicit unknown/missing/invalid behavior.
- [x] Add adapter/component tests, source substitution test, drift failure proof, and browser smoke with rendered artifacts.
- [x] Integrate frontend validation into shared -All and PR/Main gates and provision Node in CI.
- [x] Run focused checks and `./scripts/check.ps1 -All`; record exact outputs and fixture IDs.
- [x] Review scope and provenance; contributor commands and exact implementation evidence are prepared for PR review. Do not merge.

## Quality Gates

Planning-only gates (available now):

- `./scripts/check.ps1 -Docs`
- `./scripts/check.ps1 -All`
- `git diff --check`

Implementation commands:

- `npm --prefix ui ci`
- `npm --prefix ui run contracts:check` (canonical generation comparison plus fixture parity and negative drift test).
- `npm --prefix ui run typecheck`
- `npm --prefix ui test -- --run`
- `npm --prefix ui run build`
- `npm --prefix ui run test:smoke` (browser rendering plus named screenshots).
- `./scripts/check.ps1 -All` (must include frontend checks); CI `-Profile PR`/`Main` inherit them.

## Evidence Required

- Exact command, exit status and output for each gate, separating planning checks from unimplemented frontend gates.
- Fixture matrix: `claim-request.valid.team-a-engineer-json`, `claim-request.valid.team-a-engineer-yaml`; `claim-request.invalid.missing-template`, `.missing-task`, `.missing-runtime`, `.self-asserted-principal`, `.secret-value`; `issued-state.valid.team-a-engineer`, `.team-b-denial`; `issued-state.invalid.caller-effective-authority`, `.caller-claim-phase`, `.caller-backend-identity` (abbreviated suffixes retain the preceding prefix).
- Rendered proof named request-only, allow-running, deny-no-claim, invalid-caller-state, missing-field and unknown-field. Record full fixture IDs and precise derivation paths for synthetic negatives in final evidence.
- Deliberate stale-binding failure followed by restored passing check; fixture YAML/JSON equivalence; original canonical files unchanged.
- Browser smoke proves selection, text, missing-data labels and no fabricated authority; screenshots supplement assertions.

## Constraints

- Preserve `docs/product/architecture-contract.md`; do not broaden Ticket or PRD without a recorded human decision.
- Go canonical parsers/types and shared fixture manifest remain authorities. Transport result/error wrappers are permitted, duplicate governance entities are not.
- Do not make UI-side authorization decisions; present issued decisions and authority separately from requested intent.
- Keep all repository work inside the Agenova checkout. No live service, secret files, or real backend is needed.

## Decisions and Blockers

- Planning depth is Task + Spec: replacing a fixture source later and exposing incomplete data need agreed cross-component semantics. No separate design document is justified for this bounded foundation.
- Proposed implementation: Vite/React/TypeScript; deterministic Go-aware binding generation with explicit serialization handling and failure on unsupported shapes. Canonical Go validators act as fixture oracles so frontend validation cannot silently diverge. Tool/library versions will be pinned during approved implementation.
- #24 and #25 are closed and their canonical types/fixtures exist on the cloned main branch; no external contract blocker remains.
- No existing #59 packet or approval comment was present at planning start. AGENTS.md and AIDLC require stopping before implementation until the Owner and independent Reviewer approve this packet in #59. Reviewer selection remains with the Owner.
- Planning commit 3504988 was approved by the user on 2026-09-08; see [approval record](https://github.com/wunderforge/agenova/issues/59#issuecomment-5580808523). Implementation now follows that approved scope.

### Planning validation (2026-09-08)

- Base: `3365cd0e37d181146dd5b6f8e65a58e03b6fb39e`.
- `./scripts/check.ps1 -Docs`: passed all seven documentation, metadata, architecture, link, boundary and delivery-contract checks.
- `./scripts/check.ps1 -All`: passed docs, gofmt, module consistency, `go vet ./...`, `go test -count=1 ./...` (12 tested packages), and `go test -run '^$' -tags integration ./harness/integration/agentsandbox/` (compile only; no real backend test).
- `git diff --check`: passed. The baseline's line-ending-only go.mod rewrite was restored; no dependency changes are part of planning.
- Frontend checks/rendered proof: not run because implementation is gated by packet approval. These baseline results do not prove #59's frontend acceptance criteria.

### Accepted implementation and evidence (2026-09-08)

- Implementation commit: `93ebbeb`. [Exact results, full fixture matrix, raw output and six screenshots](../../docs/evidence/59/frontend/summary.md).
- Canonical types and fixtures are unchanged. The v0-only Go generator emits bindings and runtime shape metadata; a build-time fixture adapter invokes the canonical validators and emits only normalized valid data or sanitized diagnostics. No semantic/policy validator was duplicated in TypeScript.
- Clean `npm ci`, all 40 adapter/component tests, 7 browser tests, drift failure proof, TypeScript and production build passed. `./scripts/check.ps1 -All` passed including 13 Go test packages and the shared frontend gate.
- The fixture source takes canonical-parser output at build time; future HTTP integration must supply a trusted semantic boundary. Only the two existing issued positive fixture states are claimed. No real backend, live endpoint or race run was performed locally.
- Windows preview teardown uses the Vite API directly after shell process-tree teardown stalled; the accepted browser command exits cleanly.
- No implementation blocker remains. Final independent review and merge remain human decisions; this task does not merge.
