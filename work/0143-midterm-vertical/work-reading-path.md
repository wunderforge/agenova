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
