# React state motion

Owner approved the interactive HTML study and requested the React port on 15 September.
This is a visual follow-up to #143 under the internal demo coordination #107.

## Plan

- Ocean canvas #091b26, glass #112b37, ink #eaf5f8, muted #a3bbc7,
  mint #70e7d0 and transport blue #92b7ff. Rose means denial/failure.
- Bahnschrift headings, Segoe UI Variable body; retain current left-aligned navigation,
  work list, evidence pages and source controls. No appearance toggles or Replay.
- One distinctive run-flow strip; all other effects respond quietly to state or input.
- Motion is presentation only: no timers inventing Connected progress, no replayed
  provider results, no simulated streaming of a response returned as one completed value.
- Model permission is not provider completion. Task outcome is not cleanup confirmation.
  Missing stages remain unrecorded, including optional model use and pre-allocation denial.
- Running transports animate; terminal lights breathe gently. Failed/Denied stop rings
  are one-time. Native scroll, pointer light, click feedback and disclosures stay accessible.
- React owns stable keyed records. Scroll/pointer frames use refs without poll-time state
  churn; listeners and effects clean up; OS reduced motion stops decorative animation.

## Review against brief

Keep the accepted dark prototype, but do not copy its timed mock lifecycle into the
product. Derive each stage from existing facts and authoritative state. Preserve current
Demo/Connected controls: these are data controls, not the rejected appearance toggles.
No monitoring console, new product object or backend contract change.

## Gates

Focused flow derivation tests; existing UI unit and browser suite; motion-specific browser
checks for state, source isolation, keyboard, mobile, stable DOM/scroll and reduced motion;
representative screenshots; repository baseline. Live display may read existing evidence
without starting a new task or model call.

## Verified result (15 September)

- `npm test -- --run`: 81 unit tests passed, including nine flow-state cases.
- `npm run build`: canonical bindings, TypeScript and production bundle passed.
- `npm run test:smoke`: 40 browser tests passed. Added motion coverage and a Connected
  polling case that preserves flow DOM and scroll, and keeps missing cleanup visible.
- `./scripts/check.ps1 -All`: passed with normal Git access. An initial sandbox-only
  VCS error and two frontend type issues were fixed/rechecked without weakening gates.
- Rendered desktop/mobile, Running/Denied/Failed and source-isolation screenshots are
  generated in `.tmp/ui-smoke`. Visual review corrected particle travel distances to
  the actual connector width rather than viewport-dependent container units.
- Read-only inspection of existing live request
  `work-78180ebf-970f-4df7-b701-c43a3f51ba40` at port 5175 shows the actual llama response,
  all six recorded stages, inactive terminal authority and confirmed cleanup.
  This inspection made no new submission/provider call; screenshot: `.tmp/react-motion-live.png`.
- Changes remain on `codex/portal-state-motion` for owner visual review; no PR or merge
  is performed as part of this visual-preview request.
