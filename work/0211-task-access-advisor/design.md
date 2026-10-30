# Technical Design: independent preflight Advisor

- Ticket: [#211](https://github.com/wunderforge/agenova/issues/211); spec: spec.md.
- Owner decision/self-review: local scope accepted 30 Oct 2026.

## Current State and Decision

Templates and registrations are immutable; canonical Work already has requestedAccess. Console authenticates, prepares and resolves current Policy before allocation. Modelprovider.Client is bounded and replaceable. Existing evidence persistence can carry optional provenance.

Author ACCESS.md is packaged into template. An authenticated context loader passes task/guide/visible catalog to accessadvisor.Service. Replaceable RuleSelector or ModelSelector chooses a registered profile, validated into a canonical draft. Manual requests, confirmed drafts and automatic drafts all enter the existing submission/authorization path. Advisor has no claim identity and cannot alter EffectiveAuthority.

## Ownership and Contract Boundaries

- v1alpha1: optional guide/config shapes, strict validation, existing permission types.
- internal/accessadvisor: DTOs, Selector, rules/model adapters, digest, bounded owner-scoped cache and call/concurrency/time bounds. No runtime/console dependency.
- POST /api/access-advice: proposal; GET /api/access-advice/{id}: own retained receipt.
- POST /api/access-submit: automatic intent-to-submission. Existing /api/requests and CLI run never implicitly plan.
- Suggest confirmation uses /api/requests with optional X-Agenova-Advice-ID provenance. Editing access is allowed under ordinary Policy, but changed task/context cannot reuse receipt.
- Console rechecks session/project/template before inference and execution. Trusted receipt travels with internal submission key; at actual prepare boundary automatic mode verifies current grants include required profile before allocation.
- Successful evidence carries optional accessAdvice. Draft failures remain separate receipts, never fake Work/provider facts.
- Platform resolver includes config in effective revision/lock. Composition reuses configured provider with a distinct planner profile/call budget. Local real-model verification uses existing loopback Ollama.

## Idempotency and Privacy

Key by trusted subject/team + original name; digest task/project/template/runtime/name (advice input has no explicit requestedAccess). Repeated identical key reuses result; different intent conflicts; concurrent calls cannot duplicate provider calls, failed attempts are retained. Auto retries first recover stored Work by owner/name and intent digest, including restart. Acceptance checks owner/TTL/current guide. No raw model response in audit, no caller-supplied identity/catalog, no credentials, no unlimited context.

## Alternatives Considered

MD-only arbitrary permission generation is harder to validate than structured profiles plus prose. Planner inside Worker would borrow nonexistent pre-claim authority. Mandatory global toggle breaks mixed clients. A new Skills/Task resource, scheduler, database or required base image adds unrelated coupling.

## Development and Test Sequence

1. Guide/config/parser/digest compatibility tests.
2. Selector adapters and bounded receipts; ambiguity/injection/failure/budget tests.
3. Authenticated HTTP and auto/confirm orchestration; current Policy shrink, identity, zero allocation and retry/restart tests.
4. CLI packaging, effective Platform composition, generated UI contracts.
5. UI mode/review/provenance and local examples.
6. Focused/integration/manual regression, real Ollama, Playwright/render review.
7. Full gates and exact evidence; local handoff; feature-branch commit/push only.

## Risks and Verification

Provider/runtime spies prove absence of calls; httptest verifies transport/identity; immutable registry tests verify guide snapshots. The existing PostgreSQL preview remains available. Because local Docker/PostgreSQL was unavailable, an explicit loopback-only `--volatile` acceptance mode uses real Go APIs/authorization with synthetic identity, worker and tool fixtures and memory-only Work history. Its Advisor uses actual configured Ollama inference; a rules config is separate and labeled. This is not real PostgreSQL restart or AWS/network proof. Report the existing Windows Owner-bootstrap directory-sync failure without hiding/skipping it. Exact evidence and reproduction steps are in [evidence.md](evidence.md).

Mature references: [Skills metadata/disclosure](https://www.anthropic.com/research/equipping-agents-for-the-real-world-with-agent-skills); [MCP untrusted annotations](https://modelcontextprotocol.io/specification/2025-06-18/server/tools). Reuse knowledge packaging and hint/authority separation, not authorization through Markdown.
