# Technical Design: Resolve external credentials behind trusted adapters

- Ticket: [#155](https://github.com/wunderforge/agenova/issues/155)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

- Accepted main rejects credential values and credential references in Platform configuration; adapter registry supports deployment/runtime/model only. Preserve that fail-closed state until the coordinated Platform slice.
- #36 already excludes credential-bearing task/gateway/launch fields and proves credential-free worker manifests. This Ticket consumes that boundary rather than redefining it.
- No credential resolver exists. The repository targets Go 1.22; do not add a provider SDK or change its toolchain for the first slice.
- No existing cluster context is supplied. Real Kubernetes Secret evidence remains blocked, and no credentials are read during development.

## Decision

Use a small host-only `internal/credentials` boundary: a typed opaque Reference,
explicit immutable registry and host binding, a narrow resolver interface and
callback-scoped byte access. Resolver identity is explicit; references are
operator-captured configuration, never request-derived authority. Registration
captures allowed references and denies unlisted selections before any I/O.

The internal prototype uses qualified `vendor/credential/name` identities and
semantic versions, following #151's identity/version principles. The bundled
resolver is `agenova.io/credential/kubernetes-secret` at `0.1.0`; this refines the
Ticket's explicitly provisional two-part example. Adding the credential category
to the actual Platform adapter catalog/lock is deferred to the coordinated slice,
not silently enabled by this private registry.

Values are bounded, copied into owned storage for scoped use and cleared on
release; host registry/binding formatting and serialization return only a fixed redaction marker.
This is best-effort memory hygiene, not proof against a trusted adapter copying
bytes. The adapter must never put callback material into public outputs.
Errors from resolvers become stable non-disclosing errors; no diagnostic wrapping
of raw provider strings. Check caller cancellation and bound resolution deadlines.
Scoped use returns stable errors only. Adapters retain their own typed business
outcome separately and must not infer rollback or write certainty from a use error.

Keep Kubernetes details under `internal/credentials/kubernetes`: one explicitly
captured namespace/allowlist, a bounded Secret getter, strict kind/name/namespace
and key/value validation, and no list/discovery operation. The live transport is
host-owned and authorized by its Kubernetes identity; scripted getters prove
the contract but not RBAC. The first reference adapter selects Opaque Secrets
only, accepts at most 64 KiB of selected material and 2 MiB of JSON transport,
and requires the [Kubernetes Secret name/key rules](https://kubernetes.io/docs/concepts/configuration/secret/#constraints-on-secret-names-and-data).
ServiceAccount token Secrets are not external provider credentials in this scope.
Missing/forbidden/wrong-kind/backend failure collapse
to a non-disclosing resolution failure. A selected real integration gate must
require explicit context/namespace and report the absent environment.

Do not enable Platform references from the internal foundation alone. A later
coordinated slice will add the typed reference path and explicit resolver
activation using #151 registry principles, with dry validation/plan performing
zero retrieval and readiness owned by the resolver. Unsupported adapters remain
unsupported. Initial reload is restart/reconcile with later resolution; there
is no implicit environment lookup, persistent secret cache or claim restoration.

## Ownership and Contract Boundaries

- E14 #155 owns the common host-only credential boundary and Kubernetes Secret resolver, plus coordinated Platform reference acceptance.
- E4 gateways retain authority checks and own ordering before credential-backed external calls. Tool/Model/Memory consumers receive a host-created binding, never a public resolver endpoint.
- Platform/registry owns resolver selection and adapter activation; credentials and Kubernetes shapes do not enter effective authority or claims.
- Synthetic test material is isolated from public contracts and fixtures. Provider SDK integration and E17 PostgreSQL driver selection remain consumer work after acceptance.

## Alternatives Considered

- Per-provider raw environment/DSN configuration: rejected because it duplicates secret contracts and permits implicit lookup.
- A generic secret-management service or broad Kubernetes client SDK: deferred; one bounded resolver and narrow host transport suffice for this Ticket.
- Returning an exported value-bearing public struct: rejected because ordinary serialization/formatting would leak material.
- Merging the E17 branch into this producer: rejected; producer acceptance must be independently reviewable from main.

## Verification Strategy

- Contract/spy tests cover success, immutable bindings, registry validation, all named zero-call negatives, cancellation, bounded values, release and redacted diagnostics/serialization.
- Secret adapter tests cover exact scoped lookup, kind/name/namespace/key failures, synthetic rotation and provider/error sentinels with consumer external-call counters.
- Re-run #36 public/gateway/launch/worker regressions, focused race/repeated tests, then `check.ps1 -All`; capture sanitized output under task-owned evidence.
- Real Secret/RBAC/reload requires an explicit existing cluster and independent zero-call controls; absence is not silently skipped or called passing evidence.

## Risks and Compatibility

- Go cannot prevent a trusted callback copying/logging bytes; scoped APIs and synthetic exclusion tests reduce accidental escape but do not claim hostile-host isolation.
- A stable host resolver contract does not make credentialed installed consumers ready. Platform integration, real Secret evidence and independent review remain required.
- Kubernetes namespace/allowlist is trusted operator state; caller intent or a bare secret name cannot create authority.
- Rotation has no automatic hot-cache guarantee. Restart/reconcile must refresh subsequent host bindings without widening active claim authority.

