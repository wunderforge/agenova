# Task: Append-only fact spine

- Ticket: [#37](https://github.com/wunderforge/agenova/issues/37)
- Mission: Correlate admission, authority resolution, runtime and gateway observations without rewriting earlier evidence.
- Target: `internal/facts`, application/gateway producer hooks.
- User value: Inspect why one assignment was allowed, what it received and what actually happened.
- PRD outcome: [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

- [Approved checkpoint task](../0143-midterm-vertical/task.md), [spec](../0143-midterm-vertical/spec.md), [design](../0143-midterm-vertical/design.md)
- [Journal](../../internal/facts/journal.go), [producer composition](../../internal/console/service.go), [authority provenance](../../internal/authority/resolution.go)

## Scope

Append-only in-memory facts; exact request/claim/invocation and backend attribution; stable sequence ordering; typed decisions and policy/reason references; separate provider attempt/outcome. No durable storage or approval workflow.

## Acceptance Criteria

- Reject rewriting facts and cross-claim/authority/backend/invocation attribution.
- Preserve pre-claim decisions without invented claims and distinguish Allow, Deny and ApprovalRequired.
- Require a recorded Allow before provider attempt; correlate one completion with the same invocation.
- Record actual effective authority and resolver-produced narrowing reasons.

## Negative Case

Foreign attribution, duplicate attempt/outcome and denied provider attempts are rejected.

## Execution Todo

- [x] Owner approved #143 integration; independent planning/release core reviews recorded there. Original owner coordination recorded on #37.
- [x] Implement producer hooks and journal with deterministic focused tests.
- [ ] Complete repository/browser release gates and publish PR evidence.

## Quality Gates

- `go test -count=1 ./internal/facts ./internal/authority ./internal/app ./internal/console ./internal/toolgateway ./internal/modelgateway`
- `.\scripts\check.ps1 -All`

## Evidence Required

Append-only/cross-claim/pre-claim/typed/ordering/attempt-correlation tests; actual same-request kind/model output linked on #143.

## Constraints

Facts are observations, never authority. Missing or failed evidence cannot preserve Running authority.

## Decisions and Blockers

Delivered together with #143; persistence and approval execution remain outside this slice.
