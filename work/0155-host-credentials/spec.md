# Feature Specification: Resolve external credentials behind trusted adapters

- Ticket: [#155](https://github.com/wunderforge/agenova/issues/155)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority)

## Intent

An operator selects an explicit host resolver and an opaque credential reference.
Trusted service adapters resolve it only after admission; worker requests cannot
select a secret, resolver, namespace or identity. Values stay private to the
adapter invocation/lifecycle. One shared contract serves Tool, Model and Memory.

## In Scope

- Typed references, explicit resolver registry, immutable host binding, sanitized failures and bounded scoped material.
- A deterministic test resolver and a namespace/allowlist-bounded Kubernetes Secret implementation.
- Platform shape-only validation/plan, resolver-owned readiness, explicit restart/reconcile reload and regression evidence.

## Out of Scope

- A general secret manager, worker resolution, automatic environment/file discovery, raw credentials in Platform YAML, SSO/workload identity or automatic cloud-provider rotation.

## Requirements

- Given a trusted host binding for a supported reference, resolve only that captured reference with a bounded deadline and value size; never consume a worker-supplied retargeting field.
- The resolver's timeout is resolution-only. After successful resolution, release its child context and pass the original caller-owned invocation context to the trusted consumer; provider deadlines remain the Gateway/adapter owner's responsibility.
- Given malformed or unauthorized input, deny before resolver I/O or consumer external calls. Registry lookup has no implicit defaults or fallback.
- Given an unavailable/missing/forbidden/wrong-kind source, return one stable non-disclosing resolution failure without raw backend text or unrelated existence details.
- Given a resolved value, expose bytes only through host-owned scoped use. Registry/binding formatting and JSON/YAML serialization are redacted; errors never echo material. Trusted callback code must not copy bytes into public structs or diagnostics.
- Secret JSON projections keep encoded field values in clearable owned bytes, decode only the selected string field, and clear raw/encoded/failed decoded buffers. This is owned-buffer hygiene, not heap-wide erasure of library/runtime temporaries.
- Given Platform validate/plan, inspect reference shape and explicit resolver identity without retrieving values. Apply/readiness performs only authorized resolver checks; status remains metadata-only.
- Given rotation, later resolution after supported restart/reconcile observes the replacement; in-flight uses retain their captured value and active claim authority is unchanged. No persistent secret cache is implied.
- Given no real backend configuration, a selected integration campaign fails explicitly. Unit/synthetic tests are not backend acceptance.

## Negative Cases

- Malformed/empty/unknown resolver, invalid name/key, unbound reference, namespace/path injection, missing key, wrong object kind/type, provider outage, cancellation and oversized value.
- Mutable source or registry input after binding cannot expand access or retarget a handle; unauthorized reference names cause zero source calls.
- Raw source errors/output, credential values and decoded Secret objects are absent from logs, status, evidence, manifests and worker configuration.

## Compatibility

- Existing #36 exact-name public/worker credential rejection remains enforced. The coordinated Platform slice enables only the dedicated typed model-backend path; arbitrary config maps retain their rejection rules.
- ClaimRequest, AgentTemplate, claim, authority, Policy and worker identity semantics are unchanged. Kubernetes/provider types remain inside their adapters.
- Cross-provider consumers may prototype against the producer tests but cannot claim accepted/live integration before review and real evidence.

## Open Decisions

- No product/architecture expansion is requested. Verification records remain local at the Owner's request. Independent human review and public reproduction in an explicitly selected environment remain outstanding.


## Platform Slice Refinement - 2026-10-10

- Accept one dedicated `PlatformInstance.credentialRef` only for model backend instances: `{resolverRef, name, key}`. `resolverRef` selects an explicit local resolver instance, not a worker field or implicit resolver ID. All credential-shaped fields inside arbitrary config, profiles, deployment/runtime instances and requests remain rejected.
- Add optional `spec.services.credentialResolvers` instances using the explicit adapter registry's new `credential` category. The reference Kubernetes resolver configuration contains only its namespace; its exact name/key allowlist is captured from the operator's typed model backend references. No independent secret-discovery list or default credential reference is introduced.
- Catalog, validation, canonical resolution and planning perform zero Secret retrieval. Only apply/installed startup readiness checks selected references; per-invocation model resolution follows existing Gateway admission. The first reference composition permits one bundled resolver in the Control Plane namespace and one consistent credential selection for its existing shared endpoint.
- Least-privilege reference RBAC grants only `get` on explicitly selected Secret resourceNames, without list/watch. Tool/Memory/provider-specific integrations remain their consumer work; unaccepted branches are not stacked.
- Later model calls resolve replacement material; supported restart/reconcile rechecks startup readiness without altering active claim authority. Public revision/status/manifests contain only reference metadata; resolved values remain callback/request-local with owned buffers cleared.

- Every actual apply, including unchanged and adapter-activation-only plans, checks selected credentials before reporting readiness or changing local/target state. Validate, plan and status remain Secret-free.
- A model provider success that echoes the exact resolved value in a returned completion, response identifier or configured model is rejected with a fixed error and empty result. HTTP/runtime internal copies are outside owned-buffer erasure guarantees.
