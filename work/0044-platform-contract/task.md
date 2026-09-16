# Task: Define the reference installation contract

- Ticket: [#44](https://github.com/wunderforge/agenova/issues/44)
- Mission: Define one versioned, backend-neutral `Platform` desired-state contract that later install, bootstrap, CLI and UI work can share while preserving the mandatory Agenova Gateway path.
- Target: `api/v1alpha1` Platform types/strict decoding, `internal/platform` pure resolution, canonical fixtures and deterministic projection tests.
- User value: An operator can review one manifest and know where Agenova runs, which worker/model capabilities are configured and which adapter versions will be used, without editing demo code.
- PRD outcome: [Reference installation and initial policy bootstrap](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap)

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Feature specification](spec.md) and [technical design](design.md)
- [Reference Installation and Bootstrap](../../docs/product/architecture-contract.md#reference-installation-and-bootstrap)
- [Backend Neutrality](../../docs/product/architecture-contract.md#backend-neutrality)
- [Start a GitHub Ticket](../../docs/harness/playbooks.md#start-a-github-ticket)
- Existing contracts: [`ClaimRequest`](../../api/v1alpha1/claim_request.go), [`AgentTemplate`](../../api/v1alpha1/agent_template.go) and [`SandboxClaim`](../../api/v1alpha1/sandbox_claim.go)
- Current hard-coded composition: [`cmd/agenova-console`](../../cmd/agenova-console/main.go) and [`internal/app`](../../internal/app/submit.go)

## Scope

In scope:

- Add the canonical `agenova.io/v1alpha1` `Platform` envelope, parser and fail-closed validation.
- Separate versioned adapter requirements from named deployment, runtime and implemented-service instances.
- Freeze explicit references for deployment, actionable RuntimeBackend resources plus profile-to-backend mappings, downstream ModelBackend instances plus profile-to-backend mappings, and the initial Policy seed.
- Define an internal, secret-free actionable resolved form plus an inspectable digest-only lock and deterministic plan projection; actual reconciliation remains #45.
- Add valid and invalid YAML/JSON fixtures plus reference-integrity and adapter-owned validation test doubles.

Out of scope:

- Adapter registry/lifecycle commands (#151), target mutation or `platform plan/apply` (#45), Policy seeding (#46), bootstrap UX (#152), shared service composition (#146), AgentTemplate registration (#147), external adapter loading (#149), or Tool/Memory/Observability adapter design (#150).

## Acceptance Criteria

- One manifest explicitly declares adapter requirements, `spec.infrastructure`, `spec.services` and a required initial Policy reference; it never embeds Policy, AgentTemplate or Work definitions.
- Deployment, RuntimeBackend and downstream ModelBackend selections are independent named instances whose provider-specific fields remain under adapter-owned `config`.
- Runtime/model profile mappings reference configured instances but do not grant authority.
- Every resolved model path is `worker -> mandatory Agenova Model Gateway -> granted Model Profile -> ModelBackend`; a backend such as Ollama, OpenAI, Bedrock or LiteLLM never replaces the Gateway.
- Every resolved model route preserves the Gateway trust boundary. Authenticated binding of a live caller to one issued claim is delivered by #121; #44 neither implements nor claims evidence for that authentication step.
- After a caller is authoritatively attributed to an issued claim, the Gateway owns effective-profile enforcement, Allow/Deny/ApprovalRequired decision, invocation correlation and claim-scoped `ModelInvocation` evidence. Pre-attribution denial emits no fact beneath an unverified claim, denied calls make zero backend calls, and workers receive no direct backend endpoint or credential.
- A pure validation/resolution path produces a secret-free canonical resolved form for #45 plus a stable inspectable lock/plan projection without mutating a target.
- Unknown/duplicate references, version or capability mismatch, unsupported service kind, malformed adapter config, profile conflict and secret-bearing config return no resolved lock; the #44 resolver exposes no target-mutation dependency.

## Negative Case

- A manifest that selects Kubernetes deployment but omits a RuntimeBackend, maps a model profile to an unknown ModelBackend, exposes direct worker-to-backend configuration, or includes a credential value is rejected; deployment selection must never infer worker/model adapters.

## Execution Todo

- [x] Scout the relevant implementation, tests, risks and dependencies; current CLI and Portal compositions still hard-code different runtime/provider choices.
- [ ] Confirm this packet with the Owner and Reviewer before implementation.
- [ ] Add Platform/lock types and strict YAML/JSON parsing without backend/provider imports.
- [ ] Implement generic reference/capability/profile/secret-field validation plus injected adapter-owned config validation.
- [ ] Add canonical valid/invalid fixtures and deterministic resolved-plan contract tests.
- [ ] Run focused gates and `./scripts/check.ps1 -All`.
- [ ] Review scope, backend-neutrality and source-of-truth updates before PR.

## Quality Gates

- `go test ./api/v1alpha1 ./harness/fixtures/contract/v0 -run Platform`
- `go test ./internal/platform -run PlatformContract`
- `.\scripts\check.ps1 -All`

## Evidence Required

- Canonical YAML and equivalent JSON parse to one Platform value, one actionable secret-free resolved form containing the required runtime allocation resources, and one stable revision digest computed without self-referential fields.
- Table-driven negative fixtures prove all named validation failures return no partial lock; package/interface checks prove the resolver has no target-mutation dependency. #45 owns target mutation spies.
- Deterministic plan fixture shows deployment, runtime and model instances remain independently selected and produces a secret-free lock.
- Exact focused/full commands and output recorded in the PR and ticket.

## Constraints

- Preserve `docs/product/architecture-contract.md`; kind/Kubernetes is test configuration, not the product model.
- Do not broaden the Ticket or PRD without a recorded human decision.
- Do not implement resolver downloads, arbitrary executable paths, implicit `init()` registration, config-shape inference, directory scanning, hidden bootstrap state or hot rebinding.
- `spec.services` accepts only implemented service categories; schema presence must not claim Tool, Memory or Observability delivery.
- Until #155 defines and implements typed host-side credential resolution, validation rejects credential references as well as inline credential values.
- Keep the existing lightweight in-process Model Gateway as the mandatory MVP reference path. Optional LiteLLM routing is #154; typed host-side credential references/resolution are #155.

## Decisions and Blockers

- Owner approved the Platform/Adapter lifecycle model on 16 September 2026; this packet still requires explicit Owner and independent Reviewer approval before construction.
- #44 can define descriptor inputs and validation seams, but #151 owns concrete adapter identity grammar, registry/factory behavior and lifecycle commands. Implementation must not pre-empt those mechanics.
- Owner correction on 16 September 2026: the initial packet blurred Model Gateway and provider roles. The corrected contract names downstream `ModelBackend` instances and keeps Agenova governance/evidence in the non-swappable core Gateway.
- #121 owns authenticated live caller-to-claim binding. It is required before the composed live gateway may claim verified cross-process context, but it does not block defining or implementing #44's pure Platform contract and mandatory Gateway route.
