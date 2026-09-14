# Feature Specification: Bind governed authority to Running only

- Ticket: [#32](https://github.com/wunderforge/agenova/issues/32)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and the ordered [MVP user journey](../../docs/product/prd.md#mvp-user-journey).

## Intent

Bind Tool and Model gateway lifecycle eligibility to the authoritative application claim state produced by #31. Backend readiness and resource existence are evidence only; they cannot grant or prolong governed authority.

## In Scope

- One shared application-owned claim-reading contract consumed by both gateways.
- Running-only lifecycle eligibility, fail-closed lookup behavior, and existing parent/child Running checks.
- Focused phase-matrix and terminal-resource negative evidence.

## Out of Scope

- Changing authority contents, implementing capability policy, issuing credentials, network isolation, gateway transport, or resource cleanup.
- Adding lifecycle inference to runtime adapters or trusting a request's claim ID without the existing trusted invocation binding owned by later work.

## Requirements

- Given an authoritative Running claim, when a Tool or Model request reaches lifecycle eligibility, then the gateway may continue to its existing capability checks.
- Given Pending or Bound, when either gateway checks eligibility, then it denies before work authority can be exercised.
- Given Succeeded, Failed, or Expired, when either gateway checks eligibility, then it denies regardless of backend worker existence or readiness.
- Given a missing claim, reader error, or unusable snapshot, when eligibility is checked, then it fails closed.
- Given a child claim, when either the child or its recorded parent is not authoritatively Running, then the gateway denies it.

## Negative Cases

- A still-running backend worker paired with an authoritative terminal claim is denied.
- A ready Bound worker is denied because readiness is not application work start.
- Caller-supplied phase or backend status cannot override the authoritative snapshot.

## Compatibility

- Preserve the gateways' existing request validation, tool/model capability checks, fact recording, and lineage rules.
- `runtime.ClaimReader` remains only a compatibility boundary for legacy consumers; the modified gateways use #31's application-owned reader.
- No public API shape or backend contract changes are introduced by #32.

## Open Decisions

- Confirm the exact #31 reader package/type and error shape after its planning approval. Both gateways must depend on that one boundary rather than adapters.
- Confirm whether malformed snapshots are represented as not-found or an explicit error; either representation must deny and be covered by evidence.
