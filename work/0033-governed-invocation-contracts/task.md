# Task: Define minimal governed invocation contracts

- Ticket: [#33](https://github.com/wunderforge/agenova/issues/33)
- Mission: Replace the prototype error-based gateway authorization with typed governed invocation contracts that carry claim identity and scoped operation data, assign a trusted `invocationId`, and answer with a three-state decision result.
- Target: `internal/toolgateway`, `internal/modelgateway`, a small shared invocation-contract package, and invocation-fact correlation in `internal/facts`.
- User value: Every governed Tool/Model attempt is validated, correlated, and decided before any provider adapter runs, and denials become typed, inspectable decisions instead of opaque errors.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract](../../docs/product/architecture-contract.md)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [Current prototype Tool Gateway](../../internal/toolgateway/gateway.go) and [Model Gateway](../../internal/modelgateway/gateway.go) with their tests
- [Current shared API types](../../api/v1alpha1/types.go)
- [Invocation fact recording](../../internal/facts/store.go)
- [Parent/child lineage checks](../../internal/governance/lineage.go)
- [E4-T2 Tool Gateway enforcement consumer](https://github.com/wunderforge/agenova/issues/34)
- [E4-T3 Model Gateway profile consumer](https://github.com/wunderforge/agenova/issues/35)
- [E4-T4 credential boundary consumer](https://github.com/wunderforge/agenova/issues/36)
- [CAG-T3 approval interruption/resume owner](https://github.com/wunderforge/agenova/issues/90)

## Scope

In scope:

- A typed tool invocation request identifying claim identity, tool, action, and resource scope, and a typed model invocation request identifying claim identity and the approved model profile, both backend-neutral and credential-free.
- Request validation that rejects missing claim identity, secret-bearing fields, and ambiguous resource scope before any adapter invocation.
- A gateway-assigned stable `invocationId` created before policy evaluation; the policy decision, attempted external call, result, and recorded evidence correlate through that ID.
- Rejection of caller-supplied identifiers as the trusted `invocationId`.
- A typed decision result of `Allow`, `Deny`, or `ApprovalRequired`; `Deny` and `ApprovalRequired` never reach the provider adapter, and `ApprovalRequired` does not itself grant authority.
- An adapter seam (spy in tests) that makes the zero-call guarantee observable.

Out of scope:

- A full MCP protocol implementation or provider SDK abstraction catalog (Ticket non-goal).
- Approval storage, interruption, or resume behavior (owned by #90).
- Enforcement of granted capability/resource scope against effective authority (#34) and model-profile grants (#35); this Ticket defines the contract they enforce.
- Credential brokerage inside adapters (#36).
- HTTP/gRPC transport selection; the boundary stays in-process behind a stable interface per the open product decision.

## Acceptance Criteria

- A tool request identifies tool, action, and resource scope; a model request identifies the approved profile.
- The gateway assigns one stable `invocationId` before policy evaluation, and the decision, attempted external call, result, and evidence use that ID for correlation.
- Invocation decisions are exactly `Allow`, `Deny`, or `ApprovalRequired`, not a boolean flag.
- Existing Running-only and child-out-of-parent-scope denial semantics remain observable through the typed decision path.

## Negative Case

- Requests with missing claim identity, secret-bearing fields, or ambiguous resource scope are rejected before adapter invocation, with the enumerated stable categories asserted exactly.
- A caller-supplied identifier is not accepted as the trusted `invocationId`.
- `Deny` and `ApprovalRequired` produce zero provider-adapter calls, proved by adapter spy counts, and `ApprovalRequired` grants no authority.
- Every non-`Allow` decision appends one inspectable invocation fact carrying its `invocationId`, typed result, and claim attribution, proved by fact-store assertions.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Confirm this packet with the Owner and Reviewer before implementation lands on the PR.
- [x] Define the shared invocation request, decision, and `invocationId` contract types per the feature specification.
- [x] Add request validation with the named rejection reasons ahead of any adapter path.
- [x] Rework the tool and model gateway authorization onto the typed decision result with an adapter-spy seam.
- [x] Correlate recorded invocation facts with the gateway-assigned `invocationId`.
- [x] Add or update focused behavioral evidence for all three decision paths and the named negative cases.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/toolgateway/... ./internal/modelgateway/... ./internal/gateway/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- G2 focused test output covering valid and invalid request fixtures (claim inputs drawn from the shared v0 fixture set), caller-supplied-ID rejection, and all three decision-result paths.
- Test output proving the adapter spy count is zero for `Deny` and `ApprovalRequired`, and that each non-`Allow` decision persists one correlated invocation fact.
- Passing repository baseline output; exact commands recorded in the PR.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Contracts stay backend-neutral and carry no provider credential material; provider shapes remain inside adapters.
- Gateway tests consume the shared `harness/fixtures/contract/v0/` fixtures for claim inputs: valid cases must pass, invalid cases must fail, and failure categories must match the manifest's `expected.category`. Do not modify the frozen fixture set in this Ticket; new invocation-request fixtures live with the gateway tests, and any design change that invalidates a shared fixture requires updating it in a reviewed change.

## Decisions and Blockers

- Planning depth: Task + Spec because the invocation contract is consumed by #34, #35, #36, and #90; no Design because the in-process contract has one bounded implementation approach.
- Proposed placement: shared contract types in a new `internal/gateway` package consumed by `internal/toolgateway` and `internal/modelgateway`; packet approval covers this placement.
- Dependency note: #25 (E1-T4 SandboxClaim v0) and #32 (E3-T3 Running-only binding) are still open. Contract definition proceeds against the current `api/v1alpha1` claim types and the existing Running-only prototype semantics; alignment is re-checked when those Tickets land.
- Owner/Reviewer approval: pending. On the Owner's decision, implementation was prepared locally while approval is outstanding; it is pushed to the PR only after approval is recorded in the Ticket, and planning-review changes are folded in before that first implementation push.
- Decision: gateway tests draw claim identity, tool capability, resource scope, and model profile from the frozen `issued-state.valid.team-a-engineer` fixture, and the secret-rejection case reuses the key from `claim-request.invalid.secret-value`, via a small `internal/gateway/gatewaytest` loader (contracttest precedent).
- Decision (revised after automated planning review, 2026-08-30): every decision — `Allow`, `Deny`, and `ApprovalRequired` — appends one invocation fact carrying `invocationId`, typed result, and claim attribution, so denied requests are inspectable from the store, not only from the returned Decision. The E5 (#37) boundary is respected: no new fact kinds are introduced, the existing invocation fact gains correlation fields only.
- Codex automated review on the packet (2026-08-30, three findings): P1 non-Allow evidence persistence adopted as above; P1 secret-reachability answered by specifying `Parameters` as the single extensible operation-data surface with key-based secret detection; P2 category enumeration added to the specification with exact-assertion requirement.
- Decision: `Deny`/`ApprovalRequired` reach callers as typed decisions, never Go errors; the `Invoke` error channel reports adapter failures only.
- Decision: the caller-supplied identifier is modeled as untrusted `CallerReference` metadata — the request shape has no invocationId field to smuggle, and tests assert the issued ID never equals the caller value.
- Decision: the e2e multi-agent reference test migrated to the typed `Invoke` contract in the same change (Change-a-Core-Contract playbook: reference implementation and contract tests move together).
- Blockers: none.
