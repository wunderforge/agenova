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

The core Model Gateway is mandatory and not selected by Platform configuration. A resolved model route is always `{profile, gateway: agenova-core, backendRef}`. The existing lightweight in-process `internal/modelgateway` remains the MVP implementation; #121 authenticates live caller-to-claim binding and #154 may add LiteLLM only as a downstream backend. #44 preserves these boundaries in desired state but does not claim to implement either downstream behavior.

Use typed category fields rather than a universal `kind` list. This keeps deployment/runtime/model capability boundaries readable and prevents a single operational interface from being implied. Add service categories only when implemented.

## Ownership and Contract Boundaries

- `api/v1alpha1`: Platform envelope, metadata, adapter requirements, typed instance/reference structs, strict decode and structural validation. No backend/provider imports.
- A narrow descriptor lookup interface returns identity, version, declared capabilities, a side-effect-free instance canonicalizer and a side-effect-free profile-pair canonicalizer. The pair canonicalizer receives the referenced instance's canonical config plus the profile config so backend connection and the selected profile cannot validate independently into an unusable combination. Canonicalization applies adapter defaults and returns secret-free actionable config, not backend objects, digests or resolved credentials. Generic Platform resolution centrally computes every config digest from the same frozen canonical JSON/SHA-256 contract as the revision. #151 will implement the descriptors with the explicit bundled registry.
- `internal/modelgateway`: once #121 supplies authenticated claim context, mandatory profile decision, invocation ID correlation and attributable ModelInvocation evidence before the selected backend adapter is called. This governance boundary is consumed, not replaced, by Platform composition.
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

Raw config, backend output and credentials never enter the lock. `ResolvedPlatform` retains only canonical secret-free actionable configuration needed by #45. Until #155 lands, credential references are rejected instead of retained. Input order plus adapter-owned defaults are normalized for revision/plan stability; diagnostics retain field paths.

## Resolution Order

1. Strict-decode and validate envelope/field bounds.
2. Reject secret-bearing fields and unsupported service categories.
3. Index unique adapter requirements and named instances.
4. Resolve descriptors; validate exact identity/version and required capability.
5. Validate unique profile names and resolve every runtime/model profile `backendRef` to a capability-compatible named instance.
6. Invoke the resolved adapter's side-effect-free instance canonicalizer, then pass that canonical instance config together with each referencing profile config to adapter-owned pair canonicalization. For the reference Agent Sandbox path, reject host-context runtime configuration for the deployed in-cluster service and unsupported isolation/profile combinations. Artifact-specific substrate resources are materialized from the resolved AgentTemplate by #147, not invented by Platform.
7. For every model profile, emit a route through the fixed Agenova Gateway to exactly one resolved ModelBackend.
8. Build a canonical secret-free desired-state payload that explicitly excludes computed fields such as `revision`. Sort adapter requirements, instances, profiles and model routes by category then name, and sort each descriptor's capability set; preserve order-sensitive values such as commands, while adapter canonicalizers normalize their own set-like arrays. Recursively sort adapter-config object keys and encode UTF-8 JSON with Go `encoding/json.Encoder`, `SetEscapeHTML(false)`, then remove the encoder's single trailing newline; emit no other insignificant whitespace. Compute SHA-256 over those exact bytes and represent the revision as `sha256:` followed by 64 lowercase hexadecimal characters. The resolver centrally computes instance/profile config digests with the identical byte/algorithm/text contract; descriptors never supply persisted digests.

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
- Descriptor/config-canonicalizer spies prove correct adapter dispatch after reference resolution, default normalization, backend/profile pair validation, actionable canonical config and no partial result/lock on failures; an import/interface check proves no target-mutation dependency.
- Golden resolved lock/plan asserts the exact `sha256:<lowercase-hex>` revision, stable ordering/digest and independent deployment/runtime/model selection. Recomputing from the same canonical JSON desired-state bytes produces the same revision because computed fields are outside the digest input.
- Runtime fixtures distinguish the operator-side deployment target from the control plane's in-cluster runtime connection and reject an unsupported connection/profile pair. #147 supplies separate evidence that a registered AgentTemplate artifact/entrypoint, rather than a fixed demo image, reaches allocation.
- Determinism tests reverse descriptor capability order and prove the same revision/lock; config digest assertions use the centrally frozen `sha256:<lowercase-hex>` format.
- An unknown backend reference fails Platform resolution before a model route exists. Existing Gateway adapter-unavailability behavior remains an execution error; #44 does not claim a runtime denial that shared composition cannot enforce.
- Model-route fixtures prove every route retains the core Gateway. Existing `internal/modelgateway` negative tests preserve zero-backend-call behavior and the existing attributable/unattributable evidence distinction; authenticated caller-to-claim mismatch evidence remains #121.
- Boundary scan ensures Platform API packages do not import Kubernetes/provider SDKs; run the full repository gate.

## Risks and Compatibility

- Opaque config can hide secrets unless generic reserved-key checks and adapter validators both run; tests must cover nested maps/lists, canonical actionable config/digest stability and redacted diagnostics.
- Freezing adapter ID grammar here would compete with #151. Store explicit strings now and require descriptor equality; land final grammar with #151 before lifecycle commands.
- A typed services section must not advertise future integrations. The first implementation includes downstream ModelBackends only; later approved service contracts add fields compatibly.
- The current backend adapter has host-side API-key configuration. #44 must not serialize it and rejects credential references; credentialed Platform routes stay unavailable until #155 supplies typed host-side references/resolution.
- No running claim is affected by this contract-only ticket. #146 must snapshot resolved bindings so later applies cannot reroute or expand active work.
- The current host-driven Agent Sandbox adapter does not yet provide deployed in-cluster connection mode, and its allocation request does not carry the resolved AgentTemplate artifact. #146 and #147 respectively own those executable paths; #44 defines their explicit inputs without claiming them complete.
