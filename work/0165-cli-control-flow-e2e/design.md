# Technical Design: Declarative CLI control flow to kind and Ollama evidence

- Ticket: [#165](https://github.com/wunderforge/agenova/issues/165)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

Platform apply installs a status-only Control Plane and initial Policy ConfigMap. `agenova run` constructs separate in-memory governance; `agenova-console` hardcodes a loopback kind/Ollama demo. Bundled runtime/model factories validate configuration but do not instantiate working instances. The controlled worker protocol supports only compatible images.

## Decision

Keep Platform resolution and adapter validation as the only source of effective deployment/runtime/model settings. Extend the installed reference service to host the existing application RunService/evidence path. Use an immutable create/idempotent operator registry for Policy/AgentTemplate and connect CLI submission/query to that service. Keep memory execution behind a named reference mode. Never infer provider choice or grants from a CLI command.

Local access may use a bounded localhost tunnel to the internal Kubernetes Service. The tunnel is transport, not authentication; operator registration relies on authenticated cluster identity/RBAC or equivalent explicit management authorization. The service must not be public.

## Ownership and Contract Boundaries

- `platform`/`platformapply`: canonical revision, plan/apply/status; no ClaimRequest authority parsing.
- Bundled deployment adapter: Kubernetes manifests, effective configuration, Service and least-privilege runtime access.
- Runtime/model adapters: instantiate only supported configurations; isolate kubectl and provider endpoint details.
- Application/registry: validate immutable PolicyBundle/AgentTemplate, authorize management, resolve requested authority and snapshot it on issuance.
- Shared service: submission, bounded evidence and status; no caller-controlled principal or grants.
- CLI: canonical document conversion and shared-service client; explicit reference mode for memory tests.

## Alternatives Considered

- Wrapping `agenova-console` was rejected because it leaves the installed Platform disconnected from Work.
- Implementing every adapter adds no proof of the core path; missing ones stay visible.
- Loopback, Origin or user-supplied principal flags are not authentication.

## Verification Strategy

Focused registration/CLI/service tests with fake runtime/provider, then published commands against real kind/Ollama: positive claim, denied claim, worker identity, model fact, terminal outcome and cleanup. Reapply files, test conflicts/failures, restart and inspect restored registry. Run full gate and independent review.

## Risks and Compatibility

Agent Sandbox and worker protocol are reference-only with known non-production limits. This reference service holds active Work and evidence in one Pod, so applying a new Platform revision while Work is active is unsupported: the rollout can interrupt the run and its cleanup/evidence. A production upgrade must provide draining and revision-stable routing or durable handoff before making that guarantee. The dirty bootstrap worktree is not part of this branch and must not be overwritten.
