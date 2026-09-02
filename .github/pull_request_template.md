## Linked ticket

Closes #<!-- issue number -->

## Review context

- Task packet: `<!-- work/<issue>-<slug>/task.md -->`
- Additional Spec/Design: `<!-- path, or None -->`
- Product or architecture boundary: `<!-- path + relevant heading, or None -->`
- Shared fixture or contract test: `<!-- path, or None -->`

## MVP-path outcome

<!-- Which observable step in the golden flow improves? -->

## Changes

<!-- What changed, within the linked ticket's scope? -->

## Scope and deferrals

- Contract or boundary changed: <!-- Describe the boundary, or None -->
- Deferred / non-goal: <!-- State the nearest intentionally excluded work -->

## Verification

| Gate | Exact command or artifact | Result |
| --- | --- | --- |
| <!-- focused gate --> | `<!-- exact command -->` | <!-- pass / fail / blocked --> |
| repository baseline | `./scripts/check.ps1 -All` | <!-- pass / fail / blocked --> |

## Backend neutrality

- [ ] Confirmed: shared contracts remain backend-neutral and provider-specific shapes stay inside their adapter.
- [ ] Not applicable: this change does not touch runtime or application-facing contracts.

## Risks and blockers

- Risks: <!-- None or describe the remaining risk -->
- Blockers: <!-- None or describe the explicit blocker -->
