# Task: Shared installed-service API for CLI Work query and local Portal parity

- Ticket: [#167](https://github.com/wunderforge/agenova/issues/167)
- Mission: Make installed-service Work submission and later evidence query visible through one private API to both the CLI and local Portal.
- Target: `internal/console`, `internal/connectedclient`, `internal/cli`, `cmd/agenova`, `cmd/agenova-control-plane`, `ui`, reference docs and E2E tests.
- User value: A teammate can run Work from CLI and inspect that exact request/result in CLI JSON and `npm dev`, with no second demo data source.
- PRD outcome: [MVP user journey, facts, installation, console](../../docs/product/prd.md).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature spec](spec.md), [design](design.md), [architecture contract](../../docs/product/architecture-contract.md), [#165 operator path](../0165-cli-control-flow-e2e/task.md), [user-facing demo playbook](../../docs/harness/playbooks.md).

## Scope

In scope: explicit loopback connection to the installed private API; API-backed CLI Work list/show and connected run; actual registry-backed Portal setup; one documented `npm dev` connection; real kind/Ollama and Playwright parity evidence.

Out of scope: UI installation by Platform apply, production authentication, multi-user tenancy, durable evidence, multi-template selector, arbitrary adapters, broad administration and provider secret distribution.

## Acceptance Criteria

- `platform status` identifies the installed service and the local API connection command/address without claiming that an unopened tunnel is live.
- `agenova api connect` binds only loopback and reaches the private installed `/api` service through the caller's selected Kubernetes context/RBAC.
- CLI `run -f`, `work list`, and `work show <request-ref>` consume the same canonical API/evidence representation as local Portal; a request's identifiers, decision and terminal result match.
- The installed `/api/setup` reflects registered Policy and template, not the independent demo defaults; unavailable registration never renders a fabricated ready state.
- Real kind/Ollama Allow and Deny, API/UI parity, missing/unavailable and Origin/transport negatives, focused/full gates and independent review pass.

## Negative Case

- Unavailable service/context, port conflict, cross-origin request, invalid/unknown Work, missing registration, and denied admission fail visibly without memory/fixture fallback or leaked secrets.

## Execution Todo

- [x] Scout existing installed/private API, CLI exec transport, Portal connection and ticket overlaps.
- [x] Owner's 17 September instruction explicitly authorizes this E2E slice and direct merge after review/gates, superseding the normal packet pause.
- [x] Add CLI query and honest Platform/API connection discovery.
- [x] Connect local Portal to the private installed API with real setup data.
- [x] Verify one real kind/Ollama request through CLI, API and browser.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/console ./internal/connectedclient ./internal/cli ./cmd/agenova...`
- `npm --prefix ui run test:smoke`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Exact CLI commands, same request/evidence IDs across direct API and Portal, real worker/Ollama output, denied/no-claim case, browser screenshots/traces and missing/connection errors.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Never expose private Work API on ClusterIP or a non-loopback host listener. A local tunnel is transport, not authentication; document the trusted-machine/reference limit. Do not read credential-bearing files.

## Decisions and Blockers

- #146 overlaps; this ticket owns its remaining shared-API query/display slice, not a parallel service. #130 durability and #147 multi-template Portal selection are deferred and must not be marked complete by this work.
- Real kind/Ollama Allow and pre-claim Deny, CLI list/show parity, and two opt-in installed Playwright checks passed on 17 September 2026. The checked-in screenshots are synthetic-task evidence from the dedicated test cluster, not production data. Merge/board reconciliation remain gated by PR review and remote publication.
