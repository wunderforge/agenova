## Linked ticket

Closes #<!-- issue number -->

## MVP-path outcome

<!-- Which observable step in the golden flow improves? -->

## Changes

<!-- What changed, within the linked ticket's scope? -->

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
