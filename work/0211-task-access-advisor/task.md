# Task: Optional task access advisor

- Ticket: [#211](https://github.com/wunderforge/agenova/issues/211)
- Mission: translate task context and an Agent's access guide into reviewable requests, optionally submitting automatically through existing authorization.
- Target: v1alpha1 optional guide/config, internal/accessadvisor, console, CLI packaging, Connected Portal, local acceptance.
- User value: manual, suggested and automatic requests coexist without changing Agent images or explicit upstream clients.
- PRD outcome: [approved optional preflight](../../docs/product/prd.md).
- Owner decision: 30 Oct 2026 conversation approves local implementation, tests, commit and feature-branch push; no PR/merge/deploy/paid inference.

## Context to Read

- AGENTS.md; docs/product/prd.md; docs/product/architecture-contract.md Submission and Resolution / Authority and Credentials.
- spec.md and design.md in this packet.
- api/v1alpha1/{agent_template,claim_request,platform}.go; internal/app/prepare.go; internal/console/{service,http,durable}.go.
- internal/modelprovider/adapter.go; internal/platform/resolve.go; ui/src/WorkSubmission.tsx; tools/local-product-preview.

## Scope

In scope: optional immutable author guide; rules/real-model selectors; mode/config/budget limits; authenticated advice and auto-submit; owner-bound receipts/replay; unchanged canonical authorization; CLI packaging/UI/local acceptance/backup branch.

Out of scope: workflow or Skills runtime, dynamic elevation, new claim phases, executing guide code, new cloud resources, migrations, deployment or paid inference.

## Acceptance Criteria

- Old templates/platforms and explicit requests work without Advisor/model calls.
- ACCESS.md is packaged and digest-bound to immutable template; no runtime URL/file fetch.
- Suggest returns an inspectable draft and explanation without starting Work; caller can edit and submit.
- Automatic accepts one intent and starts without confirmation only after current authorization; disabled/failed Advisor never silently runs/broadens.
- Unknown/ambiguous/invalid output, missing required grant, cross-user receipt, changed task, duplicates, revoked session and exhausted bounds fail safely.
- Planning calls/evidence are separate from worker calls; successful provenance uses existing evidence persistence.
- Real local Ollama selector, provider failure tests, local HTTP/UI flows, screenshots and manual regression.

## Negative Case

An unknown/over-ceiling guide/model capability cannot grant authority. A required profile permission removed by current Policy must prevent auto allocation. A caller cannot accept another caller's plan or swap task/template.

## Execution Todo

- [x] Scout code and prior practices; self-review accepted scope and authorization boundary.
- [x] Optional guide/Platform contracts and compatibility.
- [x] Bounded selectors, hashes, owner-bound receipts and budgets.
- [x] Authenticated HTTP, confirmation/auto submission, replay and current Policy checks.
- [x] CLI guide packaging, Platform composition and UI.
- [x] Focused and combined local tests; real Ollama; browser screenshots.
- [x] Run full repository checks, distinguish pre-existing failures, review diff.
- [x] Update evidence/local guide; commit and push feature branch only.

## Quality Gates

`go test ./api/v1alpha1 ./internal/accessadvisor ./internal/console ./internal/platform ./internal/cli ./internal/evidence`

UI contracts/typecheck/affected Vitest/Playwright; `.\scripts\check.ps1 -All`.

## Evidence Required

Plan/request/grant/result, real local model metadata, zero-allocation negatives, explicit-client regression, screenshots and reproduction guide. Label synthetic local runtime separately from real inference; no claim of fresh AWS enforcement.

## Constraints

No cloud requests, merge/deploy or credential-file reads. Preserve debug.log, old execution contract and explicit API. D: caches. Independent PR review happens only when a PR is requested.

## Decisions and Blockers

- First selector chooses one registered access profile; it never emits arbitrary authority-bearing JSON. Interface permits later bounded composition.
- Draft cache/limits are bounded per process, not a durable monetary spending cap. Accepted provenance persists with Work; expired draft recovery is explicit.
- User approval expands #211 from exploration to a bounded local implementation; no unresolved architecture conflict.
- See [local evidence and acceptance guide](evidence.md). Actual local Advisor inference is verified; volatile task fixtures are explicit. A real PostgreSQL restart remains unverified because Docker/local database was unavailable. Full-check Windows Owner-bootstrap failure is recorded without skipping it.
