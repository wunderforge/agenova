# Technical Design: Freeze the supported Agent Sandbox mapping and gaps

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

- #30 defines five backend-neutral operations. The Agent Sandbox adapter translates allocation, identity-matched readiness, and confirmed cleanup while explicitly rejecting Start and Terminate.
- #66 provides retained v0.4.6 source, source hashes, four CRD schemas, and one bounded kind transcript for Allocate/Observe/unsupported Start/Terminate/Cleanup.
- #89 defines FilesystemBoundary; the adapter currently returns only `EvidenceLevel=Unsupported`.
- The formal adapter note still says the reduced-contract real-cluster run is blocked and leaves #48's filesystem mapping pending, so it no longer matches merged evidence.

## Decision

Treat #48 as a contract-classification and evidence-reconciliation change. Update the formal adapter note, package comment, status snapshot, and existing support-matrix evidence to one frozen table. Keep executable adapter behavior and all shared contracts unchanged. Reuse retained #66 real-cluster evidence for unchanged backend claims; reserve new worker and isolation experiments for #51.

The frozen table is:

| Semantic | Classification | Evidence boundary |
| --- | --- | --- |
| Allocate | `translated` | SandboxClaim creation plus observed assigned sandbox; recovery gaps remain explicit. |
| Observe/readiness | `translated` | Identity-matched Ready condition; Bound only. |
| Backend identity | `translated` | Neutral Backend/WorkerID from assigned sandbox name; process-lifetime resolution. |
| Start | `unsupported` | No explicit work-start acknowledgement. |
| Terminate | `unsupported` | No stop evidence independent of deletion. |
| Cleanup | `translated` | Claim and recorded sandbox both confirmed absent; no replacement inference. |
| Durability/restart | `unsupported` | Allocation, reservation, and release correlation remain in memory. |
| FilesystemBoundary | `unsupported` | Only Unsupported evidence level; no verified layout/retention fields. |
| General isolation | `unknown` | Requires #51 worker probes. |

## Ownership and Contract Boundaries

- `docs/backends/agent-sandbox.md` owns the formal supported mapping and gap narrative.
- `internal/runtime/agentsandbox/doc.go` summarizes the same adapter behavior without adding exported/provider types.
- The existing reduced-contract test remains the executable classification oracle; only stale evidence wording changes.
- `docs/project-status.md` reports the merged state after acceptance. Task-local evidence records reproducibility, not a second project status.
- `RuntimeBackend`, FilesystemBoundary, APIs, lifecycle, authority, facts, gateways, manifests, and the v0.4.6 adapter wire types do not change.

## Alternatives Considered

- Promote Start from Pod readiness: rejected because Ready proves infrastructure readiness, not Agenova work start.
- Implement Terminate as claim deletion: rejected because deletion lacks independent worker/descendant-stop evidence and collapses termination into cleanup.
- Populate `/workspace` and mark filesystem support from PodSpec configuration: rejected because configuration capability is not real worker enforcement evidence.
- Repeat the #66 kind run unchanged: rejected because it would add no new evidence; #51 owns the broader G5 proof.
- Rename or promote SpikeAdapter: rejected because durability, filesystem, Start, Terminate, and production-operability gaps remain.

## Verification Strategy

- Run the entire Agent Sandbox unit/integration-compilation package with verbose output, including the reduced support matrix, identity mismatch, unsupported Start/Terminate, cleanup confirmation, and unsupported filesystem assertions.
- Run documentation/boundary checks to catch provider vocabulary outside the adapter and stale/broken evidence links.
- Run the full repository gate to detect regressions in runtime consumers and generated/frontend contracts.
- Record commit/tree, UTC timestamp, exact command, complete output, exit code, and relevant source hashes in task-local evidence.
- Link the retained #66 kind transcript for real-backend support claims. Do not label the #48 focused tests as real-cluster evidence.

## Risks and Compatibility

- The largest risk is overstating support through wording. Every translated result must name both the verified behavior and its residual gap.
- `Observe` and Cleanup rely on process-local allocation records. Documentation must not imply restart recovery.
- Binding verification and deletion are separate Kubernetes calls. Documentation must retain the non-atomic identity/precondition gap.
- Empty FilesystemBoundary fields mean unsupported evidence, not unrestricted access or retention.
- If source changes beyond documentation and test wording, fresh focused and real-backend evidence must be reconsidered before delivery.
