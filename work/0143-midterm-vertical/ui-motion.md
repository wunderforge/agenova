# React state motion

Owner approved the interactive HTML study and requested the React port on 15 September.
This is a visual follow-up to #143 under the internal demo coordination #107.

## Plan

- Superseding owner correction: Run Flow is a static lifecycle summary, not an animated
  model/tool loop. Add Worker activity with correlated actual provider attempts/outcomes
  and atomic tool permission decisions. Only an unresolved model attempt in a Running
  claim gets a small animated indicator; terminal/stale/missing evidence never does.
- Remove pointer-following gradients, backdrop blur, animated shadows, mass arrival fades
  and background badge loops. Use simple hover borders and one small opacity indicator.
- Measure the same pointer and scroll workloads before/after; protect input/polling UX.
- ToolDecision proves permission only, not actual execution duration. No loop/iteration
  traces currently exist: do not invent them or add backend instrumentation in this UI fix.
- Ocean canvas #091b26, glass #112b37, ink #eaf5f8, muted #a3bbc7,
  mint #70e7d0 and transport blue #92b7ff. Rose means denial/failure.
- Bahnschrift headings, Segoe UI Variable body; retain current left-aligned navigation,
  work list, evidence pages and source controls. No appearance toggles or Replay.
- A static lifecycle strip plus a worker-call panel. One small opacity lamp is the only
  continuous animation, and only while a correlated provider call is outstanding.
- Motion is presentation only: no timers inventing Connected progress, no replayed
  provider results, no simulated streaming of a response returned as one completed value.
- Model permission is not provider completion. Task outcome is not cleanup confirmation.
  Missing stages remain unrecorded, including optional model use and pre-allocation denial.
- No rail transports, terminal loops, mouse-position lighting or animated shadows.
  Native hover/press/disclosure feedback and a transform-only scroll indicator remain.
- React owns stable keyed records. Scroll frames use refs and cached extent without
  poll-time state churn; listeners clean up; OS reduced motion stops the worker lamp.

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

## Initial port verification (15 September, before owner performance correction)

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

## Performance correction verification (15 September)

- Run Flow now has five static lifecycle stages. Model requests belong to Worker activity.
  Each provider attempt is matched to its own invocation outcome; interleaved calls,
  failed/cancelled outcomes, terminal runs and missing correlation are explicitly tested.
- Tool/Model access decisions remain static permission records, never fake execution.
  No internal agent loop/iteration facts currently exist; that is an instrumentation gap,
  not something animation can infer. This correction does not expand backend scope.
- `npm test -- --run`: 97 tests passed. `npm run build` and all 40 browser tests passed,
  including one-lamp budget, reduced motion, live polling DOM/scroll preservation,
  mobile layout and no fallback to demo evidence.
- From `ui/`, run `node scripts/profile-motion.mjs before` or `after`:
  same headless Chromium viewport and 3-second synthetic pointer/scroll workloads.
  Pointer Paint events: 362 → 0; TaskDuration: 282 → 51 ms (~82% less).
  Scroll Paint events: 128 → 0; TaskDuration: 241 → 113 ms (~53% less).
  Both baseline and corrected runs had no >25 ms frame intervals; this benchmark
  demonstrates reduced repaint/CPU work, not proof of FPS on the owner's device.
  Reports are generated in `.tmp/ui-motion-performance/{before,after}.json`.
- Browser screenshots include `connected-worker-waiting.png` and corrected desktop,
  denied/failed/mobile cases in `.tmp/ui-smoke`. Visual review fixed a section-reset
  specificity conflict so the Worker activity panel keeps its spacing and border.
- `./scripts/check.ps1 -All`: complete repository baseline passed with normal Git access.
  Read-only live inspection also shows the real completed model call as Succeeded,
  with zero looping animations and no new submission/provider call.
