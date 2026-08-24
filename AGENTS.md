# Agenova Agent Routing

Keep changes bounded, backend-neutral, and supported by reproducible evidence.

## Start With the Active Contract

1. Read the active GitHub Issue.
2. Read `docs/development/workflow.md` and any linked file under `specs/`.
3. Load only the authority relevant to the change:
   - MVP outcome: `docs/product/prd.md`;
   - architecture invariant: `docs/product/architecture-contract.md`;
   - implemented truth: `docs/project-status.md`;
   - backend mapping: the applicable file under `docs/backends/`;
   - verification: `docs/harness/quality-gates.md`.
4. Inspect the smallest relevant code and test surface.

Do not read or rewrite every product document for a local change. The source-of-truth map and conflict rules live in `docs/development/workflow.md`.

## Non-Negotiable Boundaries

The architecture contract is authoritative. In particular:

- one `SandboxClaim` is one agent worker assignment, not one tool call;
- requested access is intent and cannot grant authority;
- claim-scoped governance is the product, while Kubernetes is one runtime option;
- backend/provider shapes remain inside their adapters;
- long-lived external credentials remain behind governed interfaces;
- parent/child claims express governance scope, not workflow scheduling.

Do not broaden the PRD or architecture contract to make one implementation easier. Stop and request a maintainer decision when the accepted task conflicts with either authority.

## Standard Loop

1. Confirm acceptance criteria, negative behavior, dependencies, and the strongest relevant gate.
2. Use `specs/README.md` to decide whether the Issue is sufficient or a feature spec/design is required.
3. Implement the smallest accepted slice on a dedicated branch or worktree.
4. Add or update focused behavioral evidence with the implementation.
5. Run the focused gate, then `./scripts/check.ps1 -All` before completion.
6. If a gate fails, stop expansion, fix or narrow the responsible change, and rerun.
7. Reconcile any affected source of truth in the same PR.

## Completion Evidence

Prose alone is not evidence.

- Core contract: focused unit/contract test plus `./scripts/check.ps1 -All`.
- Backend claim: adapter test and real-backend evidence, or an explicit blocker.
- CLI/API/UI flow: executable smoke/E2E output; rendered evidence when visual behavior matters.
- Docs/harness: `./scripts/check.ps1 -Docs` plus manual review of meaning and layout.

Record exact results in the linked Issue and PR. Promote repeated failures into a focused gotcha, playbook, fixture, or deterministic check instead of adding generic ambient rules.
