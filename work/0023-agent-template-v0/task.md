# Task: Define and validate AgentTemplate v0

- Ticket: [#23](https://github.com/wunderforge/agenova/issues/23)
- Mission: Define the smallest backend-neutral `AgentTemplate v0` Go contract and validator so reusable roles can be accepted or rejected before claim resolution.
- Target: `api/v1alpha1/` AgentTemplate types, YAML parsing/validation, focused fixture-driven tests, and this task packet.
- User value: Contributors can load one reusable agent role, prove that its runnable artifact and capability ceiling are valid, and reject embedded credentials or issued authority before later request-resolution work consumes it.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract: Product Center](../../docs/product/architecture-contract.md#product-center), [Backend Neutrality](../../docs/product/architecture-contract.md#backend-neutrality), and [Authority and Credentials](../../docs/product/architecture-contract.md#authority-and-credentials)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [E1-T1 fixture specification](../0022-core-contract-fixtures/spec.md)
- [Shared contract fixture manifest](../../harness/fixtures/contract/v0/manifest.json) and its `inputs/agent-template/` files
- [Current shared API types](../../api/v1alpha1/types.go)

## Scope

In scope:

- Backend-neutral `AgentTemplate v0` types for object identity, runnable artifact, entrypoint, defaults, and capability ceiling.
- YAML parsing for the human-authored AgentTemplate surface represented by the merged E1-T1 fixtures.
- Semantic validation returning a typed error/result with stable `category` and `field/path` data for required fields, malformed capability ceilings, caller-supplied issued authority, embedded secret values, and unknown fields.
- Focused Go tests that select and load the six shared AgentTemplate fixture cases directly from the v0 manifest without copying their contents.
- Narrow unit coverage for contract invariants that are not safely expressible by duplicating fixture files.

Out of scope:

- Template registry, persistence, discovery, version migration, generated schema/CRD/OpenAPI output, CLI submission, or an instructions framework.
- `ClaimRequest`, effective-authority resolution, policy evaluation, claim creation, runtime allocation, or backend mapping.
- Kubernetes, Agent Sandbox, Docker, E2B, Daytona, or other provider-specific fields in the shared contract.
- Live credential resolution or secret delivery; long-lived external credentials remain behind governed interfaces.

## Acceptance Criteria

- The shared `agent-template.valid.engineer` YAML parses to the public v0 Go type and validates successfully.
- The parsed valid template preserves its name, artifact image, entrypoint command, defaults, capability lists, resource scopes, runtime profiles, and maximum timeout.
- `metadata.name`, `artifact.image`, and `entrypoint.command` are required and non-blank; focused unit coverage verifies the name invariant needed by later `templateRef` resolution.
- Validation returns typed data containing at least `category` and `field/path`; tests assert those fields directly and never infer categories from error-string matching.
- A missing `capabilityCeiling` is rejected with category `required-field`; an explicitly empty ceiling is valid and means default-deny across every governed capability dimension.
- Missing or blank required inputs are rejected with category `required-field` and the responsible field/path.
- A non-list capability ceiling is rejected with category `invalid-capability-ceiling`.
- Defaults outside the explicit capability ceiling are rejected with category `invalid-capability-ceiling`.
- The exact reserved path `spec.effectiveAuthority` is rejected with category `system-managed-field`.
- The exact credential-bearing path `spec.environment` is rejected with category `secret-value`.
- Other unknown fields fail closed with category `unknown-field`; reserved-field classification is path-based and does not scan field names or values heuristically.
- Focused tests discover all six AgentTemplate cases through the shared manifest and assert the manifest's expected outcome/category.
- Existing runtime-spike types and backend adapters continue to compile unchanged.

## Negative Case

- Each of the five shared negative AgentTemplate fixtures must fail closed for its manifest-declared category; focused unit cases also cover non-blank `metadata.name`, required versus explicitly empty `capabilityCeiling`, defaults outside the ceiling, and deterministic `unknown-field` handling.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Record Owner authorization to proceed and retain independent Reviewer approval as a PR gate.
- [x] Add the backend-neutral AgentTemplate v0 model and categorized validation surface in `api/v1alpha1/`.
- [x] Add strict YAML parsing that distinguishes the named fixture failure categories without introducing a separate schema artifact.
- [x] Add focused tests that load the shared v0 manifest and AgentTemplate inputs directly.
- [x] Add focused behavioral evidence and record the exact fixture case IDs exercised.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./api/v1alpha1 -run AgentTemplate`
- `go test ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused output naming all six shared AgentTemplate fixture IDs and their expected categories.
- Passing `go test ./...` and repository baseline output.
- A PR diff showing that tests consume `harness/fixtures/contract/v0/` directly and that no backend/provider types entered `api/v1alpha1`.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- The reusable template may declare only limits and safe defaults; it must not carry live external credential values or system-issued effective authority.
- The merged E1-T1 manifest and fixture files are dependencies, not files owned by this Ticket; do not fork or rewrite them to make the validator pass.
- Keep validation deterministic and side-effect free; no registry, network, filesystem lookup, or backend allocation belongs in the API contract.
- Classify reserved fields by exact document path: `spec.effectiveAuthority` is system-managed, `spec.environment` is credential-bearing, and every other unsupported field is unknown; do not use substring or value scanning.

## Decisions and Blockers

- Planning depth: Task + Spec because this Ticket defines a public contract consumed by later request and authority-resolution work; no Design is needed for the bounded Go model/parser implementation.
- Dependency: #22 is satisfied by merged PR #79 (`abc8013` on `main`).
- Planning review: `@wunderforge` requested four v0 clarifications on 2026-08-28; this revision records required non-blank identity, typed validation results, deterministic path-based reserved-field classification, and explicit empty-ceiling default deny.
- Owner authorization: after direct discussion with the project lead on 2026-08-29, the Owner directed implementation to proceed and independent review to occur on the resulting PR.
- Reviewer gate: independent approval remains required before merge.
- PRD link audit: current `main` defines `Demonstrable contributor path` as section 8, so the resolving `#8-demonstrable-contributor-path` anchor is retained and the earlier section-6 review note will be called out in the PR.
- Verification environment: Go 1.27.0 is installed and the pre-change `go test ./...` baseline passes.
- Blockers: none.

## Verification Evidence

- `go test -count=1 -v ./api/v1alpha1 -run AgentTemplate` passed and named all six shared cases: `agent-template.valid.engineer`, `agent-template.invalid.missing-artifact`, `agent-template.invalid.missing-entrypoint`, `agent-template.invalid.capability-ceiling`, `agent-template.invalid.issued-authority`, and `agent-template.invalid.secret-value`.
- Focused unit cases passed for non-blank template identity, runnable artifact/entrypoint requirements, required versus explicitly empty capability ceilings, defaults outside the ceiling, invalid list/timeout values, exact-path reserved-field classification, unknown nested fields, non-heuristic value handling, and multiple-document rejection.
- `.\scripts\check.ps1 -All` passed: documentation/contracts, Markdown links, formatting, module tidy, `go vet ./...`, `go test ./...`, and Agent Sandbox integration-package compilation.
- Diff audit: shared API additions contain no backend/provider types; tests read the E1-T1 manifest and its referenced YAML files directly; the PRD, architecture contract, fixture manifest, and fixture inputs are unchanged.
- Residual review point: public type naming and the strict parser surface require independent PR approval before merge.

