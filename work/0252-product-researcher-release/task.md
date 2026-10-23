# Task: Release accepted product journey and real-model Researcher

- Ticket: [#252](https://github.com/wunderforge/agenova/issues/252)
- Mission: release the owner-accepted product and real-model Researcher while preserving existing applications/data.
- Target: member Portal/API/SQL, research MCP and separate controlled-v1 Agent image.
- User value: two business tasks with distinct Work permissions and actual model/tool evidence.
- PRD outcome: [declarative access, facts and contributor journey](../../docs/product/prd.md).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Architecture](../../docs/product/architecture-contract.md), [accepted spec](../0218-final-demo-product/spec.md), [local evidence](../0218-final-demo-product/acceptance.md), [release sequence](../0218-final-demo-product/release.md).
- [Image contract](../../docs/development/agent-images.md), [shared CI/CD](../../docs/development/shared-eks-release.md), [MCP application](../../harness/demo/research-mcp/README.md).
- Harness playbook: Add a User-Facing Demo Slice and quality gates.

## Scope

In scope:

- Owner-approved product slice, keyless public HTTPS and synthetic CRM; separate model-driven Researcher; 010/011/minimal grants; compatible rollback, controlled release and cloud checks. Parent epic #218 is not closed by this ticket.

Out of scope:

- New model IAM, nodes/DB/PVC/Grafana, keyword-search service, direct Agent public egress, #247 changes, marketplace and authentication bypass.

## Acceptance Criteria

- Source/normal CI pass; keep the recorded Windows-only prior failure explicit.
- Same caller/template/project: distinct grants, actual model-selected tools/observations and source-grounded answers reviewed separately from execution success.
- Preserve existing Google/member and Agent/project/tool flows and historical data.
- Verify schema/checksums/minimal grants, compatibility rollback and temporary operator cleanup; hand over actual images/source and real Work/UI links.

## Negative Case

- Missing tool/scope yields no unauthorized downstream calls. Model self-refusal is not Gateway blocking. Out-of-catalog actions remain rejected by the existing host; explicit Gateway probes require independent zero receipts.
- No completed Researcher answer without observations. No direct Pod public authority. No synthetic preview deployed.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Self-review: Agent role does not grant access or alter generic Worker/host rules; task prose is never authority.
- [x] Separate Researcher image, model-selected text-input tests, no-observation failure and grant-scope tests.
- [x] Preserve local UI/SQL/MCP evidence; build compatible rollback from 8850167 (old product plus schema/checksum compatibility).
- [ ] Publish exact artifacts and pass Linux CI/review.
- [ ] Quiet-window migration/grants and temporary credential cleanup.
- [ ] Control-plane/MCP release and research registration; real model, negative, legacy and cleanup acceptance.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `python -B -m unittest discover -s harness/demo/research-mcp -p test_run_migration.py -v` (ambiguous create / confirmed absence / ownership-safe cleanup; no cloud access).

- `go test ./cmd/agenova-demo-worker ./internal/workerprotocol ./internal/runtime/agentsandbox ./tools/product-release-migrate -count=1`
- `python harness/demo/research-mcp/build-worker.py`
- UI build/unit/smoke and local actual API/SQL/MCP journeys from linked acceptance.
- `.\scripts\check.ps1 -All`

## Evidence Required

- Source/digests, schema/grant and cleanup summaries; actual model/provider evidence, independent MCP receipts, reviewed answers and current UI screenshots. Native isolation proof remains separate.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Preserve all existing topology and histories. One exact RDS managed Secret is approved only for a temporary trusted migration; no credential logging/local file/Agent exposure. New administrator merge bypass is not implied.

## Decisions and Blockers

- User explicitly authorized the #254 follow-up administrator merge after all
  checks/re-review, then publication and real-model re-acceptance. Fix issues in
  this approved scope without requesting the same permission again. Keep failed
  attempts immutable; no extra IAM/infrastructure or existing-template rewrites.

- Post-release real-model acceptance: public and CRM positive runs passed with
  exact separate grants and correlated independent MCP receipts. Missing-web
  negative exposed an Agent completion bug: generic mandatory-read instructions
  caused an unrelated CRM read followed by a refusal labelled Succeeded. No web
  authority or downstream web call escaped. Follow-up isolates the Researcher
  prompt and adds an explicit blocked control answer that exits without a result,
  even after unrelated observations. Do not parse task prose or invent a Gateway
  refusal. Existing Agent contracts and platform authorization remain unchanged.
  Original failed cloud attempt is retained; new CI/review and separately
  authorized follow-up merge/deployment precede a new immutable template version.

- Human decision resolved: Admin is the highest daily application role and may
  appoint Admins/operators and manage all organization projects/users. Owner has
  the same daily rights plus confirmed ownership transfer and protection against
  ordinary role/activation/grant edits. All mutation paths now follow this model;
  project-only administrators remain scoped. Per-Work and cloud boundaries stay
  independent. Fresh regression and independent review precede release.
- Administration now displays all-organization access for account Admins even
  without explicit project grants, rather than falsely saying no project access.

- Independent PR review: use serving-installation authorization for topology,
  track cleanup candidates before ambiguous creates, suppress current rules in
  historical Policy mode, and explicitly reject >100-version histories. Focused
  tests cover the changed boundaries; no finding is waived by source CI alone.

- 23 October Owner decision: no one-off project/user SQL bootstrap. Account
  Admin and Owner cover all organization projects; keep per-Work governance and
  cross-team denial. Add authenticated audited project creation, test revocation
  and Owner invariants, and create research through that API after release.

- AWS and personal CLI login refreshed. Owner approved 010/011 and minimal grants. Existing model is Nova Micro; real answer quality remains pending. Do not expand model IAM to pass tests. See linked release sequence.

