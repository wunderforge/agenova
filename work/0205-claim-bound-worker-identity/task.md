# Task: Claim-bound worker identity at governed Gateways

- Ticket: [#205](https://github.com/wunderforge/agenova/issues/205)
- Mission: Authenticate a live worker as exactly one issued claim before either Gateway uses its authority or invokes an adapter.
- Target: workload identity, `RunService`, Agent Sandbox launch, Tool/Model Gateway, tests and installed-kind smoke.
- User value: A worker cannot borrow another Work's rights by naming its claim ID, and loses its own rights after termination.
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority).

## Context to Read

Always: `AGENTS.md`, `docs/product/prd.md`, and this packet.

Additional task-specific context:

- [#205](https://github.com/wunderforge/agenova/issues/205), predecessor [#204](https://github.com/wunderforge/agenova/issues/204), and original worker authentication contract [#121](https://github.com/wunderforge/agenova/issues/121).
- [Feature spec](spec.md), [design](design.md), [authority/credentials architecture](../../docs/product/architecture-contract.md#authority-and-credentials), [core-contract](../../docs/harness/playbooks.md#change-a-core-contract) and [backend](../../docs/harness/playbooks.md#add-or-change-a-runtime-backend) playbooks.
- `internal/app/run_service.go`, `internal/modelgateway`, `internal/toolgateway`, `internal/runtime/agentsandbox`; [E13 design](../0175-shared-eks-bedrock/design.md) only for the AWS consumption boundary.

## Scope

In scope:

- Issue short-lived Agenova workload identity only after authoritative claim/worker binding, expiring no later than the claim deadline.
- Verify issuer, intended Gateway audience, expiry, claim/worker binding, Running phase and revocation before Tool/Model adapter calls.
- Treat caller-supplied claim IDs only as consistency checks; keep signing keys and external credentials out of worker configuration and evidence.
- Prove positive, cross-claim and post-terminal behavior in installed kind.

Out of scope:

- EKS Pod Identity, IAM roles or per-claim AWS credentials; Bedrock/S3 adapter behavior; general secret management; enterprise federation; Operator and parent/child claims.

## Acceptance Criteria

- A bound worker invokes both Gateways only under its own Running claim's immutable authority, with correlated facts.
- Live calls cannot fall back to a bare claim ID or shared static token.
- Worker launch carries only bounded Agenova identity; the trusted service retains signing and provider credentials.

## Negative Case

- Worker A presents its valid credential while nominating claim B: both Gateways deny before adapter invocation, create no B-attributed invocation fact and do not read B's authority. Missing, malformed, expired, wrong-audience, wrong-worker, revoked and terminal credentials fail closed too.

## Execution Todo

- [x] Confirm the prepared #204 authority contract was explicitly accepted by the Owner; scout the live Gateway and Agent Sandbox seams.
- [x] Assignee self-review scope, negative cases, AWS consumption boundary, dependencies and gates against #205, PRD and architecture.
- [x] Implement bounded issue/verify with claim, worker and deadline binding.
- [x] Integrate verification before authority lookup in both Gateways and deliver identity only to the bound worker.
- [x] Add focused positive and zero-external-call negative evidence; verify existing worker manifest checks and absence of token values in captured evidence.
- [x] Run installed-kind live-worker cross-claim/post-terminal smoke, focused tests and `./scripts/check.ps1 -All`.
- [x] Review diff and record exact evidence and residual AWS/E15 risks in this packet; PR/Ticket reporting follows publication.

## Quality Gates

- `go test ./internal/app ./internal/modelgateway ./internal/toolgateway ./internal/runtime/agentsandbox` plus workload-identity package tests.
- Installed-kind live-worker Gateway smoke with adapter call counts and claim-attributed facts.
- `./scripts/check.ps1 -All`.

## Evidence Required

- Sanitized issuance/verification and worker launch records without token contents.
- Adapter-spy positive and negative call counts for both Gateways, including claim-A-token/claim-B-target and terminal denial.
- Installed-kind worker transcript and exact focused/full gate output, or an explicit blocker.

## Constraints

- Preserve `docs/product/architecture-contract.md`; shared types stay backend-neutral. Do not broaden #205 or PRD without a recorded human decision.
- E13's control-plane EKS Pod Identity is a separate Gateway-to-AWS credential boundary. Do not copy it to workers or claim per-claim IAM enforcement.

## Decisions and Blockers

- 2026-10-13: Prepared after Owner asked to implement the claim-to-AWS enforcement chain. #205 supplies the E14 worker-to-Gateway foundation; E13 owns real EKS/Bedrock validation.
- 2026-10-13: #204 is open and unmerged; implementation waits for its authority contract to stabilize.
- 2026-10-13: Owner approved this packet and implementation on the prepared #204 branch. The live worker path is a pinned kubectl exec/stdio exchange; the short-lived credential is sent only in that session, never in the Pod manifest. A model-audience token and a tool-audience token are verified at their respective Gateways before authority lookup.
- 2026-10-13: Focused Go packages passed. The installed-kind local transcript (removed from Git at Owner request) shows one actual Agent Sandbox worker made two model and one tool Gateway calls; cross-claim and post-terminal attempts caused zero further adapter calls, then its resources were released. The disposable test namespace was deleted. Final `./scripts/check.ps1 -All` exited 0, including 52/52 browser smoke tests.
- 2026-10-13: EKS IAM/Bedrock enforcement and two real OIDC users remain unverified in this Ticket. The current control plane uses a per-process signing key; restart invalidates outstanding credentials, which is safe for the memory-only reference service but needs a separate durability design if the service becomes replicated.

- 2026-10-20: Ported M4 to the current member-service/M3 stack. The live executor retains main's dynamic tool catalog and per-operation membership checks; both Model and Tool Gateway calls now require claim-bound workload credentials. Focused Go packages passed. The earlier kind transcript is intentionally excluded from Git per Owner request. Real installed-kind behavior on this port remains to be rerun before acceptance.
