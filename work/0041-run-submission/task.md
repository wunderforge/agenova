# Task: Implement `agenova run -f` submission

- Ticket: [#41](https://github.com/wunderforge/agenova/issues/41)
- Mission: Make `agenova run -f` submit exactly one canonical ClaimRequest through application resolution without granting identity or authority through flags, and without allocating a runtime backend.
- Target: `cmd/agenova/`, `internal/cli/`, `internal/app/` submission path.
- User value: A teammate can submit the payment-timeout YAML and see Allow or Deny from the trusted local principal and reference policy; invalid documents fail before allocation.
- PRD outcome: [MVP User Journey](../../docs/product/prd.md#mvp-user-journey) — `agenova run -f` is the ClaimRequest submission client, not a second flag-based contract.

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Architecture contract](../../docs/product/architecture-contract.md)
- [Current implementation snapshot](../../docs/project-status.md)
- [Add a User-Facing Demo Slice playbook](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice)
- [ClaimRequest v0](../../api/v1alpha1/claim_request.go)
- [Reference assignment admission](../../internal/app/reference_admission.go)
- [Trusted local principal](../../internal/app/reference_principal.go)
- [CLI composition root](../../work/0040-cli-composition-root/task.md)
- [Canonical payment-timeout fixture](../../harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml)

## Scope

In scope:

- `agenova run -f <claim-request.yaml>` (also `--file`) as a client of the merged ClaimRequest schema.
- Application-path admission using the trusted local principal and the reference default-deny policy.
- Actionable failures for missing `-f`, malformed YAML, secret values, self-asserted principal, missing template, unknown `--backend`, and `--repo`/`--tools`/`--model`.
- G3 evidence that Team A Allow and Team B Deny never call `RuntimeBackend.Allocate`.
- Local principal selection through operator setup (`AGENOVA_LOCAL_PRINCIPAL`), not through the file or authority flags.

Out of scope:

- Request-to-claim issuance and SandboxClaim persistence (E2-T4 / #29).
- The run service that allocates, starts, and terminates a backend (E3-T2 / #31).
- OIDC login or a remote multi-user control plane.
- `agenova install`.
- A live evidence API or React console transport (#38, #68).
- Wiring the Kubernetes Agent Sandbox adapter into `run`.

## Acceptance Criteria

- The payment-timeout YAML runs through application resolution for the Team A local principal and reports Allow without allocating.
- The same YAML is Denied for the Team B local principal without allocating.
- Malformed YAML, secret values, and self-asserted principal fail before allocation.
- `--repo`, `--tools`, and `--model` remain unknown flags and are not treated as granted access.

## Negative Case

- `agenova run` without `-f`, `agenova run -f --help`, unknown `--backend`, and authority flags exit 2.
- Invalid ClaimRequest fixtures exit 2 and do not allocate.
- Team B Deny exits 1, prints `allocated: false`, and does not allocate.
- A handler that reports `allocated: true` is treated as a product error because this ticket does not own allocation.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner before implementation.
- [x] Add `run -f` command behavior that rejects authority flags and unknown backends.
- [x] Add composition-root submission that admits YAML and never calls Allocate.
- [x] Add focused unit, composition, and executable smoke evidence.
- [x] Run the focused gate and `.\scripts\check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Verification Evidence

- `go test -count=1 -v ./internal/cli ./internal/app ./cmd/agenova` passed.
- `./scripts/check.ps1 -Docs` passed, including `command behavior and shared contracts stay provider-neutral`.
- `./scripts/check.ps1 -All` passed.
- Captured artifacts: `docs/evidence/41/cli-smoke/`, `docs/evidence/41/repository-baseline/`.

## Quality Gates

- `go test -count=1 ./internal/cli ./internal/app ./cmd/agenova`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Valid/invalid fixture matrix with an allocation spy (G3).
- Executable smoke for Allow, Team B Deny, secret-value rejection, missing `-f`, and `--repo`.
- Passing repository baseline output.
- Exact commands recorded with the change; prose-only confirmation is not evidence.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Command behavior (`internal/cli`) must not import Kubernetes or other provider types.
- Possessing the CLI grants no authority.
- YAML/`run -f` is the only submission contract; do not add `--repo`, `--tools`, or `--model` authority shortcuts.
- Identity comes from the local principal boundary, never from ClaimRequest or CLI flags.
- Official dependencies #29 and #31 remain open; this slice must not pretend to issue a claim or allocate a worker.

## Decisions and Blockers

- Planning depth: Task only. Submission reuses the merged ClaimRequest schema and AdmitYAML path; no new public product schema.
- Decision: pick #41 over #68. #68 is blocked on the stable evidence view (#38), which is not merged.
- Decision: `run -f` stops after application admission. It hosts a RuntimeBackend so unknown backends fail closed, but it never calls Allocate. Issuance and lifecycle stay with #29 and #31.
- Decision: `AGENOVA_LOCAL_PRINCIPAL` selects the reference Team A or Team B preset. Empty defaults to Team A. Unknown values fail closed. This is operator setup, not a ClaimRequest field.
- Decision: Deny is a successful admission outcome (exit 1) rather than a usage error (exit 2). Validation and flag errors remain exit 2.
- Owner authorization: the assigned Owner directed implementation of one of #41 or #68 in this session; #41 is the ready P0 slice.
- Blockers: none for submission-through-admission. Full request-to-claim execution remains blocked on #29 and #31.
