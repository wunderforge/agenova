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
- Match exact action, project, and template rules with default-deny behavior.
- Preserve the last valid bundle when a replacement is invalid.
- Keep bundle inputs, stored state, and returned values isolated from caller mutation.
- Support concurrent readers and writers without data races.

Out of scope:

- Policy CRUD, hot reload, Rego/CEL, action authorization, or a general authoring language.
- Freezing Principal or issued-state fields before dependent Ticket #25 is merged.

## Acceptance Criteria

- A valid bundle loads with a stable ID and version.
- Missing or unmatched action, project, and template rules deny by default.
- Malformed and duplicate rules fail without replacing the last valid bundle.
- An identical ID/version reload is idempotent, while changed content under the same ID/version is rejected without replacing the active bundle.
- Caller mutation cannot change the loader's active bundle.
- Concurrent reads and writes pass the Go race detector.

## Negative Case

- Loading a malformed or duplicate replacement, or changed content under an existing ID/version, returns an error and retains the prior bundle.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Preserve the previously approved Ticket contract while migrating it to the current task-packet structure.
- [x] Add the validated, immutable, concurrency-safe bundle loader and exact-match default-deny behavior.
- [x] Add focused valid, malformed, duplicate, unmatched, immutable-version, rollback, defensive-copy, and concurrent-access evidence.
- [x] Merge the latest `main` and run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -race ./internal/policy`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused race-detector output for `./internal/policy`.
- Passing repository baseline output after merging the latest `main`.
- Explicit confirmation that Ticket #25 remains a dependency for final Principal and issued-state field alignment.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep this package independent of claim and Principal field definitions until Ticket #25 merges.

## Decisions and Blockers

- Decision: the loader accepts a backend-neutral in-memory bundle and does not add policy CRUD, a policy language, or backend-specific fields.
- Decision: use exact matching and absence-as-denial; no implicit normalization or wildcard authority is introduced.
- Decision: treat `ID@Version` as immutable content identity; structurally identical ordered-rule reloads are idempotent, while a changed ordered rule set requires a new version.
- Decision: align canonical test policy references with the merged v0 fixture (`reference-default-deny@1`).
- Dependency: #26 is blocked by #25. After #25 merges, rebase and confirm whether the final Principal/issued-state contract requires a small rule-field or caller-context alignment before this PR leaves Draft.

## Verification Evidence

- `go test -race ./internal/policy` passed after merging `main` at `abc8013`.
- `go test ./...` passed on the merged tree.
- `./scripts/check.ps1 -All` passed: delivery contracts, formatting, module tidy, `go vet`, all Go tests, and Agent Sandbox integration compilation.
