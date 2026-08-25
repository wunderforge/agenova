# Task: Freeze the first executable contract fixtures

- Ticket: [#22](https://github.com/wunderforge/agenova/issues/22)
- Mission: Freeze one small, machine-readable, backend-neutral fixture set that E1-T2 through E1-T4 and fixture-first consumers can execute without redefining the product contract.
- Target: `harness/fixtures/contract/v0/`, its inventory tests, and this task packet.
- User value: Contributors can implement templates, requests, issued claims, decisions, and evidence against the same reviewable Team A engineer scenario and named negative cases.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [Demonstrable contributor path](../../docs/product/prd.md#8-demonstrable-contributor-path)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Architecture contract](../../docs/product/architecture-contract.md)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [Current shared API types](../../api/v1alpha1/types.go)
- [E1-T2 AgentTemplate consumer](https://github.com/wunderforge/agenova/issues/23)
- [E1-T3 ClaimRequest consumer](https://github.com/wunderforge/agenova/issues/24)
- [E1-T4 issued-state consumer](https://github.com/wunderforge/agenova/issues/25)

## Scope

In scope:

- A versioned JSON manifest that assigns every case a stable ID, subject, purpose, input path/format, coverage, and expected valid result or invalid error category.
- YAML inputs for human-authored AgentTemplate and ClaimRequest configuration, equivalent ClaimRequest API JSON, JSON system-issued state/evidence, and focused invalid variants required by E1-T2 through E1-T4.
- Inventory tests that parse all inputs, reject duplicate or malformed metadata, prove the canonical YAML/API JSON pair is semantically equivalent, verify required coverage/cases, and scan for backend vocabulary or secret-like values.

Out of scope:

- Product type implementation or semantic validators owned by E1-T2 through E1-T4.
- A generic conformance framework, CRD/schema generation, backend mapping, runtime execution, or exhaustive future API fields.
- Changes to the PRD, architecture contract, or current runtime/API implementation.

## Acceptance Criteria

- `harness/fixtures/contract/v0/manifest.json` is the canonical inventory and all referenced JSON/YAML inputs parse.
- Human-authored AgentTemplate cases use YAML; ClaimRequest has a canonical YAML input plus equivalent API JSON; system-issued state and evidence use JSON.
- The fixture set covers `AgentTemplate`, trusted `Principal`, action, `ClaimRequest`, policy reference, effective authority, `SandboxClaim`, decision, and evidence shapes.
- Stable positive cases represent the Team A engineer request and issued claim; a Team B denial represents inspectable evidence without a fabricated claim.
- Focused invalid cases cover missing/malformed template input, missing/unsafe request input, and caller-supplied system-managed state with stable error categories.
- Case IDs and input paths are unique, expected outcomes are explicit, the YAML/JSON request pair is equivalent, and provider-specific vocabulary or secret-like values fail the inventory test.
- No reusable conformance runner or product validation logic is introduced.

## Negative Case

- The focused inventory test must fail when a case ID is duplicated, an expected result/category is absent, an input is unreadable, required coverage is missing, the canonical YAML/JSON pair drifts, or fixture content contains backend/provider vocabulary or a secret-like value.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, dependencies, and downstream fixture consumers.
- [x] Confirm this packet with the Owner and record independent review as a PR gate before implementation.
- [x] Add the v0 manifest and canonical Team A/Team B fixture inputs.
- [x] Add focused invalid template, request, and system-issued-state inputs.
- [x] Add the deterministic inventory/parse/equivalence/boundary tests.
- [x] Run the focused gate and `.\scripts\check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./harness/fixtures/contract/v0`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing focused test output naming the fixture package.
- Passing repository baseline output.
- PR inventory of stable case IDs and fixture paths; prose-only confirmation is insufficient.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- `ClaimRequest` cannot carry a trusted authoritative principal, live external secret value, issued authority, claim phase, or backend identity.
- Fixture content must stay backend-neutral; Kubernetes/provider shapes remain outside this directory.
- This packet elaborates the Issue but does not freeze fields beyond the representative v0 cases.

## Decisions and Blockers

- Planning depth: Task + Spec because the fixtures define shared behavior consumed by multiple delivery lanes; no Design is needed because the versioned manifest/raw-input layout has one bounded implementation approach.
- Decision: use a small JSON manifest plus raw JSON/YAML product inputs. The manifest owns case metadata; raw inputs remain directly consumable by downstream tests.
- Decision: error categories describe expected semantic rejection without implementing validators in this Ticket.
- Owner authorization: the project lead explicitly requested execution of E1-T1 on 2026-08-25 and will review the resulting PR.
- Reviewer gate: independent review remains required on the PR before merge.
- Blockers: none.

## Verification Evidence

- `go test -count=1 -v ./harness/fixtures/contract/v0` passed: 18 cases (5 valid, 13 invalid), all nine required contract shapes, and seven deliberate inventory-breakage subtests.
- `.\scripts\check.ps1 -All` passed after module metadata was normalized and the scoped diff staged: docs/contracts, formatting, module tidy, `go vet`, all Go tests, and Agent Sandbox integration compilation.
- Diff audit: no product Go type, runtime/backend adapter, PRD, architecture contract, or mutable project-status source changed.
- Residual review point: E1-T2 through E1-T4 still own the exact public Go schemas and semantic validators; these fixtures intentionally do not claim that implementation is complete.
