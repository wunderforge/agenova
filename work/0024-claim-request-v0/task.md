# Task: Define and validate ClaimRequest v0

- Ticket: [#24](https://github.com/wunderforge/agenova/issues/24)
- Mission: Define the smallest backend-neutral `ClaimRequest v0` Go contract and validator so one task and its requested access can be expressed identically in human-authored YAML and API JSON, and unsafe requests are rejected before resolution.
- Target: `api/v1alpha1/` ClaimRequest types, YAML/JSON parsing and validation, focused fixture-driven tests, and this task packet.
- User value: Contributors and later resolution/CLI work (#26–#29, #41) consume one declarative request contract with proven YAML-to-JSON equivalence, instead of inventing private request shapes.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract](../../docs/product/architecture-contract.md) (Product Center, Backend Neutrality, Authority and Credentials)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [E1-T1 fixture specification](../0022-core-contract-fixtures/spec.md)
- [Shared contract fixture manifest](../../harness/fixtures/contract/v0/manifest.json) and its `inputs/claim-request/` files
- [E1-T2 AgentTemplate v0 packet](https://github.com/wunderforge/agenova/pull/93) (approved sibling contract; typed-error and path-classification decisions reused here)
- [Current shared API types](../../api/v1alpha1/types.go)

## Scope

In scope:

- Backend-neutral `ClaimRequest v0` types for object identity, template reference, task definition, requested access, and runtime requirements.
- Parsing for both authored surfaces frozen by the fixtures: canonical human-authored YAML and equivalent API JSON, with proven semantic equivalence between the pair.
- Semantic validation returning typed data with stable `category` and `field/path` for missing required inputs, self-asserted principal data, embedded secret values, and unknown fields.
- Focused Go tests that select and load all seven shared ClaimRequest fixture cases directly from the v0 manifest without copying their contents.

Out of scope:

- Request persistence, submission APIs, CLI commands (#41), authorization resolution, effective-authority intersection (#28), policy evaluation (#26–#27), or claim issuance (#29).
- Trusted principal modeling: the request never carries an authoritative principal; the trusted principal arrives out-of-band (E6-T3).
- Provider-specific fields; the contract stays backend-neutral.
- Secret references or credential delivery design beyond rejecting the reserved secret-bearing path in v0.

## Acceptance Criteria

- The shared canonical YAML (`claim-request.valid.team-a-engineer-yaml`) and API JSON (`-json`) both parse to the public v0 Go type, validate successfully, and are proven semantically equivalent.
- The parsed request preserves template reference, task type and input, requested tools, resource scopes, model profile, memory scopes, and runtime profile/timeout.
- Task input remains distinct from resource scopes: task data lives under `spec.task.input`; access intent lives under `spec.requestedAccess`, and neither is derived from the other.
- Missing template, task, or runtime data is rejected with category `required-field` and the responsible field/path.
- The exact caller-authored path `spec.principal` is rejected with category `self-asserted-principal`; the exact secret-bearing path `spec.secrets` is rejected with category `secret-value`; other unknown fields fail closed with `unknown-field`. Classification is deterministic and path-based, never heuristic.
- Validation returns typed data containing at least `category` and `field/path`; tests assert those fields directly and never infer categories from error-string matching.
- Mismatched apiVersion/kind, invalid documents, and multiple YAML documents fail closed with typed data.
- Focused tests discover all seven ClaimRequest cases through the shared manifest and assert the manifest's expected outcome/category; no second fixture set is created.
- Requested access remains intent: nothing in this contract grants authority.

## Negative Case

- Missing `spec.templateRef`, `spec.task`, or `spec.runtime` (`required-field`); caller-supplied `spec.principal` (`self-asserted-principal`); embedded `spec.secrets` values (`secret-value`); unknown fields, wrong apiVersion/kind, invalid or multi-document input (fail closed) — each asserted by exact typed category and field/path.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Confirm this packet with the Owner and Reviewer before implementation lands on the PR.
- [x] Define the public ClaimRequest v0 types and both parsing surfaces per the feature specification.
- [x] Implement path-based semantic validation with typed category/field data.
- [x] Add fixture-driven tests covering all seven shared cases plus the YAML/JSON equivalence proof and focused boundary units.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./api/v1alpha1 -run ClaimRequest`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused test output naming all seven shared fixture case IDs, the equivalence proof, and the typed category assertions.
- Passing repository baseline output; exact commands recorded in the PR.

## Constraints

- Preserve `docs/product/architecture-contract.md`; requested access is intent and cannot grant authority.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Consume the frozen fixture set directly; do not modify it or copy its contents into test literals.
- Follow the approved E1-T2 contract conventions: typed validation data (`category` + `field/path`), exact-path reserved-field classification, and fail-closed parsing.

## Decisions and Blockers

- Planning depth: Task + Spec because ClaimRequest is a public contract consumed by resolution (#26–#29), CLI submission (#41), and the E1-T2/E1-T4 sibling contracts.
- Decision: reuse the typed-error and path-classification conventions the project lead fixed in the E1-T2 planning review (2026-08-28), so the two human-authored contracts stay symmetric.
- Dependency note: #22 (E1-T1 fixtures) is merged — all seven ClaimRequest cases and both valid surfaces are frozen and consumable now; no upstream blocker.
- Decision: both surfaces share one strict parsing pipeline (JSON is a YAML subset; the JSON entrypoint additionally requires `json.Valid` input), so shape classification and semantic validation cannot drift between forms and the equivalence proof compares decoded values directly.
- Coordination: the shared validation primitives (`ValidationCategory`, `ValidationError`, `Duration`, mapping/scalar shape helpers) are kept byte-compatible with the unmerged E1-T2 branch in a separate `validation.go`; whichever contract merges second folds the two copies into one source and re-runs both fixture suites.
- Owner/Reviewer approval: pending. On the Owner's decision, implementation was prepared locally while approval is outstanding; it is pushed to a PR only after approval is recorded in the Ticket, with planning-review changes folded in first.
- Blockers: none.
