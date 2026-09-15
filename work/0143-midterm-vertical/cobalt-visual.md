# Cobalt signal: visual refinement

Owner brief: keep the approved layout, reduce bright green, add a convincing dark
gaming-desktop feel without decorative motion, information noise or performance loss.
UI-only elaboration of #143; no new backend behavior, inference or product scope.

## Plan and brief review

- Base tokens: midnight `#050914`, raised navy `#0b1224`, cool ink `#e1e9fa`,
  slate `#94a4c0`, electric blue `#629dff`, ice cyan `#76d9f4`.
  Semantic success `#8cd5b2`, exception `#ff8ca4`, pending `#ffd09c`.
- Bahnschrift headings / Segoe UI body; existing left-aligned reading path unchanged.
- Navigation/selected disclosures use a blue edge and short illumination feedback.
  Blue means interaction/current execution, not permission. Green only means allowed/
  succeeded. Red shows recorded failures/denials, not general gateway health.
- Memorable focus: the actual waiting model/tool call, with a localized luminous rail,
  inset light and existing single breathing indicator. Model blue, tool cyan; labels
  remain explicit. Lifecycle/history stay dimmer and static; outcome gets a quiet blue edge.
- Hover/focus/press reveal a pre-painted light layer. No mouse tracking, particles,
  blur filters, animated shadows, layout animation or background scanning. Reduced
  motion remains enforced. No new page-load choreography or completion replay.

Brief review: reject a uniform neon card grid or full-screen RGB show. Keep the
existing hierarchy; lights encode where to click, what is waiting and where something
failed. A restrained blue/cyan split is useful here, not a vendor-coded palette.

## Research and validation

[web.dev animation guide](https://web.dev/articles/animations-guide) recommends
transform/opacity animation, avoiding layout/paint work and measuring actual rendering;
shadows/blur are expensive to repaint. Therefore glows are static, light overlays
transition opacity, and the waiting lamp is the only ongoing animation.
[MDN reduced motion](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/prefers-reduced-motion)
informs the existing system-preference fallback; no extra toggle needed.

- [x] Palette/state/interaction implementation, preserving reading order
- [x] Deterministic waiting/failed/denied/success/mobile screenshots and assertions
- [x] Before/after performance, focused/full gates and screenshot critique

Before diagnostic: `node scripts/profile-motion.mjs cobalt-before`, headless Chromium,
1440px, fixed three-second pointer/scroll workloads: p95 16.7/16.8ms, zero >25ms samples,
zero layout work. These are local diagnostics, not a guarantee for every user device.

## Verified result

- `scripts/check.ps1 -All` passed: Go/boundary/documentation gates, UI contracts/types/
  build, 101 component tests and 42 browser tests. No new dependencies or backend changes.
- Focused Cobalt browser check passed with 4x CPU throttling: model/tool waiting identity,
  hover overlay, explicit green success, red failure and exactly one ongoing opacity-only
  animation while waiting; zero ongoing animations after failure. Frame samples are
  diagnostic only, not a CI frame-rate threshold.
- Final `node scripts/profile-motion.mjs cobalt-final`: pointer/scroll p95 16.8/16.7ms,
  zero >25ms samples and zero layout work. Scroll trace: nine Paint events, 1.79ms total,
  compared with nine / 1.66ms before. No material regression in this bounded local workload;
  sustained real-device performance still needs owner testing.
- Final screenshot critique kept history/lifecycle quiet, put blue illumination only on
  navigation, the outcome edge and the actual waiting call, and removed green from generic
  status/selection. Failure row retains a localized red rail; no background scanning.
- Screenshot proof: `.tmp/ui-smoke/` Cobalt model/tool/failure images plus motion denial
  and mobile images; actual retained completed work and mobile: `.tmp/work-reading-path/`.
  Before/final runtime reports: `.tmp/ui-motion-performance/cobalt-before.json` and
  `cobalt-final.json`. Current six-turn real result was inspected read-only; no inference
  or task submission was triggered by this visual work.
