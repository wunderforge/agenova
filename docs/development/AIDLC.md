# Agenova AI-Driven Development Lifecycle

This document is the team operating contract for developing Agenova with coding agents. Humans own intent, decisions, review, and acceptance; agents explore, plan, implement, verify, and report within one approved ticket boundary.

## Delivery Unit

One delivery lane contains:

```text
one GitHub Ticket
  + one human Owner
  + one coding Agent
  + one task packet
  + one independent Reviewer
  + one reviewable PR
```

The Ticket coordinates the team. The task packet controls the Agent execution. They share an Issue number but do not own the same state.

## Sources of Truth

| Question | Authority | Updated when |
| --- | --- | --- |
| What must the MVP achieve? | `docs/product/prd.md` | Committed MVP scope or acceptance changes |
| Which boundaries must all implementations preserve? | `docs/product/architecture-contract.md` | An explicit architecture decision is approved |
| What does this Ticket ask the team to deliver? | GitHub Issue | Scope, dependency, owner, readiness, or outcome changes |
| What should the active Agent read and execute? | `work/<issue>-<slug>/task.md` | The approved execution plan or a blocker changes |
| What shared feature behavior must consumers follow? | Optional `spec.md` in the task packet | Accepted feature behavior changes |
| Which technical approach was approved? | Optional `design.md` in the task packet | A reviewed design decision changes |
| What does merged code currently prove? | `docs/project-status.md` | Merged behavior, evidence, or a known gap changes |
| How is completion verified? | `scripts/check.ps1`, focused tests, and the PR | A gate or implementation changes |

Supporting files may summarize another authority only to route readers. Mutable priority, sequence, assignment, dependency, and board status stay in GitHub.

## Agent Context Contract

Every Ticket starts with this minimal context:

1. `AGENTS.md` — repository operating rules and routing.
2. `docs/product/prd.md` — the product direction and committed MVP.
3. `work/<issue>-<slug>/task.md` — the active execution contract.
4. Only the spec, design, architecture sections, code, tests, backend notes, and playbooks linked by `task.md`.

`README.md` and the full newcomer product tour are human/public entry points, not default coding-agent context. An Agent may inspect them when the task concerns onboarding or public documentation.

Reading the PRD does not enlarge the Ticket. The task packet remains the execution boundary; a conflict with the PRD or architecture contract stops implementation and requires a human decision.

## PR Review Context

The PR description is the reviewer routing layer. It must link the Ticket and
name the active task packet; the task packet owns links to relevant product,
architecture, spec, fixture, test, and playbook context. The PR may add a
one-off review focus, but must not duplicate that context list. It must also
state the changed boundary (or `None`), the nearest deferred non-goal, and
exact verification results.

Reviewers start with that task context, then apply the repository's
`AGENTS.md` review rules. Deterministic CI proves mechanical checks; automatic
review flags likely contract, scope, or evidence gaps; a human Owner/Reviewer
still decides product fit, architecture trade-offs, approval, and merge.

Codex automatically reviews a PR when it is opened. After substantive changes,
request another pass by commenting `@codex review` on the PR; do not request a
new review for every small commit. Codex findings are advisory: a human Reviewer
still owns approval and merge.

## From Existing Ticket to Task Packet

For the current MVP, work starts from an accepted GitHub Ticket:

```text
Ticket -> Agent drafts task packet -> Owner and Reviewer approve
       -> Agent executes Todo -> focused evidence -> repository gate
       -> PR -> independent review -> merge -> Ticket evidence/status
```

1. The Owner selects or receives a Ticket and names a Reviewer.
2. The Agent reads the Ticket, `AGENTS.md`, and PRD, then follows the **Start a GitHub Ticket** playbook.
3. The Agent creates `work/<issue>-<slug>/task.md` from the canonical template and adds optional spec/design files at the required depth.
4. The Agent stops before implementation. The Owner corrects intent and context; the Reviewer approves scope, acceptance, constraints, and gates. That approval is recorded in the GitHub Ticket.
5. The Agent executes the approved Todo, updating only task-local decisions and blockers.
6. The Owner opens a PR with exact evidence and links it to the Ticket.
7. The Reviewer evaluates behavior, negative cases, scope, architecture alignment, and evidence before merge.
8. GitHub remains authoritative for delivery status. The completed packet is provenance, not a second project board.

## Adaptive Planning Depth

Choose the smallest packet that makes execution safe:

| Level | Files | Use when |
| --- | --- | --- |
| Task | `task.md` | One bounded, well-understood behavior inside one ownership boundary |
| Task + Spec | `task.md`, `spec.md` | Public/cross-component behavior, authority semantics, or multiple consumers need agreement |
| Task + Spec + Design | `task.md`, `spec.md`, `design.md` | Multiple technical approaches, backend mapping, persistence, compatibility, or cross-team integration requires approval |

Do not create a spec, design, nested `AGENTS.md`, or repo-local skill merely because a directory exists.

## Human and Agent Responsibilities

Humans:

- choose outcomes, priorities, owners, dependencies, and reviewers;
- resolve product ambiguity and approve architecture/security claims;
- approve the task packet before construction;
- provide independent review and accept evidence;
- coordinate real environments, integration order, and demos.

Agents:

- inspect the repository and propose a bounded plan;
- identify ambiguity, risk, dependencies, and negative cases;
- implement the approved Todo and keep the packet current;
- run focused and repository gates;
- report exact evidence, residual risk, and blockers without claiming unproven completion.

The human Owner is accountable for the Ticket result even when an Agent writes most of the code.

## When the PRD Changes

Do not update the PRD for routine implementation detail. Update it only when the team accepts a change to:

- committed MVP outcomes or scope;
- target users or user journey;
- MVP acceptance scenario or success measures;
- an explicit in-scope/out-of-scope boundary.

A feature spec elaborates an existing PRD outcome. It must not silently create a new product goal.

## Parallel Delivery

Parallel work begins at a reviewed boundary: a merged contract, schema, fixture, or accepted spec. Adapter, UI, example-agent, and test owners may prototype earlier, but cannot redefine a shared contract from their consumer branch.

Each Ticket has one Owner and one Reviewer. Several people may pair, but two branches must not independently claim ownership of the same outcome. Reviewers should come from another delivery lane whenever practical.

## Rules, Skills, and Harness Memory

| Content | Location |
| --- | --- |
| Mission-critical repository routing | `AGENTS.md` |
| Tool-specific loading syntax only | Thin adapters such as `CLAUDE.md` |
| Recurring Agenova mistake prevention | `docs/harness/gotchas.md` |
| Repeated Agenova workflows | `docs/harness/playbooks.md` |
| Failures that should improve the process | `docs/harness/learnings.md` |
| Canonical task/spec/design shapes | `docs/harness/templates/` |
| Deterministic enforcement | `scripts/check.ps1` and `scripts/checks/` |
| Generic personal methods, including external coding styles | User-level skills, not copied into this repository |

A repository-local skill is justified only for a recurring Agenova workflow with defined inputs, outputs, and verification that cannot be expressed more simply through routing, a playbook, template, or script.

## Learning Loop

- First useful failure: record it in `docs/harness/learnings.md`.
- Repeated failure: promote it to a project-specific gotcha or playbook.
- Mechanically detectable failure: add a test, fixture, lint, or check.

Generic advice and external skill text do not belong in the repository harness. Convert only recurring Agenova-specific behavior into routing, playbooks, gotchas, templates, or executable checks.
