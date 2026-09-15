# Technical Design: Real kind runtime proof

- Ticket: [#51](https://github.com/wunderforge/agenova/issues/51)
- Feature spec: `spec.md`

The adapter already creates upstream SandboxTemplates, warm pools and claims, reads bound sandbox name/Ready, and confirms deletion for Cleanup. Start/Terminate currently return ErrUnsupported. The controller launches a Pod before task assignment, so Pod Running/Ready is not a task-start signal.

Keep neutral RuntimeBackend fixed. Probe v0.4.6 Pod control first; prefer a tiny adapter-owned start/stop acknowledgement protocol inside the bound worker over interpreting controller conditions. If no safe channel is proven, leave Start/Terminate unsupported and report the exact gap. The test worker and kind script are test-only. #31 later owns application phases/outcome. Reject Pod Ready as Start, delete as Terminate, or waiting for #53's general agent. Run focused identity/state/error tests and a timed kind E2E showing real claim, worker, task output, stop and absence, then baseline checks. Pod exec permissions, async processes, warm-pool reuse and volatile allocation state are explicit risks.
