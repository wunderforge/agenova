# Task: Supply a trusted local principal boundary

- Ticket: [#42](https://github.com/wunderforge/agenova/issues/42)
- Mission: Supply deterministic Team A or Team B identity through one explicit reference authentication boundary outside ClaimRequest, then consume the existing assignment authorization gate.
- Target: A small local reference identity boundary, composition in `internal/app/`, and focused integration/demo tests; exact new file names follow dependency acceptance.
- User value: Run identical YAML under two local principals and inspect a real denial without a fabricated claim.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario).

## Context to Read

Always:

- [Agent routing](../../AGENTS.md)
- [MVP PRD](../../docs/product/prd.md)
- this task packet

Additional task-specific context:

- [Feature spec](spec.md)
- [Submission and Resolution](../../docs/product/architecture-contract.md#submission-and-resolution) and [Evidence Surfaces](../../docs/product/architecture-contract.md#evidence-surfaces)
- [AIDLC](../../docs/development/AIDLC.md#adaptive-planning-depth)
- [Start a GitHub Ticket](../../docs/harness/playbooks.md#start-a-github-ticket) and [Add a User-Facing Demo Slice](../../docs/harness/playbooks.md#add-a-user-facing-demo-slice)
- [ClaimRequest parser](../../api/v1alpha1/claim_request.go), [canonical Principal/Decision/Evidence](../../api/v1alpha1/sandbox_claim.go), and [composition root](../../internal/app/runtime.go)
- [Merged #27 / PR #96](https://github.com/wunderforge/agenova/pull/96): [gate](../../internal/authorization/authorization.go), [spec](../0027-authorize-assignment/spec.md), and [tests](../../internal/authorization/authorization_test.go)
- [#40 composition packet](../0040-cli-composition-root/task.md); [#41 submission owner](https://github.com/wunderforge/agenova/issues/41) is a coordination boundary, not extra implementation scope.

## Scope

In scope:

- One explicitly local/demo authentication boundary configured outside request data with exactly two deterministic identity presets.
- Inject canonical Principal separately from parsed ClaimRequest into #27's existing gate, reusing policy evaluation and Admission.
- Identical-YAML A/B integration, request/decision evidence, tamper tests and a bounded executable reference smoke path.

Out of scope:

- OIDC, SSO, production authentication, user directory/management, session management, credentials or identity-provider integration.
- Reimplementing #27 authorization, policy administration, effective-authority resolution (#28), claim issuance (#29), runtime allocation or append-only storage (#37).
- Implementing #41's full run command, changing PR #96 or copying its implementation onto main.

## Acceptance Criteria

- The boundary constructs fixed non-empty subject/team/authentication-context values. Request data cannot select or overwrite them.
- Identical ClaimRequest bytes run under either principal; only out-of-band reference setup changes.
- With canonical active policy, Team A reaches assignment authorization and its authorized continuation exactly once. A continuation spy proves reachability, not actual issuance.
- Team B receives Deny before effective-authority resolution, claim creation or backend allocation, with zero downstream calls.
- Team B output uses canonical Principal/Action/Decision/Evidence in IssuedState, with request reference, decision ID, actual policy ID/version and reason. Claim, effective authority, claim ID and backend identity are absent.
- Reserved YAML principal fields are rejected; identity-looking permitted task input cannot change the injected principal.

## Negative Case

- Run as Team B with Team A principal data inserted at reserved YAML paths: parser rejects it and downstream calls remain zero.
- Insert Team A identity strings into permitted opaque task input: trusted principal stays Team B and authorization denies.
- Missing or unknown local preset fails before authorization and any side effect, without fallback to Team A.

## Execution Todo

- [x] Scout relevant contracts, composition, dependency head and reviews.
- [x] Confirm this packet with the Owner and Reviewer in #42 before implementation.
- [x] Record the exact accepted #96 head and establish the required implementation stack.
- [x] Add the single local identity boundary and inject Principal through the composition edge into the existing gate.
- [x] Add same-YAML integration, tamper tests and executable reference evidence output.
- [x] Run focused G2/G3 evidence and the repository baseline; record exact commands/artifacts.
- [x] Review the diff for scope, regressions and source-of-truth changes.

## Quality Gates

- Planning: `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -Docs` and `git diff --check`.
- Later G2: `go test -race -count=1 -v ./internal/app/... ./internal/authorization/... ./api/v1alpha1`; include any new boundary package explicitly in the final command.
- Later G3: executable same-file A/B smoke plus tamper cases. Record the exact implemented entrypoint, commands, fixture SHA-256, outputs and exit codes in the implementation PR; no nonexistent command is claimed as evidence here.
- Later baseline: `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All`.

## Evidence Required

- Planning: docs-check output, whitespace-check exit code, docs-only diff, exact commit and draft PR URL in the #42 handoff.
- Implementation: fixture digest before/after both runs; actual principal and authorization output; one Allow continuation versus zero Deny continuation/claim/backend calls.
- Team B serialized canonical evidence validates without a fabricated claim. Empty runtime/tool/model collections remain canonical.
- Reserved-field rejection, opaque-input non-authority and missing/unknown preset evidence.
- A continuation spy is not issuance or real-backend proof. Record dependency blockers instead of inventing policy/claim/backend identities.

## Constraints

- Preserve [architecture contract](../../docs/product/architecture-contract.md); do not broaden the Ticket or PRD without a recorded human decision.
- Preset selection is explicit local operator/test setup outside request data. CLI possession grants no authority; no production authentication claim.
- Consume #27's `authorization.Request`, `Gate.Admit` and `Admission`. Only validated project/template references become Action context; task input supplies neither project nor principal.
- Keep the implementation local/demo-only: no production authentication, merge, issue closure or completion claim before review.

## Decisions and Blockers

- Planning depth: Task + Spec, the minimum for shared authority semantics; no separate design document.
- #42 originally stacked on the Owner-approved #96 head. PR #96 merged on 2026-09-10, and this branch was then reconciled with the merged contract on `main`.
- #40 and #27 dependencies are resolved. GitHub remains authoritative for live status and assignments.
- Canonical A/B evidence uses a real active versioned policy; it never fabricates a policy or claim identity.
- The implementation PR uses `Closes #42` to satisfy the PR-body contract, remains draft, and must not merge before implementation review.
- Focused G2/G3 passed with `go test -count=1 -v ./internal/app/... ./internal/authorization/... ./api/v1alpha1`; the shared YAML SHA-256 was `778d101e8ef88c245fe3433d1901020b16cd138a4848c921e5a7e4daccbe7915` for both principals. Team A produced Allow and one continuation; Team B produced Deny, zero continuations, and no claim.
- Repository baseline passed with `GOFLAGS=-buildvcs=false` and `CGO_ENABLED=0`: `pwsh -NoLogo -NoProfile -File scripts/check.ps1 -All`. The flag avoids nested-worktree VCS stamping only; it does not skip repository checks. Local race execution is blocked because this Windows host has no C compiler; CI owns the race result.
