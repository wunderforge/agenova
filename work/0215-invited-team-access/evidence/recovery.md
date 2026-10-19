# Interrupted Work recovery evidence

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215).
- Extends the [durable foundation](durable-work.md); this is implementation and
  local-backend evidence, not a production/cloud release claim.

## Behavior

Installed member mode now acquires the PostgreSQL writer lease before constructing
its Work service and reconciles incomplete Work before opening admission. Reference
mode retains its explicitly selected process-local behavior.

Known allocations reattach through the Agent Sandbox adapter's cleanup-only
capability. The adapter checks the recorded claim/worker binding, never calls
Allocate or Start, and requires a confirmed stop before destructive cleanup. An
already absent claim AND worker prove release, not a worker-stop acknowledgement.
Changed bindings, API errors and orphan workers do not authorize deletion.

Recovery records a failed interrupted run with verified runtime release. An existing
terminal success retains its previously staged response; it never calls the model
again. Pending Work with no recorded backend identity remains blocked rather than
inventing allocation/release evidence. Previously recorded CleanupFailed evidence
also requires operator reconciliation; recovery does not overwrite it.

Missing invocation observations remain missing. The existing RunOutcome reason code
`run-recovered-interruption` explicitly identifies a reconciled interruption. CLI
accepts an uncompleted invocation only with that code and Failed/Expired/Cancelled
outcome; the code cannot validate a successful result. Portal distinguishes an
allowed but unattempted call (`Not started`) from an attempted call with no observed
completion (`Outcome unknown`). No synthetic ProviderOutcome is created.

## Focused evidence

With a disposable PostgreSQL URL in `AGENOVA_TEST_POSTGRES`:

```sh
GOTOOLCHAIN=go1.27.2 go test -race ./internal/console ./internal/runtime/agentsandbox ./internal/connectedclient ./internal/workstore ./cmd/agenova-control-plane
```

Passing behavioral checks cover:

- Failure before ProviderAttempt commit: zero model calls, restart does not replay.
- Lost ProviderOutcome commit: one observed synthetic provider call, recovery leaves
  the response unknown and does not call it again.
- Failed cleanup blocks finalization; a later verified cleanup completes recovery
  without duplicate lifecycle facts or a second allocation/start.
- Recovered evidence is accepted by the actual HTTPS CLI validator; an altered
  Succeeded outcome with uncompleted invocations is rejected.
- Unrecorded allocation identity stays blocked without stop/delete operations.
- Agent Sandbox adapter tests verify cleanup-only reattachment, stop-before-delete,
  changed-binding denial, orphan denial, API-read failure, and confirmed absence.

UI checks:

```sh
cd ui
npm test -- --run src/WorkerActivity.test.tsx
npm run build
npm run test:smoke -- --grep 'recovered interrupted'
```

The rendered browser case shows `Outcome unknown` and the interruption explanation,
with no green successful model result or active polling animation. Screenshot:
`.tmp/ui-smoke/connected-recovered-interr-dccb5--without-fabricated-success/recovered-work.png`.
The browser API is a synthetic fixture; SQL/CLI recovery is tested separately above.

## Remaining release gaps

Real Kubernetes interrupted-worker reconciliation is not yet backend-verified.
Unobserved allocations and previously failed cleanup need an operator reconciliation
procedure and acceptance evidence; they currently block admission. Retention and
storage ceilings, readiness behavior, migrations/rollback, least-privilege runtime
SQL permissions, costed RDS/hosting and real Google/OS-keychain/two-account acceptance
remain required. No cloud cutover or destructive infrastructure change occurred.
