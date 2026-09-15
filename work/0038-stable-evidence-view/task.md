# Task: Shared evidence view

- Ticket: [#38](https://github.com/wunderforge/agenova/issues/38)
- Mission: Assemble one canonical, transport-independent evidence representation.
- Target: `internal/evidence`, `internal/console` query assembly and generated UI bindings.
- User value: CLI/API/UI inspect the same decisions, lifecycle, result and identities.
- PRD outcome: [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

- [Approved checkpoint task](../0143-midterm-vertical/task.md), [spec](../0143-midterm-vertical/spec.md), [design](../0143-midterm-vertical/design.md)
- [View](../../internal/evidence/view.go), [canonical state](../../api/v1alpha1/sandbox_claim.go), [query assembly](../../internal/console/service.go)

## Scope

Canonical request/state plus ordered immutable facts and optional outcome/provider metadata. No additional decision engine, durable history or approval workflow.

## Acceptance Criteria

- Preserve trusted principal, request, effective authority, typed decision/reasons, lifecycle, stable invocation IDs, result and backend binding.
- Missing optional evidence stays absent, including pre-claim denials and unimplemented approvals.
- JSON serialization is deterministic; query snapshots are defensive.

## Negative Case

Denial/ApprovalRequired cannot fabricate a claim, successful provider outcome or resumed execution.

## Execution Todo

- [x] Scope and planning approved through #143; core independent review completed.
- [x] Implement shared representation, defensive queries and golden projection cases.
- [ ] Finish repository/browser release gates and record PR evidence.

## Quality Gates

- `go test -count=1 ./internal/evidence ./internal/console`
- `.\scripts\check.ps1 -All`

## Evidence Required

Success, denial, terminal failure and typed approval-not-granted JSON cases, plus real result/correlation output from #143.

## Constraints

Retain canonical governance types; do not create a UI- or CLI-owned permission model.

## Decisions and Blockers

Detailed observations live above unchanged canonical IssuedState fixture shapes.
