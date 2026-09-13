# Feature Specification: De-scope parent/child multi-agent lineage from the committed MVP

- Ticket: [#109](https://github.com/wunderforge/agenova/issues/109)
- PRD outcome: [MVP Deliverables](../../docs/product/prd.md#mvp-deliverables) and [Out of Scope](../../docs/product/prd.md#out-of-scope-unless-re-prioritized)

## Intent

The committed MVP proves one governed agent assignment from request through evidence on a reference and real runtime backend. Parent/child authority inheritance and lineage remain possible future governance extensions, but do not participate in current acceptance, dependencies, UI, or demo delivery.

## In Scope

- One engineer ClaimRequest and one system-managed SandboxClaim per demonstrated run.
- Allow, narrowing, pre-claim denial, governed-invocation denial, terminal revocation, fact attribution, backend evidence, and a single-claim console view.
- Isolation between independent claims so one claim never absorbs another claim's facts.

## Out of Scope

- Parent/child authority inheritance, child lifecycle coupling, lineage queries, reviewer child assignments, and multi-agent orchestration.
- Removal of already-implemented experimental reference behavior.

## Requirements

- Given the committed MVP documentation and issue graph, when a contributor follows the critical path, then no parent/child ticket is required to complete the governed single-claim vertical slice.
- Given two independent claims, when facts are recorded or queried, then each fact remains attributable to exactly one claim without requiring a lineage model.
- Given a governed invocation, when its caller-supplied target differs from the claim bound to the system-established worker context, then the invocation is denied without an external call or fabricated invocation fact.
- Given future parent/child work, when it is revisited, then it requires explicit reprioritization and must remain governance scope rather than workflow scheduling.

## Negative Cases

- A retained MVP ticket must not depend on #39 or #54 or require `lineage`/`reviewer child` in its acceptance criteria.
- Existing experimental parent/child code must not be presented as a committed or production-ready MVP capability.

## Compatibility

- Existing single-claim public contracts, lifecycle behavior, gateway authorization, fact attribution, and experimental parent/child reference tests remain source-compatible.
- The trusted-context rule states a required security property, not a production workload-identity implementation or new public identity schema.
- Optional future lineage fields may be added later only through an approved contract change; current consumers cannot require or fabricate them.

## Open Decisions

- None. The Owner approved single-claim MVP scope on 2026-09-08; the final review and merge follow the recorded one-day Owner fallback rule.
