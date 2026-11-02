# Task: Recover malformed Researcher actions within Work budgets

- Ticket: [#268](https://github.com/wunderforge/agenova/issues/268)
- Mission: recover invalid reference-Agent actions without premature exit, and default the Agent directory to the latest registered version per project/family.
- Target: reference Researcher loop, exact-image harness, Agent directory.
- User value: transient formatting mistakes no longer end a demo with unused call budget; old registrations no longer overwhelm discovery.
- PRD outcome: [Researcher repair and discovery](../../docs/product/prd.md).

## Context to Read

- [Architecture contract](../../docs/product/architecture-contract.md).
- [Start a GitHub Ticket](../../docs/harness/playbooks.md).
- `cmd/agenova-demo-worker/researcher_fast.go` and its behavioral tests.
- `internal/workerprotocol/feedback.go`, `exchange.go`, `research.go`.
- `harness/demo/research-mcp/build-worker.py`.
- `ui/src/ProductPages.tsx`, `connected-source.ts`.

## Scope

In scope:

- Strict validation, followed by a fresh governed model request with precise repair guidance while calls remain.
- Preserve observations and actual denials; suppress duplicate successful reads without changing denied authority.
- Distinguish final exhaustion from an announced retry that never occurs.
- Default catalog to latest numeric version per project/family, with an older-versions control. Preserve exact registration identity and server-filtered visibility.
- User confirmed directory cleanup only; retain ECR images, rollback artifacts, registrations and historical Work.

Out of scope: model IAM/provider changes, authority or protocol relaxation, infrastructure changes, marketplace, native Codex/Claude integration, automatic merge/deployment.

## Acceptance Criteria

- Successful read -> malformed finish -> malformed finish -> valid finish recovers with one actual read and four governed model calls.
- Malformed finish -> repeated successful read -> valid finish also recovers without duplicating the read.
- Denied tool remains denied after repair, never becomes a completed or unrelated-data answer; no backend execution on denial.
- Permanent invalid output stops at min(Work model-call budget, reference safety limit), emits exhaustion evidence and no result.
- Provider/transport denial stops immediately; a feedback recording failure does not launch another model call.
- Happy path remains one model call for ordinary knowledge or two for a single successful read.
- Directory keeps separate projects/families, sorts v10 after v9, retains unrelated unversioned Agents and exposes all registrations on request. Viewing/running remain independently authorized.
- Rendered UI proof and exact-image subprocess acceptance cover the changed paths.

## Negative Case

Malformed JSON cannot execute a tool or produce a result. Repair cannot expand authority, reset the Work budget, bypass denied operations or invent observations. Catalog filtering cannot expose records excluded by the server.

## Execution Todo

- [x] Scout current code and bounded latest production evidence through the normal authenticated API; retain raw records locally only.
- [x] Self-review scope, acceptance, negative cases and stable authority boundaries.
- [ ] Implement bounded repair and regression cases.
- [ ] Implement latest-version directory and rendered acceptance.
- [ ] Exercise the actual Worker image with synthetic governed responses.
- [ ] Run focused gates and `./scripts/check.ps1 -All`.
- [ ] Review diff, record exact residual failures, commit and push feature branch.

## Quality Gates

- `go test ./cmd/agenova-demo-worker ./internal/workerprotocol`.
- `py -3 harness/demo/research-mcp/build-worker.py` (local build; no push).
- UI focused tests, typecheck/build and rendered catalog smoke.
- `.\scripts\check.ps1 -All`.

## Evidence Required

Operation counts and output from scripted Worker conversations; exact-image acceptance; catalog screenshot/interactions; full-gate result. Scripted model replies are local regression evidence, not proof of cloud model quality. Live verification is a separate release gate.

## Decisions and Blockers

- 2 Nov: fresh v6 evidence confirms successful CRM observation followed by two malformed finishes; shared two-error counter exits after only 3/6 model calls. Another captured sequence mixes a malformed finish with duplicate-read rejection. Use existing Work budget/safety/deadline bounds instead of this premature counter.
- 2 Nov: discovery is display-only. Version grouping is project-qualified and numeric; known reference researcher / researcher-advisor registrations form one family. No registration or image is deleted.
- 2 Nov: known Windows full-suite baseline includes TestOwnerBootstrapDeliveryGuards; report its current result explicitly. Independent review remains a PR gate; no new administrator-merge authorization exists.
