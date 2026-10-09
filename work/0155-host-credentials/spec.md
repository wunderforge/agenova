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
- Given malformed or unauthorized input, deny before resolver I/O or consumer external calls. Registry lookup has no implicit defaults or fallback.
- Given an unavailable/missing/forbidden/wrong-kind source, return one stable non-disclosing resolution failure without raw backend text or unrelated existence details.
- Given a resolved value, expose bytes only through host-owned scoped use. Registry/binding formatting and JSON/YAML serialization are redacted; errors never echo material. Trusted callback code must not copy bytes into public structs or diagnostics.
- Given Platform validate/plan, inspect reference shape and explicit resolver identity without retrieving values. Apply/readiness performs only authorized resolver checks; status remains metadata-only.
- Given rotation, later resolution after supported restart/reconcile observes the replacement; in-flight uses retain their captured value and active claim authority is unchanged. No persistent secret cache is implied.
- Given no real backend configuration, a selected integration campaign fails explicitly. Unit/synthetic tests are not backend acceptance.

## Negative Cases

- Malformed/empty/unknown resolver, invalid name/key, unbound reference, namespace/path injection, missing key, wrong object kind/type, provider outage, cancellation and oversized value.
- Mutable source or registry input after binding cannot expand access or retarget a handle; unauthorized reference names cause zero source calls.
- Raw source errors/output, credential values and decoded Secret objects are absent from logs, status, evidence, manifests and worker configuration.

## Compatibility

- Existing #36 exact-name public/worker credential rejection remains enforced. Platform credential references stay unsupported until the coordinated Platform slice enables an explicit typed path; no parser-wide exception is introduced by the internal foundation.
- ClaimRequest, AgentTemplate, claim, authority, Policy and worker identity semantics are unchanged. Kubernetes/provider types remain inside their adapters.
- Cross-provider consumers may prototype against the producer tests but cannot claim accepted/live integration before review and real evidence.

## Open Decisions

- No product/architecture expansion is requested. The user reports no kind context; live Secret/RBAC/reload and independent reproduction are outstanding environmental/review gates.
