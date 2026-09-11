# Feature Specification: Issue one system-managed SandboxClaim

- Ticket: [#29](https://github.com/wunderforge/agenova/issues/29)
- PRD outcome: [Claim lifecycle](../../docs/product/prd.md#2-claim-lifecycle), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), and [MVP Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Intent

After #27 admits an assignment and #28 resolves its effective authority, record what was actually granted as one system-issued claim. The claim is the first durable statement that a specific principal received a specific authority for one worker run. It is not a second authorization decision, an authority recomputation, a backend allocation, or a lifecycle transition.

Issuance is the boundary where system-managed identity appears. Everything before it is either caller intent or a derived in-memory value; everything after it refers to the claim by its issued identity.

## In Scope

- The internal request-bound admission token issued by #27's exact-`Allow` Gate.
- The validated ClaimRequest, the trusted Principal supplied out-of-band, and the evidence-ready Decision.
- The EffectiveAuthority resolved by #28, which deliberately carries no identity yet.
- System-managed `claim.id` and `effectiveAuthority.id` generation.
- One immutable Allow-form `IssuedState` using the public shapes owned by #25.
- Deterministic repeat behavior and explicit refusal behavior.

## Out of Scope

- Admission, authority intersection, backend allocation, worker binding, credentials, persistence, or governed invocation enforcement.
- Any phase beyond `Pending`, any `backendIdentity` value, and any runtime, tool, or model fact.
- A claim store, idempotency records, distributed transactions, or claim search.

## Issuance Contract

```text
#27 Gate-issued admission bound to this request
        +
ClaimRequest + Principal + Decision(Allow)
        +
#28 EffectiveAuthority (no identity yet)
        ↓
IssuedState{ effectiveAuthority(+id), claim(Pending), evidence } or IssuanceFailure
```

Issuance returns a new value and never mutates its inputs. `Deny` and `ApprovalRequired` leave the existing denial path unchanged and produce no claim.

## Field Rules

| Field | MVP rule | Failure rule |
| --- | --- | --- |
| `claim.id` | System-derived from admitted inputs; never caller-suppliable | Missing or caller-supplied identity fails |
| `claim.phase` | Always `Pending` at issuance | Any other requested phase fails |
| `claim.backendIdentity` | Always absent at issuance | A present value fails |
| `claim.requestRef` | Equals the admitted request reference | Mismatch fails |
| `claim.templateRef` | Equals `action.templateRef` | Mismatch fails |
| `claim.authorityRef` | Equals the assigned `effectiveAuthority.id` | Mismatch fails |
| `effectiveAuthority.id` | System-derived at issuance | Missing identity fails |
| `effectiveAuthority` dimensions | Copied unchanged from the #28 resolution | Any recomputation or enlargement fails |
| `evidence.claimId` | Equals `claim.id` | Mismatch fails |
| `evidence.decisionIds` | Contains the emitted decision identity | Missing decision identity fails |
| `evidence` invocation lists | Empty, non-nil at issuance | Prepopulated facts fail |

## Identity Derivation

Repeated issuance is deterministic by derivation, not by storage. `claim.id` and `effectiveAuthority.id` are content-addressed over the admitted request reference, principal, action, policy reference, decision identity, and every resolved authority dimension, using the length-prefixed sha256 construction #27 already uses for `decision:<requestRef>:authorization:<hex>`.

Consequences the consumers rely on:

- Identical admitted inputs always reproduce an identical snapshot, including identity, with no clock, counter, random source, or stored state involved.
- Any change to the principal, action, policy, decision, or resolved authority produces a different claim identity, so two different grants can never collide on one identity.
- Sequencing, renewal, and collision handling across separate runs belong to the later storage Ticket, not to v0.

## Requirements

- Given no Gate-issued admission, or an admission bound to another request, project, or template, when issuance is attempted, then no claim and no authority identity are produced.
- Given a `Deny` or `ApprovalRequired` decision, when issuance is attempted, then it fails explicitly and produces no claim, matching the architecture rule that a denied submission creates no claim.
- Given the canonical Team A admitted request and its resolved authority, when issuance runs, then exactly one claim is produced with `Pending` phase, absent `backendIdentity`, and a validated Allow-form `IssuedState`.
- Given a successful issuance, when the snapshot is inspected, then the principal, action, policy reference, and authority dimensions equal the admitted inputs and the decision that authorized them.
- Given a successful issuance, when `claim.id`, `claim.authorityRef`, and `evidence.claimId` are compared, then all three correlate and none originated from caller input.
- Given the same admitted inputs twice, when issuance runs twice, then both snapshots are byte-identical.
- Given admitted inputs differing in any principal, action, policy, decision, or authority value, when issuance runs, then the claim identity differs.
- Given a resolved authority that is missing, empty of required runtime profile, or carrying a non-positive timeout, when issuance runs, then it fails closed and produces no claim.
- Given a successful result, when any source request, template, policy, or authority object is later mutated, then the issued snapshot remains unchanged.
- Given an issued snapshot serialized and reparsed through `ParseSystemIssuedState`, when it is revalidated, then it round-trips unchanged; the same document rejected through `ParseCallerIssuedState` proves the fields stayed system-managed.

## Negative Cases

- Admission is absent, replayed from a public `Decision`, or bound to a different request, project, or template.
- Decision result is `Deny` or `ApprovalRequired`.
- Resolved authority is absent or internally invalid.
- A caller-shaped payload carries `claim.id`, `claim.phase`, `claim.backendIdentity`, or `effectiveAuthority`.
- A required correlation between request, authority, claim, decision, and evidence cannot be satisfied.

## Compatibility

- The canonical Team A issued-state and Team B denial fixtures remain the behavioral targets; the fixture's `Running` phase and `backendIdentity` describe a later lifecycle state that issuance must not produce.
- #27 continues to own admission and #28 continues to own the authority intersection; issuance reevaluates neither.
- #30 and #31 own backend allocation, `backendIdentity`, and every phase transition after `Pending`; they consume the issued claim rather than reissuing it.
- #34 and #35 bind governed Tool and Model calls to the issued claim identity and must not reinterpret the authority snapshot.
- #41 and #60 display requested versus effective values and the claim identity through the shared evidence contract.
- `ValidateIssuedState`, `ParseSystemIssuedState`, and `ParseCallerIssuedState` remain unchanged; this Ticket satisfies those invariants rather than relaxing them.
- RuntimeBackend and provider adapters remain unchanged.

## Open Decisions

1. **Open — Fixture identity:** confirm that issuance uses the #27 content-addressed convention and that the fixture identity `claim:fix-payment-timeout:1` stays an illustrative shape oracle rather than a literal expected value.
2. **Open — Issuance evidence:** confirm that `evidence.runtimeEvents` is empty at issuance because the fixture's `ClaimRunning` event belongs to the later transition owned by #30/#31.
3. **Proposed — Return shape:** issuance returns the complete Allow-form `IssuedState` rather than a bare `SandboxClaim`, so the existing cross-object invariants validate the result in one place.
4. **Proposed — Determinism model:** identical admitted inputs reproduce an identical snapshot; no idempotency record, sequence counter, or claim store is introduced in v0.
