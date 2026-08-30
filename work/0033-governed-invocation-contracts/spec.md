# Feature Specification: Define minimal governed invocation contracts

- Ticket: [#33](https://github.com/wunderforge/agenova/issues/33)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

Tool and Model gateways accept only typed, backend-neutral invocation requests that carry claim identity and scoped operation data, and answer with a typed decision correlated by a gateway-assigned `invocationId`. E4-T2 (#34), E4-T3 (#35), E4-T4 (#36), and the approval path (#90) build on this contract, so its shape needs shared agreement before enforcement work depends on it.

## In Scope

- Tool invocation request: claim identity plus tool name, action, and resource scope.
- Model invocation request: claim identity plus the approved model profile.
- Operation data travels in one typed string-map `Parameters` field — the only extensible request surface. Credential material cannot enter through typed fields (they do not exist) and is rejected by key inspection on `Parameters` before any adapter invocation, so the secret-bearing negative case is reachable without weakening the typed boundary.
- Pre-adapter request validation rejecting missing claim identity, secret-bearing fields, and ambiguous resource scope, each with an enumerated stable category (below).
- One stable, gateway-assigned `invocationId` per governed attempt, created before policy evaluation and carried by the decision, the attempted external call, the result, and recorded evidence.
- Typed decision result: `Allow`, `Deny`, `ApprovalRequired`.
- Persisted evidence for every attempt: each decision — including `Deny` and `ApprovalRequired` — appends one invocation fact carrying the `invocationId`, the typed result, and the claim attribution, so denied requests stay inspectable.

## Stable Rejection Categories

Focused tests assert these identifiers exactly; downstream consumers must not invent alternates.

| Category | Condition |
| --- | --- |
| `missing-claim-identity` | request carries no claim identity |
| `incomplete-operation` | tool request lacks tool or action; model request lacks the approved profile |
| `ambiguous-resource-scope` | resource scope is empty or contains a wildcard |
| `secret-value` | a `Parameters` key names credential material (same category as the frozen fixture set) |
| `unknown-claim` | claim is not known to the runtime backend |
| `claim-not-active` | claim exists but is not in the Running phase |
| `out-of-parent-scope` | child claim whose parent is no longer Running |

## Out of Scope

- MCP protocol implementation or a provider SDK abstraction catalog.
- Approval storage, interruption, or resume behavior (#90).
- Enforcement of granted capability/resource scope (#34) and model-profile grants (#35); credential brokerage (#36).
- HTTP/gRPC transport selection; the contract is an in-process boundary behind a stable interface.

## Requirements

- Given a tool request, when it reaches the Tool Gateway, then it identifies tool, action, and resource scope, or it is rejected before any adapter invocation.
- Given a model request, when it reaches the Model Gateway, then it identifies the approved model profile, or it is rejected before any adapter invocation.
- Given a request missing claim identity, carrying a secret-bearing field, or naming an ambiguous resource scope, when it is validated, then it is rejected before any adapter invocation with a stable rejection reason.
- Given an accepted request, when the gateway begins processing, then it assigns one stable `invocationId` before policy evaluation, and the policy decision, attempted external call, result, and evidence all carry that ID.
- Given a caller-supplied identifier, when the gateway assigns the `invocationId`, then the caller value is not adopted as the trusted correlation identity.
- Given policy evaluation, when a decision is produced, then it is exactly one of `Allow`, `Deny`, or `ApprovalRequired`, never a boolean flag.
- Given a `Deny` or `ApprovalRequired` decision, when the decision is returned, then no provider adapter call occurs, and `ApprovalRequired` does not itself grant authority.
- Given any non-`Allow` decision, when it is returned, then one invocation fact carrying that `invocationId`, the typed result, and the claim attribution has been appended, with zero adapter calls.

## Negative Cases

- Missing claim identity is rejected before adapter invocation.
- A secret-bearing request field is rejected before adapter invocation.
- An ambiguous resource scope is rejected before adapter invocation.
- A caller-supplied identifier is not adopted as the trusted `invocationId`.
- `Deny` and `ApprovalRequired` produce zero adapter calls, proved by adapter spy counts.

## Compatibility

- `api/v1alpha1` claim types (`SandboxClaim`, `ClaimPhase`) remain unchanged.
- The prototype gateways' Running-only and child-out-of-parent-scope denial semantics are preserved through the typed decision path; their authoritative binding is owned by #32.
- `internal/facts` invocation records remain attributable to the correct claim and gain `invocationId` correlation without dropping existing fields.
- The typed decision aligns with the E1-T1 fixture convention of typed authorization results rather than boolean flags; gateway tests consume the shared v0 fixtures for claim inputs and keep failure categories consistent with the fixture manifest.

## Open Decisions

- None. The `internal/gateway` package placement proposed in the task packet is accepted or corrected as part of packet approval.
