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

- Requests with missing claim identity, reserved secret-bearing keys, or ambiguous resource scope are rejected before adapter invocation, with the enumerated stable categories asserted exactly.
- A caller-supplied identifier is not accepted as the trusted `invocationId`.
- `Deny` and `ApprovalRequired` produce zero provider-adapter calls, proved by adapter spy counts, and `ApprovalRequired` grants no authority.
- A rejection raised before claim identity resolves (structural rejection, unknown claim) returns the issued `invocationId` and appends no invocation fact, proved by fact-store assertions: an unresolved claim must not gain fabricated claim-attributed evidence.
- Once claim identity resolves, every `Allow`, `Deny`, and `ApprovalRequired` decision appends exactly one claim-scoped invocation fact carrying its `invocationId` and typed result.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Confirm this packet with the Owner and Reviewer before implementation lands on the PR.
- [ ] Define the shared invocation request, decision, and `invocationId` contract types per the feature specification.
- [ ] Add request validation with the enumerated rejection categories ahead of any adapter path.
- [ ] Rework the tool and model gateway authorization onto the typed decision result with an adapter-spy seam.
- [ ] Correlate invocation facts with the gateway-assigned `invocationId` under the claim-resolution rule above.
- [ ] Add or update focused behavioral evidence for all three decision paths and the named negative cases.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/toolgateway/... ./internal/modelgateway/... ./internal/gateway/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- G2 focused test output covering valid and invalid request fixtures (claim inputs drawn from the shared v0 fixture set), caller-supplied-ID rejection, and all three decision-result paths.
- Test output proving the adapter spy count is zero for `Deny` and `ApprovalRequired`, that pre-resolution rejections append no fact, and that each post-resolution decision persists exactly one correlated invocation fact.
- Passing repository baseline output; exact commands recorded in the PR.
- Evidence is recorded here only once the corresponding commits exist on this PR and are reviewable; locally prepared code is not repository evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Contracts stay backend-neutral and carry no provider credential material; provider shapes remain inside adapters.
- Gateway tests consume the shared `harness/fixtures/contract/v0/` fixtures for claim inputs: valid cases must pass, invalid cases must fail, and failure categories must match the manifest's `expected.category`. Do not modify the frozen fixture set in this Ticket; new invocation-request fixtures live with the gateway tests, and any design change that invalidates a shared fixture requires updating it in a reviewed change.

## Decisions and Blockers

- Planning depth: Task + Spec because the invocation contract is consumed by #34, #35, #36, and #90; no Design because the in-process contract has one bounded implementation approach.
- Proposed placement: shared contract types in a new `internal/gateway` package consumed by `internal/toolgateway` and `internal/modelgateway`; packet approval covers this placement.
- Decision (planning review, 2026-08-31): the gateway assigns `invocationId` at entry, before structural validation and policy evaluation, so every returned result is correlatable. A rejection raised before claim identity resolves returns that ID without fabricating a claim-attributed fact; once the claim resolves, each `Allow`, `Deny`, and `ApprovalRequired` decision appends exactly one claim-scoped invocation fact. This supersedes the earlier "fact on every decision path" wording.
- Decision (planning review, 2026-08-31): secret rejection on `Parameters` is deterministic — an exact documented reserved-key set matched after one documented normalization rule (lowercase, then remove `-` and `_`). No substring or value heuristics, and the contract does not claim to detect arbitrary secret values; provider credentials still originate only behind adapters.
- Owner/Reviewer approval: pending planning approval on #33.
- Decision: gateway tests draw claim identity, tool capability, resource scope, and model profile from the frozen `issued-state.valid.team-a-engineer` fixture, and the secret-rejection case reuses the key from `claim-request.invalid.secret-value`, via a small `internal/gateway/gatewaytest` loader (contracttest precedent).
- Decision (automated review, 2026-08-30): denied attempts must be inspectable from the fact store rather than only from the returned Decision. The E5 (#37) boundary is respected: no new fact kinds are introduced, the existing invocation fact gains correlation fields only. The exact persistence rule is the claim-resolution rule recorded above.
- Codex automated review on the packet (2026-08-30, three findings): P1 evidence persistence adopted; P1 secret-reachability answered by specifying `Parameters` as the single extensible operation-data surface; P2 category enumeration added to the specification with exact-assertion requirement.
- Decision: `Deny`/`ApprovalRequired` reach callers as typed decisions, never Go errors; the `Invoke` error channel reports adapter failures only.
- Decision: a gateway with no provider adapter configured fails closed. An allowed invocation reports a configuration failure rather than succeeding silently, so recorded evidence never describes a call that was never attempted; the reference scenario wires an explicit adapter for the same reason.
- Decision: an unrecognised policy result is denied with `invalid-policy-outcome` rather than passed through, so the three-value contract cannot be bypassed by a policy that leaves the result unset.
- Residual: `InvocationResult` defines three values, but Go still admits any untyped string constant at the fact-store append boundary. `IsValid` makes the set assertable; enforcing it on append belongs to the evidence contract in #37.
- Decision: the caller-supplied identifier is modeled as untrusted `CallerReference` metadata — the request shape has no invocationId field to smuggle, and tests assert the issued ID never equals the caller value.
- Decision: the e2e multi-agent reference test migrated to the typed `Invoke` contract in the same change (Change-a-Core-Contract playbook: reference implementation and contract tests move together).
- Blockers: #25 (E1-T4 SandboxClaim v0) and #32 (E3-T3 Running-only binding) block implementation. Planning may be approved ahead of them, but product code must reuse the final E1-T4 decision/result types and the E3-T3 Running-only binding rather than provisional duplicates, so implementation commits wait for those Tickets to land.
