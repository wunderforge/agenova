# Technical Design: Bundled adapter registry and lifecycle

- Ticket: [#151](https://github.com/wunderforge/agenova/issues/151)
- Feature spec: `spec.md`

## Current State and Constraints

- #44 defines a pure `platform.DescriptorLookup`, adapter-owned instance/profile canonicalization, deterministic `ResolvedPlatform` and digest-only lock.
- Reference descriptor doubles currently live only in Platform tests; no production catalog, activation lock or lifecycle CLI exists.
- `internal/cli` is provider-neutral and `cmd/agenova`/`internal/app` are the composition edge.
- #45 must reuse this lifecycle service and cannot add a second lookup/installer.

## Decision

Add three layers:

1. `adapterregistry.Registry` is an immutable, explicitly constructed catalog of validated `Registration` values. Each registration contains a public manifest, the existing pure Platform descriptor and a factory per advertised capability. A factory returns the capability-owned implementation as `any`; consumers assert their own narrow interface, so no universal operational API is introduced.
2. `adapterregistry.Lifecycle` combines a catalog with an injected `Store`. It owns install/list/inspect/init and also delegates descriptor lookup for Platform resolution. `MemoryStore` supports tests/composition; `FileStore` writes one canonical, secret-free lock atomically beneath the selected state directory.
3. `internal/adapters/bundled` explicitly builds the three reference registrations. `internal/app` composes them. `internal/cli` consumes only lifecycle methods and never switches on provider IDs.

The CLI accepts an explicit `--state-dir` for adapter activation state; the executable otherwise uses the OS user config directory. This identifies a local Agenova installation, not a cluster credential or authority grant.

## Ownership and Contract Boundaries

- `internal/adapterregistry`: identity parsing, manifests, schema/default projection, registration validation, deterministic registry, store and lifecycle.
- `internal/adapters/bundled`: reference adapter manifests, canonicalizers, defaults and capability-specific implementation constructors.
- `internal/app`: production composition and safe state-directory selection.
- `internal/cli`: subcommand parsing/presentation only.
- `cmd/agenova`: wires the lifecycle factory.
- `internal/platform`: unchanged descriptor/resolution authority.

The installation lock contains only `{id, version, protocol, capabilities}` and update time is deliberately omitted so idempotent output remains deterministic. Init fragments contain explicit reference configuration defaults but never secrets.

## Alternatives Considered

- Package `init()` registration: rejected because order, duplicates and available code become implicit.
- Directory/executable scanning: rejected because it broadens into untrusted plugin loading and violates #151 non-goals.
- One `Adapter` operational interface: rejected because deployment, runtime and model operations do not share useful semantics.
- CLI-only catalog maps: rejected because #45 must call the same programmatic lifecycle.
- Platform YAML as activation state: rejected because desired instances and implementation availability are distinct; Platform apply will reconcile them through this lifecycle.

## Verification Strategy

- Registry table tests for identity, duplicate, protocol, capabilities, schema and factory behavior.
- Lifecycle/store tests for first/idempotent/conflicting install, atomic no-change failures and corrupt state.
- Bundled adapter test composing generated fragments into one strict Platform and resolving it through the same registry.
- CLI exact JSON/YAML tests and compiled smoke using `t.TempDir()` as `--state-dir`.
- Focused packages, Platform contract tests and full repository gate.

## Risks and Compatibility

- File locking across concurrent processes is not required by this ticket; atomic replacement prevents partial files, and #45 may add a control-plane reconciliation lock if needed.
- Schema defaults are reference starting values, not proof that a target exists or is reachable. #45 owns target validation/readiness.
- The factory result is intentionally capability-owned. Callers must fail with a capability/type diagnostic rather than assuming an implementation type.
