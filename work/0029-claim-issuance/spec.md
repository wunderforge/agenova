# Feature Specification: Issue one system-managed SandboxClaim

- Ticket: [#29](https://github.com/wunderforge/agenova/issues/29)
- PRD outcome: [Claim lifecycle](../../docs/product/prd.md#2-claim-lifecycle), [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), and [MVP Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Intent

After #27 admits an assignment and #28 resolves its effective authority, record what was actually granted as one system-issued claim. The claim is the first durable statement that a specific principal received a specific authority for one worker run. It is not a second authorization decision, an authority recomputation, a backend allocation, or a lifecycle transition.

Issuance is the boundary where system-managed identity appears. Everything before it is either caller intent or a derived in-memory value; everything after it refers to the claim by its issued identity.

## In Scope

- The internal request-bound admission token issued by #27's exact-`Allow` Gate, with a narrow read-only exact-context matcher.
- The validated full ClaimRequest, the trusted Principal supplied out-of-band, and the evidence-ready Decision, both checked against that admission.
- An internal #28 resolution bound to the complete ClaimRequest, carrying an EffectiveAuthority with no identity yet. The public EffectiveAuthority is data, not issuance proof.
- System-managed `claim.id` and `effectiveAuthority.id` generation.
- One immutable Allow-form `IssuedState` using the public shapes owned by #25.
- Deterministic repeat behavior and explicit refusal behavior.

## Out of Scope

- Admission, authority intersection, backend allocation, worker binding, credentials, persistence, or governed invocation enforcement.
- Any phase beyond `Pending`, any `backendIdentity` value, and any runtime, tool, or model fact.
- A claim store, idempotency records, distributed transactions, or claim search.

## Issuance Contract

```text
#27 Gate-issued admission bound to the full trusted authorization context
        +
validated ClaimRequest + matching Principal + matching Decision(Allow)
        +
#28 request-bound Resolution containing EffectiveAuthority (no identity yet)
        ↓
IssuedState{ effectiveAuthority(+id), claim(Pending), evidence } or IssuanceFailure
```

Issuance returns a new value and never mutates its inputs. Before constructing any issued state, it validates the ClaimRequest, reconstructs the authorization `Request` from its request/project/template references and the separately supplied trusted Principal, then requires exact equality with the Gate admission's bound authorization `Request` and Decision. A read-only `Admission.MatchesContext(Request, Decision)` provides that check without exposing mutable admission state or reevaluating policy. The internal resolution additionally checks the canonical full request, including task and requested access. `Deny` and `ApprovalRequired` leave the existing denial path unchanged and produce no claim.

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

Repeated issuance is deterministic by derivation, not by storage. The identity input contains the canonical JSON encoding of the **entire validated ClaimRequest**, including `spec.task.type` and recursively structured `spec.task.input`, plus the exact admitted Principal, Action, full Decision (including policy reference), and every #28-resolved authority dimension before either issued ID is assigned. `encoding/json` on the validated Go request supplies deterministic struct order and sorted map keys. Each input component is length-prefixed before sha256, as in #27's decision identity construction. The resulting digest is formatted separately as `claim:<requestRef>:issuance:<hex>` and `authority:<requestRef>:issuance:<hex>`; the fixture's `:1` IDs are illustrative shape values, not literal outputs.

Consequences the consumers rely on:

- Identical admitted inputs always reproduce an identical snapshot, including identity, with no clock, counter, random source, or stored state involved.
- A changed validated request, including a changed task with the same `metadata.name`, changes the derived identity. A changed admitted context, decision, or resolved authority also changes it; a separately supplied principal or decision that does not match the admission is rejected rather than issued.
- Sequencing, renewal, and collision handling across separate runs belong to the later storage Ticket, not to v0.

## Requirements

- Given no Gate-issued admission, or an admission bound to another request, project, template, principal subject/team/authentication context, or decision, when issuance is attempted, then no claim and no authority identity are produced.
- Given a `Deny` or `ApprovalRequired` decision, when issuance is attempted, then it fails explicitly and produces no claim, matching the architecture rule that a denied submission creates no claim.
- Given the canonical Team A admitted request and its resolved authority, when issuance runs, then exactly one claim is produced with `Pending` phase, absent `backendIdentity`, and a validated Allow-form `IssuedState`.
- Given a successful issuance, when the snapshot is inspected, then the principal, action, policy reference, and authority dimensions equal the admitted inputs and the decision that authorized them.
- Given a successful issuance, when `claim.id`, `claim.authorityRef`, and `evidence.claimId` are compared, then all three correlate and none originated from caller input.
- Given the same admitted inputs twice, when issuance runs twice, then both snapshots are byte-identical.
- Given two otherwise identical validated requests with the same reference but different task objectives or base branches, when each is admitted and issued, then their claim identities differ. Given an admitted context or resolved authority value that differs, identity changes or issuance rejects the mismatch.
- Given a missing, zero, or other-request resolution, issuance fails closed with no claim. Invalid authority inputs are rejected by #28 before a resolution token exists; issuance still validates the internal snapshot before issuing.
- Given a successful result, when any source request, template, policy, or authority object is later mutated, then the issued snapshot remains unchanged.
- Given an issued snapshot serialized and reparsed through `ParseSystemIssuedState`, when it is revalidated, then it round-trips unchanged; the same document rejected through `ParseCallerIssuedState` proves the fields stayed system-managed.

## Negative Cases

- Admission is absent, replayed from a public `Decision`, or bound to a different request, project, template, principal subject, team, authentication context, or decision ID/content.
- Decision result is `Deny` or `ApprovalRequired`.
- Request-bound resolution is absent, zero, or belongs to a different full request.
- A caller-shaped payload carries `claim.id`, `claim.phase`, `claim.backendIdentity`, or `effectiveAuthority`.
- A required correlation between request, authority, claim, decision, and evidence cannot be satisfied.

## Compatibility

- The canonical Team A issued-state and Team B denial fixtures remain the behavioral targets; the fixture's `Running` phase and `backendIdentity` describe a later lifecycle state that issuance must not produce.
- #27 continues to own admission and #28 continues to own the authority intersection; issuance reevaluates neither.
- #30 and #31 own backend allocation, `backendIdentity`, and every phase transition after `Pending`; they consume the issued claim rather than reissuing it.
- #34 and #35 bind governed Tool and Model calls to the issued claim identity and must not reinterpret the authority snapshot.
- #41 and #60 display requested versus effective values and the claim identity through the shared evidence contract.
- `ValidateIssuedState`, `ParseSystemIssuedState`, and `ParseCallerIssuedState` remain unchanged; this Ticket satisfies those invariants rather than relaxing them. #27's existing `Admission.Matches` remains available to #28; #29 only adds the narrow exact-context matcher.
- The claim remains backend-neutral. The Agent Sandbox adapter preserves existing safe resource names, reserves a `hash-` namespace for mapped names, and maps unsafe public claim IDs to DNS-label Kubernetes names without changing the issued ID in Agenova facts.

## Decisions and Planning Gate

1. **Owner-approved direction — Fixture identity:** use #27's content-addressed convention; fixture `:1` IDs are illustrative. Focused tests assert generated IDs and all references directly.
2. **Owner-approved direction — Issuance evidence:** issue `Pending` with no `backendIdentity`; runtime, tool, and model lists are empty and non-nil. #30/#31 own later lifecycle events.
3. **Owner-approved direction — Return shape:** return the complete validated Allow-form `IssuedState`, not a bare claim.
4. **Owner-approved direction — Determinism:** identical admitted inputs reproduce an identical snapshot without a store or counter; the full validated request participates in identity derivation.
5. **Reviewer findings addressed:** require exact admission-context binding for separately supplied Principal and Decision, and distinguish same-reference/different-task requests. The Owner directed implementation and planning review together on the complete PR; merge still requires code and evidence review.
6. **Owner-approved Codex review corrections:** require a private request-bound #28 resolution token, and map colon-bearing issued claim IDs to deterministic Kubernetes-safe resource names in the Agent Sandbox adapter.
7. **Follow-up hardening:** reserve mapped resource names so they cannot alias preserved safe names; reject invalid UTF-8 in the full request before computing the authority binding digest without adding a new nesting limit to the core contract.
