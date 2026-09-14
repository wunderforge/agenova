# Task: Prove a real claim-to-worker run on kind

- Ticket: [#51](https://github.com/wunderforge/agenova/issues/51)
- Mission: Prove one actual Kubernetes SIGs Agent Sandbox v0.4.6 run with a tiny deterministic worker, including work start and verified stop/release.
- Target: `internal/runtime/agentsandbox`, a kind E2E harness, backend documentation.
- User value: Friday's vertical demo can use a real attributable worker rather than a simulated backend or Ready condition.
- PRD outcome: `docs/product/prd.md` §3 and final demo step 12.

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet, `internal/runtime/backend.go`, #66 report, #50 kind substrate, #89 filesystem contract.

## Scope and Acceptance

- Preserve claim/worker identity across Allocate, Observe Ready, Start, task result, Terminate, Cleanup.
- Start must acknowledge actual work; Terminate must prove stop/cancellation independently of deletion; Cleanup must confirm claim and worker absence.
- Show filesystem evidence only if a real inside/outside probe supports it; otherwise keep Unsupported.
- Test wrong identity, premature/duplicate Start, stale binding and unconfirmed release.
- Do not close full #51 on a partial Friday slice.
- Out of scope: #53 final demo agent, #31 application run service, policy/gateway/UI integration, production credentials, multi-tenant isolation claims.

## Execution Todo

- [x] Scout adapter, contract, PRD and kind spike.
- [x] Record Task/Spec/Design; owner waived separate approval for this sprint.
- [x] Implement smallest worker-control and E2E slice without changing neutral interface.
- [x] Run focused tests, kind E2E and `.\scripts\check.ps1 -All`.
- [ ] Review evidence, commit bounded changes, advance PR and comment on #51.

## Quality Gates and Evidence

- `go test ./internal/runtime/...`
- Pinned-version kind E2E with timeouts and cleanup trap.
- `.\scripts\check.ps1 -All`
- Record version, cluster, claim/worker IDs, Ready, task result, stop evidence, confirmed cleanup, exact commands/output.

## Constraints

Preserve `docs/product/architecture-contract.md` and RuntimeBackend. Use a tiny E2E worker tonight rather than wait for #53 or Sonia's #31. Never equate Pod Ready with Running or a delete request with stop/release.
