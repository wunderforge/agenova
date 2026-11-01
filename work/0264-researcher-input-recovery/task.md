# Task: Researcher input recovery and actionable failures

- Ticket: [#264](https://github.com/wunderforge/agenova/issues/264)
- Mission: recover bounded fixed-choice argument mistakes and expose the actual terminal validation failure.
- Target: workerprotocol, reference Researcher, console diagnosis and Work notices.
- User value: ordinary customer questions execute reliably; failures show a specific cause and evidence.
- PRD outcome: [reference repair](../../docs/product/prd.md); within the accepted execution/diagnostic scope.

## Context to Read

AGENTS.md; docs/product/prd.md; [architecture](../../docs/product/architecture-contract.md) Submission and Resolution / Authority and Credentials; [spec](spec.md); internal/workerprotocol/{research,gateway_schema,exchange}.go; cmd/agenova-demo-worker/researcher_fast.go; internal/console/service.go; ui/src/{WorkAuthority,record-presentation}.tsx and corresponding tests.

## Scope

In scope: raw input hints/enums, exact bounded retry guidance, safe action-failure classification and evidence-linked UI, synthetic regressions, CI and immutable release/cloud acceptance.
Out of scope: new providers/permissions, general coercion, changing execution binding, new API schema/migrations, paid search or infrastructure.

## Acceptance Criteria

- Fixed-choice raw arguments are explicit; the observed parameter=value wrapper remains invalid.
- A subsequent valid model action passes the actual exchange and Gateway normally; repeated malformed actions stop without a backend call.
- The last unrecovered action error is shown prominently on failed Work, with its evidence link, including historical generic no-result outcomes.
- Recovered retries, successful tasks, provider errors and real denials are not relabeled.
- Existing public-text tools, old schemas and strict authority/correlation stay compatible.
- Verify rendered desktop/mobile and exact original cloud task after deployment; do not publish runtime identifiers or task text to GitHub.

## Negative Case

Wrong scope/value, unknown tools, malformed output and a genuine ungranted tool call cannot be converted into an allowed call by repair guidance or UI presentation.

## Execution Todo

- [x] Scout and self-review scope against PRD/architecture; no unresolved product conflict.
- [x] Implement bounded hints/recovery and terminal diagnostics.
- [x] Regress actual malformed-output shape, exchange binding and safe UI evidence selection.
- [ ] Focused checks and browser render passed; full local gate has the known Windows owner-file failure. Linux CI remains pending. See [evidence](evidence.md).
- [ ] Review diff, commit/push, PR review and release gate.
- [ ] Deploy verified immutable images/template and run original task, public and denied regressions through cloud services.

## Quality Gates

`go test ./internal/workerprotocol ./cmd/agenova-demo-worker ./internal/console ./internal/connectedclient`; UI Vitest/typecheck/build/Playwright; `./scripts/check.ps1 -All`; Linux CI/image smoke.

## Evidence Required

Synthetic regression output, actual exchange/call counts, rendered notices, CI/image checksums, and local-only cloud acceptance report.

## Constraints

Preserve debug.log, all dirty work, existing execution and authority contracts. New files/caches on D:. No raw operational records in Git/PR. Branch-protection bypass requires its own authorization if needed; no implicit credential expansion.

## Decisions and Blockers

Use the existing schema and strict parser, not unsupported conditional JSON schema or silent input normalization. Mixed free-text catalogs use explicit per-tool hints; fixed-only catalogs use bounded enums. A provider's successful HTTP response is not proof of valid Agent action. Gateway checks remain authoritative. Independent review is at the PR boundary.
