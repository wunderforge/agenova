# Durable Work foundation evidence

- Ticket: [#215](https://github.com/wunderforge/agenova/issues/215).
- Scope: private PostgreSQL evidence adapter, application commit boundaries and
  authenticated HTTPS client. This is not a production deployment or a claim of
  complete runtime crash recovery.

## Implemented boundaries

Migration 005 adds Work ownership/input-digest records and append-only facts. The
runtime does not migrate its own database. A schema-scoped PostgreSQL session
advisory lock permits one writer; every write uses that same dedicated connection.
Connection loss never silently acquires a new lease. Read access uses current
membership/project grants in the database predicate.

The authenticated service maps the caller's task name to an opaque member-scoped
reference. Same member/name/content returns the committed Work; changed content
conflicts. Different members can use the same name without sharing evidence.
ClaimRequest has no new identity/provider fields. The HTTPS CLI follows the returned
reference only after matching the submitted task specification, and retains its
existing strict evidence validation.

Admission and its initial facts commit before runtime dispatch. Governed facts
commit before publication/side effects; lifecycle facts and snapshots commit
atomically. Final response data is staged before the terminal transition. Finishing
requires a matching durable RunOutcome and unchanged request/authority binding.
Limits currently admit 32 incomplete and 1,000 retained Work records per team.

A restart can repair a lost final-publication acknowledgement from an already
recorded terminal outcome and staged success result. It performs no provider or
runtime operation. Any other unfinished Work prevents admission with
`ErrRecoveryRequired`; it is NEVER automatically dispatched again. This protective
block is not a completed operator reconciliation implementation.

## Reproducible focused checks

Use only a disposable PostgreSQL database and set `AGENOVA_TEST_POSTGRES` to it.
Each SQL test creates and removes its own schema. The writer-disconnection test
terminates only the connection holding that schema's dedicated test lock.

```sh
GOTOOLCHAIN=go1.27.2 go test -race ./internal/console ./internal/workstore ./internal/connectedclient ./internal/facts
```

Observed passing cases include:

- Actual PostgreSQL admission, evidence and session records with native client
  submission/polling over a local TLS HTTP server; synthetic runtime/provider.
- Completed Work and claim lookup after rebuilding the service; duplicate submission
  does not increase the provider counter, changed input conflicts, foreign read denies.
- Concurrent same-input reservations commit exactly once; member/project visibility.
- Second writer rejected; terminating the exact writer connection cannot reconnect
  and append, and failed writes do not advance the durable fact sequence.
- Failed ProviderAttempt persistence causes zero provider calls. Subsequent storage
  failure leaves incomplete evidence; restart refuses to replay it.
- Lost final publication repairs on restart with the original response and no second
  provider invocation. A terminal result never revives active claim authority.

## Remaining release gates

The installed production composition has not enabled this adapter yet. In-flight
runtime reconciliation, operator recovery, storage/retention limits and readiness
integration must be completed before enabling it. No running EKS/Bedrock resources
were used by these tests. Managed RDS, production least-privilege database roles,
real Google/two-account login and native OS credential-store acceptance are still
separate unfinished release gates. No production cutover occurred.

Full repository gate also passed with the disposable PostgreSQL environment:
`pwsh -NoProfile -File scripts/check.ps1 -All` completed Go formatting/module/vet/unit,
frontend contracts/types/components/build and all 52 browser smoke tests. A docs-only
rerun covers this evidence update. This gate does not exercise a live cloud backend.
