# Task: E16 Slice 1 tool contracts and installed wiring

- Ticket: [#193](https://github.com/wunderforge/agenova/issues/193)
- Parent Epic: [#178](https://github.com/wunderforge/agenova/issues/178); delivery [PR #192](https://github.com/wunderforge/agenova/pull/192).
- Mission: Deliver the reviewed Slice 1 configuration and provider contracts through the installed service, with an explicit unavailable gate until live tool execution is ready.
- Target: internal/toolbackend, Platform resolution, adapterregistry/bundled tool support, installed control plane and shared-service composition/tests.
- User value: Operators can configure governed tool routes without mistaking configured capability for live tool availability.
- PRD outcome: [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [facts](../../docs/product/prd.md#5-facts-and-accountability), [installed service](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

Always: AGENTS.md, docs/product/prd.md, this packet.

- [Epic specification](../0178-governed-mcp-tools/spec.md) and [design](../0178-governed-mcp-tools/design.md), scoped to the Slice 1 delivery boundary.
- [Epic task and later-slice Todos](../0178-governed-mcp-tools/task.md) and [Slice 1 review/evidence](../0178-governed-mcp-tools/review-slice1.md).
- [Architecture contract](../../docs/product/architecture-contract.md), especially backend neutrality, authority, evidence and installed service.
- The implementation/test files mapped to Slice 1 in the review packet.

## Scope

In scope: the Slice 1 implementation, deterministic tests and evidence reviewed in PR #192 at 313576dcbeaf7a9f51131612f1c868eb05a922d9; additive tool configuration, neutral contracts, registry lifecycle and installed composition; S1 evidence cleanup and S4 exact HTTP fixture restriction.

Out of scope: live transport, worker catalog propagation, resultRef projection, S2 incomplete-invocation repair, S3 concurrency enforcement, D2 fixture selection, kind E2E, E14 credentials and #121 authentication. These remain in the Epic packet, not this child's Execution Todo. Keep Go 1.22 and CI contracts unchanged.

## Acceptance Criteria

1. Validate/canonicalize/resolve tool backends and profiles, rejecting invalid configuration and conflicting routes.
2. Support tool adapter lock/install/inspect/init without changing scalar schema grammar or regressing existing adapters.
3. Keep the immutable bounded catalog and provider/result contract backend-neutral, with no console import from bundled adapters.
4. Construct and verify the catalog/provider through the installed command; report notConnected and reject configured tool Work as tool_transport_unavailable before journaling/runtime setup, without mock fallback.
5. Deterministic successful calls preserve decision/attempt-before-dispatch and equal Target; outputs are bounded/untrusted and failures explicit.
6. Deliver the child-linked PR and reproducible Slice 1 evidence; keep later work and Epic closure separate.

## Negative Case

Reject malformed endpoint/configuration, duplicate logical-operation/resource routes, and configured-tool admission while transport is unavailable. Zero-call double evidence does not prove server behavior. Preserve disclosure of the deferred stage-1 invocation/CLI validation gap and missing Set concurrency enforcement.

## Execution Todo

- [x] Deliver the reviewed neutral provider/catalog/result implementation and deterministic tests.
- [x] Deliver flat-string Platform/adapter configuration, registry lifecycle and installed unavailable gate.
- [x] Deliver Slice 1 evidence cleanup, exact HTTP fixture restriction, and documented later-slice deferrals.
- [x] Bind the child issue and this packet to PR #192 under the existing delivery contract; review scope against the accepted Ticket and Epic spec/design.
- [x] Recheck docs, PR body, source hashes and complete staged diff; retain exact focused/full-gate results. Remote CI status is reported in PR #192 checks.
- [ ] Routinely merge origin/main into codex/e16-slice1 without rebase when synchronization is needed; verify any merge and rerun the required gates.
- [ ] Complete independent PR review and close only this child on merge.

## Quality Gates

Use the exact Go 1.22.12 focused command and environment in the [review packet](../0178-governed-mcp-tools/review-slice1.md).

- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`
- `pwsh -NoProfile -File ./scripts/check-pr-body.ps1 -BodyPath .tmp/e16-slice1/pr-body.md`
- `shasum -a 256 -c docs/evidence/178/source-sha256.txt`
- `git diff --cached --check`

## Evidence Required

The [review packet](../0178-governed-mcp-tools/review-slice1.md) maps every changed file to Slice 1 and records base/head, commands, results and risks. Focused/boundary logs and source hashes remain under docs/evidence/178/. Both full-gate logs stay local. Report actual failures and pending checks; no live MCP/kind claim is made.

## Constraints

- Preserve the architecture contract, claim-scoped enforcement, provider neutrality and Go 1.22 baseline.
- Keep #178 open; this child covers only PR #192 Slice 1.
- No contracts.ps1 bypass and no history rewrite. Merging main is routine branch maintenance and does not depend on PR #191.

## Decisions and Blockers

- Tom accepted the Slice 1 code and instructed this child split to satisfy the existing closing-ticket/packet-number rules. The previous maintainer-conflict diagnosis was incorrect.
- Tom approved publication of the three drafts with the final corrections on 2026-09-26. This packet uses the actual child issue number in a four-digit, zero-padded directory.
- Recorded local full gate: Go and frontend build/unit checks passed; browser smoke 51/52 with the unchanged keyboard case timing out. The previous CI stopped at the former Epic-linked PR contract. The actual #193 body and packet pass local documentation and delivery-contract checks; all 21 reviewed Go source hashes still match. Remote CI runs after publication and is reported in PR #192 checks.
- S2/S3 and all remaining Epic acceptance stay in the parent packet.
