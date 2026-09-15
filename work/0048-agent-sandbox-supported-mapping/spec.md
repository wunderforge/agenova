# Feature Specification: Freeze the supported Agent Sandbox mapping and gaps

- Ticket: [#48](https://github.com/wunderforge/agenova/issues/48)
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution)

## Intent

Publish one stable, evidence-backed interpretation of Agent Sandbox v0.4.6 at the RuntimeBackend boundary so downstream code can consume the supported subset without inferring work start, worker stop, durability, filesystem guarantees, or isolation from weaker Kubernetes signals.

## In Scope

- The five RuntimeBackend v0 operations, neutral backend identity, restart durability, FilesystemBoundary, and the residual general-isolation statement.
- Source, simulated-controller, and retained real-cluster evidence already accepted through #30, #66, #89, and #124.
- Formal backend/package/status documentation and focused support-matrix evidence.

## Out of Scope

- New runtime operations or public fields.
- Production adapter promotion, durable recovery, work execution protocol, termination protocol, filesystem/mount implementation, gateway integration, or new cluster experiments.
- The #51 real-worker proof and the #49 claim/evidence round trip.

## Requirements

- Given an allocation request accepted by the current adapter, when the upstream controller assigns a Sandbox, then `Allocate` is classified `translated` and returns only the neutral claim/worker identity and explicitly unsupported filesystem evidence.
- Given a known identity, when the same upstream claim still names that worker, then `Observe` is classified `translated`; `Ready=True` is Bound-level evidence only, while mismatches and query failures remain errors.
- Given a valid Ready allocation, when `Start` is called, then the adapter returns `ErrUnsupported`; readiness is never elevated to work-start evidence.
- Given a valid unreleased allocation, when `Terminate` is called, then the adapter returns `ErrUnsupported`; resource deletion and cleanup are not worker-stop evidence.
- Given the recorded worker binding, when Cleanup deletes the claim and independently confirms both claim and sandbox absence, then Cleanup is classified `translated` and may report `Released=true`; `Replaced` remains false.
- Given an adapter process restart, when no durable allocation/release correlation can reconstruct the neutral identity, then durability is classified unsupported rather than adapter-held parity.
- Given the current Agent Sandbox template and allocation path, when filesystem evidence is requested, then `EvidenceLevel` remains `Unsupported`, the other FilesystemBoundary fields remain empty/zero, and general isolation remains unknown.
- Given #51 later produces real worker evidence, when its results are accepted, then a separate ticket may promote only the specifically proven filesystem fields or operations; #48 does not pre-authorize that promotion.

## Negative Cases

- No upstream field, source reference, focused test, or retained real-cluster behavior means no `native` or `translated` classification.
- Pod readiness, pod process existence, Finished conditions, claim deletion, pool replica counts, or local adapter flags cannot substitute for Agenova Start, Terminate, application outcome, replacement, or durability semantics.
- Configurability of PodSpec volumes, security context, HOME, temp, or cache does not establish worker-visible enforcement or `BackendVerified` filesystem evidence.

## Compatibility

- RuntimeBackend and all API/v1alpha1 types remain byte-for-byte unchanged.
- Current adapter behavior remains unchanged: existing callers continue to receive unsupported Start/Terminate/filesystem results and confirmed Cleanup release only.
- Provider-specific vocabulary remains in the adapter/spike boundary. The in-memory runtime remains the full shared-contract oracle.

## Open Decisions

- None. Any request to implement Start, Terminate, restart recovery, or filesystem enforcement expands the Ticket and requires a revised packet plus new approval.
