# Feature Specification: Reference Platform contract

- Ticket: [#44](https://github.com/wunderforge/agenova/issues/44)
- PRD outcome: [Reference installation and initial policy bootstrap](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Intent

`Platform` is the declarative desired state for one Agenova installation. It says how the control plane is hosted and which named runtime/model capabilities are available. It does not grant installation authority, define agents or tasks, or turn downstream backend configuration into claim authority.

Agenova Model Gateway is a mandatory core service, not a swappable backend in this contract. Platform model configuration selects only what sits behind it:

```text
worker + verified claim context
  -> Agenova Model Gateway
  -> effective Model Profile decision and correlated evidence
  -> selected ModelBackend
```

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
    - name: openai-compatible-backend
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
          connection:
            mode: in-cluster
            namespace: agenova-workers
    runtimeProfiles:
      - name: standard-isolated
        backendRef: primary-runtime
        config:
          isolation: dedicated
  services:
    modelBackends:
      - name: local-ollama
        adapterRef: openai-compatible-backend
        config:
          endpoint: https://ollama.agenova-models.svc.cluster.local/v1
    modelProfiles:
      - name: approved-coding-model
        backendRef: local-ollama
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
3. Adapter requirement names are unique. Instance names are unique within their category. Runtime/model profile names are unique and each profile explicitly references one named backend instance.
4. Every `adapterRef` resolves to one declared requirement whose descriptor advertises the required capability: deployment, runtime or model.
5. Exactly one deployment instance, at least one RuntimeBackend instance, at least one runtime profile mapping and one initial Policy reference are required. ModelBackends may be absent for validation-only installations, but a referenced model profile must resolve before work can be admitted.
6. Selecting deployment never selects or configures runtime/model instances implicitly. `runtimeProfiles[].backendRef` and `modelProfiles[].backendRef` are the only profile-to-instance mappings.
7. Generic validation performs envelope, reference, supported-category and secret-field checks, then invokes adapter-owned side-effect-free validation/canonicalization. Instance config is canonicalized first; profile canonicalization receives both the referenced instance's canonical config and the profile config so the adapter can reject unusable resource relationships. #44 has no target-mutation dependency.
8. Reserved credential fields and inline credential-bearing values recognized by the generic boundary or adapter schema are rejected. Until #155 supplies the typed host-side resolver boundary, any normalized reserved credential name with a `Ref` suffix (including `apiKeyRef`, `tokenRef` and `passwordRef`) is also rejected. Arbitrary adapter error text is never returned; only bounded typed safe diagnostics cross the adapter boundary. Neither the lock nor diagnostics expose credentials.
9. The initial Policy is a reference only. #46 owns its seed/verification semantics and the operator's existing identity/RBAC authorizes installation.
10. Adapter validation returns secret-free canonical config after applying adapter defaults. Runtime pair validation proves the selected profile is usable with the backend connection and rejects unsupported connection/profile combinations. Deployment target context is operator-side input; a runtime used by the deployed control plane must declare an in-cluster connection rather than inherit host kubeconfig. AgentTemplate artifact/entrypoint and any substrate template/pool materialization are supplied through the trusted #147 registration/allocation path, never replaced by a fixed Platform demo image. Internal `ResolvedPlatform` owns a deep normalized snapshot of every accepted JSON-compatible container, so later input mutation cannot diverge from its revision/lock. The public/inspectable lock retains only exact identities, instance/profile paths and centrally computed canonical digests. Both revision and lock are deterministic for semantically equivalent input and never contain credentials.
11. Platform/profile availability never grants an agent permission. AgentTemplate ceilings, policy and ClaimRequest resolution remain authoritative.
12. Unsupported service categories fail explicitly. Tool, Memory and Observability remain absent until #150 defines and delivers their boundaries.
13. All governed model calls use the installed lightweight Agenova Model Gateway. The Platform route fixes that boundary; #121 owns authentication that binds a live caller to one issued claim. Once a call is authoritatively attributed, the Gateway verifies the effective Model Profile, returns the decision, records one correlated claim-scoped `ModelInvocation`, and only then calls the resolved ModelBackend.
14. An unknown or otherwise unattributable caller/claim is denied with zero ModelBackend calls and zero claim-scoped `ModelInvocation` facts. Terminal claim, ungranted profile or failed post-attribution decision records the attributable denial and produces zero ModelBackend calls. An unresolved backend reference is instead a Platform validation failure: no model route is emitted and Gateway invocation is never configured. Live adapter unavailability after valid composition retains the current execution-error semantics and is not reclassified by #44. Workers never receive a backend endpoint or credential and cannot select a backend through ClaimRequest.
15. Ollama, OpenAI, Bedrock and optional LiteLLM are downstream ModelBackends. LiteLLM integration remains #154 and must not replace Gateway governance. Raw credentials remain forbidden; the future host-side `credentialRef`/resolver is #155.

## Negative Cases

- Unknown API version/kind, duplicate names, unknown `adapterRef`, version/capability mismatch or unsupported service category.
- Missing deployment/runtime, a host-context runtime selected for the in-cluster control plane, an unsupported runtime profile/backend pairing, duplicate runtime/model profile, unknown ModelBackend/profile reference or malformed adapter-owned config.
- Kubernetes/provider fields outside `config`, provider SDK/CRD-shaped shared fields, credential references, inline tokens/passwords/secret values, or system-managed status/lock input.
- Any resolved model path that omits the mandatory Gateway, gives the worker a backend endpoint/credential, or permits a denied call to reach a ModelBackend.
- A validation failure produces no partial lock. The pure resolver has no installer, deployment, runtime, provider or target-mutation dependency; #45 proves fail-before-mutation when it adds reconciliation.

## Compatibility

- `ClaimRequest`, `AgentTemplate`, `SandboxClaim`, `RuntimeBackend`, authority resolution and evidence contracts remain unchanged.
- Existing kind/Ollama demo configuration becomes a future Platform fixture; #44 does not replace current startup paths.
- The canonical Kubernetes example uses an in-cluster HTTPS ModelBackend endpoint. A concrete reference fixture must prove the selected endpoint is reachable from the deployed control plane; loopback host configuration cannot be copied into a pod deployment.
- Deployment `config.context` selects the existing target for operator-side plan/apply only. Runtime configuration uses `connection.mode: in-cluster`; it does not copy the operator's host kube-context into the deployed control plane. #146 owns the live in-cluster composition evidence.
- A Platform runtime profile selects connection/isolation capability, not an agent image. #147 resolves the registered AgentTemplate artifact/entrypoint into the runtime allocation and must reject any mismatch; #44 does not claim executable AgentTemplate mapping evidence.
- #151 may extend resolution sources without changing explicit adapter references; #45 consumes this contract to reconcile targets.
- Existing `internal/modelgateway` remains the no-fee reference enforcement/evidence path. #44 configures its downstream ModelBackend; it does not make the core Gateway swappable.

## Open Decisions

- Final adapter ID grammar and protocol-compatibility vocabulary are owned by #151. #44 stores the explicit ID/version and validates them through an injected descriptor lookup; it must not invent a second registry.
