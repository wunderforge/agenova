# Feature Specification: Resolve effective authority by intersection

- Ticket: [#28](https://github.com/wunderforge/agenova/issues/28)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), and [MVP Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Intent

After #27 admits an assignment, compute the exact authority the future SandboxClaim may carry. The result is derived only from requested access that survives the AgentTemplate and runtime ceilings. It is not a second admission decision, claim, credential, or backend configuration.

## In Scope

- Requested tools, resource scopes, model profile, memory scopes, runtime profile, and timeout from ClaimRequest.
- Capability ceiling and safe defaults from the resolved AgentTemplate, with defaults prohibited from adding unrequested authority.
- The `Allow` admission result and policy reference from #27.
- One immutable EffectiveAuthority value using the public shape owned by #25.
- Deterministic narrowing and rejection behavior.

## Out of Scope

- Principal/project/template admission, policy administration, claim issuance, credentials, backend allocation, or governed invocation enforcement.
- General resource-policy expressions, recursive globbing, wildcard tools/models, profile substitution, or backend-specific runtime limits.

## Resolution Contract

```text
#27 result must be Allow
        +
ClaimRequest.requestedAccess/runtime
        intersect
AgentTemplate.capabilityCeiling
        ↓
EffectiveAuthority or ResolutionFailure
```

The request remains available for audit comparison. The resolver returns a new value and never mutates its inputs.

## Dimension Rules

| Dimension | MVP rule | Failure rule |
| --- | --- | --- |
| Tools | Exact intersection; retain request order | Non-empty request resolving empty fails |
| Resource scopes | Concrete request; exact match or containment by one terminal-`*` ceiling; emit concrete request | Wildcard request, unsupported ceiling wildcard, or non-empty request resolving empty fails |
| Model profile | Requested scalar must be present in allowed model profiles | Missing match fails; no silent replacement |
| Memory scopes | Exact intersection; retain request order | Non-empty request resolving empty fails |
| Runtime profile | Requested scalar must be present in allowed runtime profiles | Missing match fails; no silent replacement |
| Timeout | `min(requestedTimeout, templateMaxTimeout)` | Missing/non-positive usable ceiling fails |

Omitted access dimensions remain omitted. An empty requested list is valid default-deny for that dimension. AgentTemplate defaults do not populate EffectiveAuthority when the request omitted that access.

## Requirements

- Given any admission result other than `Allow`, when resolution is attempted, then no EffectiveAuthority is returned.
- Given the canonical request and engineer template, when resolution runs, then the resolved dimensions equal the shared Team A issued-state EffectiveAuthority; system-managed authority ID is assigned later during issuance.
- Given a list with allowed and disallowed values, when resolution runs, then only allowed concrete values survive in request order.
- Given a non-empty requested list with no allowed value, when resolution runs, then it fails explicitly rather than returning a misleading empty authority.
- Given `repo:acme/*` in the ceiling and `repo:acme/payments` in the request, when scope containment is evaluated, then the concrete requested scope survives.
- Given an exact scope, when request and ceiling are equal, then it survives.
- Given `*` in a requested resource scope, when resolution runs, then it fails because v0 requests concrete authority rather than wildcard authority.
- Given wildcard syntax anywhere except one terminal `*`, when resolution runs, then it fails closed; wildcard text never appears in EffectiveAuthority unless it was itself a concrete requested scope, which v0 does not permit.
- Given a requested model or runtime profile absent from the corresponding ceiling, when resolution runs, then it fails; the resolver does not choose a default or alternative profile.
- Given an allowed runtime profile and a timeout above the template maximum, when resolution runs, then the effective timeout is capped at the maximum.
- Given an allowed timeout below the maximum, when resolution runs, then the requested timeout survives unchanged.
- Given a template default absent from requested access, when resolution runs, then the default does not create authority.
- Given a successful result, when any source slice or object is later mutated, then the EffectiveAuthority remains unchanged.

## Negative Cases

- Admission result is `Deny` or `ApprovalRequired`.
- Only requested tool, memory scope, or resource scope is outside the ceiling.
- Model profile or runtime profile is not explicitly allowed.
- Runtime ceiling has no matching profile or positive maximum timeout.
- Resource ceiling contains unsupported wildcard syntax.
- Resolver input is missing a required request, template, admission decision, or authority target contract.

## Compatibility

- The canonical Team A request and issued-state fixtures remain the positive behavioral target.
- #27 continues to own principal/project/template admission and its policy decision; #28 does not reevaluate identity eligibility.
- #29 remains the only Ticket that creates a SandboxClaim and copies the resolved authority into a system-managed snapshot.
- #34/#35 consume EffectiveAuthority for governed Tool/Model calls; they do not reinterpret requested access.
- CLI/API/UI may display requested versus effective values but cannot compute a different intersection.
- RuntimeBackend and provider adapters remain unchanged.

## Contract Decisions

1. **Resolved — Policy capability limits:** #27's matching rule gates the principal/project/template, while #28 intersects capabilities against the platform-approved AgentTemplate ceiling and runtime ceiling. Adding per-principal capability limits to PolicyBundle is separate scope.
2. **Resolved — Resource containment:** v0 accepts exact scope equality or one terminal-`*` ceiling and emits only the concrete requested scope.
3. **Resolved — Complete removal:** a partially allowed list is narrowed, while an explicitly non-empty dimension resolving completely empty fails.
4. **Resolved — Defaults:** AgentTemplate defaults do not add unrequested claim authority.

The implementation is complete as a stacked change; #72 and #96 must merge before this Ticket can merge in dependency order.
