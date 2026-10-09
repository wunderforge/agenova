# E17 Original Authentication Identity

Date: 10 October 2026 (Australia/Sydney).
Ticket: [#179](https://github.com/wunderforge/agenova/issues/179),
[task packet](../../../work/0179-scoped-memory/task.md), draft
[PR #212](https://github.com/wunderforge/agenova/pull/212).
Baseline: `5a9167355e9b6a9c819dbb7ed514c71d741452b4`, passed CI run 442.
Finding: [authenticated identity versus session authorization](https://github.com/wunderforge/agenova/pull/212#discussion_r4235436330).

## Reference Configuration

The existing current_user/session_user equality cannot establish the original
login. A privileged connection can replace both with SET SESSION AUTHORIZATION
and later reset to its original superuser authority. PostgreSQL's
[system_user](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-INFO-SESSION-TABLE)
reports the authentication method and identity; trust authentication yields NULL.

The reference adapter now additionally requires
`COALESCE(system_user = 'scram-sha-256:' || current_user, false)`.
Only SCRAM authentication directly as the checked data-only role is supported.
NULL, another method or another original identity fails capability readiness.
This conservative adapter restriction does not change shared authority or add a
credential interface. The existing single bounded read-only query retains five
result flags and performs no grant or configuration repair.

## Reproduction and Real Controls

Before changing production SQL, the revised
[live fixture](../../../internal/memory/postgres/sql_integration_test.go) ran
against the exact baseline query. All three new negatives failed in
[captured output](../../../work/0179-scoped-memory/review-auth-old-macos.log):

- A trust-authenticated superuser masks both session/current roles as memory_app.
- A SCRAM-authenticated superuser masks both roles as memory_app while system_user
  continues to identify the privileged login.
- A directly connected, data-only trust login lacks original authentication proof.

For both masked superusers, independent statements prove equality after masking,
original authentication identity, and successful reset to SUPERUSER. The corrected
query rejects each while actual memory_app SCRAM logins pass. Native hidden-role
SET/TRUNCATE controls now also start from a directly authenticated memory_app
connection, with transaction rollback preserving the fixture data.

Fresh synthetic passwords are generated per owned container. They travel through
private stdin and a restrictive container-local password file, never command-line
arguments, test output or published artifacts. Group-role privilege oracles may
still use operator session authorization; they are not readiness positive controls.
Each campaign owns a network-isolated digest-pinned PostgreSQL 18.6 container and
anonymous volume, and removes both by the ID it created.

## Verification and Limits

[Focused/race/repeated output](../../../work/0179-scoped-memory/review-auth-focused-macos.log)
records Memory, facts, lifecycle and connected-reader package checks, 20 race-enabled
readiness repetitions and SQL-tag vet. The
[real SQL campaign](../../../work/0179-scoped-memory/review-auth-postgres-macos.log)
records three fresh race-enabled full PostgreSQL runs, retaining role/delegation,
RLS, transaction-local context, receipt and database restart controls. The
[repository gate](../../../work/0179-scoped-memory/review-auth-all-macos.log) and
[publication checks](../../../work/0179-scoped-memory/review-auth-publication-macos.log)
record the final source and documentation checks.

This is executed authentication/SQL/schema proof through psql, not a Go-driver
pool, accepted host credential consumer, installed Memory or worker-network proof.
No existing database, cluster, Secret or PVC changed. #155/#213, #192 and #207
remain unmerged. Installed A/B/restart/C, independent backend/network/data-use
oracles, live CLI/API/Portal parity and team lead/Sonia review/reproduction remain
outstanding. Keep #212 draft and #179 open. Published commit CI and automatic
re-review are separate follow-up evidence.
