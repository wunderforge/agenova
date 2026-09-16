# Feature Specification: Reference Platform contract

- Ticket: [#44](https://github.com/wunderforge/agenova/issues/44)
- PRD outcome: [Reference installation and initial policy bootstrap](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Intent

`Platform` is the declarative desired state for one Agenova installation. It says how the control plane is hosted and which named runtime/model capabilities are available. It does not grant installation authority, define agents or tasks, or turn provider configuration into claim authority.

## Canonical Shape

```yaml
apiVersion: agenova.io/v1alpha1
kind: Platform
metadata:
  name: reference-local
spec:
  adapters:
    - name: kubernetes-deployment
      id: agenova.io/deployment/kubernetes
      version: 0.1.0
    - name: agent-sandbox-runtime
      id: agenova.io/runtime/agent-sandbox
      version: 0.1.0
    - name: openai-compatible-model
      id: agenova.io/model/openai-compatible
      version: 0.1.0
  infrastructure:
    deployment:
      name: control-plane
      adapterRef: kubernetes-deployment
      config:
        context: kind-agenova
        namespace: agenova-system
    runtimeBackends:
      - name: primary-runtime
        adapterRef: agent-sandbox-runtime
        config:
          context: kind-agenova
          namespace: agenova-workers
    runtimeProfiles:
      - name: standard-isolated
        backendRef: primary-runtime
        config:
          template: reference-engineer-runtime
  services:
    modelProviders:
      - name: local-model
        adapterRef: openai-compatible-model
        config:
          endpoint: http://127.0.0.1:11434/v1
    modelProfiles:
      - name: coding-standard
        providerRef: local-model
        config:
          model: llama3.1:latest
  initialPolicyRef:
    id: reference-default-deny
    version: "1"
```

Names are installation-local references. Adapter IDs are explicit qualified identities; #151 freezes their final grammar and resolution behavior before lifecycle implementation. `config` is an opaque map to generic Platform code and is parsed/validated only by the selected adapter.

## Requirements

1. The parser accepts strict YAML or JSON for the same versioned contract and rejects unknown/system-managed fields.
2. `metadata.name`, every adapter requirement name, adapter ID/version, instance name and referenced profile are non-empty, bounded and normalized by contract validation.
3. Adapter requirement names are unique. Instance names are unique within their category. Runtime/model profile names are unique and each profile explicitly references one named backend/provider instance.
4. Every `adapterRef` resolves to one declared requirement whose descriptor advertises the required capability: deployment, runtime or model.
5. Exactly one deployment instance, at least one RuntimeBackend instance and one initial Policy reference are required. Model providers may be absent for validation-only installations, but a referenced model profile must resolve before work can be admitted.
6. Selecting deployment never selects or configures runtime/model instances implicitly. `runtimeProfiles[].backendRef` and `modelProfiles[].providerRef` are the only profile-to-instance mappings.
7. Generic validation performs envelope, reference, supported-category and secret-field checks, then invokes adapter-owned side-effect-free validation/canonicalization. #44 has no target-mutation dependency.
8. Reserved credential fields and inline credential-bearing values recognized by the generic boundary or adapter schema are rejected. An adapter may define non-secret credential references, but neither the lock nor diagnostics expose resolved credentials.
9. The initial Policy is a reference only. #46 owns its seed/verification semantics and the operator's existing identity/RBAC authorizes installation.
10. Adapter validation returns a secret-free canonical config digest after applying adapter defaults. The resolved projection and Platform revision are deterministic for semantically equivalent input and contain exact adapter identities/versions/capabilities, named instance/profile paths and canonical config digests—never raw config or secrets.
11. Platform/profile availability never grants an agent permission. AgentTemplate ceilings, policy and ClaimRequest resolution remain authoritative.
12. Unsupported service categories fail explicitly. Tool, Memory and Observability remain absent until #150 defines and delivers their boundaries.

## Negative Cases

- Unknown API version/kind, duplicate names, unknown `adapterRef`, version/capability mismatch or unsupported service category.
- Missing deployment/runtime, duplicate runtime/model profile, unknown instance/profile reference or malformed adapter-owned config.
- Kubernetes/provider fields outside `config`, provider SDK/CRD-shaped shared fields, inline tokens/passwords/secret values, or system-managed status/lock input.
- A validation failure produces no partial lock. The pure resolver has no installer, deployment, runtime, provider or target-mutation dependency; #45 proves fail-before-mutation when it adds reconciliation.

## Compatibility

- `ClaimRequest`, `AgentTemplate`, `SandboxClaim`, `RuntimeBackend`, authority resolution and evidence contracts remain unchanged.
- Existing kind/Ollama demo configuration becomes a future Platform fixture; #44 does not replace current startup paths.
- #151 may extend resolution sources without changing explicit adapter references; #45 consumes this contract to reconcile targets.

## Open Decisions

- Final adapter ID grammar and protocol-compatibility vocabulary are owned by #151. #44 stores the explicit ID/version and validates them through an injected descriptor lookup; it must not invent a second registry.
