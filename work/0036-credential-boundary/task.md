# Task: Keep credentials behind gateway adapters

- Ticket: [#36](https://github.com/wunderforge/agenova/issues/36)
- Mission: Close every supported path that could carry a long-lived provider credential from public input into a claim or worker configuration.
- Target: `api/v1alpha1/`, `internal/app/`, `internal/gateway/`, `internal/runtime/agentsandbox/`, and focused boundary tests
- User value: Agent workers receive only claim-scoped identity and task data; provider credentials remain private to the service adapter that uses them.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture authority and credential boundary](../../docs/product/architecture-contract.md#authority-and-credentials)
- [ClaimRequest validation](../../api/v1alpha1/claim_request.go)
- [Run-service launch boundary](../../internal/app/run_service.go)
- [Gateway parameter validation](../../internal/gateway/validate.go)
- [Agent Sandbox worker shape](../../internal/runtime/agentsandbox/types.go)

## Scope

In scope:

- Define one deterministic, exact-name rule for credential-bearing keys and reuse it across ClaimRequest, runtime launch, and gateway request validation.
- Reject credential-bearing keys anywhere in structured task input, including requests constructed directly in Go.
- Reject credential-bearing runtime launch input before backend allocation.
- Prove the Agent Sandbox template rendered by the reference adapter contains no environment, Secret, or credential material.
- Prove gateway provider configuration can remain adapter-private and is never supplied by the worker request.

Out of scope:

- Heuristic inspection or redaction of arbitrary task prose and source files.
- A production secret manager, credential broker, workload identity implementation, or network anti-bypass guarantee.
- Changing AgentTemplate, claim, authority, gateway, or RuntimeBackend public shapes.

## Acceptance Criteria

- The existing secret fixtures remain rejected with `secret-value`.
- Exact reserved credential keys in nested ClaimRequest task input are rejected deterministically on YAML/JSON and direct-Go validation paths.
- Exact reserved credential keys in `ResolvedLaunch.Input` fail before any `RuntimeBackend.Allocate` call.
- Tool and Model requests cannot carry reserved credential parameters, while an adapter may use configuration it owns privately.
- The reference Agent Sandbox worker template contains only the declared image/command and no credential-bearing environment or Secret fields.

## Negative Case

- A launch input containing `GITHUB_TOKEN` must return `ErrInvalidRun` and leave the backend call trace empty.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and Reviewer before implementation. The Owner authorized continuous delivery on 2026-09-15.
- [x] Centralize the exact credential-key rule without adding heuristic value scanning.
- [x] Apply it to public structured input, runtime launch, and existing gateway validation.
- [x] Add the worker-template and adapter-private configuration assertions.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth accuracy.

## Quality Gates

- `go test -count=1 ./api/v1alpha1 ./internal/app ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/runtime/agentsandbox`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused tests for nested public input rejection, zero backend allocation, gateway denial before adapters, adapter-private configuration, and credential-free Agent Sandbox manifests.
- Passing repository baseline output.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Reject by exact normalized field name; do not scan arbitrary values or claim protection against deliberately disguised secrets.
- Claim-scoped control tokens are identity material, not upstream provider credentials.

## Decisions and Blockers

- Decision: keep credential-key vocabulary in the API contract package so every input boundary uses one rule; gateway compatibility helpers delegate to it.
- Decision: structural exclusion plus mechanical fixture/config assertions are the v0 guarantee. Arbitrary prose and repository content are not a secret-detection surface.
- Evidence: `go test -count=1 ./api/v1alpha1 ./internal/app ./internal/gateway ./internal/toolgateway ./internal/modelgateway ./internal/runtime/agentsandbox` and `.\scripts\check.ps1 -All` pass, including nested public-input rejection, zero-allocation launch rejection, adapter-private configuration, and credential-free worker-template cases.
- Blockers: none; #34 and #35 are merged.
- Review follow-up: added normalized OAuth `client_secret`/`clientSecret` rejection across task, launch, and gateway inputs, and direct JSON nested-field coverage. The focused gate and full baseline pass again.
- Review follow-up: reserve `AZURE_CLIENT_SECRET` across the same boundaries; reject custom JSON encoders and named byte slices in direct-Go task input so serialized shapes cannot bypass structural validation. The focused gate and `.\scripts\check.ps1 -All` pass again, including all seven browser smoke cases.
