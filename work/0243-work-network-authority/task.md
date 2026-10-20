# Task: Per-Work network authority and fail-closed egress

- Ticket: [#243](https://github.com/wunderforge/agenova/issues/243)
- Mission: Enforce outbound networking as immutable per-Work authority, outside arbitrary images.
- Target: canonical contracts, Policy/authority, RunService, runtime adapters, evidence and Portal.
- User value: Reusing an image does not silently give every task Internet access.
- PRD outcome: [Authority](../../docs/product/prd.md#4-claim-scoped-authority).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Spec](spec.md), [design](design.md), [architecture](../../docs/product/architecture-contract.md), [AIDLC](../../docs/development/AIDLC.md), [playbooks](../../docs/harness/playbooks.md).
- `api/v1alpha1`, `internal/policy`, `internal/authority`, `internal/app`, `internal/runtime`, `internal/console/recovery.go`; findings #202/#208/#201 and E15 #52.

## Scope

In scope:

- Optional networkProfiles request/ceiling/constraint/grant; public-web profile; default deny from startup; Work-specific activation/release, backend expiry, recovery and safe evidence. Integrate locally with MCP #242 before one release.

Out of scope:

- FQDN/L7 proxy, new CNI/cloud resources without approval, credential-store expansion, LLM grants, new Worker protocol/base-image constraints.

## Acceptance Criteria

- Same user/template/Policy, two Works: only explicitly requested and approved public-web reaches public TCP443 and DNS. Missing request is denied; unapproved profile fails before allocation.
- Idle/new Workers have no standing egress. Running activation and terminal/cancel/expiry/recovery are claim-bound; actual enforcement/release are distinguished from business outcome.
- Real MCP/model tasks and image/auth/history/observability regressions pass. Local/full Linux/race/UI gates and independent review precede integrated merge; AWS probes and real-model reliability precede completion.

## Negative Case

- Reject unknown/unrequested profiles, unsupported enforcement, forged worker bindings, failed writes and ambiguous cleanup. Test startup, public DNS/IP, existing connections, peer isolation and service outage; never fall back to permissive networking.

## Execution Todo

- [ ] Scout the relevant implementation, tests, risks, and dependencies.
- [ ] Assignee self-review: check scope, acceptance, negative case, constraints, and gates against the Ticket and PRD; record material decisions.
- [ ] Implement shared authority contracts and runtime enforcement/deadline lifecycle; complete MCP acceptance in parallel.
- [ ] Complete local joint acceptance and independent review; one integrated PR/release, then AWS acceptance/repair.
- [ ] Add or update focused behavioral evidence.
- [ ] Run the focused gate and `./scripts/check.ps1 -All`.
- [ ] Review the diff for scope, regressions, and source-of-truth updates.

## Quality Gates

- Focused API/Policy/authority/app/runtime/operator tests, affected race suites and actual SDK composition.
- `npm --prefix ui run check`; supported Linux full gate and real-backend probes.
- `.\scripts\check.ps1 -All`

## Evidence Required

- Record all attempts; A/B MCP each target 9/10 real-model business success and all security cases pass with independent positive-controlled receipts. Scripted inference is not model evidence; simulated backend is not CNI proof.

## Constraints

- Preserve `docs/product/architecture-contract.md`.
- Do not broaden the Ticket or PRD without a recorded human decision.
- User authorized this scope, parallel work, integrated merge/deployment and AWS repairs on 20 October 2026. This does not waive branch protection or approve new infrastructure/wider shared RBAC.
- Preserve dirty MCP work, credentials and unrelated shared resources; no half-integrated main merge.

## Decisions and Blockers

- Owner wunderforge. Public-web is broad public TCP443 and necessary DNS, not domain/read-only enforcement. Unsupported startup isolation must fail visibly. New shared RBAC requires separate approval. See design for lane ownership.
- Owner explicitly approved namespace-local NetworkPolicy get/create/patch/delete for the shared control plane on 20 October 2026. No Secret, IAM, cross-namespace, Pod exec or administrator expansion was authorized by this approval.
- Owner separately approved namespace-local Pod patch on 20 October 2026, solely for guarded original-Pod incarnation labeling. The implementation checks the original Pod UID and claim label atomically; no Pod create/delete/exec, Secret, IAM, cross-namespace or administrator expansion is included.
- Policy caps are ceilings: an offline request remains offline even when the same Policy permits requesting public-web. Opaque optional backend allocation incarnation is persisted to prevent same-name replacement adoption during restart recovery.
- On 21 October 2026 the Owner explicitly authorized administrator merge for PR #244 only, conditional on all gates passing. This does not waive fixing independent-review findings or expand cloud/RBAC scope. AWS SSO must be refreshed before coordinated release preparation.
