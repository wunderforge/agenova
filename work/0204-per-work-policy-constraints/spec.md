# Feature Specification: Per-Work organizational authority constraints

- Ticket: [#204](https://github.com/wunderforge/agenova/issues/204)
- PRD outcome: [declarative request and authorization resolution](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution).

## Intent

An admitted Work receives temporary authority calculated for its trusted user, project, template, request and active reference Policy. Operators can inspect why the authority was narrowed. The result is backend-neutral and fixed before a worker or gateway acts.

## In Scope

- Optional authority caps in the reference Policy, selected using trusted identity from #203.
- Version-attributed immutable issuance and common CLI/API/Portal evidence.

## Out of Scope

- Policy editor/DSL, external authorization engines, delegated grants, worker JWT, AWS IAM and EKS/Bedrock credentials.

## Requirements

1. Given two verified users and byte-equivalent `ClaimRequest.spec` under one template and Policy version, different matching caps may admit both but issue different effective authority.
2. Per dimension, resolution intersects requested access, template ceiling, matched Policy cap and runtime restrictions. No cap can add a tool, resource, memory scope, model/runtime profile or timeout.
3. An absent optional constraint block means legacy admission-only behavior. An explicit empty cap means no authority in that dimension, never unrestricted; wholly removing a required requested dimension fails closed.
4. Matching uses trusted identity and exact action/project/template context, producing one unambiguous result. A missing or mismatched match denies.
5. One evaluation at issuance captures Policy ID/version, requested/effective authority and stable narrowing reasons. Later Policy versions cannot change an issued claim.
6. CLI JSON, evidence API and Portal display the same captured result without re-evaluating Policy.
7. Existing valid admission-only YAML parses and registers as before; changed constraints with the same Policy ID/version conflict.

## Negative Cases

- Malformed caps, duplicate or ambiguous matches, identity mismatch, invalid scope patterns and attempts to add unrequested authority deny before allocation or Tool/Model/AWS calls.
- A caller-controlled request field cannot select another principal's caps; stale admission or a Policy change between evaluation and issuance cannot mix versions.
- Denial leaves inspectable evidence without inventing authority.

## Compatibility

Constraints are opt-in for reference Policy YAML. Existing admission-only documents and unextended Work flows retain behavior. Evidence fields are additive; no provider type enters ClaimRequest or effective authority.

## Open Decisions

- Confirm the principal key and evaluation result types against merged #197/#203; this packet does not prescribe an identity provider.
- Confirm with Owner/Reviewer whether a cap listing values outside a submitted request is always an invalid expansion attempt, as #204's negative case suggests. Until resolved, fail closed and never grant those values.
