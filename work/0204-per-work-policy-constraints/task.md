# Task: Per-Work organizational authority constraints

- Ticket: [#204](https://github.com/wunderforge/agenova/issues/204)
- Mission: Apply versioned reference Policy constraints to each admitted Work before issuing immutable, claim-scoped authority.
- Target: reference Policy, authority resolution/issuance, shared evidence, CLI/API/Portal, focused tests and installed kind smoke.
- User value: Two verified users submitting identical Work intent through one template can receive distinct, inspectable temporary authority.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Ticket #204](https://github.com/wunderforge/agenova/issues/204), prerequisite [#203](https://github.com/wunderforge/agenova/issues/203), and this packet's `spec.md` and `design.md`.
- [Architecture authority and evidence](../../docs/product/architecture-contract.md#authority-and-credentials), [core-contract playbook](../../docs/harness/playbooks.md#change-a-core-contract).
- `internal/policy/{bundle,document}.go`, `internal/authority/{resolver,resolution}.go` and their tests; trace merged #197/#203 admission and issuance interfaces before implementation.

## Scope

In scope:

- Optional caps in compatible, versioned reference Policy YAML; intersect request, template, matched constraints and runtime restrictions before allocation.
- Persist Policy ID/version, requested/effective authority and stable narrowing reasons in the issued snapshot and common CLI/API/Portal evidence.
- Show different effective authority for two verified users with byte-equivalent `ClaimRequest.spec`, one template and one Policy version; show later Policy versions affect only later Work.

Out of scope:

- General Policy DSL/editor, OPA/Cedar, delegated admin, worker JWT (M4), Kubernetes Operator, general secrets and AWS-specific EKS/Bedrock integration ([E13 #175](https://github.com/wunderforge/agenova/issues/175)).

## Acceptance Criteria

- Both users are admitted yet receive different, correctly narrowed issued snapshots solely through matched Policy constraints; no cap introduces authority.
- Absent and explicit-empty constraints behave deterministically; legacy admission-only Policy remains compatible.
- CLI, API and Portal render the same Policy identity, requested/effective authority and reasons.

## Negative Case

- Malformed, ambiguous, mismatched or expanding constraints fail before claim allocation and Tool/Model/backend calls, with inspectable denial.

## Execution Todo

- [x] Scout current main implementation, tests, risks and dependencies; #203 remains unmerged.
- [x] User explicitly waived packet approval on 2026-10-13 and authorized implementation.
- [x] Reconcile prepared #197/#203 contracts with latest main and self-review scope, negative cases and gates.
- [x] Add validated Policy caps and focused parsing/matching/identity tests.
- [x] Thread evaluated caps through resolution and issuance with immutable snapshots and fail-closed tests.
- [ ] Capture installed kind two-user smoke and rendered CLI/API/Portal parity proof.
- [x] Run focused gates and `./scripts/check.ps1 -All`; finish diff review and report residual risk.

## Quality Gates

- Focused Policy, authority, issuance, evidence and UI tests selected after #203 merges; installed kind two-user smoke.
- `.\scripts\check.ps1 -All`

## Evidence Required

- Redacted comparison of byte-equivalent request specs, same Policy/template, distinct trusted users and issued authority with stable reasons.
- Before/after Policy-version snapshots; denial plus zero-side-effect assertions; CLI/API/Portal parity and rendered Portal proof; real installed kind output or explicit blocker.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Trusted identity is upstream of `ClaimRequest`; no IAM or provider shape enters shared authority. E13/adapters own AWS permission mapping and may not broaden issued authority.

## Decisions and Blockers

- 2026-10-13: #203 and #197/#207 have not merged on the base checkout. Implementation must use their final trusted-identity and admission interfaces.
- 2026-10-13: The user explicitly waived packet approval and authorized implementation; the prepared #203 branch was combined with latest `origin/main` in an isolated M3 branch.
- 2026-10-13: Full check passed locally after installing locked UI dependencies and Chromium. Installed kind two-user smoke remains unproven; no AWS-specific IAM claim is made here.
- 2026-10-13: In the user's KodeKloud AWS CloudShell (Linux/amd64, Go 1.24.9), checked out commit `5beca11` and ran `go test ./internal/policy ./internal/authority ./internal/authorization ./internal/app ./internal/registration ./internal/console`; all six packages passed. A fresh `-count=1 -v` run also passed `TestSameWorkSpecGetsUserSpecificIssuedAuthority`, `TestResolveDoesNotTreatPolicyConstraintsAsAuthorityGrant`, `TestResolveExplicitEmptyPolicyCapDenies`, and `TestReferencePolicySelectsUserSpecificCaps`. This verifies the portable E14 authority boundary in the sandbox; it does not exercise EKS, AWS IAM, Bedrock, or the remaining installed kind smoke.
