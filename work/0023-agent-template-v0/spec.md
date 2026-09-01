# Feature Specification: Define and validate AgentTemplate v0

- Ticket: [#23](https://github.com/wunderforge/agenova/issues/23)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution)

## Intent

Define one human-authored, backend-neutral v0 contract for a reusable agent role. The contract identifies a runnable artifact and entrypoint, supplies safe reusable defaults, and caps the tools, resources, model profiles, memory scopes, runtime profiles, and execution time that later request resolution may grant. It is configuration and maximum authority, never issued run state.

## In Scope

- Public Go types in `api/v1alpha1` for `AgentTemplate`, its spec, artifact, entrypoint, defaults, and capability ceiling.
- A YAML parser and deterministic semantic validator for the v0 human-authored surface.
- Typed categorized validation failures containing at least `category` and `field/path`, compatible with the six merged AgentTemplate fixture cases.
- Fixture-driven tests that use the canonical v0 manifest and input paths directly.

## Out of Scope

- Template storage, lookup, registry APIs, version conversion, generated JSON Schema/OpenAPI/CRDs, admission webhooks, or CLI commands.
- Prompt/instruction content, task semantics, policy evaluation, requested-access intersection, effective authority, claim state, backend identity, or runtime allocation.
- Provider-specific image/runtime objects; `artifact.image` is an opaque runnable artifact reference rather than a Kubernetes or other backend resource.
- Secret references or credential delivery design beyond rejecting the reserved credential-bearing `spec.environment` path in v0.

## Requirements

- Given the canonical engineer YAML, when it is parsed and validated, then it yields `apiVersion=agenova.io/v1alpha1`, `kind=AgentTemplate`, metadata name `engineer`, the declared artifact and command, safe defaults, and the complete capability ceiling.
- Given an AgentTemplate, when `metadata.name`, `artifact.image`, or `entrypoint.command` is absent or blank, then validation fails with `required-field` and identifies the responsible field/path; `metadata.name` has focused unit coverage because later `templateRef` resolution depends on it.
- Given any validation failure, then the parser/validator returns typed data containing at least `category` and `field/path`; callers and tests must not derive either value by matching error strings.
- Given an AgentTemplate without `capabilityCeiling`, validation fails with `required-field`; given an explicitly empty `capabilityCeiling`, validation succeeds and the template grants no capability by default across all governed dimensions.
- Given `capabilityCeiling`, when a collection such as `tools` is encoded as a scalar instead of a list, then parsing/validation fails with `invalid-capability-ceiling`.
- Given template defaults, when a default model profile or memory scope is outside the corresponding capability ceiling, then validation fails with `invalid-capability-ceiling`; reusable defaults cannot enlarge the ceiling.
- Given a capability ceiling, when a supplied list contains blank or duplicate entries or `maxTimeout` is not a positive Go duration, then validation fails with `invalid-capability-ceiling`.
- Given the exact caller-authored path `spec.effectiveAuthority`, when the document is parsed, then it fails with `system-managed-field` and that path; issued authority belongs to claim resolution.
- Given the exact credential-bearing path `spec.environment`, when the document is parsed, then it fails with `secret-value` and that path; v0 carries neither values nor a credential-delivery mechanism.
- Given any other unknown field, when the document is parsed, then it fails with `unknown-field` and the exact path. Classification is deterministic and path-based; it does not scan field-name substrings or document values for authority or credential hints.
- Given a mismatched API version/kind, invalid YAML, or multiple YAML documents, when the document is parsed, then it fails closed with typed category and field/path data rather than silently accepting or discarding input.
- Given the shared manifest, when focused tests run, then they select the six `AgentTemplate` cases by subject/ID, read the referenced YAML files directly, and compare actual success or validation category with `expected`.

## Negative Cases

- Missing `artifact` or empty `artifact.image`.
- Missing `entrypoint` or an empty command list/element.
- Missing or structurally malformed capability ceiling; an explicitly present empty ceiling is valid default-deny.
- Internally inconsistent, blank, or duplicate capability-ceiling values.
- Defaults outside their declared ceiling.
- Caller-supplied `spec.effectiveAuthority`, classified only by its exact path.
- Caller-supplied `spec.environment`, classified only by its exact path.
- Unknown fields, unsupported `apiVersion`/`kind`, invalid YAML, or multiple YAML documents.

## Compatibility

- Existing `AgentSandboxTemplate` and runtime-spike types remain unchanged; `AgentTemplate` is the application-facing reusable-role contract and is not renamed to match a backend type.
- E1-T1 fixture IDs, paths, expected outcomes, and categories remain unchanged and are consumed rather than copied.
- Later `ClaimRequest` and effective-authority work may depend on these public limit/default types, but this Ticket does not implement request resolution or grant authority.
- JSON tags may mirror the eventual API representation, but v0 parsing in this Ticket is limited to the human-authored YAML fixture surface.

## Decisions and Approval

- The 2026-08-28 planning review fixed the v0 identity, typed-error, path-classification, and empty-ceiling semantics recorded above.
- Task + Spec remains the selected planning depth; no separate Design is required for this bounded Go contract/parser change.
- After direct discussion with the project lead on 2026-08-29, the Owner authorized implementation and submission of the resulting PR; independent Reviewer approval remains required before merge.

