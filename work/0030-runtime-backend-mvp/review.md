# Ticket #30 implementation review

- Date: 2026-09-10
- Scope: PR #106's complete diff against base `b060842e67883e3f5108d73701903998b4b77f67`, including its approved packet, runtime/adapter code, consumers, tests and evidence.
- Code revision: `e2e8af43fa624544ed3239e1b4994c160d2cead2` plus the explicit-context wrapper fix. Go sources match the [Go manifest](../../docs/evidence/30/reference-contract/source-sha256.txt); the changed scripts are pinned by the [wrapper manifest](../../docs/evidence/30/repository-baseline/source-sha256.txt).
- Status: technical review and #31 handoff complete. The cleanup and wrapper-context findings are fixed. Build, both focused groups and full race passed on the unchanged Go sources; the nine wrapper regression cases and all 13 repository checks passed after the wrapper fix. Independent human review and acceptance remain outstanding.
- Downstream surface: [handoff to #31](handoff-0031.md).

## Acceptance and evidence map

| Ticket requirement | Review result | Executable evidence |
| --- | --- | --- |
| Small, neutral backend boundary | RuntimeBackend has five operations; it reuses the public Backend/WorkerID pair. No public claim/schema change or provider object enters the shared input/results. | Build, app/CLI tests and the repository provider-boundary check. |
| Allocation identity and explicit failures | Duplicate/unknown identities are rejected; unavailable reference capacity creates no usable identity. Adapter recovery checks ownership, retains uncertain attempts and protects reservations. | TestRuntimeBackendContract; TestAllocate_failsExplicitlyWithoutCapacity; adapter TestAllocate_* and TestIdentity_* cases. |
| Readiness does not start work | Repeated Observe cannot acknowledge reference start; not-ready/repeated/terminated/released start is rejected. Both adapter start entry points explicitly report unsupported semantics. | Shared readiness/start cases; TestLegacyStartClaim_doesNotGrantRunningFromReadiness; TestReducedContractSupportMatrix. |
| Cleanup distinct from outcome | Reference backend operations do not create legacy claims or call application outcome setters. Failed start/termination/replacement are visible and retryable. Application outcome publication remains #31's job. | Four shared failure cases; TestTerminate_neverChangesLegacyClaimPhase; adapter TestCleanup_* cases. |
| Consumer compatibility | Gateway field/constructor types change to ClaimReader; authorization bodies remain unchanged. Fact behavior, unknown/non-Running/parent-denial regressions and the reference E2E remain. | Focused consumer command, including both gateway suites and harness/e2e. |
| Preserve reference-specific regressions | All six old pool lifecycle cases remain in operator tests. Pool counters are absent from the new shared suite. | TestReferencePoolLifecycle_* plus the shared twelve-case suite. |
| Honest provider claims | Adapter support matrix and legacy limits are documented. Real integration harness checks identity, unsupported start/termination and independent claim/worker absence. | Adapter unit tests; integration compile and explicit-context guard; real-backend run remains blocked. |

Commands and raw results are in [contract evidence](../../docs/evidence/30/reference-contract/summary.md), [repository evidence](../../docs/evidence/30/repository-baseline/summary.md), and the [real-backend blocker](../../docs/evidence/30/agent-sandbox/summary.md). No application run service or new gateway authority path is claimed by these backend tests.

## Finding corrected during final review

Ordinary Cleanup previously deleted the upstream claim without re-reading its binding. When that claim had switched to a second allocation's worker, deleting it could destroy that other worker even though release confirmation still checked the original identity. Recovery and Observe already checked the binding; normal Cleanup did not.

Cleanup now verifies a present claim still carries its recorded worker before deletion. Changed, missing or unreadable binding returns an error with identity retained and no delete. If the claim is already absent, cleanup only confirms absence of both recorded resources. Four regression variants cover another live allocation's worker, missing binding, empty worker and query failure; each checks zero destructive calls and a successful retry after the original binding is restored. The regression failed on the pre-fix source and passed after the fix.

## Follow-up: explicit context at the wrapper boundary

The [automated follow-up review](https://github.com/wunderforge/agenova/pull/106#discussion_r3977957850) found that `check.ps1` defaulted to a named kind context and therefore bypassed the Go harness's empty-context guard. The default is removed and both integration modes reject blank context before any checks. A child-process test copies the real entry point and substitutes only its check modules: all six omitted/empty/whitespace cases reject with zero check calls, both explicit cases forward the exact context and namespace, and ordinary `-All` remains usable without a cluster. The test failed on the old wrapper and passes after the fix. The repository baseline now runs it automatically.

## Bounds of this review and handoff

- The memory implementation is a serial reference state machine. Setup is completed before calls; active pools are not replaced. The full race suite does not establish general thread safety. The handoff requires serialization and separates legacy and reduced orchestration paths.
- Backend identity, resource observations and legacy claim state are in-process. There is no durable restore, full provider reconciliation, stable cross-instance identity service, or generic retry classification.
- The flat Input field is not a delivered agent-task transport. The interface has no caller context; #31 must handle application deadlines and late results without republishing authority.
- Agent Sandbox cannot acknowledge actual work start or worker stop separately. It cannot enumerate a worker if the claim disappeared before the worker was learned; conflicts and such missing identities can require manual recovery.
- Binding read and delete are separate upstream requests. This spike does not provide atomic identity/precondition guarantees against external rebinding or resource recreation between them; promotion requires a stronger verified mechanism and real-cluster evidence.
- Parent/child gateway cases are compatibility regressions only under the Owner's single-claim clarification. Meeting proposals do not expand this Ticket or replace the committed PRD/architecture.

## Acceptance still required

The independent human Reviewer is not named in the Ticket and no final implementation acceptance is recorded. Real-cluster evidence may remain an explicit environment blocker under the [Owner's approval](https://github.com/wunderforge/agenova/issues/30#issuecomment-5610181744). That allowance does not turn compilation, simulated-controller tests, or this technical review into a real-backend pass or merge approval.
