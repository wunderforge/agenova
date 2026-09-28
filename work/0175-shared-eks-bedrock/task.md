# Task: E13 shared EKS service with governed Bedrock inference

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175); shared API [#146](https://github.com/wunderforge/agenova/issues/146).
- Mission: Run the existing Platform-selected service on EKS and prove authorized Bedrock inference through the same protected API for two independent clients.
- Target: internal/adapters/bundled, internal/modelprovider, cmd/agenova-control-plane, internal/connectedclient, internal/console, cloud examples and evidence.
- User value: Teammates submit and inspect independent governed Work against one real cloud service.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap). Cloud integration is the E13-selected extension, preserving the local reference path and architecture.

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this task, [spec](spec.md), [design](design.md).
- [Architecture](../../docs/product/architecture-contract.md), especially Installation, authority and external credentials.
- [Infrastructure handoff](../0175-aws-demo-bootstrap/HANDOFF.md).
- [Existing installed API task](../0167-shared-installed-api/task.md).
- [Runtime backend](../../docs/backends/agent-sandbox.md).
- [E14](https://github.com/wunderforge/agenova/issues/176), [E15](https://github.com/wunderforge/agenova/issues/177), #91 and #121 for identity contracts; current code at target paths.
- Official AWS Bedrock Converse, model availability, IAM workload identity and EKS network-policy documentation before implementing those integrations.

## Scope

In scope:

- Remote, operator-selected compatible images and pull behavior within existing Kubernetes deployment/runtime adapters.
- Native Bedrock provider selected by Platform, logical model-profile mapping, bounded invocation and fail-closed errors.
- Shared API connection/authentication contract with E14; integrate its verified principals and authorization for management, submission and evidence.
- Two-client live cloud scenario with positive inference and zero-provider-call denial evidence, worker credential and network checks coordinated with E15.
- Preserve kind/Ollama, provide reproducible setup, exact evidence, costs and cleanup.

Out of scope:

- LiteLLM, generic routing, token-budget product, enterprise SSO, HA, arbitrary worker images, durable Work history or installing Portal through platform apply.
- Silently implementing a parallel identity system or replacing E14 policy semantics.

## Acceptance Criteria

- Real EKS validate → plan → apply → status and identical reapply succeed using remote compatible images.
- One registered policy/template supports two independently authenticated clients against the same service and canonical evidence API.
- An authorized Gateway request succeeds against Bedrock with model/request metadata and correlated claim evidence.
- An unauthorized model request produces denial evidence and no provider call; a positive control proves the invocation counter actually observes calls.
- CLI and rendered local Portal agree on Work, policy, requested/effective authority and results.
- Same template and same request content under two trusted users demonstrate E14 policy narrowing, with both Work admitted and distinct effective permissions.
- Worker has no Bedrock/platform credentials; direct-provider bypass claims require live EKS enforcement evidence.
- Broken provider/configuration/authentication fails visibly; no Ollama/mock/memory fallback.
- Focused tests, kind/Ollama regression and full gate results are recorded with blockers honestly separated from passes.

## Negative Case

Invalid remote image/protocol, unknown provider/profile, anonymous or forged identity, unauthorized management/evidence, cross-claim and terminal invocations fail at their owning boundary. Denial cannot allocate unauthorized runtime or call the provider. Worker-controlled values cannot select IAM identity or endpoints.

## Execution Todo

- [x] Recheck main, Epic and shared API state, current local assumptions and open PRs.
- [x] Prepare explicit scope/spec/design from canonical template.
- [x] Owner explicitly approved this full task packet in the conversation on 2026-09-28.
- [x] Finish existing EKS readiness and ten-hour cleanup arrangement.
- [x] Implement and verify remote compatible image configuration.
- [x] Implement and verify bounded Bedrock adapter and Platform wiring.
- [ ] Integrate E14 shared principal/authorization contract; record unavailable dependency explicitly.
- [x] Capture real cloud positive/negative evidence and rendered Portal proof for the reference-identity slice.
- [ ] Run focused gates, kind/Ollama regression and full gate; review diff and document residual risks.

## Quality Gates

- `go test ./internal/modelprovider ./internal/adapters/bundled ./internal/platformapply ./cmd/agenova-control-plane ./internal/connectedclient ./internal/console`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`
- Existing reference kind/Ollama flow plus explicit EKS context cloud E2E; exact commands/results in evidence.

## Evidence Required

Sanitized install/status/reapply output; both client identities and correlated Work/claim evidence; Bedrock success/request ID and counted provider invocations; denial with zero-call delta; worker no-credential checks; real egress test before claiming bypass prevention; CLI/Portal consistency and rendered proof; cleanup deadline and inventory.

## Constraints

Keep provider details in adapters and long-lived credentials outside workers/repository. AUD100/month, no NAT/LB by default, cloud session at least ten hours after readiness then destroy. User messages authorize this wider outcome; no external messages to collaborators without explicit instruction.

## Decisions and Blockers

- 2026-09-28: user explicitly restored full E13 outcome; infrastructure alone is insufficient. This packet expands product work beyond the separately approved infrastructure task.
- main fetched and unchanged at c56ba3a; no currently open E14 implementation PR found; #91/#121 remain open and unassigned. Do not presume their contracts are delivered.
- Current image allowlist/hardcoded control-plane image, OpenAI-only provider, fixed reference Team A and loopback-only connected client remain in code.
- Existing All gate has one UI keyboard smoke failure (51 passed); no baseline/flakiness determination yet.
- A full enterprise SSO is excluded by E13/E14, but any narrow trusted identity implementation must be explicitly designed and tested; two fixed reference identities do not satisfy real multi-user acceptance.

- Owner approved on 2026-09-28 and selected server-verified demo credentials for two users (explicitly not enterprise SSO). Remote image and Bedrock provider initial implementation has focused passing tests. Full Go gate passes; UI remains 51/1 with the existing keyboard failure. Direct AWS CLI Nova Micro text and structured-tool preflight passed; installed Gateway path remains unverified.

- Latest Owner steering: defer E14 implementation; prioritize the E13 EKS/Gateway/Bedrock vertical path. Initial identity draft removed from active code (saved outside repository). Keep two-real-user, policy-narrowing and shared API acceptance explicitly pending E14. Do not expose the fixed-reference-identity API publicly.

- Cloud substrate verified: EKS ACTIVE, node Ready, Pod Identity agent ACTIVE, Agent Sandbox v0.4.6 controller Running. Both linux/amd64 images pushed to ECR by digest. Platform validate/plan succeeded against EKS; apply in progress. Model gateway role permits only regional Nova Micro; workers have no association.
- Bedrock structured Converse tool output preflight succeeded. A process-local SDK entry counter supports before/after denial evidence, with SDK retries disabled. This does not replace correlated Work facts or cloud request IDs.

- Final slice results: [evidence summary](evidence/summary.md). Full gate passed (52/52 UI), real installed E2E 2/2, two real Bedrock Work successes, admission denial with zero invocation delta, repeated installation unchanged. E14 is Owner-deferred; E15 bypass proof and live kind/Ollama regression remain explicitly unverified. The Epic remains open.

## Owner-authorized temporary HTTPS demo (2026-09-28)

Owner requested provider-issued HTTPS hostname and minimum access control, while E14 stays deferred. Supersedes the earlier prohibition on any public endpoint only for this authenticated demo perimeter. Deploy a separate EKS Pod with static Portal, password-protected loopback nginx and outbound Cloudflare Quick Tunnel. An explicit Pod-name-scoped Kubernetes port-forward connects to the unchanged private reference API; no control-plane restart or public Service. Only the forwarding container mounts its projected service-account token. Store a random shared demo password outside Git and its verifier in a Secret. This is shared reference-Team-A access, not two-user authorization. Check anonymous/wrong-password rejection, authenticated UI/API, cross-origin rejection and lack of public origin listeners. Keep the original cleanup deadline; remove the demo Deployment, RBAC, ConfigMaps and Secret with the cluster. Quick Tunnel hostname is temporary and has no availability guarantee.
