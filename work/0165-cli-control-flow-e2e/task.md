# Task: Declarative CLI control flow to kind and Ollama evidence

- Ticket: [#165](https://github.com/wunderforge/agenova/issues/165)
- Mission: Make one declared Platform, registered Policy and AgentTemplate, and canonical Work request drive the same real kind/Ollama application path through the CLI.
- Target: Platform reconciler, bundled adapters, application composition, operator registry, Control Plane service, CLI, and reference tests.
- User value: Reproduce and inspect Agenova's core flow without a separately hardcoded demo service or UI-only setup.
- PRD outcome: [MVP user journey and reference installation](../../docs/product/prd.md).

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, and this packet.
- [Specification](spec.md), [design](design.md), [architecture contract](../../docs/product/architecture-contract.md), [Platform contract](../0044-platform-contract/spec.md), [Platform apply](../0045-platform-plan-apply/task.md), [kind run](../0136-kind-application-run/design.md), [adapter registry](../0151-bundled-adapter-registry/spec.md).

## Scope

In scope: effective Platform revision/status; supported kind/Ollama composition; create-only PolicyBundle and compatible AgentTemplate registration from YAML; connected CLI submission/evidence; honest unsupported-capability errors; documentation and real positive/negative E2E.

Out of scope: arbitrary adapters/images, public-provider credentials, general CRUD, production identity/HA/durability, Tool/Memory/Observability implementation, and UI redesign.

## Acceptance Criteria

- `platform validate/plan/apply/status` uses one effective revision and reports actual service readiness.
- `policy apply` and `agent-template apply` validate canonical YAML, repeat identically without mutation, reject conflicting identity/content and unauthorized operators, and survive service restart.
- `run -f` submits to the installed shared service, not an implicit in-memory stack. A named reference mode remains available.
- Registered Policy/template determine admission, authority, worker artifact and model/runtime profiles. Missing/invalid configuration has zero worker/provider side effect.
- CLI evidence shows kind worker, governed Ollama call, outcome/failure reason. A request outside the trusted Team A policy is denied before allocation; switching to a genuine Team B identity is not available in this reference installation.
- PRD, architecture, quickstart and implementation status distinguish verified behavior from future adapters.

## Negative Case

Unknown template/profile, unavailable Ollama, conflicting registration, invalid YAML, unauthorized assignment/operator, and unsupported adapter fail closed with no silent fallback.

## Execution Todo

- [x] Read ticket, PRD and architecture; isolate from dirty bootstrap worktree.
- [x] Record the owner's 17 September 2026 direction as approval for this narrow E2E scope; independent review remains required.
- [x] Synchronize PRD and design decisions before implementation.
- [x] Add effective revision/status and connected service deployment.
- [x] Add narrow Policy and AgentTemplate registration with trusted management boundary.
- [x] Connect CLI run/evidence to shared service; retain explicit reference mode.
- [x] Add tests and real kind/Ollama E2E evidence.
- [x] Run focused and full gates.
- [x] Independently review: fix three in-scope findings; disclose the remaining mid-run rollout/durability limitation as future scope.
- [x] Owner superseded the manual-test hold on 17 September 2026 and authorized independent review plus direct merge when the gates pass.
- [ ] Merge, then reconcile related tickets/dependencies in the API/console follow-up.

## Quality Gates

- `go test ./internal/... ./cmd/...`
- `./scripts/check.ps1 -All`
- The exact kind/Ollama reproduction in `docs/evidence/165/`.

## Evidence Required

Capture CLI inputs/outputs, Platform revision, registrations, Team A allow and out-of-policy deny, worker identity/cleanup, real model response, and correlated evidence. Label synthetic tool activity. Genuine Team B identity switching is deferred to a production identity boundary, not simulated by a caller flag.

## Constraints

- Preserve backend-neutral contracts and claim-scoped authority.
- Do not read or commit credential-bearing files; all examples are secret-free.
- Do not equate loopback transport with authentication or expose a public listener.
- Never delete the user's pre-existing cluster or unrelated resources; use dedicated test namespaces.

## Decisions and Blockers

- Owner direction prioritizes this integration ahead of stale ticket dependencies; reconcile #146/#147/#47 only after verification.
- Reference identity and transport limitations must remain visible and not be described as production authentication.
- The independent review found that an active Work can be interrupted by a Platform rollout because the reference service is single-Pod and process-local. The seven-command idle-install path is verified, but mid-run upgrades are explicitly unsupported and need a later draining/revision-stable routing or durable handoff ticket. Do not describe this as production-safe.
