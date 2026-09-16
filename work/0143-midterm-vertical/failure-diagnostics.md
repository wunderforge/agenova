# Failure diagnostics and UI boundaries — 16 September

Owner follow-up: move the waiting lamp onto each actual call, remove call-label
underlines, fix the unexplained failed investigation and separate UI logic.

## Observed cause

The old console binary did not contain the recent failure-record fix.
Its retained run had six successful model calls, three successful mock reads
and successful cleanup, but no recorded explanation of the agent's exit.
Export: `.tmp/retained-failure-564801e1.json`; not a live data source.

A rebuilt diagnostic console reproduced the same synthetic task on kind:
three reads, then three invalid tool/finish action responses, exhausted six turns.
The actual run `diagnostic-repro-0916` recorded
`agent-invalid-action-limit` with a matching outcome reason and clean teardown.
Its browser failure page and record drill-down passed the read-only gate.

## Implementation boundary

- Keep strict action/result validation. Do not accept malformed answers.
- Choose a finish-only output schema when the trusted demo edge observes all
  three artifacts read, or the final model turn. This mirrors the worker's
  finishing phase; it does not widen tools, scopes, profiles or authority.
- Per-call output schema is a bounded private provider option, not a worker
  permission. Claim, gateways and RuntimeBackend remain unchanged.
- Record safe action-validation issues with turn and invocation IDs. Do not
  store raw model output or reasoning as diagnostic facts.
- Preserve successful call evidence. Format retries are separate amber checks;
  final failure is a distinct red outcome. A retry is not an extra tool/model call.

## UI structure

| Module | Owns |
| --- | --- |
| `connected-source.ts` | HTTP, timeout and canonical response validation |
| `useConnection.ts` | polling, cancellation, terminal/cleanup and route guards |
| `worker-activity-model.ts` | pure invocation correlation and turn grouping |
| `record-presentation.ts` | pure category/status/reason projections |
| `WorkerActivity.tsx`, page components | rendering and disclosure interaction |

Pure projections depend on canonical facts, not Kubernetes or Ollama.
Demo and Connected stay explicit, isolated sources. Correlation uses indexed
lookups rather than repeatedly reversing/scanning the entire event collection.

The lamp lives to the left of each call; only an unresolved correlated call
on a Running claim animates opacity. Terminal calls and format checks are static.
Keyboard focus and reduced-motion support remain intact.

## Gates and evidence

- Go: protocol exhaustion, sanitized action issues, per-call schema isolation,
  invalid schema makes zero HTTP calls, final outcome provenance.
- UI: pure retry projections, active lamp placement, no underline, one opacity
  animation, terminal stop, polling/navigation and existing regression suite.
- Real failed-run drill-down (read-only):
  `node harness/e2e/ui-failure-diagnostics.mjs --base-url http://127.0.0.1:5175 --request-ref diagnostic-repro-0916`.
  Passed; screenshot evidence in `.tmp/failure-diagnostics/`.
  Requires the process retaining that run; restarting does not migrate it.
- Actual rebuilt console + worker:
  `node harness/e2e/ui-kind-checkpoint.mjs --live-model --base-url http://127.0.0.1:5175`.
  Passed: `work-a17f5827-ec51-4c76-b978-446f8f549059`, worker
  `agenova-pool-reference-engineer-pool-rvrh9`, real `llama3.1:latest`,
  48 correlated facts, final answer and confirmed cleanup. Requires current
  action diagnostics, so an older binary cannot silently pass.
- Repository baseline: `./scripts/check.ps1 -All` passed, including Go/vet,
  contracts, production build, 109 frontend unit tests and 48 browser tests.

No production identity, persistent history, real Git tools or coding framework
has been added. Successful execution is not proof that every model conclusion
is correct; model-quality evaluation remains a separate concern.
