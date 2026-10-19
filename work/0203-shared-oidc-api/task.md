# Task: Shared member identity for E14 Work

- Ticket: [#203](https://github.com/wunderforge/agenova/issues/203)
- Mission: Reconcile E14 M2 with the current invited-member service while preserving claim-scoped authorization and owner-only evidence.
- User decision (20 October 2026): reuse main's member service; do not merge the older standalone `internal/identity`/OIDC implementation.
- PRD: [invite-only production extension](../../docs/product/prd.md#owner-approved-invite-only-production-extension) and [trusted-principal submission](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution).

## Context to Read

Read `AGENTS.md`, `docs/product/prd.md`, this packet, [#203](https://github.com/wunderforge/agenova/issues/203), `spec.md`, `design.md`, the [architecture contract](../../docs/product/architecture-contract.md), `internal/authentication`, `internal/console`, `internal/connectedclient`, and [#215 member evidence](../0215-invited-team-access/evidence/acceptance-20261019.md).

## Scope and Negative Behavior

The current main service already verifies Google identity during login, issues revocable application sessions, resolves current membership on each operation, protects connected CLI/Portal transport, and filters evidence by owner. E14 M2 must prove that this existing boundary supplies the principal to M1 Policy and never accepts a caller-authored team or subject. Invalid identity and cross-owner references must stop before worker/provider effects. Keep Policy and AgentTemplate management behind operator authorization.

The original #203 text requires direct company JWT/JWKS bearer verification, a token file, and explicit fixed-principal mode. The current Google/member implementation uses application sessions and OS credential storage. Those transport details are **not yet reconciled in the Ticket**; this packet does not silently declare them equivalent or remove the original acceptance criteria.

## Acceptance and Evidence

- [x] One service accepts two distinct verified members and attributes Work/evidence to the correct subject/team in a focused integration test.
- [x] Caller-authored team header does not override membership; cross-owner evidence is hidden; invalid identity has zero worker/provider side effects.
- [ ] Reconcile #203 transport requirements with the Owner-approved Google/member design in the Ticket and PR review.
- [ ] Record a real installed two-member Google login and Work/evidence transcript, including CLI/API/Portal parity and named failure cases. Synthetic test identities do not satisfy this.
- [ ] Run focused and full gates on the final rebased branch; review diff and report residual risks.

## Gates and Dependencies

`go test -count=1 ./internal/console -run TestE14TwoMembersShareOnePolicyService` and `./scripts/check.ps1 -All`. M1 PR #207 must merge before M2 can be merged. This branch is a draft until the Ticket contract and installed two-member acceptance are resolved.

## Decisions and Blockers

- 20 October 2026: User selected the existing main member service over the older standalone E14 OIDC stack. No duplicate verifier is brought into main.
- M1 #207 passed local and CI gates but branch protection currently requires a fresh independent approval.
- Real installed two-member Google acceptance and direct-JWT versus application-session Ticket reconciliation remain open. Do not mark M2 accepted based on the focused synthetic test.
