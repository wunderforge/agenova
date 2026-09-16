# Feature Specification: idempotent Platform plan/apply

- Ticket: [#45](https://github.com/wunderforge/agenova/issues/45)
- PRD outcome: [`docs/product/prd.md` §6](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Intent

One Platform document is the reviewable source of desired installation state. Operators can validate and inspect its plan without mutation, then reconcile the same document on an explicitly named existing target using only their current target identity and RBAC.

## In Scope

- Validate/plan/apply, exact adapter activation, dependency-ordered reconciliation, initial default-deny policy seed, status, persisted revision/lock, confirmation, and Kubernetes reference evidence.

## Out of Scope

- Cluster lifecycle, production release management, arbitrary plugins, provider credentials, policy administration, and deployment of external runtime/model prerequisites.

## Requirements

- Given a valid Platform, `validate` resolves all references and adapter-owned configs without mutation.
- Given a valid Platform, `plan` reports stable ordered changes and component status without mutation.
- Given approval, `apply` activates missing exact adapter versions, preflights the selected target, reconciles deployment resources, waits for Ready, and records the effective revision/lock.
- Given the same ready revision, a second plan/apply reports zero changes and creates no additional named resources.
- Given an unavailable adapter, invalid capability/config, wrong target, missing prerequisite, or denied RBAC, the command fails with a bounded secret-free diagnostic.
- Platform-selected model backends remain behind Agenova's mandatory Model Gateway; applying the Platform does not authorize direct worker-to-provider calls.
- Existing claims retain their issued authority and bindings after a later Platform apply.

## Negative Cases

- Denied target RBAC stops before mutation where the target supports preflight authorization checks.
- A changed object after partial reconciliation is reported as failed with completed and pending components distinguishable.
- Private adapter config is never emitted in status or stored in the public lock.

## Compatibility

- Existing `run` and `adapters` commands remain unchanged.
- The canonical Platform v1alpha1 and #151 lifecycle remain the only validation/activation sources.
- Backend/provider objects do not enter application-facing Platform output.

## Open Decisions

- None for this ticket. Production packaging remains an explicitly later decision.
