# Contributing to Agenova

Contributions should strengthen the shared MVP path without changing the product boundary by accident.

## 1. Choose a Bounded Task

Start from `tasks/task-template.md`. Every task needs:

- one mission and owner;
- explicit in-scope and out-of-scope boundaries;
- observable acceptance criteria;
- exact quality gates and evidence;
- a link to the MVP flow it improves.

Prefer work that can be reviewed in one or two days. Split larger work by behavior or evidence gate.

## 2. Read Only What You Need

Always read:

- `AGENTS.md`;
- `docs/product/prd.md`;
- `docs/product/architecture-contract.md`;
- the active task.

Then load only task-relevant code, backend notes, or harness playbooks.

## 3. Implement and Verify

Add or update the strongest relevant behavior check with the implementation.

```powershell
.\scripts\check.ps1 -All
```

If the task touches Agent Sandbox behavior, run the integration gate when the required cluster is available. Do not replace real backend evidence with static YAML or a prose claim.

## 4. Open a Reviewable PR

Include:

- what user-visible or contract behavior changed;
- files and boundaries touched;
- exact commands run and results;
- evidence location;
- remaining risks or blockers;
- confirmation that the change did not leak backend-specific types into shared APIs.

## Definition of Done

A task is done when:

- acceptance criteria are observable and pass;
- focused and repository gates pass;
- docs match implemented behavior;
- a teammate can reproduce the evidence;
- no unrelated abstraction or refactor was added;
- blockers are recorded instead of being described as completed work.

## Review Priorities

Review in this order:

1. Product and scope boundary.
2. Behavior and negative cases.
3. Evidence quality and reproducibility.
4. Backend neutrality.
5. Code clarity and maintainability.
