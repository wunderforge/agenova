# Technical Design: Per-Work network authority and fail-closed egress

- Ticket: [#243](https://github.com/wunderforge/agenova/issues/243)
- Feature spec: `spec.md`

## Current State and Constraints

- Main already intersects Policy caps; AllocateRequest carries no network authority/expiry. Upstream Agent Sandbox v0.4.6 supports custom template networkPolicy and per-claim shutdownTime, but omitted rules allow public Internet. Additive deny cannot override allow. The current exec/stdio Worker needs no network. EKS reports strict CNI mode; configuration alone is not enforcement proof.

## Decision

Use explicit networkProfiles references with bounded public-web semantics. Set all managed templates to custom deny network rules, including warm workers. Apply temporary allow only to the concrete worker bound to the current Running claim; never change a template to grant one Work. Pass absolute expiry into upstream lifecycle. Integrate activation/revoke/recovery with RunService and durable evidence. Do not trust arbitrary images to self-report network isolation or termination.

## Ownership and Contract Boundaries

- Authority lane owns api/v1alpha1, internal/policy, internal/authority, internal/issuance and relevant canonical fixtures. Add optional networkProfiles to request/ceiling/grant and StringSetConstraint to Policy caps. Explicit Policy allow is required for public networking; old omission grants none.
- Runtime lane owns internal/runtime, internal/operator, internal/app, runtime adapter configuration/RBAC and their tests. Define minimal NetworkBackend activation/revoke evidence plus resolved profiles/expiry in AllocateRequest; communicate exact types to integrator. No public Kubernetes types.
- MCP lane owns cmd/agenova-control-plane incident demo tests, fixture/data/manifests and acceptance drivers/docs. Preserve dirty console rejection diagnostics; no cloud writes.
- Integrator owns console/evidence/UI/schema, PRD/status alignment, review, gates and deployment. No lane may overwrite another's files or weaken this contract to pass tests.

## Alternatives Considered

- Template-level grants widen peers; image flags/base-image controls are bypassable; FQDN proxy adds unnecessary scope; adding deny beside default allow does not revoke; waiting/self-check cannot close startup escape.

## Verification Strategy

- Focused contracts then lifecycle/race tests, actual SDK composition, existing Linux/UI gates and independent review. Finally AWS positive-controlled startup/idle/running/terminal/recovery probes and real-model tasks; all attempts retained.

## Risks and Compatibility

- CNI updates are asynchronous: verify existing connections and actual release, preserving explicit failure evidence. Cluster operators remain trusted. Historical snapshots stay unknown; shared workers require drained migration. New shared RBAC needs Owner approval; do not silently restore permissive networking during rollback.

## AWS follow-up: independent worker identity

The 21 October cloud campaign did not pass the online positive control even
though the Pod, grant and compiled PolicyEndpoint matched. Separately, pinned
AWS agent source confirms that same-prefix warm-pool peers share a BPF identity
after the last Pod-name segment is removed. This source-level incompatibility
must be fixed without claiming it has conclusively explained every timeout.

Governed allocations (including offline ones) select `warmpool: none` and a
reserved claim-specific cold name with its digest before a fixed final marker.
The assigned worker must be that dedicated name. Controlled template setup
caches immutable template content without creating a shared warm pool. Preserve
legacy spike behavior and recovery of old recorded identities; do not change
the general naming function or auto-delete old pools. No CNI/add-on, IAM,
Worker-contract or shared API changes. Cold-start latency is the explicit
tradeoff. Test the AWS grouping function as an isolation oracle, plus binding,
replacement, cancellation, deadlines and both recovery layouts; then repeat
real AWS positive/negative controls.
