# Feature Specification: Shared installed-service API for CLI Work query and local Portal parity

- Ticket: [#167](https://github.com/wunderforge/agenova/issues/167)
- PRD outcome: [Facts, reference install and claim console](../../docs/product/prd.md).

## Intent

After installation, an operator can submit Work and later ask for its evidence without retaining the first CLI response. A local Portal can read that same installed service; a separate demo process or fixture must never masquerade as connected Work.

## In Scope

- Private `/api/setup`, `/api/requests` and `/api/requests/{ref}/evidence` remain the shared HTTP contract. Setup reflects the installed registry and selected platform capabilities.
- CLI Work run/list/show and local Portal display use those API records. Transport may differ: CLI uses authenticated Kubernetes exec into the private in-pod API; an explicit operator-controlled loopback port-forward serves the dev Portal.
- Platform status describes the local connection address and command separately from installation readiness.

## Out of Scope

- Production user authentication, cross-restart history, broad Work search, multi-template Portal controls, general UI deployment and arbitrary provider plugins.

## Requirements

- Given an applied supported Platform and registered Policy/template, a CLI submission yields one request reference and a later CLI query returns the canonical evidence view even after the submitting process exits.
- Given the local tunnel, Portal setup/list/detail reads that same service and preserves the exact request/claim IDs, decision, outcome and failure facts.
- Given a denied request, all surfaces show denial with no invented claim, runtime allocation or provider call.

## Negative Cases

- Missing/invalid reference, unsupported method/media, cross-Origin browser attempt, unavailable cluster/tunnel, wrong context/RBAC, and absent registry data return explicit errors. Connected mode never falls back to fixtures or the memory service.

## Compatibility

- Preserve ClaimRequest and evidence v0 schemas, trusted principal separation, create-only management boundary, existing local demo tests and backend-neutral application contracts.

## Open Decisions

- The owner accepted a separately started local UI, with `platform apply` installing only the platform; the demo bootstrap may start UI but is not this ticket's install contract.

