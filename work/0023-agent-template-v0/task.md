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
- Semantic validation with stable categories for required fields, malformed capability ceilings, caller-supplied issued authority, and embedded secret values.
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
- Missing artifact or entrypoint inputs are rejected with category `required-field`.
- A non-list capability ceiling is rejected with category `invalid-capability-ceiling`.
- Caller-authored `effectiveAuthority` is rejected with category `system-managed-field`.
- Raw environment/credential values are rejected with category `secret-value`.
- Focused tests discover all six AgentTemplate cases through the shared manifest and assert the manifest's expected outcome/category.
- Existing runtime-spike types and backend adapters continue to compile unchanged.

## Negative Case

- Each of the five shared negative AgentTemplate fixtures must fail closed for its manifest-declared category; unknown or malformed document structure must not be accepted as a valid reusable role.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Confirm this packet with the Owner and Reviewer before implementation.
- [ ] Add the backend-neutral AgentTemplate v0 model and categorized validation surface in `api/v1alpha1/`.
- [ ] Add strict YAML parsing that distinguishes the named fixture failure categories without introducing a second schema.
- [ ] Add focused tests that load the shared v0 manifest and AgentTemplate inputs directly.
- [ ] Add or update focused behavioral evidence and record the exact fixture case IDs exercised.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

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

## Decisions and Blockers

- Planning depth: Task + Spec because this Ticket defines a public contract consumed by later request and authority-resolution work; no Design is needed for the bounded Go model/parser implementation.
- Dependency: #22 is satisfied by merged PR #79 (`abc8013` on `main`).
- Blocker: the GitHub Issue does not yet record the required Owner and independent Reviewer approval of this Task + Spec packet, so implementation must not begin.
- Environment blocker for later verification: `go` is not currently available on this Windows PATH.

