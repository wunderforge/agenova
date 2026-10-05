# Technical Design: Backend-neutral Policy evaluation kernel

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- Feature spec: `spec.md`

## Current State and Constraints

- `internal/authorization` imports the concrete `internal/policy` bundle/source shape and its `Authorizer` evaluates reference rules directly.
- The current evaluation records an authorization request and decision, but there is no engine-neutral context/result contract that another evaluator can implement.
- Admission already provides a trusted internal handoff to preparation and issuance. M1 strengthens this seam rather than changing public Work or claim schemas.
- Authority resolution currently intersects requested access and the AgentTemplate ceiling. M1 may carry an optional trusted constraint snapshot to that boundary, but M3 owns authoring and observable per-user authority behavior.

## Decision

Introduce one explicit Policy trust boundary:

1. `EvaluationContext` owns engine-neutral, trusted primitives for principal, action, resource, and environment.
2. `Evaluator.Evaluate` returns a typed result containing decision, Policy reference, reason data, and optional constraints.
3. An immutable snapshot/source selects one evaluator version for the admission attempt.
4. The authorization gate validates the result, binds it to an exact cloned context, and produces the existing internal admission capability.
5. Downstream authority code can receive only a cloned constraint snapshot. Constraints are caps and never bypass request, template, or runtime limits.

Keep these contracts in or adjacent to `internal/policy` for M1, with the reference implementation wrapping the current `BundleSource`/`PolicyBundle`. Do not create a repository-wide package migration. If introducing these types there creates an import cycle, move consumer-neutral primitives into a small internal contract package and record the change before expanding scope.

## Ownership and Contract Boundaries

- `internal/policy`: engine-neutral context, result, constraints, evaluator, immutable source/snapshot, reference evaluator, and cloning/validation helpers.
- `internal/authorization`: constructs the trusted context, calls only the evaluator interface, validates and binds the result, and exposes an opaque admission capability.
- `internal/authority`: receives only the opaque admission capability. M1 proves evaluator constraints cannot act as grants and preserves existing resolution; M3 will consume the cloned caps as an additional ceiling.
- `internal/issuance` and `internal/app`: consume admission through existing application flow without importing the reference rule schema.
- Public API, Gateways, HTTP transport, identity, adapters, CLI, Portal, and Kubernetes resources are unchanged in M1.

## Detailed Data Flow

1. Application code obtains the immutable evaluator snapshot selected for this admission attempt.
2. Authorization builds and clones a generic trusted evaluation context.
3. The selected evaluator evaluates that context. The reference adapter translates it to current exact-match lookup; a fake evaluator uses no reference types.
4. Authorization validates decision, Policy reference, reason data, and constraints, then binds the result to the full context.
5. A denied or invalid result stops before issuance/allocation. An allowed result produces the existing admission capability plus an internal immutable constraint snapshot.
6. Preparation, authority resolution, and issuance continue through current behavior in M1. Constraints cannot add authority; M3 owns their narrowing semantics. No later Policy reload can mutate the bound result.

## Compatibility and Migration

- Introduce the evaluator behind compatibility constructors so current callers and fixtures continue to use the reference bundle without configuration changes.
- Keep exact-match fields and YAML decoding inside the reference implementation.
- Clone contexts, results, constraints, rules, and snapshot identity at every trust-boundary handoff.
- Treat absent constraints as current admission-only behavior. M1 does not add ceiling fields to public/reference YAML; M3 owns that versioned document change.
- Keep `ClaimRequest`, `SandboxClaim`, evidence, Gateway, runtime, and adapter interfaces unchanged.

## Alternatives Considered

- **Policy admission plus a later Policy lookup:** rejected because versions can change and the two evaluations can disagree.
- **Promoting `team/project/templateRef` into the universal Policy model:** rejected because it couples Agenova to one organizational vocabulary and makes future OPA/Cedar/company integrations reshape core contracts.
- **Implementing a general Policy DSL:** rejected because Agenova needs a stable evaluation contract, not another enterprise policy language.
- **Adding reference YAML ceiling syntax in M1:** deferred to M3 so the kernel can land independently with compatibility evidence.
- **Repository-wide layer/package migration:** rejected for M1 because the stable seam can be introduced without mixing structural churn into the contract change.

## Verification Strategy

- Run one table-driven contract suite against the reference evaluator and fake external evaluator.
- Prove allow/deny, immutable Policy identity, exact-context binding, malformed-result rejection, defensive copying, and missing snapshot failure.
- Preserve existing reference tests for exact matching, ambiguity, same-version changed content, and immutable snapshots.
- Prove a permissive evaluator result cannot create authority outside the existing request/template/runtime calculation.
- Run `go test ./internal/policy ./internal/authorization ./internal/authority ./internal/issuance ./internal/app` and `./scripts/check.ps1 -All`.

## Risks and Compatibility

- The evaluator interface could become an accidental universal DSL. Keep only decision inputs/outputs in the shared contract; matching language remains implementation-owned.
- Absence and explicit emptiness can diverge once M3 authors constraints. M1 must preserve an explicit optional representation and test cloning/validation without inventing YAML semantics.
- Package placement may create import cycles. Prefer the smallest consumer-neutral internal contract rather than making authorization and Policy implementations import each other.
- An immutable snapshot that retains caller-owned slices/maps is not immutable. Clone nested values and add mutation-after-evaluation tests.
- Compatibility wrappers can hide accidental fallback. Tests must prove current constructors select only the reference evaluator and that nil/invalid snapshots fail closed.
