# Task: Verify provider outage fail-closed behavior

- Ticket: [#199](https://github.com/wunderforge/agenova/issues/199)
- Mission: Measure RI-3 on an isolated real kind installation without changing product behavior.
- Target: harness/security, docs/evidence/199.
- User value: Reviewers can distinguish provider failure from successful Work and verify recovery.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet.
- `docs/product/architecture-contract.md`: Claim Lifecycle, Authority and Credentials, Reference Installation and Bootstrap.
- `docs/reference-cli-kind-ollama.md`: supported installation and connected Work path.
- `deploy/reference/platform.kind.yaml`, `deploy/reference/demo/`: baseline fixtures.
- `internal/console/service.go`, `internal/modelgateway/`: outcome and gateway evidence.
- #177 latest maintainer decisions; #194 RI-3.

## Scope

In scope: pre-Demo RI-3 only; healthy control, actual dedicated Ollama provider stop, unchanged-spec Work during outage, provider restart and recovery; independent provider completion counts, CLI JSON, cleanup evidence and limitations.

Out of scope: post-Demo worker/controller/control-plane fault matrix; product fixes; persistence; production or EKS claims; global service interruption; public publication without user review.

## Acceptance Criteria

- Provider-down Work never reports Succeeded or returns a fabricated model answer.
- No observed mock/memory fallback, with the measured evidence scope stated explicitly.
- Provider recovery permits identical Work spec (unique metadata names) to succeed.
- Record real backend/image identities, provider state/counts, claim outcomes, cleanup and residual uncertainty.
- Partial delivery does not close #199 or imply complete fault-matrix acceptance.

## Negative Case

A real installed Work submitted while its unchanged configured provider is stopped must fail rather than synthesize success. Failed invocation is not admission Deny; missing evidence is unknown.

## Execution Todo

- [x] Scout relevant implementation, tests, risks and dependencies.
- [x] Self-review packet against Ticket, PRD and architecture contract.
- [x] Prepare owned namespace and dedicated real provider; record UIDs and image identities.
- [x] Execute healthy/outage/recovery campaign and focused assertions.
- [x] Record terminal access and cleanup evidence, or explicit proof limitations.
- [x] Run focused gate, Docs and `./scripts/check.ps1 -All`.
- [x] Review diff and prepare reviewable public drafts.

## Quality Gates

- Executable campaign assertions over real installed CLI JSON and provider logs.
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

`docs/evidence/199/`: exact commands, baseline commit, cluster and image identity, fault state, provider request/completion counts, CLI JSON before/during/after, owned-resource cleanup receipts, matrix and limitations. Rendered proof if UI assertions are added.

## Constraints

Preserve the architecture contract and backend neutrality. E15 reports only; do not modify product logic or weaken checks. Synthetic inputs only; no real credentials. Refuse pre-existing namespace/container names and delete only recorded owned resources. No shared provider shutdown. High-risk new findings remain private.

## Decisions and Blockers

- 2026-10-19: self-review found no product/architecture conflict. Task depth only.
- Main baseline ae24a67261dff289a7063f7435c7c3616fd36dc0; separate worktree avoids disturbing #198.
- Use a dedicated real Ollama container with read-only model files. Stop/start that container while Platform and Work spec remain unchanged. This simulates provider process unavailability, not global host Ollama failure or slow-provider behavior.
- Docker 29.8.0, kind node Ready (Kubernetes v1.37.0), existing local llama3.1:latest confirmed. Resource preparation and execution pending.

- 2026-10-19: two installed campaigns passed; final evidence includes kind-node probe and one-to-one invocation correlation. Full gate passed after installing missing frontend prerequisites. See `docs/evidence/199/summary.md` for proof limits and partial-delivery status. Public drafts require user approval; no automatic-closing keyword is used for incomplete #199.
