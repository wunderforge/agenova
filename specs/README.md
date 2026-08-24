# Feature Specifications

Feature specs preserve accepted behavior and design when a GitHub Issue alone is not enough. They do not replace Issues, the Delivery Project, PRs, or the MVP PRD.

## Choose the Smallest Useful Level

### Issue only

Use the GitHub Issue without repository spec files when all are true:

- behavior and acceptance criteria are unambiguous;
- the change stays inside one ownership boundary;
- it does not change a public schema, CLI/API contract, architecture invariant, or authority rule;
- no parallel consumer needs an early stable contract.

### Feature spec

Add `specs/<issue>-<slug>/spec.md` when any is true:

- a public or cross-component behavior changes;
- multiple contributors need the same contract or fixture;
- claim authority, denial, lifecycle, or other security-sensitive behavior changes;
- important edge cases do not fit clearly in the Issue;
- a reviewer needs to approve what the system will do before implementation.

The spec owns feature-specific behavior. It must link the parent Issue and the PRD outcome it elaborates; it must not invent a new product goal.

### Feature spec plus technical design

Also add `design.md` when the implementation crosses ownership boundaries, has multiple credible approaches, introduces a dependency or persistence model, maps a backend semantic, or requires an explicit compatibility/migration decision.

## Layout

```text
specs/
  README.md
  0072-example-feature/
    spec.md
    design.md        # optional
```

Use the zero-padded GitHub Issue number as the stable prefix. Do not create `tasks.md`: owner, dependency, sequence, status, and completion evidence remain in GitHub.

## Minimum `spec.md`

```md
# <Feature name>

- Issue: #<number>
- PRD outcome: <link and section>
- Status: Draft | Accepted | Superseded

## Intent
<What observable outcome changes and why?>

## In Scope
- ...

## Out of Scope
- ...

## Requirements
- Given ..., when ..., then ...

## Negative Cases
- ...

## Compatibility
<Existing behavior that must remain unchanged.>
```

Keep exact test commands and completion evidence in the Issue and PR unless they are themselves part of the stable feature contract.

## Minimum `design.md`

Record only decisions reviewers need before implementation:

- relevant current state and constraints;
- chosen approach and ownership boundaries;
- alternatives rejected and why;
- contract/fixture changes and compatibility impact;
- verification strategy and unresolved risks.

## Lifecycle

1. The ticket owner proposes spec depth during elaboration.
2. A reviewer accepts the spec before dependent work treats it as stable. A small spec may merge with its implementation when no parallel consumer depends on it.
3. Update the spec when accepted feature behavior changes. Update the PRD only when committed MVP scope or acceptance changes.
4. Mark a replaced spec `Superseded` and link its replacement; do not keep two active authorities for the same behavior.
5. After implementation, GitHub carries the delivery result and evidence. The spec remains only if it is useful as a durable contract.

Nested `AGENTS.md`, directory-level `SPEC.md`, and tool-specific rules are not implied by this layout. Add path-scoped instructions only when that subtree has materially different commands, risks, or boundaries.
