# Technical Design: Reference Platform contract

- Ticket: [#44](https://github.com/wunderforge/agenova/issues/44)
- Feature spec: `spec.md`

## Current State and Constraints

- `cmd/agenova-console` constructs Agent Sandbox, warm-pool, worker image/command, principal and downstream model backend from flags/code; `agenova run` has a separate reference composition.
- Stable public contracts already keep backend/provider shapes outside claim and application types. The new contract must preserve that boundary.
- #45 owns reconciliation, #151 owns the concrete adapter registry/lifecycle, and #146 owns shared long-running composition. #44 supplies only their reviewed declarative input and pure resolution contract.
- The MVP installs onto an existing authenticated target and does not create clusters or derive operator authority from YAML.

## Decision

Add a small `api/v1alpha1` Platform contract with three layers:

1. `spec.adapters` declares exact implementation requirements by local reference, qualified ID and version.
2. `spec.infrastructure` and `spec.services` contain typed categories of named instances and separate explicit profile mappings. Each profile references one instance; instance/profile `config` remains adapter-owned. Model instances are named `ModelBackend`, never Gateway/provider replacements.
3. A pure resolver validates references/capabilities/config and returns both an actionable internal `ResolvedPlatform` and deterministic inspectable lock projection. Its descriptor port offers lookup plus side-effect-free validation/canonicalization only; it has no target mutation methods.

The core Model Gateway is mandatory and not selected by Platform configuration. A resolved model route is always `{profile, gateway: agenova-core, backendRef}`. The existing lightweight in-process `internal/modelgateway` remains the MVP implementation; #154 may add LiteLLM only as a downstream backend.

Use typed category fields rather than a universal `kind` list. This keeps deployment/runtime/model capability boundaries readable and prevents a single operational interface from being implied. Add service categories only when implemented.

## Ownership and Contract Boundaries

- `api/v1alpha1`: Platform envelope, metadata, adapter requirements, typed instance/reference structs, strict decode and structural validation. No backend/provider imports.
- A narrow descriptor lookup interface returns identity, version, declared capabilities and a side-effect-free validator/canonicalizer for instance/profile config. Canonicalization applies adapter defaults and returns secret-free actionable config plus its digest, not backend objects or resolved credentials. #151 will implement it with the explicit bundled registry.
- `internal/modelgateway`: mandatory verified-claim/profile decision, invocation ID correlation and ModelInvocation evidence before the selected backend adapter is called. This governance boundary is consumed, not replaced, by Platform composition.
- `internal/platform` performs ordered structural → reference/capability → adapter-owned config validation, canonical projection and revision hashing.
- `harness/fixtures/contract/v0`: canonical valid YAML/JSON and named invalid fixtures.
- `internal/platform` exposes the deterministic resolved projection that #45's operator plan will consume; no target reconcile is added in #44.

Proposed internal/app-facing resolution and inspectable lock:

```text
ResolvedPlatform             # internal input to #45
  platformName, revision
  instances[]                # canonical secret-free actionable config
  profiles[]                 # canonical secret-free actionable config
  modelRoutes[]              # fixed Gateway plus downstream backend ref
  initialPolicyRef

PlatformLock
  platformName
  revision              # digest of canonical desired state
  adapters[]            # local ref, qualified ID, exact version, capabilities
  instances[]           # category/path, name, adapter ref, canonical config digest
  profiles[]            # capability/path, name, explicit instance ref, canonical config digest
  modelRoutes[]         # profile, fixed agenova-core gateway, downstream backend ref
  initialPolicyRef      # reference only
```

Raw config, backend output and resolved credentials never enter the lock. `ResolvedPlatform` retains only canonical secret-free configuration/credential references needed by #45. Input order plus adapter-owned defaults are normalized for revision/plan stability; diagnostics retain field paths.

## Resolution Order

1. Strict-decode and validate envelope/field bounds.
2. Reject secret-bearing fields and unsupported service categories.
3. Index unique adapter requirements and named instances.
4. Resolve descriptors; validate exact identity/version and required capability.
5. Validate unique profile names and resolve every runtime/model profile `backendRef` to a capability-compatible named instance.
6. Invoke the resolved adapter's side-effect-free config validator/canonicalizer for instance and profile config.
7. For every model profile, emit a route through the fixed Agenova Gateway to exactly one resolved ModelBackend.
8. Emit actionable `ResolvedPlatform`; hash its canonical secret-free representation into the inspectable lock/plan projection.

Any error returns no resolved result. There is no mutation seam in #44; #45 later proves no target mutation on validation failure.

## Alternatives Considered

- **One flat universal adapter-instance list:** rejected because it obscures infrastructure/service meaning and suggests one operational adapter interface.
- **Infer adapters from config fields or deployment target:** rejected because it creates hidden coupling and makes manifests non-deterministic.
- **Embed Policy, AgentTemplate and Work:** rejected because they have independent authority/lifecycle contracts and would create a universal YAML.
- **Put backend/provider fields directly in shared structs:** rejected because it leaks Kubernetes/model shapes into product contracts.
- **Make LiteLLM or a provider proxy the Gateway:** rejected because routing software does not own Agenova claim validation, effective authority, decisions or canonical evidence.
- **Allow workers to call ModelBackends directly:** rejected because denied calls could bypass Gateway enforcement/evidence and workers would require long-lived backend credentials.
- **Implement registry/apply now:** rejected because #151/#45 own those independently reviewable behaviors.

## Verification Strategy

- Round-trip canonical YAML/JSON fixtures and reject unknown/system-managed fields.
- Table-test names, reference integrity, capability/version mismatch, profile uniqueness, unsupported categories and secret-bearing config.
- Descriptor/config-canonicalizer spies prove correct adapter dispatch after reference resolution, default normalization, actionable canonical config and no partial result/lock on failures; an import/interface check proves no target-mutation dependency.
- Golden resolved lock/plan proves stable ordering/digest and independent deployment/runtime/model selection.
- Model-route fixtures and existing `internal/modelgateway` negative tests prove every route retains the core Gateway and denied/ungranted calls make zero backend calls with correlated decision evidence.
- Boundary scan ensures Platform API packages do not import Kubernetes/provider SDKs; run the full repository gate.

## Risks and Compatibility

- Opaque config can hide secrets unless generic reserved-key checks and adapter validators both run; tests must cover nested maps/lists, canonical actionable config/digest stability and redacted diagnostics.
- Freezing adapter ID grammar here would compete with #151. Store explicit strings now and require descriptor equality; land final grammar with #151 before lifecycle commands.
- A typed services section must not advertise future integrations. The first implementation includes downstream ModelBackends only; later approved service contracts add fields compatibly.
- The current backend adapter has host-side API-key configuration. #44 must not serialize it; credentialed Platform routes stay unavailable until #155 supplies typed host-side references/resolution.
- No running claim is affected by this contract-only ticket. #146 must snapshot resolved bindings so later applies cannot reroute or expand active work.
