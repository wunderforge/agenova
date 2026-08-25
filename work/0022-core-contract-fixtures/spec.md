# Feature Specification: Freeze the first executable contract fixtures

- Ticket: [#22](https://github.com/wunderforge/agenova/issues/22)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution), [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and [MVP Acceptance Scenario](../../docs/product/prd.md#acceptance-scenario)

## Intent

Give contract producers and fixture-first consumers one provider-neutral v0 example set for the canonical Team A engineer flow, pre-claim denial, and the invalid boundaries each later E1 validator must enforce. The fixtures are executable inputs and expectations, not an additional product schema or runtime implementation.

## In Scope

- `harness/fixtures/contract/v0/manifest.json` with `schemaVersion` and a `cases` array.
- Each case records `id`, `subject`, `purpose`, `input`, `format`, `coverage`, and `expected`.
- `expected.outcome` is `valid` or `invalid`; invalid cases also carry one stable `expected.category`.
- YAML `AgentTemplate` inputs for human-authored reusable configuration.
- A valid YAML `ClaimRequest` and equivalent API JSON input for the payment-timeout scenario.
- JSON system-issued-state inputs for the Team A claim and Team B pre-claim denial evidence.
- Focused negative inputs named by E1-T2, E1-T3, and E1-T4.

## Out of Scope

- Go product structs and semantic validation functions.
- Generated JSON Schema, OpenAPI, CRDs, version migration, or public certification/conformance APIs.
- Backend-specific allocation, Kubernetes resources, gateway transports, or live credentials.

## Requirements

- Given the manifest, when inventory tests run, then every case ID and input path is unique, every file parses in its declared format, every purpose/coverage/expectation is non-empty, and invalid cases name an error category.
- Given the format boundary, when fixtures are reviewed, then human-authored AgentTemplate and ClaimRequest configuration is represented as YAML, ClaimRequest API equivalence is represented as JSON, and system-issued state/evidence is represented as JSON.
- Given the canonical request YAML and API JSON cases, when normalized as data, then they are semantically equal.
- Given the fixture inventory, when coverage is aggregated, then it includes `AgentTemplate`, `Principal`, `Action`, `ClaimRequest`, `PolicyReference`, `EffectiveAuthority`, `SandboxClaim`, `Decision`, and `Evidence`.
- Given the canonical Team A scenario, when downstream work selects fixtures by stable ID, then it can load the engineer template, both request encodings, and one issued-state snapshot without copying those files.
- Given a Team B denial, when evidence is inspected, then it has request/decision correlation and no fabricated claim ID or backend allocation.
- Given template negatives, then cases exist for missing artifact, missing entrypoint, malformed capability ceiling, embedded issued authority, and a secret-value field.
- Given request negatives, then cases exist for missing template, task, or runtime data; self-asserted principal; and an external secret-value field.
- Given system-issued-state negatives, then cases exist for caller-supplied effective authority, claim phase, and backend identity.
- Given fixture bytes, when boundary scans run, then provider vocabulary and common live-secret signatures are rejected.

## Negative Cases

- Duplicate or malformed case ID.
- Missing/nonexistent/duplicate input path, unsupported format, or unreadable JSON/YAML.
- Missing purpose, coverage, expected outcome, or invalid-case category.
- Missing required case or required shape coverage.
- Drift between canonical request YAML and API JSON.
- Provider-specific vocabulary or live-secret signature in fixture input.

## Compatibility

- Existing `api/v1alpha1` runtime-spike types remain unchanged; these fixtures define target v0 cases for E1-T2 through E1-T4 rather than retrofitting provider-oriented spike types.
- Existing runtime, gateway, facts, and Agent Sandbox tests remain unchanged.
- E1-T2 through E1-T4 consume the shared files directly and own semantic validators; they must not fork private case copies.

## Open Decisions

- None for E1-T1. Exact public schema fields remain reviewable and are implemented/validated only by E1-T2 through E1-T4.
