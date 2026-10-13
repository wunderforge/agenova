# Technical Design: Governed BAU/SRE investigation contract

- Ticket: [#220](https://github.com/wunderforge/agenova/issues/220)
- Feature spec: [`spec.md`](spec.md)

## Current State and Constraints

- `ClaimRequest` and `AgentTemplate` already carry string tool names and resource scopes; the authority resolver intersects tools exactly and supports one terminal wildcard only in template scope ceilings.
- `internal/toolgateway` already validates structure, resolves an authoritative Running claim, enforces exact tools/resources, evaluates policy, records a correlated decision, and dispatches only Allow.
- `internal/gateway.AmbiguousResourceScope` currently rejects only blank and wildcard scopes. It does not distinguish an exact canonical resource from an arbitrary or ambiguous string.
- Existing `repo:owner/name` fixtures are the compatibility baseline.
- The branch baseline does not include E16's accepted real provider transport/catalog slice. M1 must not create a competing provider framework.
- A bare `ClaimID` remains unauthenticated correlation until E14's claim-bound identity work lands.

## Decision

Add a small backend-neutral parser/formatter in `internal/gateway` and invoke it during Tool Gateway structural validation, before claim resolution.

The parser:

1. splits the scope once at `:` into a closed logical kind and path;
2. rejects whitespace, controls, backslashes, wildcards, query/fragment delimiters, empty segments, and `.`/`..` segments;
3. checks a kind-specific segment shape;
4. returns immutable logical data and a formatter that round-trips to the identical canonical string.

The Tool Gateway retains the existing order:

```text
issue invocation ID
  -> clone caller parameters
  -> structural validation and canonical scope parse
  -> resolve authoritative claim
  -> require Running claim
  -> enforce exact operation/resource authority
  -> evaluate additional policy
  -> record decision
  -> dispatch Allow only
```

`ambiguous-resource-scope` remains the category for empty or wildcard input. Other grammar failures use `invalid-resource-scope`, which lets operators distinguish an unbounded request from a malformed/unsupported logical resource without revealing provider details.

## Ownership and Contract Boundaries

- `internal/gateway`: logical scope parser, formatter, validation error, stable rejection category, and focused unit tests.
- `internal/toolgateway`: calls the parser during existing structural validation and proves no claim fact/policy/adapter call occurs on failure.
- `internal/authority`: remains the owner of request/template intersection and terminal-wildcard ceilings; no public type change.
- `internal/facts`: remains the evidence/correlation owner; no schema change in M1.
- E16: provider route/catalog/transport and result handling.
- E14: authenticated claim-bound worker identity.
- E18: durable journal, diagnosis, and observability.
- Private enterprise integration: agent skill/knowledge, policy values, environment mapping, internal endpoints, credentials and deployment values.

## Dependency and Gap Matrix

| Capability | Current M1 use | Owning follow-up |
| --- | --- | --- |
| Claim admission and effective authority | Reuse current exact tools/resources | Existing core |
| Canonical logical resources | Implement and test | E21 M1 |
| Decision/attempt/outcome correlation | Reuse current decisions/facts; real provider outcome remains transport work | E16 |
| Real enterprise adapters | Contract only | E21 M2 coordinated with E16 |
| Worker-to-Gateway authentication | Explicitly unverified | E14 |
| Durable evidence across restart | Explicitly unverified | E18 |
| Runtime/network isolation | Static contract tests are insufficient | Existing runtime/security Epics and E21 M4 |
| Company-specific knowledge/configuration | Forbidden in public repo | Private integration repository |
| Helm/Argo non-production deployment | Not part of M1 | E21 M5 after identity/evidence gates |

## Alternatives Considered

- Keep arbitrary opaque strings and document conventions only: rejected because malformed or unsupported resources would reach policy/authority comparison, creating inconsistent adapter interpretations and weak negative evidence.
- Add provider SDK resource types to `ClaimRequest`: rejected because it breaks backend neutrality and makes one provider's identifiers part of shared authority.
- Put parsing in each adapter: rejected because adapters could disagree on which string is authoritative, and structural rejection would occur after provider selection.
- Replace resource strings with a new public tagged union now: rejected as an unnecessary public compatibility change for M1; a strict internal parser proves the contract while keeping current YAML/API shapes.
- Add hierarchical/wildcard matching at invocation time: rejected. Effective authority is already resolved; governed invocations must match one exact issued scope.

## Verification Strategy

- Parser table tests cover every supported kind, canonical round-trip, and malformed/ambiguous inputs.
- Tool Gateway tests prove malformed input is rejected before claim attribution, policy and adapter calls.
- Existing authority tests prove exact operation/resource intersection and template ceiling behavior.
- Existing inactive-claim and cross-claim tests remain green.
- A positive control proves one allowed canonical operation produces one Allow fact and one adapter call; deny cases prove zero calls.
- A repository scan covers credential field names and company-specific identifiers in the new fixtures/evidence.

## Risks and Compatibility

- A closed logical-kind set means new resource categories require an explicit contract change. This is intentional fail-closed behavior.
- Some future list/query operations may need collection scopes or typed selectors. M1 accepts only the bounded namespace/target forms and defers selectors to real adapter evidence.
- Existing third-party consumers using arbitrary scope prefixes would be rejected at the Tool Gateway after this change. The repository's supported fixtures use `repo:`; the change is documented as an intentional contract tightening for governed provider access.
- Canonical syntax does not prove authorization, identity, isolation or safe provider behavior. Those claims remain separately evidenced.
