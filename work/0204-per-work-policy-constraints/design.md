# Technical Design: Per-Work organizational authority constraints

- Ticket: [#204](https://github.com/wunderforge/agenova/issues/204)
- Feature spec: `spec.md`

## Current State and Constraints

`internal/policy` reference rules currently match team/action/project/template and carry no caps. `internal/authority.Resolve` checks admission and intersects a request with the AgentTemplate ceiling, but not Policy constraints. #197/#207 admission and #203 trusted identity are unmerged prerequisites. The [architecture contract](../../docs/product/architecture-contract.md#authority-and-credentials) requires request ∩ template ∩ Policy ∩ runtime and shared evidence.

## Decision

Extend existing versioned Policy rules/documents with optional, explicitly present authority constraints. Validate and select one cap using only trusted admission context; pass an immutable, version-attributed evaluation into backend-neutral authority resolution. Resolve each dimension, record deterministic narrowing reasons, and fail closed before allocation on invalid evaluation or wholly removed required access. Deep-copy the issued authority and Policy identity once; all evidence surfaces read that snapshot.

Absent constraint blocks retain admission-only semantics. Preserve explicit empty sets through YAML parsing so `[]` cannot become unrestricted. Reject ambiguous matching and same-ID/version content changes. Reuse final #197/#203 types rather than creating a parallel principal or decision model.

## Ownership and Contract Boundaries

- `internal/policy`: parsing, validation, matching and immutable evaluated caps; no AWS or Kubernetes types.
- Admission/issuance: bind trusted principal and exact request/project/template to one active Policy version; reject stale/mismatched results.
- `internal/authority`: pure intersections and structured failure/reason output.
- Shared claim/evidence: snapshot Policy identity and requested/effective authority once. CLI/API/Portal render only that common evidence.
- Backend adapters: consume issued authority without broadening. E13 #175 owns EKS/Bedrock IAM mapping.

## Alternatives Considered

- AWS IAM statements in shared Policy/Claim: violates backend neutrality.
- Re-evaluate Policy on evidence reads or gateway calls: changes issued authority after a Policy update.
- Separate CLI/Portal policy model: risks contradictory evidence.

## Verification Strategy

Unit/contract tests cover omitted versus explicit empty, policy identity conflicts, ambiguous/mismatched matches, no expansion, dimension intersections, stable reasons, immutable snapshots and zero effects before allocation. Installed kind smoke submits byte-equivalent request specs through two verified users and compares issued snapshots under one Policy/template. Capture CLI/API/Portal parity and rendered Portal proof. Run focused gates, then `./scripts/check.ps1 -All`.

## Risks and Compatibility

Main integration risk is guessing unmerged #197/#203 interfaces; reconcile after merge and rerun their contracts. YAML presence semantics and deep-copy behavior must survive registration, issuance and serialization. Policy-update races deny rather than mix versions. Cloud permission mapping remains E13 work.
