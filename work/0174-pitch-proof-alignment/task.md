# Task: Align product acceptance and demo narrative with per-Work template reuse

- Ticket: [#174](https://github.com/wunderforge/agenova/issues/174)
- Mission: Align the committed product contract and demo narrative with safe reuse of one AgentTemplate across independently authorized Work.
- Target: `docs/product/prd.md`, `docs/product/architecture-contract.md`, `docs/product/mvp-epics.md`, and linked GitHub ticket descriptions.
- User value: A presenter can show what the current demo proves and what final-MVP cross-team proof remains, without overstating policy or identity behavior.
- PRD outcome: [Product Statement and Acceptance Scenario](../../docs/product/prd.md)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Authority and Credentials](../../docs/product/architecture-contract.md#authority-and-credentials)
- [MVP Epic Map](../../docs/product/mvp-epics.md)
- [#107 mid-term demo](https://github.com/wunderforge/agenova/issues/107) and [#47 installed golden path](https://github.com/wunderforge/agenova/issues/47)

## Scope

In scope:

- State the organizational value: a reusable template with no growing standing permission, and a fresh temporary claim per Work.
- Clarify that v0 PolicyBundle gates assignment by trusted team/project/template; request, template ceiling, and runtime restrictions determine the concrete capability snapshot.
- Add final-MVP two-team positive and negative evidence, and label the 24 September installed demo as a narrower single-principal proof.
- Keep E9's example responsibility and #11/#13/#46/#47/#107 ticket narrative consistent with these boundaries.

Out of scope:

- Implementing a multi-principal transport boundary, broad policy limits, OIDC/SSO, multi-agent orchestration, or new UI features.

## Acceptance Criteria

- PRD, architecture contract, and E9 map describe the same per-Work authority semantics.
- The final acceptance distinguishes one template reused by two independently verified teams from the current Team A-only installed checkpoint.
- #172 owns trusted dual-identity implementation, #46 baseline deny, and #47 cross-team installed proof; #107 does not silently depend on them.

## Negative Case

- No document or ticket may imply that caller-authored `ClaimRequest` can assert identity, that v0 PolicyBundle supplies per-team tool/resource ceilings, or that the current installed demo already demonstrates Team B submission.

## Execution Todo

- [x] Scout the PRD, architecture contract, v0 policy/resolver code, and existing ticket dependencies.
- [x] Confirm product direction with Owner: explicit 21 September request to proceed with the proposed alignment; independent PR review remains separate.
- [x] Update only the three product documents and linked ticket descriptions.
- [x] Record #172/#46/#47 native dependency and board sequence as delivery evidence.
- [x] Run the focused documentation gate; full PR CI will verify the repository baseline.
- [ ] Review PR diff and CI for source-of-truth consistency before merge.

## Quality Gates

- `./scripts/check.ps1 -Docs`
- `git diff --check`
- PR CI baseline (`./scripts/check.ps1 -Profile PR` in CI)

## Evidence Required

- `./scripts/check.ps1 -Docs` passes all documentation, links, architecture, and delivery-contract checks.
- `git diff --check` passes.
- GitHub issue/board state shows #172 before #46 and #47, with #107 still the sole in-progress mid-term item.
- PR #173 CI result and reviewable diff; this docs ticket does not claim #172/#47 implemented.

## Constraints

- Preserve claim-scoped and backend-neutral architecture while clarifying its policy-gate wording.
- Owner's 21 September request authorizes this product-story correction; no new runtime or authentication feature is silently added.
- Keep mutable status on GitHub, not in the Epic map.

## Decisions and Blockers

- Decision: final-MVP installed cross-team reuse is #172 -> #46 -> #47; #107 remains the narrower 24 September demonstration.
- Decision: public v0 claim stores a template reference, not a template-version field; the #47 harness retains registered template content for comparison.
- Blocker: PR #173 initially failed the required PR-body/task-packet contract; this packet and linked #174 resolve that delivery-format failure. Product document checks passed.

