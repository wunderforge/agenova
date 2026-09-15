# Task: Internal console evidence API

- Ticket: [#68](https://github.com/wunderforge/agenova/issues/68)
- Mission: Expose the shared evidence view by request and claim reference.
- Target: `internal/console/http.go`, `cmd/agenova-console`.
- User value: The console can poll real progress without interpreting backend objects.
- PRD outcome: [Read-only claim console](../../docs/product/prd.md#7-read-only-claim-console)

## Context to Read

- [Approved checkpoint task](../0143-midterm-vertical/task.md), [spec](../0143-midterm-vertical/spec.md), [design](../0143-midterm-vertical/design.md)
- [Handler](../../internal/console/http.go), [shared view](../../internal/evidence/view.go), [server](../../cmd/agenova-console/main.go)

## Scope

Read-only request/claim evidence transport and bounded polling. #143 separately owns the approved internal submission and current-session list/setup exception; these do not turn #68 into an administration API.

## Acceptance Criteria

- Request and claim queries return the same shared view, including denial without invented claim.
- Reads expose actual Running-to-terminal progress and preserve provider-failure versus permission distinction.
- Invalid/unknown references and failures return stable sanitized errors.
- Reference composition binds literal loopback, validates Host/origin and supplies operator identity outside input.

## Negative Case

Caller identity envelopes, cross-origin requests, excessive bodies and malformed references cannot authorize execution or leak host errors.

## Execution Todo

- [x] Planning approved via #143.
- [x] Implement and pass HTTP contract tests.
- [x] Finish repository/browser release gates and publish examples: #144, full baseline and real browser/kind/model gates passed; read-only request examples are in #143 demo.md; merge remains subject to #36 safety fixes.

## Quality Gates

- `go test -count=1 ./internal/console -run '^TestHTTP'`
- `.\scripts\check.ps1 -All`

## Evidence Required

Real handler success/polling/denial/unknown/malformed/redaction cases, plus independently queried actual kind run.

## Constraints

No production SSO, public exposure, policy mutations, historical search or push streams.

## Decisions and Blockers

Configuration is not health; missing gateway/memory connections remain explicit.
