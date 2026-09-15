# Task: Enforce Model Gateway profile

- Ticket: [#35](https://github.com/wunderforge/agenova/issues/35)
- Mission: Bind every structurally valid Model Gateway attempt to the active claim's system-issued model profile before any provider call.
- Target: `internal/app/`, `internal/gateway/`, `internal/modelgateway/`, and focused reference tests
- User value: A running worker can use only the logical model profile granted to its claim; it cannot select a different model tier by changing request data.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture authority boundary](../../docs/product/architecture-contract.md#authority-and-credentials)
- [Current gateway contract](../../internal/modelgateway/gateway.go)
- [Application-owned claim state](../../internal/app/run_service.go)
- [Team A effective-authority fixture](../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json)

## Scope

In scope:

- Reuse the application-owned claim-plus-authority read boundary introduced with #34.
- Require the requested logical model profile to exactly match the system-issued effective model profile.
- Keep the existing optional policy hook as an additional restriction only; it cannot widen effective authority.
- Record stable Deny outcomes for resolved claims and prove every denial makes zero provider calls.

Out of scope:

- Provider-model routing, billing, token accounting, prompt governance, network transport, or provider credentials.
- Claim-ID authentication or context-mismatch enforcement, which remains owned by #121.
- Durable facts or production provider adapters.

## Acceptance Criteria

- The canonical running Team A claim can invoke its exact granted logical model profile and reaches the adapter once.
- An ungranted profile returns a stable Deny decision and makes zero provider calls.
- Unknown and terminal claims fail closed and make zero provider calls.
- A custom Allow policy cannot bypass the effective-authority ceiling; a custom Deny or ApprovalRequired result can further restrict an otherwise granted call.
- The same application-owned snapshot supplies lifecycle and authority to both Tool and Model Gateway enforcement.

## Negative Case

- Replacing the granted profile with another non-empty logical profile must return Deny and leave the provider call count at zero.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation. The Owner explicitly authorized continuous takeover and delivery on 2026-09-15; #34 and #35 are one reviewed implementation batch.
- [x] Reuse the application-owned claim-plus-authority read boundary.
- [x] Enforce exact logical model profile before the optional policy hook.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 ./internal/app ./internal/modelgateway ./harness/e2e`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused tests naming the exact granted case, ungranted profile, unknown claim, terminal claim, policy non-bypass, and zero provider-call assertions.
- Passing repository baseline output.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Provider credentials and provider-specific model names stay behind the adapter; the gateway contract uses only logical profiles.
- A caller-supplied claim ID remains correlation data, not authentication; do not claim #121 is complete.

## Decisions and Blockers

- Decision: implement #34 and #35 in one PR because both require the same system-owned claim-plus-authority snapshot and non-bypass ordering; each keeps its own task packet and focused tests.
- Decision: model authority is exact for v0. Routing a logical profile to provider models is a later adapter concern.
- Evidence: `go test -count=1 -v ./internal/app ./internal/toolgateway ./internal/modelgateway ./harness/e2e` and `.\scripts\check.ps1 -All` pass on Windows; the reference E2E records one Tool Allow/Deny pair and one Model Allow/Deny pair with one adapter call each.
- Blockers: none; #29 and #33 are merged.
