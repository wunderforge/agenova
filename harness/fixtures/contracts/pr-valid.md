## Linked ticket

Closes #19

## MVP-path outcome

Every contribution carries enough information for independent implementation and review.

## Changes

Added the bounded delivery-contract templates and their deterministic validator.

## Verification

| Gate | Exact command or artifact | Result |
| --- | --- | --- |
| contract fixtures | `./scripts/check.ps1 -Docs` | pass |
| repository baseline | `./scripts/check.ps1 -All` | pass |

## Backend neutrality

- [x] Not applicable: this change does not touch runtime or application-facing contracts.

## Risks and blockers

- Risks: None
- Blockers: None
