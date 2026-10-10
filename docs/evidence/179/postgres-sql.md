# E17 Real PostgreSQL SQL Campaign

Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Source baseline: `a249a21c31b94372f61783b0dfc9195805e97cf2` plus the
[tenant-privilege correction](column-privilege-review.md) and SQL test.

## Reproduction

The selected gate requires Docker and a pre-pulled official image digest.
It fails when the explicit image is missing or malformed; it never silently
skips a selected real-backend check or downloads an unpinned substitute.

```sh
docker pull postgres@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336
AGENOVA_MEMORY_POSTGRES_IMAGE='postgres@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336' \
  go test -tags memorypostgres -v -race -count=3 -timeout=3m \
  ./internal/memory/postgres -run '^TestPostgresSQLIntegration$'
```

The [test](../../../internal/memory/postgres/sql_integration_test.go) creates a
randomly named, ticket-labelled container with network `none`, no published
ports and socket-only PostgreSQL. All data and roles are synthetic. It waits
for the final PostgreSQL process rather than the entrypoint's temporary server,
then executes the actual production migration and readiness SQL via local psql.
The migration owner and application role are separate; the application role
has only schema USAGE, marker SELECT and tenant-table SELECT/INSERT.
Session authorization is switched by the test operator; no actual external
credential is read or provided.

Each run removes only the container ID it created and that container's anonymous
volume, including on assertion failure. Cleanup does not name an existing
database, Kubernetes namespace or PVC. No existing cluster resources are changed.

## Executed Evidence

Three fresh race-enabled campaigns passed on macOS arm64, Docker 29.7.2,
Go 1.27.1 and PostgreSQL `18.6 (Debian 18.6-1.pgdg13+2)`.

- Safe application privileges make all five production readiness flags true.
- Direct records/receipts, inherited-role and PUBLIC column UPDATE grants make
  tenant readiness false even when table UPDATE is false. Revocation restores
  readiness. Column INSERT/UPDATE on the marker is also rejected.
- The real migrated RLS rules hide records and receipts without ownership
  settings, across teams, across projects and across logical scopes. A foreign
  namespace INSERT is denied with a successful authorized INSERT as control.
- One psql session commits Team A data, then immediately sees zero rows after
  transaction-local settings clear; subsequent foreign-namespace transactions
  also see zero. This proves database-session reset, not Go pool reuse.
- Literal case-insensitive matching treats `%` and `_` as characters, not
  wildcard operators. Duplicate receipt keys and orphan receipt references are
  rejected. A read-only transaction rejects INSERT.
- Restarting the same database container preserves its identified volume;
  production readiness still passes and the authorized row/receipt pair remains.
- Selected execution without the image fails before any container creation.

Captured [passing campaign](../../../work/0179-scoped-memory/postgres-sql-final-macos.log),
[pre-fix privilege reproduction](../../../work/0179-scoped-memory/postgres-column-before-macos.log)
and [missing-image rejection](../../../work/0179-scoped-memory/postgres-sql-missing-image-macos.log)
contain test names, image/server metadata and sanitized assertion results,
without SQL text, retrieved bodies, query text, credentials or raw server errors.

`go vet -tags memorypostgres ./internal/memory/postgres` and
[20 repeated readiness runs](../../../work/0179-scoped-memory/review-column-repeated-macos.log)
passed. The post-SQL `pwsh -NoProfile -File ./scripts/check.ps1 -All`
[campaign](../../../work/0179-scoped-memory/postgres-sql-all-macos.log) passed
all Go tests, generated contracts, 189 frontend tests, production build and 59
browser cases. Only documentation/publication edits followed that source run.

## Acceptance Limits

This is executed SQL/schema proof. It does not exercise a Go driver or host
connection pool, an accepted #155 credential consumer, installed Platform or
controlled-worker wiring, lost COMMIT acknowledgement, backend-side invocation
counts, model use of retrieved data, worker network controls, database Pod/PVC
restart, Control Plane restart or live CLI/API/Portal parity. It does not satisfy
the independent Work A/B/restart/C campaign or teammate reproduction.

The existing Mac kind context and local-path storage remove this machine's
earlier no-context blocker. Its CNI/policy inventory still needs actual positive
and negative network probes. #155/#213 and the shared producers remain
unaccepted; a supported reviewed driver/host composition is still required.
Keep #212 draft and #179 open.
