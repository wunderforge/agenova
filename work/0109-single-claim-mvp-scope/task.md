# Task: De-scope parent/child multi-agent lineage from the committed MVP

- Ticket: [#109](https://github.com/wunderforge/agenova/issues/109)
- Mission: Align the committed MVP, issue graph, and demo path around one governed agent assignment while retaining parent/child governance as a future extension.
- Target: Product authorities, implementation-status routing, E5/E9/E11 issue scopes, and mid-term delivery dependencies.
- User value: Contributors can deliver and explain the MVP without an unplanned multi-agent lineage dependency.
- PRD outcome: [MVP Deliverables](../../docs/product/prd.md#mvp-deliverables) and [Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture facts and future-lineage boundary](../../docs/product/architecture-contract.md#facts-and-future-lineage)
- [Implementation evidence snapshot](../../docs/project-status.md)
- [Mid-term coordination issue #107](https://github.com/wunderforge/agenova/issues/107)
- [Feature specification](spec.md)

## Scope

In scope:

- Remove parent/child authority inheritance, lineage, and the reviewer child assignment from the committed MVP and demo acceptance path.
- Preserve single-claim fact attribution, cross-claim isolation, terminal revocation, and denied-request evidence.
- Record the minimum trusted invocation-context invariant needed to test cross-claim isolation without defining a production identity mechanism.
- Reclassify the direct multi-agent tickets and remove their dependencies from retained MVP work.
- Update public/product routing so existing experimental reference behavior is not mistaken for committed MVP scope.

Out of scope:

- Removing existing experimental parent/child implementation or tests.
- Redesigning public claim, fact, or evidence schemas.
- Implementing a workflow engine or a replacement multi-agent scenario.

## Acceptance Criteria

- PRD, architecture guidance, status, README, project design, Epics, tickets, dependencies, milestones, and priorities agree on a single-claim committed MVP.
- #39 and #54 remain discoverable future work but block no committed MVP ticket.
- #37, #38, #55, #58, and #60 retain their single-claim value without required lineage.
- Two explicit reviews find no hidden multi-agent dependency in the committed vertical slice.

## Negative Case

- A residue scan must fail the review if any committed MVP deliverable, acceptance step, or retained ticket still requires parent/child lineage or a reviewer child claim.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Confirm this packet with the Owner and resolve the independent automated review findings before merge.
- [x] Update the product authorities and public/status routing.
- [x] Update the affected GitHub issue scopes, dependencies, and delivery metadata.
- [x] Run the multi-agent residue scan and dependency/vertical-slice closure review.
- [x] Run the focused gate and `./scripts/check.ps1 -All`.
- [x] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `.\scripts\check.ps1 -Docs`
- `rg -n -i "parent|child|lineage|multi-agent|multi agent|engineer/reviewer|orchestrator" AGENTS.md README.md docs harness work/0109-single-claim-mvp-scope`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Passing docs and repository gates, the two review summaries, and a PR list of every updated issue/dependency.

## Constraints

- Update `docs/product/architecture-contract.md` only to distinguish committed behavior from the retained future boundary and to make the approved minimum trusted invocation-context invariant explicit.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Do not remove existing experimental code or weaken claim attribution, cross-claim isolation, or terminal-state denial.

## Decisions and Blockers

- Owner approved removing parent/child multi-agent lineage from the committed MVP on 2026-09-08; the existing implementation may remain as experimental evidence.
- Owner approved on 2026-09-10 that governed calls use system-established context bound to the executing worker claim; a caller-supplied claim ID alone cannot authorize another claim. This records the minimum assumption needed to preserve the existing cross-claim isolation requirement, while leaving production workload-identity design out of scope.
- Calibration pass 1 (scope residue): all remaining parent/child, lineage, or multi-agent references in current documentation are explicitly future, experimental, or out of scope. The scan also found and corrected one stale statement that treated the merged `AgentTemplate` and `ClaimRequest` contracts as unimplemented. A broader follow-up scan using independent `parent` and `child` terms caught residual parent/child acceptance clauses in #32 and #35; both were removed before the pass was repeated.
- Calibration pass 2 (dependency closure): E5 now contains only #37/#38, E9 contains only #53/#55, retained MVP tickets do not depend on #39/#54, and the #107 single-claim critical path remains complete. #39/#54 are P2 Backlog items with no milestone or Epic/Wave/Sequence assignment.
- `./scripts/check.ps1 -Docs` and `./scripts/check.ps1 -All` passed on 2026-09-08.
- After reconciling with current `main`, the expanded residue scan, `./scripts/check.ps1 -Docs`, and `./scripts/check.ps1 -All` passed again on 2026-09-10.
- The automated independent review found four gaps: harness routing, stale implementation status, undefined trusted invocation context, and missing cross-claim attribution evidence. All four are corrected in the final diff. Under the Owner's one-day review fallback, a clean final review and green CI permit merge without waiting for another approval.
