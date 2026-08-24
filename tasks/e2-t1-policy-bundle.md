# Task

- Mission: Load one deterministic, versioned, default-deny policy source for the reference control plane.
- Parent Epic: https://github.com/wunderforge/agenova/issues/6
- Target: `internal/policy/`
- MVP-path outcome: Establish the policy source used before claim creation and backend allocation.

## Scope

In scope:

- Validate and load one static `PolicyBundle`.
- Match exact action, project, and template rules with default-deny behavior.
- Preserve the last valid bundle when a replacement is invalid.

Out of scope:

- Policy CRUD, hot reload, Rego/CEL, action authorization, or a general authoring language.

## Acceptance Criteria

- A valid bundle loads with a stable ID and version.
- Missing or unmatched action, project, and template rules deny by default.
- Malformed and duplicate rules fail without replacing the last valid bundle.

## Negative Case

- Loading a malformed or duplicate replacement returns an error and retains the prior bundle.

## Quality Gates

- `go test ./internal/policy`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Exact focused and baseline test output recorded in the PR for #26.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden scope without recording a blocker or requesting a decision.

## Known Gotchas

- #26 depends on #25; keep this package independent of claim fields until that contract merges.
