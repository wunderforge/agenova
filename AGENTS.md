# Agenova Agent Routing

Mission-critical rule: deliver the active Ticket's acceptance criteria with reproducible evidence while preserving Agenova's claim-scoped, backend-neutral product boundary.

## Required Context

For every implementation Ticket, read in this order:

1. this file;
2. `docs/product/prd.md` for the committed MVP direction;
3. the active `work/<issue>-<slug>/task.md`;
4. only the spec, design, architecture sections, code, tests, backend notes, and playbooks linked by that task.

If the task packet does not exist, follow **Start a GitHub Ticket** in `docs/harness/playbooks.md`, create it from the canonical template, and stop for Owner/Reviewer approval before implementation.

`README.md` and `docs/project-design.md` are human/public explanations, not default coding context. The collaboration contract and source ownership map are in `docs/development/AIDLC.md`.

## Stable Boundaries

- One `SandboxClaim` is one agent worker assignment, not one tool call.
- Requested access is intent and cannot grant authority.
- Claim-scoped governance is the product; Kubernetes is one runtime option.
- Backend/provider shapes remain inside their adapters.
- Long-lived external credentials remain behind governed interfaces.
- Parent/child claims express governance scope, not workflow scheduling.

The architecture contract is authoritative. Stop and request a maintainer decision if the Ticket conflicts with the PRD or architecture contract; do not broaden either to make implementation easier.

## Execution Loop

1. Confirm the approved task packet, negative behavior, dependencies, and strongest gate.
2. Scout the smallest relevant implementation and test surface.
3. Execute one Todo slice at a time and keep task-local decisions/blockers current.
4. Add or update focused behavioral evidence with the implementation.
5. Run the focused gate, then `./scripts/check.ps1 -All`.
6. On failure, stop expansion, fix or narrow the responsible change, and rerun.
7. Review the diff for scope, regressions, and source-of-truth changes.
8. Report exact evidence and residual risk in the PR and Ticket.

Prose alone is not evidence. Backend claims require real-backend output or an explicit blocker; user-facing flows require executable smoke/E2E evidence and rendered proof when visual behavior matters.

## Code Review Rules

For a PR, read the linked Ticket, its active task packet, and only the
product, architecture, spec, fixture, and test paths declared in the PR's
**Review context**. Go upward to the PRD or architecture contract when the
change touches a shared contract or stable boundary.

- For claim, authority, decision, evidence, or gateway changes, verify that
  caller intent remains distinct from system-issued state; missing,
  ambiguous, or mismatched authority data must fail closed.
- For application-facing changes, reject backend/provider vocabulary leaking
  from an adapter into shared contracts. Shared contract changes must consume
  their named fixtures directly and cover the stated positive and negative
  behavior.
- Review product scope and observable evidence before code style. Formatting,
  static analysis, links, and other mechanical checks belong to CI; do not
  report them as review findings when the deterministic gate already covers
  them.
