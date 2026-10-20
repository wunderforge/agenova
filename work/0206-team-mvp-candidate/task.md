# Task: E14 team candidate on the member-service path

- Ticket: [#206](https://github.com/wunderforge/agenova/issues/206)
- Parent: [E14 #176](https://github.com/wunderforge/agenova/issues/176)
- Mission: Prove one installed claim-scoped governance path for two real members, with reproducible evidence and bounded cleanup.
- User decision (20 October 2026): reuse main's Google/member service, not the older direct-JWT verifier.
- PRD outcome: [trusted principal](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [claim authority](../../docs/product/prd.md#4-claim-scoped-authority), and [invite-only extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension).

## Context to Read

Read `AGENTS.md`, `docs/product/prd.md`, this packet, [#206](https://github.com/wunderforge/agenova/issues/206), [architecture contract](../../docs/product/architecture-contract.md), [spec](spec.md), [design](design.md), [runbook](RUNBOOK.md), [release check](RELEASE.md), and the M1–M4 linked task packets/PRs. The current [member-service guide](../../deploy/production/member-service.md) owns its installation path.

## Scope and Negative Behavior

Package the current member-service acceptance path, a canonical Work fixture and the host-token provider boundary test. Invalid identity, over-scope requests, cross-owner reads, cross-claim and post-terminal Gateway calls must fail before side effects. No direct-JWT fixture, browser token file, IAM permission or backend-specific claim field is introduced.

## Execution Todo

- [x] Review old M5 package against the selected main member service; remove obsolete direct-JWT/JWKS/Secret/token-file instructions and fixture.
- [x] Retain and run the host-token provider boundary test.
- [ ] Reproduce the integrated installed path with two real Google members, equivalent Work and CLI/API/Portal parity.
- [x] Re-run live installed worker cross-claim/post-terminal checks on this integrated branch: two model and one tool call; zero additional adapter calls on cross-claim/terminal denial, confirmed cleanup on kind.
- [ ] Obtain a second teammate's independent clean-environment run.
- [x] Run focused and full gates on the candidate branch; review diff and record exact evidence. Installed real-user acceptance remains open.

## Gates and Evidence

`go test -count=1 ./internal/console -run TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow` and `./scripts/check.ps1 -All`; installed two-user and worker tests from [RUNBOOK.md](RUNBOOK.md). Record exact revision, versions, commands, redacted outcomes, zero-side-effect counts and screenshot paths outside Git. No claim of real-user, kind or AWS acceptance without actual output.

## Decisions and Blockers

- M1 #207 and the consolidated M2–M5 PR #231 merged on 20 October 2026. Merge establishes the code baseline, not M5 acceptance. Prior segment PRs #228–#230 were closed as superseded. Real two-member installed evidence and independent reproduction remain open.
- The old M5 runbook targeted standalone JWT/JWKS plus `AGENOVA_TOKEN_FILE`, which conflicts with the Owner's selected current Google/member service. It was replaced rather than merged mechanically.
- Two real approved Google members, a clean member-service installation path and a second teammate's independent run are not yet recorded. M5 remains draft until these are available.

- 2026-10-20: The M4 kind worker smoke passed on the integrated stack using a unique image built from this branch. This proves worker-to-Gateway enforcement and cleanup only; real Google dual-user and independent clean-environment acceptance remain pending.
