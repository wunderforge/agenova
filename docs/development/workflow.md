# Development Workflow and Sources of Truth

This document defines how Agenova contributors and coding agents collaborate. It is the authority for repository workflow and document ownership; it is not a product specification.

## Source of Truth Map

There is no single file that answers every question. Each question has one authority.

| Question | Authority | Update trigger | Approval |
| --- | --- | --- | --- |
| What must the committed MVP achieve? | `docs/product/prd.md` | Accepted product outcome or scope changes | Product maintainer |
| Which architecture rules must every change preserve? | `docs/product/architecture-contract.md` | Explicit architecture decision | Maintainer review |
| How is the product explained to a newcomer? | `docs/project-design.md` | Public concepts or examples change | Feature reviewer |
| What does the merged repository currently prove? | `docs/project-status.md` | Merged behavior, evidence, or a known gap changes | Implementing PR reviewer |
| What does each Epic own? | `docs/product/mvp-epics.md` | Stable Epic boundary changes | Product maintainer |
| What behavior/design does one complex feature commit to? | `specs/<issue>-<slug>/` | The feature is clarified or its accepted design changes | Ticket reviewers |
| Who owns work, what is ready, and what evidence closes it? | GitHub Issue and Delivery Project | Delivery state changes | Ticket owner/reviewer |
| What is known about one runtime backend? | `docs/backends/` | Backend evidence or supported mapping changes | Adapter reviewer |
| How are changes verified? | `scripts/check.ps1`, `scripts/checks/`, and `docs/harness/quality-gates.md` | A gate or invocation changes | Harness reviewer |
| How should agents operate in this repository? | `AGENTS.md` | Repository-wide agent behavior changes | Maintainer review |

Supporting documents may summarize another authority only to route readers to it. If two sources disagree, fix the non-authoritative summary in the same PR. Mutable owner, status, dependency, sequence, and evidence data must not be mirrored into Markdown.

## Adaptive Planning Depth

Use the smallest artifact set that removes meaningful ambiguity:

1. Every change starts with a GitHub Issue containing scope, acceptance criteria, a negative case, and evidence.
2. Use the Issue alone for bounded, local, well-understood work.
3. Add a feature spec when a public contract changes, several components or contributors must agree on behavior, or authority/security semantics need explicit negative cases.
4. Add a technical design only when reviewers must choose or approve an implementation approach before code is safe to parallelize.

The exact decision test and format live in `specs/README.md`. Do not create a spec, plan, or nested instruction file merely because a directory exists.

## Delivery Loop

```text
Issue -> elaborate -> choose spec depth -> human approval
      -> implement on a task branch -> focused evidence -> repository gate
      -> PR review -> merge -> update implementation truth
```

For each ticket:

1. **Elaborate:** the owner uses AI to inspect the relevant code and propose acceptance criteria, risks, dependencies, and the strongest gate.
2. **Approve intent:** a human reviewer confirms scope and product/architecture alignment before broad implementation.
3. **Stabilize shared inputs:** contract, schema, fixture, or spec changes land before dependent adapter, UI, or example work claims integration readiness.
4. **Construct:** the owner and coding agent implement the smallest accepted slice in a dedicated branch or worktree.
5. **Verify:** run a focused behavioral check, then the applicable repository profile. A failing gate narrows or redirects work; it is not waived by prose.
6. **Review:** the PR links the Issue and reports exact evidence, risks, and blockers.
7. **Reconcile:** if merged behavior changes product requirements, architecture rules, public explanation, backend claims, or implementation truth, update that authority in the same PR.

## Ten-Person Collaboration

- One person owns each ticket; one named reviewer owns the next decision. Additional contributors pair or provide evidence without creating competing branches for the same outcome.
- Parallel work begins at an accepted boundary: a merged contract, schema, fixture, or feature spec. Consumers may prototype earlier, but cannot redefine that boundary from an adapter, UI, or demo.
- Team elaboration is reserved for product boundaries, public contracts, security/authority behavior, and cross-team design. Routine implementation and formatting decisions stay with the ticket owner and mechanical gates.
- AI may draft requirements, design, code, tests, and review findings. Humans approve scope, architecture changes, security claims, accepted evidence, and merge decisions.
- Repeated review feedback becomes a focused gotcha, playbook, fixture, or deterministic check. Do not grow ambient rules from one-off preferences.

## Rules, Skills, and Code Style

- `AGENTS.md` is the tool-neutral agent contract. Files such as `CLAUDE.md` remain thin adapters that route to it; they do not copy product or workflow rules.
- A nested `AGENTS.md` is appropriate only when a subtree has different commands, safety constraints, or ownership boundaries that the root file cannot express concisely.
- A repository-local skill is optional automation, not a source of truth. Add one only for a recurring Agenova workflow with defined inputs, outputs, and verification that cannot be expressed more simply by routing or scripts.
- Formatters, linters, static analysis, tests, and boundary checks are implemented once under the shared check entry point. When a new language or subsystem lands, extend that entry point instead of adding a prose-only style guide or a second CI implementation.

## Human Decision Gates

Maintainer approval is required before a change:

- alters committed MVP scope or an architecture invariant;
- exposes a new public contract or changes authority semantics;
- changes the meaning of a claim to fit one backend;
- makes a security, isolation, or production-readiness claim;
- adds a new always-loaded rule, nested `AGENTS.md`, or repository-local skill.

Code style does not require a separate approval document. Formatting, static analysis, tests, source boundaries, and delivery contracts are enforced through the shared check scripts.
