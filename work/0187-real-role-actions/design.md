# Technical Design: Real role-scoped PR and rollback proof

- Ticket: [#187](https://github.com/wunderforge/agenova/issues/187)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

The Tool Gateway already checks Running claim, effective operation and resource scope, but the console adapter returns static incident strings. The kind worker and Ollama inference are real. Installed Control Plane Pods cannot use the host's GitHub keyring without moving a long-lived token into Kubernetes; the demo must not do that implicitly.

## Decision

Use the local, loopback-only controlled-kind console as the trusted host composition for this bounded demonstration. It still allocates real Agent Sandbox workers and sends all model/tool requests through the same claim-bound gateways. Inject one host provider with a fixed repository checkout and fixed `kind` context/namespace/Deployment. `gh` uses existing host keyring credentials; `kubectl` uses the host's selected context. Provider commands use argument arrays, bounded timeouts and exact target validation, never a shell.

The agent sees available operation names and may choose whether to call them. The Tool Gateway—not the prompt—decides allow/deny. A successful tool decision is followed by a separately recorded provider attempt/outcome. The adapter returns bounded, sanitized output including PR URL or rollout revision; no token or raw command output enters evidence. The PR operation accepts model-authored replacement content for one allowlisted source file and runs its tests in a pinned, unprivileged Docker image with no network, a read-only checkout, resource limits, and no host credentials before it commits and opens a PR. The model-authored code is never executed by host `go test`. The rollback operation is pinned to one disposable Deployment and verifies the expected bad-to-good transition. For denial Work, the objective asks the model to attempt the forbidden operation and then report the result; no call is synthesized if the model refuses.

## Ownership and Contract Boundaries

- `internal/authority` and `internal/policy`: generic team tool-ceiling intersection and evidence reason.
- `internal/console`: injected provider seam and correlated provider facts; default synthetic provider remains explicitly labeled.
- `internal/workerprotocol`: operation grammar and task-driven multi-turn behavior, no authority decision.
- `cmd/agenova-console` plus a demo-only host provider package: operator configuration, target allowlist and real `gh`/`kubectl` commands.
- Dedicated demo repo/namespace and Playwright checks: external side effects and read-only UI proof.

## Alternatives Considered

- Copy host GitHub token into an installed Control Plane Secret: rejected for this demo because it expands credential handling and cluster exposure before the E14 credential boundary is ready.
- Preprogram a patch or replay fixed ToolInvocation facts: rejected because it would not prove model-authored, governed action.
- Present `change.propose` or `rollback.propose` as execution: rejected because neither changes an external target.

## Verification Strategy

Unit tests exercise exact target/input validation, isolated test invocation, and no-command-on-Deny. Real evidence records the model's chosen action, Tool Gateway decision, provider attempt/outcome, PR URL/commit and Deployment before/after revision. The buggy fixture must fail and the model-authored PR must pass in the same restricted test container. A read-only browser test asserts that both permitted and denied facts appear without inventing provider outcomes for denied calls.

## Risks and Compatibility

The local demo still simulates identities and incident data; disclose both. Rollback must only touch the named disposable Deployment, and PR creation must only touch the new demo repo. If model behavior fails to call a desired tool, report the attempt as unproven and adjust the task/evidence—not the journal. A pre-attempt journal failure blocks the provider. A post-action evidence failure cannot undo an external PR or rollout and requires checking that target directly. Existing installed Platform behavior stays unchanged.
