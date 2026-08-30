# Harness Gotchas

Only recurring or high-risk Agenova mistakes belong here.

## One question must have one authority

- Failure: updating the same requirement, status, or task detail in several Markdown files and GitHub.
- Correct boundary: use `docs/development/AIDLC.md` to locate the authority; summaries link to it and mutable delivery state stays in GitHub.
- Gate: source-of-truth structure and local-link checks in `./scripts/check.ps1 -Docs`.

## Claim is not a tool call

- Failure: modeling one `SandboxClaim` per tool invocation.
- Correct boundary: one claim is one worker run; tool/model calls are facts below it.
- Gate: lifecycle and gateway tests plus the architecture-authority check in `scripts/check.ps1`.

## A request is not authority

- Failure: treating YAML fields or CLI flags as permissions already granted to the worker.
- Correct boundary: `ClaimRequest` expresses requested access; task input alone grants nothing. Agenova intersects requested tools and resource scopes with template, policy, and runtime limits before creating the system-managed claim.
- Gate: resolver negative tests plus the architecture-authority check in `scripts/check.ps1`.

## Backend readiness is not work success

- Failure: mapping Pod/worker readiness to `Succeeded`, or Pod existence directly to Agenova `Running`.
- Correct boundary: allocation maps to `Bound`; Agenova runner start maps to `Running`; runner outcome sets the terminal phase.
- Gate: backend contract tests and real adapter evidence.

## Backend types must not leak

- Failure: importing Kubernetes or provider types into `api`, shared runtime, gateways, or clients.
- Correct boundary: translate only inside the adapter package.
- Gate: `./scripts/check.ps1 -Docs` runs the runtime-boundary scan.

## Gateway tests are not isolation proof

- Failure: claiming secrets or egress are secure because an in-process authorization test passes.
- Correct boundary: distinguish policy behavior, gateway transport, workload identity, network bypass prevention, and runtime isolation.
- Gate: integration evidence appropriate to each layer.

## A backend gap is not parity

- Failure: using local adapter state to claim a provider natively satisfies durable terminal semantics.
- Correct boundary: label the gap, test the supported subset, and define promotion criteria.
- Gate: backend note, adapter test, and reproducible real-backend output.

## Vision is not implementation status

- Failure: listing CLI, CRDs, Helm, memory, UI, or OpenTelemetry as current capabilities because they are target concepts.
- Correct boundary: update `docs/project-status.md` with exact implemented/spike/missing labels.
- Gate: docs review and runnable evidence.
