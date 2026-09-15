# Task: Enforce Tool Gateway capability and resource scope

- Ticket: [#34](https://github.com/wunderforge/agenova/issues/34)
- Mission: Bind every structurally valid Tool Gateway attempt to the active claim's system-issued effective authority before any adapter call.
- Target: `internal/app/`, `internal/gateway/`, `internal/toolgateway/`, and focused reference tests
- User value: A running worker can use only the exact tool operation and resource scope granted to its claim; changing request fields cannot expand authority.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture authority boundary](../../docs/product/architecture-contract.md#authority-and-credentials)
- [Current gateway contract](../../internal/toolgateway/gateway.go)
- [Application-owned claim state](../../internal/app/run_service.go)
- [Team A effective-authority fixture](../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json)

## Scope

In scope:

- Add a defensive application read boundary that returns the claim and its immutable effective authority from the same system-owned snapshot.
- Require the exact `tool.action` and exact concrete resource scope to appear in effective authority.
- Keep the existing optional policy hook as an additional restriction only; it cannot widen effective authority.
- Record stable Deny outcomes for resolved claims and prove every denial makes zero adapter calls.

Out of scope:

- Network transport, workload credentials, provider credential brokerage, broad MCP catalogs, or wildcard matching at invocation time.
- Claim-ID authentication or context-mismatch enforcement, which remains owned by #121.
- Durable facts or production adapters.

## Acceptance Criteria

- The canonical running Team A claim can invoke one exact granted repository operation and reaches the adapter once.
- A forbidden tool action and a wrong repository scope return stable Deny decisions and make zero adapter calls.
- Unknown and terminal claims fail closed and make zero adapter calls.
- A custom Allow policy cannot bypass the effective-authority ceiling; a custom Deny or ApprovalRequired result can further restrict an otherwise granted call.
- The authority returned to callers is a defensive copy and remains correlated with `claim.authorityRef`.

## Negative Case

- Supplying a different repository scope for an otherwise granted tool must return Deny and leave the adapter call count at zero.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation. The Owner explicitly authorized continuous takeover and delivery on 2026-09-15; #34 and #35 are one reviewed implementation batch.
- [x] Add the smallest application-owned claim-plus-authority read boundary.
- [x] Enforce exact tool operation and resource scope before the optional policy hook.
- [x] Add or update focused behavioral evidence.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 ./internal/app ./internal/toolgateway ./harness/e2e`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused tests naming the exact granted case, forbidden tool, wrong repository, unknown claim, terminal claim, policy non-bypass, and zero adapter-call assertions.
- Passing repository baseline output.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Requested access and request fields are never an authority source at invocation time.
- A caller-supplied claim ID remains correlation data, not authentication; do not claim #121 is complete.

## Decisions and Blockers

- Decision: implement #34 and #35 in one PR because both require the same system-owned claim-plus-authority snapshot and non-bypass ordering; each keeps its own task packet and focused tests.
- Decision: effective authority is an unavoidable ceiling. Optional policy hooks run afterward and may only retain or narrow the result.
- Evidence: `go test -count=1 -v ./internal/app ./internal/toolgateway ./internal/modelgateway ./harness/e2e` and `.\scripts\check.ps1 -All` pass on Windows; the reference E2E records one Tool Allow/Deny pair and one Model Allow/Deny pair with one adapter call each.
- Blockers: none; #29 and #33 are merged.
