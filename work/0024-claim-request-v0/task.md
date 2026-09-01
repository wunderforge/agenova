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
- The nested v0 invariants hold: `metadata.name`, `spec.task.type`, and `spec.runtime.profileRef` are required and non-blank, and `spec.runtime.timeout` must be a positive Go duration. Focused unit cases cover each of these nested invariants.
- `spec.task.input` accepts JSON-compatible structured task data — string-keyed mappings whose values may be strings, finite numbers, booleans, nulls, arrays, or nested string-keyed mappings — and rejects YAML constructs that cannot be represented consistently in JSON (non-string or duplicate keys, aliases and merge keys, non-JSON tags, non-finite floats) with typed data at the exact path. `ValidateClaimRequest` enforces the same invariants on decoded values, so directly constructed requests cannot bypass the parser. One focused case proves a structured input round-trips between the YAML and JSON surfaces with semantic equality.
- `spec.task.input` may be absent or empty because its shape is task-specific, and `spec.requestedAccess` may be absent or explicitly empty, meaning default-deny; neither is treated as a required field.
- The exact caller-authored path `spec.principal` is rejected with category `self-asserted-principal`; the exact secret-bearing path `spec.secrets` is rejected with category `secret-value`; other unknown fields fail closed with `unknown-field`. Classification is deterministic and path-based, never heuristic.
- Validation returns typed data containing at least `category` and `field/path`; tests assert those fields directly and never infer categories from error-string matching.
- Mismatched apiVersion/kind, invalid documents, and multiple YAML documents fail closed with typed data.
- Focused tests discover all seven ClaimRequest cases through the shared manifest and assert the manifest's expected outcome/category; no second fixture set is created.
- Requested access remains intent: nothing in this contract grants authority.

## Negative Case

- Missing `spec.templateRef`, `spec.task`, or `spec.runtime` (`required-field`); caller-supplied `spec.principal` (`self-asserted-principal`); embedded `spec.secrets` values (`secret-value`); unknown fields, wrong apiVersion/kind, invalid or multi-document input (fail closed) — each asserted by exact typed category and field/path.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation lands on the PR.
- [x] Sync with merged #93 and reuse its shared validation primitives (merge instead of rebase: the branch was already pushed and force-push is not allowed).
- [x] Define the public ClaimRequest v0 types and both parsing surfaces per the feature specification.
- [x] Implement path-based semantic validation with typed category/field data, including the nested required-field invariants.
- [x] Add fixture-driven tests covering all seven shared cases, the YAML/JSON equivalence proof, and focused units for the nested invariants, the structured task-input round-trip, and empty task input / default-deny requested access.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./api/v1alpha1 -run ClaimRequest`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused test output naming all seven shared fixture case IDs, the equivalence proof, and the typed category assertions.
- Passing repository baseline output; exact commands recorded in the PR.
- Evidence is recorded here only once the corresponding commits exist on this PR and are reviewable; locally prepared code is not repository evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`; requested access is intent and cannot grant authority.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Consume the frozen fixture set directly; do not modify it or copy its contents into test literals.
- Follow the approved E1-T2 contract conventions: typed validation data (`category` + `field/path`), exact-path reserved-field classification, and fail-closed parsing.

## Decisions and Blockers

- Planning depth: Task + Spec because ClaimRequest is a public contract consumed by resolution (#26–#29), CLI submission (#41), and the E1-T2/E1-T4 sibling contracts.
- Decision: reuse the typed-error and path-classification conventions the project lead fixed in the E1-T2 planning review (2026-08-28), so the two human-authored contracts stay symmetric.
- Dependency note: #22 (E1-T1 fixtures) is merged — all seven ClaimRequest cases and both valid surfaces are frozen and consumable now.
- Decision: both surfaces share one strict parsing pipeline (JSON is a YAML subset; the JSON entrypoint additionally requires `json.Valid` input), so shape classification and semantic validation cannot drift between forms and the equivalence proof compares decoded values directly.
- Decision (planning review, 2026-08-31): #93 is approved, so this Ticket does not introduce a second copy of the shared validation primitives. After #93 merges, this branch rebases onto it and reuses its `ValidationCategory`, `ValidationError`, `Duration`, and parsing helpers; only the ClaimRequest-specific category and shape validators are added here.
- Fold instructions for that rebase (the two copies are not identical, so deleting either outright loses behavior):
  1. Keep #93's `ValidationCategoryInvalidCapabilityCeiling`; it has no ClaimRequest equivalent.
  2. Carry over the `Duration` marshallers (`MarshalYAML`, `MarshalJSON`, `UnmarshalJSON`) and the `encoding/json` import added here; without them the ClaimRequest JSON and YAML round-trip tests fail, because the default encoding emits nanoseconds that the strict parser rejects.
  3. `validateMapping`'s reserved-field and unknown-field `Detail` strings must be contract-neutral in the shared helper. #93 currently names AgentTemplate, which would mislabel ClaimRequest failures; this branch uses neutral wording. Category and field path are unaffected, and tests assert only those two, so a careless fold would pass while emitting misleading text.
- Decision (planning re-review, 2026-08-31): `spec.task.input` is JSON-compatible structured task data, not `map[string]string` — the field is task-specific and the contract must not freeze it to the current all-string fixture. Non-JSON-representable YAML constructs fail closed, and one focused structured-input round-trip case is required.
- Decision (2026-08-31): the JSON-compatibility rules for `task.input` are enforced twice by design — on the YAML document tree before decoding (a tag such as `!!binary` need not survive decoding) and again on decoded Go values from `ValidateClaimRequest` (callers can construct the public `map[string]any` directly, including with NaN or binary values). Aliases and merge keys inside `task.input` are rejected in v0 to avoid expansion and duplicate-key ambiguity; anchors without aliases are inert. The round-trip fixture authors numbers with identical spellings on both surfaces and uses direct value equality, because a numeric-tolerant comparison could hide precision loss.
- Owner/Reviewer approval: granted 2026-09-01. After the two re-review items were addressed in `a62229d`, the Owner delegated the final check: implementation may proceed once the self-review against the acceptance criteria passes. The self-review passed (all focused tests, fold instructions verified, `check.ps1 -All` green after merging main).
- Blockers: none. #93 (E1-T2) merged on 2026-08-31; this branch merged `origin/main` (`ac08840`) and folded the duplicate shared primitives per the instructions above.
