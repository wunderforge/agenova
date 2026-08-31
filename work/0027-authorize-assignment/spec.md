# Feature Specification: Authorize assignment creation before side effects

- Ticket: [#27](https://github.com/wunderforge/agenova/issues/27)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [MVP Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Intent

Establish one deterministic admission boundary for assignment creation. Agenova receives a trusted principal separately from the ClaimRequest, evaluates that principal's requested action/project/template against the active PolicyBundle, records the decision, and permits claim/runtime side effects only after `Allow`.

This specification elaborates authorization order and observable behavior. It does not define authentication, claim issuance, runtime allocation, or a general policy language.

## In Scope

- One validated trusted `Principal` supplied outside ClaimRequest.
- One assignment action containing `claim.create`, requested project, and template reference.
- One active immutable PolicyBundle and exact-match, default-deny evaluation.
- One typed `Decision` with principal, action, result, policy reference, and reason.
- One pre-side-effect gate whose authorized continuation may later compose claim issuance and backend allocation.
- Evidence-ready allow and deny decisions using the shared issued-state fixture vocabulary.

## Out of Scope

- Principal authentication, OIDC/SSO, caller-managed identity fields, or device posture.
- Policy administration, reload transport, rule inheritance, wildcard matching, exceptions, approvals, or natural-language rules.
- Effective-authority intersection, claim construction, runtime allocation, or append-only evidence storage owned by #37.
- Backend-specific types or Kubernetes authorization.

## Contract Inputs and Ownership

| Input | Meaning | Source of truth |
| --- | --- | --- |
| `Principal` | Upstream-authenticated subject and trusted Team A/Team B context | #25; supplied outside ClaimRequest |
| `Action` | `claim.create` plus requested project and template | #25 shape; project source alignment with #24 |
| `PolicyBundle` | Active immutable exact-match rules and ID/version | #26 |
| `requestRef` | Correlation for pre-claim evidence | Validated ClaimRequest from #24 |

The authorizer consumes these contracts. It must not redefine them in a private package or derive trusted principal/project data from arbitrary task input.

## Requirements

- Given the canonical Team A principal, payments project, engineer template, `claim.create`, and a matching active rule, when the request is evaluated, then the result is `Allow` and the authorized continuation is invoked exactly once.
- Given the canonical Team B principal with the same request context, when no principal-scoped rule matches, then the result is `Deny` and the authorized continuation is not invoked.
- Given no active PolicyBundle, when authorization is attempted, then the request is denied before the authorized continuation and the reason identifies unavailable policy.
- Given an unknown or missing principal/action/project/template, when authorization is attempted, then it cannot produce `Allow` and no side effect occurs.
- Given an exact rule mismatch in any required dimension, when authorization is evaluated, then default-deny applies; partial matching cannot grant authority.
- Given a completed evaluation, then its decision uses the shared typed result vocabulary and carries the active policy ID/version when one was evaluated plus a non-empty reason suitable for evidence.
- Given `Deny` or `ApprovalRequired`, then the submission path must not treat it as granted authority or invoke the authorized continuation.
- Given pre-claim denial, then evidence is correlated by request and decision without fabricating claim or backend identity.

## Ordering Boundary

```text
trusted Principal + validated ClaimRequest
  -> derive/validate Action context
  -> evaluate active PolicyBundle
  -> return Decision (later recorded by #37)
  -> if result != Allow: stop
  -> authorized continuation
       -> effective-authority resolution (#28)
       -> claim issuance (#29)
       -> runtime allocation (E3)
```

The implementation may use a narrow callback or interface for the authorized continuation so tests can prove ordering without implementing #28, #29, or E3 inside this Ticket. The seam must not expose backend types or become a second claim-issuance interface.

## Negative Cases

| Case | Expected result | Claim spy | Backend spy |
| --- | --- | ---: | ---: |
| Team B, otherwise identical request | `Deny` | 0 | 0 |
| No active PolicyBundle | not `Allow` | 0 | 0 |
| Unknown principal/team | `Deny` | 0 | 0 |
| Unknown action | `Deny` | 0 | 0 |
| Unknown project | `Deny` | 0 | 0 |
| Unknown template | `Deny` | 0 | 0 |
| `ApprovalRequired` from any future evaluator | not treated as allow | 0 | 0 |

## Compatibility

- The shared Team A engineer and Team B pre-claim-denial issued-state fixtures remain the behavioral target.
- `ClaimRequest` continues to exclude an authoritative principal and all system-issued state.
- #28 remains the only Ticket that computes effective authority, and #29 remains the only Ticket that issues a SandboxClaim.
- #37 remains the owner of append-only fact recording; #27 returns the decision it will consume.
- RuntimeBackend and provider adapters remain unchanged.
- The later upstream-identity contract may enrich trusted Principal metadata without changing the rule that identity arrives outside ClaimRequest.

## Open Decisions

1. **Principal match in PolicyBundle:** after #25 freezes Principal, #26 needs one exact principal-scoped rule dimension. Recommended v0: exact match on the canonical trusted `team` value. Do not add wildcards or a generic selector engine in #27.
2. **Requested project source:** the current ClaimRequest fixture does not carry the `payments` project used by Action/Policy. Recommended v0: add `spec.projectRef: payments` to ClaimRequest and treat it like `templateRef`—caller-requested, validated, and authorized, but never authority by itself. Do not derive it from `task.input.repository`.

Implementation cannot begin safely until both decisions are accepted and reflected in the producer contracts.
