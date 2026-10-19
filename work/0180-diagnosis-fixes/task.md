# Task: Diagnosis correctness for retained records and pre-resolution gateway denials

- Ticket: [#180](https://github.com/wunderforge/agenova/issues/180)
- Mission: Fix four defects that make the console's own diagnosis untrue — a capacity message that names a remedy the code cannot deliver, refused submissions that keep their capacity slot, an unrecoverable nil dereference on the run goroutine, and gateway denials raised before claim resolution that leave no evidence at all.
- Target: `internal/console/service.go`, `internal/modelgateway/gateway.go`, `internal/toolgateway/gateway.go`, with focused tests in the same packages. No contract, schema, storage or UI change.
- User value: An operator who hits the record cap is told what actually frees it; a refused submission no longer silently consumes a capacity slot and shadows its own reference until restart; a missing record can no longer take down the console and every other request's evidence with it; and a denial raised before the claim is resolved — missing claim identity, reserved credential key, unknown claim — leaves a decision fact instead of disappearing.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md) — governed denials must be observable rather than silent. This packet delivers **no** durable storage, telemetry or authorization capability; those remain Epic #180 work tracked separately.

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [GitHub Epic #180](https://github.com/wunderforge/agenova/issues/180) — the outcome responsibility this packet contributes to, and the current-state verification comment that found these four defects
- `internal/console/service.go` — `Submit` reservation lifecycle, the record cap, and the `run` outcome write
- `internal/facts/journal.go` — the attribution invariants an appended fact must satisfy
- `internal/modelgateway/gateway.go`, `internal/toolgateway/gateway.go` — `validate` / `resolveClaim` denial paths and the `record` sink
- `internal/facts/store.go` — the claim-keyed invocation store that must not receive an unverified claim ID

## Scope

In scope:

- Replace the `ErrCapacity` message with one that names the only remedy the code actually provides.
- Release a submission's placeholder record on every failure path, decided by pointer identity so that terminal denial and template-configuration records — which intentionally replace the placeholder — survive.
- Guard the run-outcome write against a missing record so a nil dereference cannot panic the run goroutine.
- Route pre-resolution gateway denials through a new `notify` helper that reports to the trusted observer without writing the claim-attributed fact store.
- Document, at the two `_, _ = s.journal.Append(...)` call sites, why they are best-effort by construction rather than by oversight.

Out of scope:

- Durable history, pagination, viewer authorization, OpenTelemetry, platform health aggregation — the rest of Epic #180.
- Changing the record cap itself, evicting records, or altering what is retained.
- Any change to `internal/evidence` types, generated UI bindings, or the fact vocabulary.
- The nil-receiver "gateway is unavailable" branch: it has no observer, no store and no ID source, so its `Decision{}` is structurally unrecordable.
- Portal-side failure presentation, which belongs to #60.

## Acceptance Criteria

- The capacity diagnostic names restart as the remedy and no longer states that waiting for active work frees capacity.
- A submission refused after its request is registered leaves no capacity slot, no queryable record, and no reusable-reference conflict; 40 consecutive refusals never surface `ErrCapacity`.
- A terminal denial record and a template-configuration-failure record remain queryable and keep their reference reserved.
- The run-outcome write tolerates a missing record: the goroutine finishes, `Close` returns, and the record is not recreated.
- A gateway denial raised before claim resolution reaches the observer carrying its invocation ID, in both gateways.

## Negative Case

- Pre-resolution denials must not write `facts.Store` for the requested claim ID or for the empty string — no fabricated claim-attributed invocation.
- A denial whose record cannot be written must fail closed: `Invoke` returns an error and the provider or tool adapter is never called.
- The nil guard must skip, not resurrect: a removed record stays absent from `QueryRequest` and `List`.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [x] Apply the four fixes as separate commits.
- [x] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/console ./internal/modelgateway ./internal/toolgateway ./internal/facts -count=1`
- `go build ./... && go vet ./...`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Each new test verified to fail before its fix and pass after it, with the pre-fix failure recorded in the PR body — including that the nil-guard test crashes the whole `internal/console` test binary before the fix rather than failing an assertion.
- Full `go test ./...` output after the change.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Keep the diff reviewable: no refactors, no new dependencies, no behaviour change on paths that are already correct.
- Any fact appended for a denial must satisfy `facts.Journal.Append` attribution rules; never attribute evidence to an unverified claim ID.

## Decisions and Blockers

- **Both `_, _ = s.journal.Append(...)` call sites stay best-effort.** Both run after `RegisterRequest` and `BindClaim` have succeeded with facts that carry no `Decision`, `Authority`, `BackendIdentity`, `InvocationID` or `Result`, so every rejection in `facts.Journal.Append` is unreachable; the surrounding decisions are already fail-closed. Converting them into hard failures would create a contradictory view (`Phase=Succeeded` with `Outcome.Status=Failed`) on a provably unreachable branch. Observability for these sites is Epic #180 follow-up work, not this packet.
- **Pre-resolution denials notify the observer but do not write the store.** The console observer attributes a decision to the run that issued the request, not to `req.ClaimID`, so notifying it is correct; `facts.Store` is keyed by claim ID and would otherwise record an orphan entry under an unverified or empty claim.
- **Reservation release keys on pointer identity, not a boolean flag**, so records that intentionally replace the placeholder survive, and the release happens before `wg.Done()` so `Close` never returns with a reservation still in the map.
- Blocker: `scripts/check.ps1` requires PowerShell, which is not installed on this machine; the Go gates above were run locally and CI owns the full gate.
