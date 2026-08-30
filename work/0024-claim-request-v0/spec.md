# Feature Specification: Define and validate ClaimRequest v0

- Ticket: [#24](https://github.com/wunderforge/agenova/issues/24)
- PRD outcome: [Declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution)

## Intent

Define one declarative, backend-neutral v0 contract for requesting an agent worker assignment: which reusable template, what task, what requested access, and what runtime requirements. The same request must be expressible as human-authored YAML and as API JSON with identical meaning. Requested access is intent for later resolution — the request can never carry an authoritative principal, issued authority, or credential values.

## In Scope

- Public Go types in `api/v1alpha1` for `ClaimRequest`, its spec, task definition, requested access, and runtime requirements, shaped exactly after the frozen fixture surfaces.
- Parsing for the canonical YAML surface and the equivalent API JSON surface, plus a semantic-equivalence proof between the fixture pair.
- Typed categorized validation failures containing at least `category` and `field/path`, compatible with the seven merged ClaimRequest fixture cases.
- Fixture-driven tests using the canonical v0 manifest and input paths directly.

## Out of Scope

- Request storage, submission endpoints, CLI (#41), resolution and policy evaluation (#26–#28), claim issuance (#29), and effective-authority types (E1-T4 #25).
- Trusted principal modeling; the control plane supplies the principal out-of-band (E6-T3).
- Provider-specific runtime shapes; `runtime.profileRef` is an opaque backend-neutral reference.

## Requirements

- Given the canonical Team A YAML, when it is parsed and validated, then it yields `apiVersion=agenova.io/v1alpha1`, `kind=ClaimRequest`, metadata name `fix-payment-timeout`, template reference `engineer`, the declared task type and input map, requested tools/resource scopes/model profile/memory scopes, and runtime profile/timeout.
- Given the equivalent API JSON fixture, when both surfaces are parsed, then the resulting values are semantically equal, proven by direct comparison rather than string formatting.
- Given a request missing `spec.templateRef`, `spec.task`, or `spec.runtime`, then validation fails with `required-field` and the responsible field/path.
- Given the exact caller-authored path `spec.principal`, then parsing fails with `self-asserted-principal` and that path; the trusted principal never travels in the request.
- Given the exact secret-bearing path `spec.secrets`, then parsing fails with `secret-value` and that path; v0 carries neither credential values nor a delivery mechanism.
- Given any other unknown field, then parsing fails closed with `unknown-field` and the exact path; classification is deterministic and path-based, never a scan of names or values.
- Given any validation failure, then the parser/validator returns typed data containing at least `category` and `field/path`; callers and tests must not derive either from error strings.
- Given a mismatched apiVersion/kind, an invalid document, or multiple YAML documents, then parsing fails closed with typed data.
- Given the shared manifest, when focused tests run, then they select the seven `ClaimRequest` cases by subject/ID, read the referenced files directly, and compare actual outcomes with `expected`.
- Task input and resource scopes remain distinct fields with distinct meanings: `spec.task.input` is work data, `spec.requestedAccess.resourceScopes` is access intent; validation never merges or derives one from the other.

## Negative Cases

- Missing `spec.templateRef` (`required-field`).
- Missing `spec.task` (`required-field`).
- Missing `spec.runtime` (`required-field`).
- Caller-supplied `spec.principal` (`self-asserted-principal`), classified only by its exact path.
- Embedded `spec.secrets` values (`secret-value`), classified only by its exact path.
- Unknown fields, unsupported `apiVersion`/`kind`, invalid documents, or multiple YAML documents (fail closed).

## Compatibility

- E1-T1 fixture IDs, paths, expected outcomes, and categories remain unchanged and are consumed rather than copied.
- The approved E1-T2 AgentTemplate conventions (typed `category` + `field/path`, exact-path reserved-field classification, fail-closed parsing) are reused so the two human-authored contracts stay symmetric.
- `templateRef` names an `AgentTemplate` by `metadata.name` (the invariant fixed in the E1-T2 review); this Ticket references but does not resolve it.
- Existing runtime-spike types, gateways, and backend adapters remain unchanged.

## Open Decisions

- None. This specification elaborates the Ticket against the frozen fixture surfaces; packet approval covers the `api/v1alpha1` placement beside AgentTemplate v0.
