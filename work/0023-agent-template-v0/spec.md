# Feature Specification: Define and validate AgentTemplate v0

- Ticket: [#23](https://github.com/wunderforge/agenova/issues/23)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution)

## Intent

Define one human-authored, backend-neutral v0 contract for a reusable agent role. The contract identifies a runnable artifact and entrypoint, supplies safe reusable defaults, and caps the tools, resources, model profiles, memory scopes, runtime profiles, and execution time that later request resolution may grant. It is configuration and maximum authority, never issued run state.

## In Scope

- Public Go types in `api/v1alpha1` for `AgentTemplate`, its spec, artifact, entrypoint, defaults, and capability ceiling.
- A YAML parser and deterministic semantic validator for the v0 human-authored surface.
- Categorized validation failures compatible with the six merged AgentTemplate fixture cases.
- Fixture-driven tests that use the canonical v0 manifest and input paths directly.

## Out of Scope

- Template storage, lookup, registry APIs, version conversion, generated JSON Schema/OpenAPI/CRDs, admission webhooks, or CLI commands.
- Prompt/instruction content, task semantics, policy evaluation, requested-access intersection, effective authority, claim state, backend identity, or runtime allocation.
- Provider-specific image/runtime objects; `artifact.image` is an opaque runnable artifact reference rather than a Kubernetes or other backend resource.
- Secret references or credential delivery design beyond rejecting raw credential-bearing fields in v0.

## Requirements

- Given the canonical engineer YAML, when it is parsed and validated, then it yields `apiVersion=agenova.io/v1alpha1`, `kind=AgentTemplate`, metadata name `engineer`, the declared artifact and command, safe defaults, and the complete capability ceiling.
- Given an AgentTemplate, when `artifact.image` or `entrypoint.command` is absent or blank, then validation fails with `required-field` and identifies the responsible field.
- Given `capabilityCeiling`, when a collection such as `tools` is encoded as a scalar instead of a list, then parsing/validation fails with `invalid-capability-ceiling`.
- Given template defaults, when a default model profile or memory scope is outside the corresponding capability ceiling, then validation fails with `invalid-capability-ceiling`; reusable defaults cannot enlarge the ceiling.
- Given a capability ceiling, when a supplied list contains blank or duplicate entries or `maxTimeout` is not a positive Go duration, then validation fails with `invalid-capability-ceiling`.
- Given caller-authored `effectiveAuthority` anywhere under the template spec, when the document is parsed, then it fails with `system-managed-field`; issued authority belongs to claim resolution.
- Given raw environment, secret, or credential-bearing fields under the template spec, when the document is parsed, then it fails with `secret-value`; v0 carries neither values nor a credential-delivery mechanism.
- Given an unknown field or a mismatched API version/kind, when the document is parsed, then it fails closed rather than silently discarding the field.
- Given the shared manifest, when focused tests run, then they select the six `AgentTemplate` cases by subject/ID, read the referenced YAML files directly, and compare actual success or validation category with `expected`.

## Negative Cases

- Missing `artifact` or empty `artifact.image`.
- Missing `entrypoint` or an empty command list/element.
- Missing, structurally malformed, internally inconsistent, blank, or duplicate capability-ceiling values.
- Defaults outside their declared ceiling.
- Caller-supplied `effectiveAuthority`.
- Raw environment, secret, token, password, or credential-bearing fields.
- Unknown fields, unsupported `apiVersion`/`kind`, invalid YAML, or multiple YAML documents.

## Compatibility

- Existing `AgentSandboxTemplate` and runtime-spike types remain unchanged; `AgentTemplate` is the application-facing reusable-role contract and is not renamed to match a backend type.
- E1-T1 fixture IDs, paths, expected outcomes, and categories remain unchanged and are consumed rather than copied.
- Later `ClaimRequest` and effective-authority work may depend on these public limit/default types, but this Ticket does not implement request resolution or grant authority.
- JSON tags may mirror the eventual API representation, but v0 parsing in this Ticket is limited to the human-authored YAML fixture surface.

## Open Decisions

- Approval required: confirm that Task + Spec is sufficient planning depth, that default-within-ceiling validation belongs in E1-T2, and that v0 rejects raw environment/credential-bearing fields rather than defining a secret-reference schema.

