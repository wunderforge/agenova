# Feature Specification: Bundled adapter registry and lifecycle

- Ticket: [#151](https://github.com/wunderforge/agenova/issues/151)
- PRD outcome: [Reference installation and initial policy bootstrap](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Intent

Agenova needs one explicit answer to two different questions: which adapter implementations this distribution can resolve, and which exact versions a selected installation has activated. The catalog registry owns the first; an injected lifecycle store owns the second. Platform validation and later apply use these same boundaries instead of command-specific provider branches.

## In Scope

- Exact qualified identity and version selection.
- Manifest protocol, capability, configuration-schema and capability-factory contracts.
- Deterministic catalog, inspect, install/list lock and Platform-fragment initialization.
- Bundled deployment, runtime and model reference registrations.
- Human and JSON CLI representations.

## Out of Scope

- Fetching external packages or executing untrusted code.
- Infrastructure reconciliation, credentials, policy mutation or claim authority.
- One operational interface shared by every adapter kind.

## Requirements

- Given valid explicit registrations, when the registry is constructed, then catalog order and lookup are deterministic and the registry satisfies `platform.DescriptorLookup`.
- Given an invalid identity, protocol, capability/schema pair, missing factory or duplicate ID/version, when registration is attempted, then the registry rejects the complete construction without a partial returned registry.
- Given an empty selected-installation lock, when an exact catalog adapter is installed, then one exact identity/version is recorded atomically; repeating it reports unchanged.
- Given the same qualified ID at another version is already active, when install is requested, then it fails without changing the lock.
- Given a bundled adapter and local name, when init is requested, then adapter-owned schemas produce a secret-free Platform fragment with one explicit requirement and the appropriate deployment/backend/profile section.
- Given catalog/list/inspect/install/init with `--json`, when the command succeeds, then exactly one stable JSON document is written to stdout.
- Given an init command without `--json`, when it succeeds, then reviewable YAML is written to stdout.
- Given a Platform assembled from generated fragments, when it is parsed and resolved, then resolution uses the same registry and succeeds without provider branches.

## Negative Cases

- Unknown/ambiguous identity or version, malformed local name, corrupt lock, conflicting active version and unsupported subcommand fail closed.
- An adapter schema containing credential-bearing fields or unsupported value shapes is rejected before it can emit a fragment.
- Install/init output must not imply authority or include provider credentials.

## Compatibility

- Existing `agenova run`, help, version and backend-neutral composition behavior remain unchanged.
- `internal/platform.Resolve` keeps its existing `DescriptorLookup` contract; the registry implements it rather than changing Platform semantics.
- Existing Platform YAML identity/version values remain valid.

## Open Decisions

- None for this ticket. External package sources and credential references remain #149 and #155.
