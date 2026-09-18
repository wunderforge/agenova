# Feature Specification: Declarative CLI control flow to kind and Ollama evidence

- Ticket: [#165](https://github.com/wunderforge/agenova/issues/165)
- PRD outcome: [Operator journey and acceptance](../../docs/product/prd.md).

## Intent

The effective Platform applied by an operator must select the live application service, runtime and model provider for subsequently registered Policy/templates and submitted Work. CLI commands must not compose a hidden second demo stack.

## In Scope

- One supported reference Platform using Kubernetes/Agent Sandbox and OpenAI-compatible Ollama.
- `platform status`, narrow create-only `policy apply`, compatible `agent-template apply`, `run -f`, and evidence query.
- Registration, submission and evidence through the installed application path; explicit reference mode isolated.

## Out of Scope

General CRUD, arbitrary agent frameworks, Tool/Memory/Observability adapters, public-provider credentials, production auth/HA/history and unbounded plugins.

## Requirements

- Applied Platform's revision and service/adapter readiness are inspectable.
- Authorized registration persists across restart; identical reapply is a no-op and same-identity different content conflicts.
- Registered template and matching trusted principal yield one governed kind/Ollama claim and correlated evidence.
- Missing Policy rule denies before claim allocation or model call.
- Unsupported/unavailable capabilities produce actionable non-secret diagnostics, never implicit memory/mock fallback.

## Negative Cases

Invalid/missing profiles, unknown template, unauthorized operator, unsupported artifact, mutable registration, unavailable model endpoint and cross-claim invocation fail closed.

## Compatibility

Preserve v0 ClaimRequest/AgentTemplate/PolicyBundle/evidence/RuntimeBackend semantics; keep provider settings and Kubernetes types out of application-facing authority contracts. The existing in-memory reference path remains explicitly isolated.

## Decision

The owner's 17 September request approves narrow operator registration as a target capability, not self-service Policy CRUD. Reference-only identity/durability limits must be labelled and tested.
