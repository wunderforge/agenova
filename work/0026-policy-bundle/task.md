# Task: Load one versioned default-deny PolicyBundle

- Ticket: [#26](https://github.com/wunderforge/agenova/issues/26)
- Mission: Load one deterministic, versioned, default-deny policy source for the reference control plane.
- Target: `internal/policy/` and this task packet.
- User value: The control plane can replace a validated policy bundle atomically and deny requests that have no exact rule before claim creation or backend allocation.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md#submission-and-resolution)
- [Canonical contract fixtures](../../harness/fixtures/contract/v0/manifest.json)
- [Policy bundle implementation](../../internal/policy/bundle.go)
- [Policy bundle tests](../../internal/policy/bundle_test.go)
- [Dependent issued-state contract Ticket #25](https://github.com/wunderforge/agenova/issues/25)

## Scope

In scope:

- Validate and load one static `PolicyBundle`.
- Match exact trusted team, action, project, and template rules with default-deny behavior.
- Preserve the last valid bundle when a replacement is invalid.
- Keep bundle inputs, stored state, and returned values isolated from caller mutation.
- Retain previously seen ID/version content identities so an older audit reference cannot later be rebound to different rules.
- Support concurrent readers and writers without data races.

Out of scope:

- Policy CRUD, hot reload, Rego/CEL, action authorization, or a general authoring language.
- Defining authentication or accepting caller-authored identity from `ClaimRequest`; the trusted team value is supplied by the later authorization boundary from the #25 Principal contract.

## Acceptance Criteria

- A valid bundle loads with a stable ID and version.
- Missing or unmatched trusted team, action, project, and template rules deny by default.
- Malformed and duplicate rules fail without replacing the last valid bundle.
- An identical ID/version reload is idempotent, while changed content under the same ID/version is rejected even after another version became active.
- Caller mutation cannot change the loader's active bundle.
- Concurrent reads and writes pass the Go race detector.

## Negative Case

- Loading a malformed or duplicate replacement, or changed content under an existing ID/version, returns an error and retains the prior bundle.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Preserve the previously approved Ticket contract while migrating it to the current task-packet structure.
- [x] Add the validated, immutable, concurrency-safe bundle loader and exact-match default-deny behavior.
- [x] Add focused Team A allow, Team B/default-deny, malformed, duplicate, immutable-version, rollback, defensive-copy, and concurrent-access evidence.
- [x] Merge the latest `main` and run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -race ./internal/policy`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused race-detector output for `./internal/policy`.
- Passing repository baseline output after merging the latest `main`.
- Explicit confirmation that the merged Ticket #25 `Principal.Team` and issued-state policy reference align with this package.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep this package independent of claim issuance and authentication. `Rule.Team` represents the exact trusted `Principal.team` value selected by #27; callers must not source it from `ClaimRequest`.

## Decisions and Blockers

- Decision: the loader accepts a backend-neutral in-memory bundle and does not add policy CRUD, a policy language, or backend-specific fields.
- Decision: use exact matching and absence-as-denial; no implicit normalization or wildcard authority is introduced.
- Decision: add one principal-scoped v0 dimension using the canonical trusted `team` value required by #27; broader metadata selectors remain out of scope.
- Decision: treat `ID@Version` as immutable content identity for the Loader lifetime; structurally identical ordered-rule reloads are idempotent, while a changed or reordered rule set requires a new version.
- Decision: expose a structured `Match` value rather than positional string arguments so #27 can explicitly map trusted `Principal.team` and `Action` fields into the four exact-match dimensions.
- Decision: align canonical test policy references with the merged v0 fixture (`reference-default-deny@1`).
- Dependency resolved: merged Ticket #25 exposes canonical `Principal.Team` and `PolicyReference`; `Rule.Team` consumes that trusted value through the later #27 boundary without creating a duplicate public Principal type.

## Verification Evidence

- `go test -race -count=1 -v ./internal/policy` passed after merging `main` at `3365cd0`.
- `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All` passed on the merged tree: delivery contracts, formatting, module tidy, `go vet`, all Go tests, and Agent Sandbox integration compilation.
