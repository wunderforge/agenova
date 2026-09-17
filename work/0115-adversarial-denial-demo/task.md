# Task: Demonstrate adversarial denial before claim creation

- Ticket: [#115](https://github.com/wunderforge/agenova/issues/115)
- Mission: Show that a Team B (unauthorized principal) ClaimRequest is denied before any claim is created and that the denial produces inspectable evidence — proving the policy evaluation path end-to-end in fixture mode.
- Target: `examples/adversarial/` (new package: main binary and focused tests); `harness/fixtures/contract/v0/inputs/claim-request/invalid-team-b-unauthorized.yaml` (case ID `claim-request.invalid.team-b-unauthorized` in `manifest.json`)
- User value: A teammate can run the Team B denial scenario locally and observe that the governance boundary works before claim creation — no backend, no live credentials.
- PRD outcome: [Declarative request + authorization resolution](../../docs/product/prd.md#1-declarative-request--authorization-resolution)

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md)
- [E1-T1 contract fixtures](../../harness/fixtures/contract/v0/manifest.json) — canonical fixture index and naming conventions
- [E9-T4 Claude Code fixtures](https://github.com/wunderforge/agenova/issues/116) — adds the over-ceiling fixture referenced by E9-T8
- [PolicyBundle implementation](../../internal/policy/bundle.go) — existing policy evaluation logic
- [E2-T2 action authorization](../../internal/authorization/authorization.go) — merged 2026-09; produces `Decision` evidence (principal, action, result, policy ID/version, reason)
- [E2-T4 reference admission](../../internal/app/reference_admission.go) — merged 2026-09-14; the reference deny-before-issuance path this demo should mirror
- [Add a User-Facing Demo Slice playbook](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice)

## Scope

In scope:

- `harness/fixtures/contract/v0/inputs/claim-request/invalid-team-b-unauthorized.yaml` — canonical Team B ClaimRequest fixture (principal not in any authorized team; add case ID `claim-request.invalid.team-b-unauthorized` to `manifest.json`, following the existing `inputs/claim-request/` layout)
- `examples/adversarial/main.go` — reads ClaimRequest YAML/JSON via `--task`, evaluates it through the merged E2 authorization path (`internal/authorization` with a fixture PolicyBundle), prints `[DENIED]` with the real `Decision` evidence — principal subject, policy ID/version, and denial reason — before any claim is created; exits non-zero on denial and on malformed input (no panic)
- Focused tests: denial case (no claim created, Deny decision in evidence output), malformed-input case (non-zero exit, no panic)

Out of scope:

- Changes to the merged E2 packages (`internal/policy`, `internal/authorization`, `internal/authority`, `internal/issuance`, `internal/app`); this demo consumes them read-only.
- PolicyBundle CRUD or policy administration.
- Live backend, Kubernetes execution, or real credentials.
- General policy testing framework or chaos lab.
- Changes to shared API types in `api/v1alpha1/`.

## Acceptance Criteria

- `examples/adversarial/main.go` accepts `--task` pointing to a ClaimRequest YAML or JSON file.
- Team B fixture submission prints `[DENIED]` before any `SandboxClaim` is created; output includes principal subject, policy reference, and denial reason.
- Binary exits non-zero on denial and on malformed/schema-invalid input; does not panic in either case.
- No `SandboxClaim` is created on a denied submission — verified by test assertion on the claim store.
- Focused tests pass: denial case and malformed-input case are machine-verifiable.

## Negative Case

- Supplying a well-formed but unauthorized Team B ClaimRequest must produce a `[DENIED]` output with evidence and non-zero exit; the claim store must remain empty.
- Supplying a malformed ClaimRequest (missing required fields) must exit non-zero with an actionable error and no panic; no policy evaluation or claim creation attempted.

## Execution Todo

- [x] Scout `internal/policy/bundle.go`, `internal/authorization/authorization.go`, and `internal/app/reference_admission.go` to understand the merged evaluation API, the `Decision` shape, and what PolicyBundle fixture to use.
- [x] Add the Team B fixture. Placed demo-local at `examples/adversarial/testdata/team-b-unauthorized.yaml` (see Decision 2026-09-17) instead of the shared contract manifest, which cannot represent a schema-valid, authorization-denied case.
- [x] Add `examples/adversarial/main.go` with arg parsing, YAML/JSON loading, authorization evaluation via the E2 path, denial output, and non-zero exit.
- [x] Add focused tests: denial case and malformed-input case.
- [x] Run focused gate and the Go portion of `check.ps1 -All` (pwsh unavailable on this host: gofmt, go mod tidy, go vet, `go test ./...`, integration-tag compile all pass).
- [x] Review diff for scope, provider imports, and source-of-truth updates.

## Quality Gates

- `go test -count=1 -v ./examples/adversarial/...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Denial run output: exact command and output showing `[DENIED]`, principal, policy reference, denial reason, non-zero exit.
- Malformed-input run output: non-zero exit and actionable error message.
- Passing focused test output naming the `examples/adversarial` package.
- Passing repository baseline output.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- No live backend, real credentials, or provider SDK imports.
- No claim must be created on a denied submission — the denial boundary is the entire point.
- Fixture/mock execution must be labeled; no consumer should mistake this for a live Agenova policy enforcement path.

## Decisions and Blockers

- Integration Stage: Fixture / Mock — in-process, no live backend, no credentials. (Superseded premise: this packet originally deferred E2 and planned raw `internal/policy/bundle.go` evaluation.)
- Decision (2026-09-14, packet revision after pulling `b060842..10b0fbe`): E2-T2/T3/T4 merged (#96, #97, #129), so the demo evaluates through the merged authorization path and surfaces the real `Decision` evidence (policy ID/version, reason) instead of a bespoke bundle-only result. Fixture depth is unchanged; the deferred live-gateway integration stage still stands.
- Decision (2026-09-14): the new fixture follows the actual E1-T1 layout — file under `inputs/claim-request/`, case ID in `manifest.json` — not the flat path originally written in this packet. E1-T1 fixtures also now carry an optional `projectRef` field; the Team B fixture may include it for consistency.
- Decision: [#115](https://github.com/wunderforge/agenova/issues/115) is the canonical E9-T3 ticket. The older duplicate [#55](https://github.com/wunderforge/agenova/issues/55) is closed as superseded; its live-enforcement demo (against the real Tool Gateway, post-E4-T2 #34) is recorded in #115's delivery contract as this deliverable's deferred integration stage.
- Decision (2026-09-17, implementation): the Team B fixture ships as demo-local `examples/adversarial/testdata/team-b-unauthorized.yaml`, NOT in the shared E1-T1 contract manifest as the packet's Target/Scope originally specified. Reason: a Team B ClaimRequest is schema-valid (principal is out-of-band, never in the request), so it cannot be an `invalid` manifest case, and there is no authorization error category — only schema ones. The merged E1-T1 contract also asserts that every `valid` claim-request fixture *is* the canonical Team A request (`api/v1alpha1/claim_request_test.go`) and hard-codes fixture counts there and in `ui/contractgen/main_test.go`. Placing it in the manifest would require reshaping the merged E1-T1 contract and editing `api/v1alpha1/` tests, which this packet lists as out of scope. Owner approved demo-local placement; flagged in the PR for reviewer confirmation. The demo's acceptance behavior is unchanged.
- Blockers: none. E2 authorization path and E1-T1 fixtures (#22) are merged.
