# Task: Shared member identity for E14 Work

- Ticket: [#203](https://github.com/wunderforge/agenova/issues/203)
- Mission: Reconcile E14 M2 with the current invited-member service while preserving claim-scoped authorization and owner-only evidence.
- User decision (20 October 2026): reuse main's member service; do not merge the older standalone `internal/identity`/OIDC implementation.
- PRD: [invite-only production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension) and [trusted-principal submission](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution).

## Context to Read

Read `AGENTS.md`, `docs/product/prd.md`, this packet, [#203](https://github.com/wunderforge/agenova/issues/203), `spec.md`, `design.md`, the [architecture contract](../../docs/product/architecture-contract.md), `internal/authentication`, `internal/console`, `internal/connectedclient`, and [#215 member evidence](../0215-invited-team-access/evidence/acceptance-20261019.md).

## Scope and Negative Behavior

The current main service already verifies Google identity during login, issues revocable application sessions, resolves current membership on each operation, protects connected CLI/Portal transport, and filters evidence by owner. E14 M2 must prove that this existing boundary supplies the principal to M1 Policy and never accepts a caller-authored team or subject. Invalid identity and cross-owner references must stop before worker/provider effects. Keep Policy and AgentTemplate management behind operator authorization.

The Owner-approved #215 Google/member path replaces #203's original direct company JWT/JWKS bearer and CLI token-file transport requirements. Google identity is verified at login; the service uses revocable application sessions and native OS credential storage. The named authentication, identity-forgery, isolation, and zero-side-effect outcomes remain required. The explicitly selected fixed-principal local mode remains separate and cannot be a fallback from authenticated mode.

## Acceptance and Evidence

- [x] One service accepts two distinct verified members and attributes Work/evidence to the correct subject/team in a focused integration test.
- [x] Caller-authored team header does not override membership; cross-owner evidence is hidden; invalid identity has zero worker/provider side effects.
- [x] Reconcile #203 transport requirements with the Owner-approved Google/member design in the Ticket and PR review.
- [ ] Record a real installed two-member Google login and Work/evidence transcript, including CLI/API/Portal parity and named failure cases. Synthetic test identities do not satisfy this.
- [x] Run focused and full gates on the reconciled branch; review diff and report residual risks. Focused `go test -count=1 ./internal/console -run TestE14TwoMembersShareOnePolicyService` passed; `./scripts/check.ps1 -All` passed, including 56/56 browser smoke.

## Gates and Dependencies

`go test -count=1 ./internal/console -run TestE14TwoMembersShareOnePolicyService` and `./scripts/check.ps1 -All`. M1 PR #207 merged as `1b3cd68`. This branch remains a draft until installed two-member acceptance is complete.

## Decisions and Blockers

- 20 October 2026: User selected the existing main member service over the older standalone E14 OIDC stack. No duplicate verifier is brought into main.
- M1 #207 merged on 20 October 2026. #203 now records the Google/member session transport decision.
- Real installed two-member Google acceptance remains open. Do not mark M2 accepted based on the focused synthetic test.
