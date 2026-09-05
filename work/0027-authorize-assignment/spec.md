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
- One pre-side-effect admission stage that returns its decision to #28 in the same assignment-resolution pipeline.
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

- Given the canonical Team A principal, payments project, engineer template, `claim.create`, and a matching active rule, when the request is evaluated, then the result is `Allow` and #28 authority resolution is entered exactly once.
- Given the canonical Team B principal with the same request context, when no principal-scoped rule matches, then the result is `Deny` and #28 authority resolution is not entered.
- Given no active PolicyBundle, when authorization is attempted, then the request is denied before authority resolution and the reason identifies unavailable policy.
- Given an unknown or missing principal/action/project/template, when authorization is attempted, then it cannot produce `Allow` and no side effect occurs.
- Given an exact rule mismatch in any required dimension, when authorization is evaluated, then default-deny applies; partial matching cannot grant authority.
- Given a completed evaluation, then its decision uses the shared typed result vocabulary and carries the active policy ID/version when one was evaluated plus a non-empty reason suitable for evidence.
- Given `Deny` or `ApprovalRequired`, then the submission path must not treat it as granted authority or enter #28.
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

The production entry point may remain one assignment-resolution operation that calls #27 and then #28 internally. Do not create a callback framework, network boundary, or persisted intermediate state merely to mirror the Ticket split. Focused tests may use simple spies to prove that non-`Allow` results cannot reach later claim/backend side effects.

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

## Contract Decisions

1. **Resolved — Principal match in PolicyBundle:** merged #25 exposes canonical trusted `Principal.Team`; #26 PR #72 implements exact `policy.Match.Team` matching without wildcards or a generic selector engine. #27 consumes that value only from the trusted out-of-band Principal.
2. **Resolved — Requested project source:** add optional `spec.projectRef: payments` to ClaimRequest and require it at assignment admission through `Action.Project`. It is caller-requested, validated authorization context, never authority by itself, and is not derived from `task.input.repository`.

The implementation consumes #72 directly while the PRs are stacked; #72 must merge before this Ticket can merge cleanly to `main`.
