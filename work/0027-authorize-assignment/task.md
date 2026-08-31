# Task: Authorize assignment creation before side effects

- Ticket: [#27](https://github.com/wunderforge/agenova/issues/27)
- Mission: Authorize `claim.create` against one trusted principal and active PolicyBundle before any claim issuance or runtime allocation can begin.
- Target: `internal/authorization/`, the claim-submission composition boundary, focused tests, and this task packet.
- User value: The same engineer request is allowed for Team A and denied for Team B, with an inspectable policy decision and no side effects on denial.
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature specification](spec.md)
- [Submission and resolution architecture](../../docs/product/architecture-contract.md#submission-and-resolution)
- [Canonical contract fixtures](../../harness/fixtures/contract/v0/manifest.json)
- [Change a Core Contract playbook](../../docs/harness/playbooks.md#change-a-core-contract)
- [Principal, Action, Decision, and Evidence producer #25](https://github.com/wunderforge/agenova/issues/25)
- [PolicyBundle producer #26](https://github.com/wunderforge/agenova/issues/26)
- [ClaimRequest producer #24](https://github.com/wunderforge/agenova/issues/24)

## Scope

In scope:

- Evaluate the trusted `Principal` and requested `claim.create` action against one active, immutable PolicyBundle.
- Match principal, project, template, and action using the stable contracts owned by #24 through #26.
- Return an explicit typed `Allow` or `Deny` decision with the active policy ID/version and a non-empty reason.
- Put authorization on the submission path before any authorized continuation that may issue a claim or allocate a backend.
- Prove the canonical Team A allow and Team B pre-claim denial cases using the shared fixtures.
- Deny when no active policy or no exact rule is available.

Out of scope:

- Authenticating identity, trusting principal data supplied by `ClaimRequest`, or integrating an IdP.
- Resolving effective authority, issuing a `SandboxClaim`, allocating a backend, or defining those implementations.
- Approval workflows, policy CRUD/hot reload, wildcards, inheritance, Rego/CEL, or a general policy language.
- Deriving project identity from arbitrary task input such as a repository string.

## Acceptance Criteria

- The canonical Team A principal and `claim.create` action for the payments project and engineer template return `Allow` under the matching active policy.
- The otherwise identical Team B request returns `Deny` because no principal-scoped rule matches.
- Missing policy, missing required authorization context, and unmatched principal/action/project/template return explicit denial or validation failure according to the approved spec; none invokes the authorized continuation.
- Every completed evaluation returns or records a typed decision containing principal, action, result, policy ID/version when available, and a non-empty evidence-suitable reason.
- Only `Allow` may enter the authorized continuation; `Deny` and `ApprovalRequired` are never treated as granted authority.
- Claim and backend spies prove zero calls for every denial case.
- Shared application-facing types remain backend-neutral and no provider-specific type enters the authorization package.

## Negative Case

- Given Team B with the same requested action, project, and template as Team A, authorization returns `Deny` that can later be recorded as pre-claim evidence without a fabricated claim ID, and invokes neither the claim nor backend spy.

## Execution Todo

- [x] Scout the relevant contracts, fixtures, adjacent Tickets, risks, and dependencies.
- [ ] Confirm this packet and its two contract-alignment decisions with the Owner and Reviewer before implementation.
- [ ] Rebase on the merged #24, #25, and #26 contracts; do not create provisional duplicate public types.
- [ ] Add the smallest backend-neutral assignment authorizer and pre-side-effect gate.
- [ ] Add fixture-driven Team A/Team B and table-driven default-deny tests with claim/backend spies.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test -race ./internal/authorization/...`
- `go test ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Focused test output naming the Team A allow and Team B denial fixture cases.
- Spy counts proving zero claim-issuance and backend-allocation calls for Team B, missing-policy, and unmatched-rule cases.
- One allow-path test proving the authorized continuation is entered exactly once.
- Passing repository baseline after rebasing on the merged dependency contracts.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Consume `Principal`, `Action`, `Decision`, and policy-reference vocabulary from #25 and PolicyBundle behavior from #26; do not fork those contracts.
- The trusted principal is supplied outside `ClaimRequest`; request data cannot select or overwrite it.
- Requested project/template identify what the caller asks to use and never grant authority by themselves.
- Keep the authorization decision separate from effective-authority resolution owned by #28.
- Return evidence-ready decisions here; append-only fact recording remains owned by #37.

## Decisions and Blockers

- Planning depth: Task + Spec because this Ticket defines authority semantics shared by submission, evidence, claim issuance, CLI/API, and later gateway paths; no separate Design is needed if the two alignment decisions below are accepted.
- Decision: authorization is a backend-neutral precondition. A denied request never reaches the continuation that owns claim issuance or runtime allocation.
- Decision: the v0 evaluator is exact-match and default-deny. It does not introduce wildcard, hierarchy, or policy-merging semantics.
- Decision: `ApprovalRequired` remains part of the shared result vocabulary but is not an allow result and is not produced by this assignment-creation MVP path.
- Blocker: #25 must first freeze the trusted Principal, Action, Decision, policy-reference, and pre-claim Evidence shapes.
- Blocker: #26 currently matches only action/project/template. It must consume the approved #25 Principal shape and add one exact trusted-principal dimension so Team A and Team B can produce different outcomes. The recommended MVP match is the canonical trusted `team` value; broader metadata selectors remain outside this Ticket.
- Owner decision required: the current ClaimRequest fixture has no explicit project reference even though authorization requires project. Recommendation: add a requested `spec.projectRef` to ClaimRequest v0 and the shared fixture; it is caller-requested context, not granted authority. Do not infer project from arbitrary task input or repository naming.
- Implementation remains paused until these alignments are recorded in #24 through #26 and the packet is approved.
