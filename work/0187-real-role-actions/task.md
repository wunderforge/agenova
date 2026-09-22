# Task: Real role-scoped PR and rollback proof

- Ticket: [#187](https://github.com/wunderforge/agenova/issues/187)
- Mission: Prove that one reusable agent template can perform different real, governed actions for Developer and SRE, while cross-role attempts are denied before side effects.
- Target: reference identity/policy resolution, local controlled-kind console composition, Tool Gateway provider adapter, ReAct worker, isolated GitHub and kind targets, evidence API/UI tests.
- User value: A viewer can inspect both what the agent actually did and what Agenova prevented, without confusing a proposal or synthetic fixture with an external operation.
- PRD outcome: [claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [facts/accountability](../../docs/product/prd.md#5-facts-and-accountability).

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, this packet.

- [Specification](spec.md), [design](design.md), [architecture contract](../../docs/product/architecture-contract.md)
- [Tool Gateway](../../internal/toolgateway/gateway.go), [console composition](../../internal/console/service.go), [worker protocol](../../internal/workerprotocol/react.go)
- [controlled kind runtime](../../internal/runtime/agentsandbox/controlled.go), [browser checks](../../harness/e2e)

## Scope

In scope: one dedicated payment demo repository and one isolated kind Deployment; host-side authenticated GitHub/Kubernetes tool operations; two operator-selected reference principals; a shared template with per-team policy ceilings; four real Work runs (two permitted, two cross-role denied); CLI/API/UI evidence.

Out of scope: production SSO, arbitrary repository writes, general Kubernetes administration, platform upgrade rollback, production credential issuance, and claiming that this local reference composition is a production identity boundary.

## Acceptance Criteria

- Developer's real model-directed `github.pr.create` invocation creates a reviewable PR containing model-supplied, test-validated code in the dedicated repo; SRE's real model-directed `kubernetes.rollback` changes the dedicated Deployment revision and reaches Ready.
- One shared template requests both operations; effective authority differs by trusted operator preset and policy, not by prompt text.
- A Developer attempt to roll back and an SRE attempt to create a PR each produce a Tool Gateway Deny with no downstream mutation; result and reason remain queryable.
- External credentials stay with the host-side provider, not the worker Pod, Work request, browser, or facts. Observed PR/revision and provider attempt/outcome are correlated to the same claim.
- No post-hoc fabricated model/tool decisions, fake PR, or fake rollout. The synthetic incident scenario and reference identity switch are disclosed to the owner.

## Negative Case

- Ungranted tool calls never invoke `gh` or `kubectl`; malformed model code, unsafe path, unsupported revision, terminal claim, or failed *pre-attempt* evidence append cannot mutate the target. A failed post-action evidence append cannot undo an external effect; verify the target independently.

## Execution Todo

- [x] Scout current mock-tool limitation, policy ceiling, local model/worker path, and isolated target requirements.
- [x] Owner approved dedicated external targets and real allow/deny demo in this conversation on 22 Sep 2026; independent release review remains required.
- [x] Implement a narrow host-side real provider seam and retain the explicit mock adapter only for existing reference tests.
- [x] Create isolated repository and Deployment; register role-scoped policy/template; run four live Work cases.
- [x] Add deterministic allow/deny/provider tests and browser evidence assertions; run focused and full quality gates.
- [x] Review the diff and [live evidence](../../docs/evidence/187/role-actions.md), then hand owner the [recording path](../../deploy/reference/demo/role-rehearsal/README.md). Do not merge without the requested owner acceptance.

## Quality Gates

- `go test ./internal/authority ./internal/console ./internal/toolgateway ./internal/workerprotocol ./cmd/agenova-console`
- `go test ./...`
- `.\scripts\check.ps1 -All`
- Isolated kind/Ollama and Playwright evidence with independent GitHub PR/Deployment checks.

## Evidence Required

Exact Work IDs, policy version, two effective grants, permitted and denied ToolDecision/ProviderAttempt counts, created PR URL and commit, before/after Deployment image/revision, real model turns, cleanup, and UI screenshots/video if stable.

## Constraints

- Preserve `docs/product/architecture-contract.md`; tool/provider specifics stay out of ClaimRequest and RuntimeBackend.
- `gh` authentication remains in the local OS keyring, invoked only by the host provider. Never print/copy tokens or mount them into agent Pods.
- Bind any local API to loopback; target only the dedicated repo and namespace. Do not change the project's main branch or non-demo workloads.

## Decisions and Blockers

- The owner approved creating the dedicated GitHub repository and isolated kind Deployment. The hosted example may be synthetic, but PR creation, rollback, model calls, governance decisions and side-effect checks must be real.
- The existing installed reference path still has synthetic `git.read` and no PR/rollback adapter. This ticket uses a clearly labeled local host-side Tool Gateway composition so keyring credentials never enter kind; it does not claim to complete general installed Tool Backend or #172's same-service multi-identity requirement.
