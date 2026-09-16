# Work detail: one reading path

Owner requested a clearer work page on 15 September. The previous layout exposed
Run flow, Worker activity, Progress and Recent activity as competing summaries.
This is presentation only: no evidence, backend, permissions or product scope changes.

## Design

Keep the approved ocean palette: canvas #091b26, panel #112b37, ink #eaf5f8,
muted #a3bbc7, mint #70e7d0 and transport blue #92b7ff; rose for failure/denial.
Bahnschrift headings and Segoe UI body; left aligned. Spend emphasis on the user's
current question, not equally weighted technical panels.

```text
Task + who requested it
Work status             compact lifecycle, one status summary
Result                  only when an actual result exists
Agent activity          current wait; earlier turns collapsed
  Turn 1                model/tool calls and observed results
  Turn 2 [expanded]     current or last turn; user can choose another
Access & limits [+]     scope, tools, model, memory, time limit
Execution details [+]   lifecycle facts, IDs and policy correlation
Full activity record    existing detailed evidence page
```

Review against brief: remove the default dashboard card stack and duplicate Progress/
Recent activity. Lifecycle is one compact overview; agent turns are its execution detail,
not another lifecycle. Result moves ahead of history after completion. Permission checks
are not provider execution and remain separately inspectable. Unnumbered legacy evidence
must not be assigned invented turns. No private reasoning trace.

Motion answers an event or user action: one existing waiting lamp, a short active-turn
reveal and disclosure-chevron feedback. No background motion, animated blur/shadows,
height animation, automatic scrolling or replay. Respect reduced motion and keyboard use.
Polling preserves stable turn keys and user's chosen expansion. Demo and Connected use
the same reading order but never mix evidence.

## Acceptance and gates

- No separate Progress or Recent activity section on work detail.
- Work status, outcome, agent steps, limits and raw records remain reachable.
- Host-recorded turns group calls; final history retains prior tool failures and recovery.
- Only latest turn opens by default; manual expansion persists through polling.
- Result appears above agent history; cleanup failure does not hide a successful result.
- Denied/pending/missing evidence does not invent calls, grants, turns or completion.
- Focused component and Playwright tests, full UI gate and repository baseline.
- Read-only screenshot of the current real run; no new inference needed for this UI change.

## Todo

- [x] Compact lifecycle and reorganize both work-detail sources
- [x] Group actual agent turns with controlled disclosure and lightweight motion
- [x] Focused/full gates, screenshots and local review

## Review evidence (15 September)

- Final `scripts/check.ps1 -All` passed: documentation/contracts, Go format/tidy/vet/
  tests, integration compile, UI bindings/types/build, 101 component tests and 41 browser
  tests. This UI-only change did not rerun live inference or claim new backend evidence.

- Shared presentation layout keeps Demo/Connected data separate. Work detail removes
  competing Progress/Recent activity; every lifecycle fact remains in Execution details.
- Calls group under host-recorded turn markers, with correlated completion and visible
  failed-call counts. Legacy permission checks are labelled checks, not executed calls.
- Browser regression proves latest-turn default, keyboard expansion, preserved manual
  history through polling, result-before-history and default-collapsed inspection.
- Screenshot critique: reduce repeated title/opaque request ID, shrink desktop heading,
  give mobile title full width, remove large status/worker card styling. Existing palette
  and single waiting lamp retained; no blur, replay or background animation added.
- Read-only inspection of retained `work-8e380c6e-b5f8-44b3-bc70-4a5aa90c5e29`
  shows six turns, last turn expanded, zero ongoing animations after completion,
  no horizontal overflow at 1360px/390px and no page errors. No new inference run.
  Reproduce from `ui`: `node scripts/inspect-work-layout.mjs <existing-local-work-url>`.
  Local desktop/history/mobile screenshots and report: `.tmp/work-reading-path/`.

## Failure visibility follow-up (16 September)

Owner reported that a failed work's detail record said Recorded and its successful
model calls hid the failed overall outcome. Runtime/RunOutcome failure operations
now render Failed badges in record lists, detail pages and execution history.
Agent activity has a separate red work-outcome record; successful model/tool calls
stay successful. Only genuine pending calls animate.

This follow-up also corrects evidence production, not only presentation. The local
console writes a sanitized reason/code into RunOutcome and the same reason into
Outcome.failure. Typed demo-edge errors identify turn-limit, missing-final-result
and invalid-final-result failures. Provider failures/cancellations get safe reasons
in their own records. Raw provider/transport error text is never exposed. Recovered
earlier call failures are not used to explain an unrelated terminal failure.

Historical facts remain immutable: when no specific cause was recorded, say so.
The owner's retained work-411260e1-6547-448f-b650-e5cfd6e5d5de has six successful
model calls, three successful mock reads, a failed task and confirmed cleanup; it
does not record the underlying cause. No new inference was made for this correction.
Read-only screenshots: `.tmp/failure-record-detail.png`, `.tmp/failure-work-detail.png`.
The running console binary was deliberately not restarted: its process-local
evidence would be lost. New backend reason production requires loading the updated
binary after the owner finishes inspecting this session.

Focused tests cover typed transport errors, sanitized/correlated reasons, recovered
call handling, failed-detail/list colors, missing historical causes and successful
call/cleanup preservation. The connected browser suite has 12 passing tests.
Final `scripts/check.ps1 -All` passed: documentation/contracts, formatting/tidy/vet,
Go tests, integration compilation, UI types/build, 101 component tests and 44 browser
tests. Read-only inspection of the owner's retained failure confirmed the Failed
badge, explicit missing cause, separate agent outcome and zero ongoing animations.
