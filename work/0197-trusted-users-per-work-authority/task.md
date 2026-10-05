# Task: Backend-neutral Policy evaluation kernel

- Ticket: [#197](https://github.com/wunderforge/agenova/issues/197)
- Mission: Decouple authorization from the concrete `PolicyBundle` by defining a backend-neutral, immutable Policy evaluation contract while preserving current reference-policy behavior.
- Target: `internal/policy`, `internal/authorization`, `internal/authority`, `internal/issuance`, `internal/app`, and focused tests/fixtures.
- User value: Agenova can later adopt a company Policy engine, OPA, or Cedar without changing `ClaimRequest`, `SandboxClaim`, Gateway, or evidence semantics.
- PRD outcome: [`Declarative request and authorization resolution`](../../docs/product/prd.md#1-declarative-request-and-authorization-resolution) and [`Claim-scoped authority`](../../docs/product/prd.md#4-claim-scoped-authority).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [`spec.md`](spec.md)
- [`design.md`](design.md)
- [`docs/product/architecture-contract.md`](../../docs/product/architecture-contract.md)
- [`docs/development/AIDLC.md`](../../docs/development/AIDLC.md)
- [`docs/harness/playbooks.md`](../../docs/harness/playbooks.md#change-a-core-contract)
- [`internal/policy/bundle.go`](../../internal/policy/bundle.go)
- [`internal/authorization/authorization.go`](../../internal/authorization/authorization.go)
- [`internal/authority/resolver.go`](../../internal/authority/resolver.go)
- [`internal/app/prepare.go`](../../internal/app/prepare.go)
- [E14 Team MVP Epic #176](https://github.com/wunderforge/agenova/issues/176)

## Scope

In scope:

- Define backend-neutral `EvaluationContext`, `Evaluation`, `AuthorityConstraints`, and evaluator/snapshot interfaces.
- Bind an evaluation to the exact trusted context used for admission so it cannot be replayed for another request.
- Adapt the current exact-match `PolicyBundle` as the bundled reference evaluator without changing existing YAML behavior.
- Carry optional, defensively copied constraints through admission as a trusted cap; absent constraints preserve current behavior.
- Run one reusable contract suite against the reference evaluator and a fake external evaluator.

Out of scope:

- Shared OIDC API and user/evidence authorization ([#203](https://github.com/wunderforge/agenova/issues/203)).
- Reference-YAML authority authoring and the two-user different-authority flow ([#204](https://github.com/wunderforge/agenova/issues/204)).
- Claim-bound workload identity and Gateway changes ([#205](https://github.com/wunderforge/agenova/issues/205)).
- kind/Ollama packaging and team MVP release candidate ([#206](https://github.com/wunderforge/agenova/issues/206)).
- OPA/Cedar/company-engine integrations, a general Policy DSL, Kubernetes Operator, repository moves, EKS, or Bedrock.

## Acceptance Criteria

- Authorization consumers depend on the evaluator contract, not the concrete `PolicyBundle` rule shape.
- An allowed result is usable only for the exact immutable evaluation context that produced it; mismatches fail before claim issuance or backend allocation.
- Existing reference Policy YAML keeps its current exact-match allow/deny behavior.
- The same evaluator contract suite passes for the reference evaluator and a fake external evaluator.
- Optional constraints are validated and defensively copied, cannot themselves grant authority, and preserve current behavior when absent.
- Public `ClaimRequest` and `SandboxClaim` contracts remain unchanged.

## Negative Case

- Return `Allow` with an invalid Policy reference or bind it to a different principal/action/resource context. Authorization must fail closed before claim creation or backend calls.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks, and dependencies.
- [x] Self-review this packet against #197, the PRD, and the architecture contract.
- [ ] Define the generic evaluation context/result/constraint and immutable snapshot interfaces.
- [ ] Adapt the exact-match `PolicyBundle` as the reference evaluator.
- [ ] Bind the returned evaluation to admission and expose only the trusted constraint snapshot to downstream authority code.
- [ ] Add the reusable evaluator contract suite and fake external evaluator.
- [ ] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- `go test ./internal/policy ./internal/authorization ./internal/authority ./internal/issuance ./internal/app`
- `./scripts/check.ps1 -All`

## Evidence Required

- Focused output proving reference/fake evaluator conformance, exact-context binding, invalid-result fail-closed behavior, defensive copying, and compatibility with existing Policy fixtures.
- Full repository gate output and final scope/security diff review.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- `ClaimRequest` cannot carry principal, trusted metadata, effective authority, or Policy-engine payloads.
- Policy evaluation can deny or supply authority caps; it cannot grant capabilities by itself.
- Policy-engine, Kubernetes, cloud, and company-specific types stay outside the shared contract.
- Do not claim M2–M5 or E14 Team MVP completion from this Ticket.

## Decisions and Blockers

- `PolicyEvaluator` is the stable product boundary; `team/action/project/templateRef` matching remains a reference implementation detail.
- `AuthorityConstraints` are an internal cap, never a grant. M3 owns reference-YAML constraint authoring and full authority behavior.
- No public `ClaimRequest` or `SandboxClaim` schema change is expected in M1.
- Self-review completed after rebasing onto the advisory packet-approval workflow from #186. The packet matches #197, the PRD, and the architecture contract; no unresolved scope or architecture conflict blocks implementation.
