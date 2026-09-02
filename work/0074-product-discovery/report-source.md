# Product Discovery Source Ledger

**Research date:** 2026-08-25
**Companion report:** [Product Discovery: Where Agenova Can Still Matter](report.md)

This ledger separates first-person problem signals from official capability evidence. A signal means that a public participant described a problem or workaround; it does not establish prevalence, budget, or intent to adopt Agenova.

## First-Person Problem Signals

| ID | Date | Ecosystem / persona | Public signal, workaround, and consequence | Possible Agenova fit | Confidence |
| --- | --- | --- | --- | --- | --- |
| P01 | 2026-05-20 | MCP Enterprise IG; Okta identity practitioner | Enterprise admins can govern SaaS login but cannot see or revoke MCP connections to the same systems, leaving an unmonitored access path ([catalog §1.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Preserve trusted principal and connection/claim evidence above individual MCP sessions. | High signal; one draft catalog entry, not prevalence data. |
| P02 | 2026-05-20 | MCP Enterprise IG; TraceForce practitioner | Enterprise AI client identity and MCP-server credentials can represent different people; audit attribution and policy break ([catalog §1.2](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Bind upstream principal and assignment to gateway facts. | High signal; named contributor. |
| P03 | 2026-05-20 | MCP Enterprise IG; EmpowerID practitioner | Recursive delegation loses human/organization lineage and revocation cannot propagate across trust domains ([catalog §2.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Parent/child authority narrowing and lineage. | High signal; proposed need, no implementation adoption evidence. |
| P04 | 2026-05-20 | MCP Enterprise IG; TraceForce practitioner | Downstream services cannot verify who authorized an MCP action or whether authority is still valid ([catalog §2.2](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Claim-bound principal/decision reference carried to enforcement adapters. | High signal. |
| P05 | 2026-05-20 | MCP Enterprise IG; Blue Shield of California practitioner | A healthcare query may cross five to seven tools while PHI classification and complete audit context are lost between steps ([catalog §3.2](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Evidence correlation can join steps; content classification is not an Agenova MVP capability. | High signal for problem; partial product fit. |
| P06 | 2026-05-20 | MCP Enterprise IG; audit contributors | Gateway/server logs are operator-controlled and there is no standard independently verifiable action record ([catalog §3.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Append-only claim facts and later receipt adapters. | Medium-high; desired trust level exceeds current MVP. |
| P07 | 2026-05-20 | MCP Enterprise IG; Saxo Bank practitioner | The bank built non-MCP interceptor middleware for PII and compliance because no portable pattern exists; every organization repeats the work ([catalog §6.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Adapter/conformance profile around existing gateways. | High signal; middleware need is broader than Agenova. |
| P08 | 2026-05-20 | MCP Enterprise IG; Silex/Solo practitioners | Gateways disagree on where on-behalf-of token exchange occurs, breaking multi-vendor interoperability ([catalog §4.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Record/mapping profile, not a new token exchange protocol. | High signal; specific integration gap. |
| P09 | 2026-05-20 | MCP Enterprise IG; developer-platform practitioner | The same MCP server needs separate credential/config formats for VS Code, Cursor, Claude Code, and Claude Desktop, increasing exposure and operations burden ([catalog §5.1](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761)). | Supports portability motivation, but ClaimRequest does not solve client config. | High signal; adjacent, not direct fit. |
| P10 | 2026-08-08 | MCP production multi-agent pipeline builder | Cannot verify scoped authorization, exact execution, or causal chain after 40+ tool calls; proposes a separate work-contract layer ([discussion #3215](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/3215)). | Very close conceptual validation of assignment plus facts. | Medium: author is building a competing protocol and the thread has limited engagement. |
| P11 | 2026-03-14 | AutoGen contributor/framework user | AutoGen lacks one standard interception point before tool execution for policy approval, audit, and sanitization ([issue #7405](https://github.com/microsoft/autogen/issues/7405)). | Tool Gateway adapter boundary and conformance test. | Medium: concrete proposal, no merged adoption. |
| P12 | 2026-04-05 | AutoGen contributor/multi-agent builder | Delegated agents can inherit the orchestrator's full tool set; most frameworks cannot attenuate capability through the chain ([issue #7528](https://github.com/microsoft/autogen/issues/7528)). | Parent/child claim narrowing. | Medium: proposed by another protocol author. |
| P13 | 2026-02-06 | kagent shared-cluster operator/contributor | kagent API/UI uses an allow-all authorizer; all users can access all namespaces, while Kubernetes RBAC cannot provide application actions or secret masking ([issue #1270](https://github.com/kagent-dev/kagent/issues/1270)). | Trusted caller plus `claim.create`/project/template policy above backend RBAC. | High signal; explicit affected personas and implementation plan. |
| P14 | 2026-07 | Kubernetes Agent Sandbox integrator | Wants an E2B-compatible API because E2B-dependent frameworks cannot move to self-hosted K8s without rewrites; cites compliance, sovereignty, and latency ([issue #1154](https://github.com/kubernetes-sigs/agent-sandbox/issues/1154)). | Supports backend/API portability and self-hosted niche. | Medium-high: direct request; may be solution advocacy. |
| P15 | 2026 | Kubernetes Agent Sandbox batch user | In a 30-claim batch, most claims timed out because Ready never became true; described as a complete blocker at scale ([issue #274](https://github.com/kubernetes-sigs/agent-sandbox/issues/274)). | Backend evidence and capability-gap visibility; lifecycle fix belongs upstream. | High signal for backend maturity, not governance demand. |
| P16 | 2026 | Kubernetes Agent Sandbox user | Requests idle lifecycle and argues that the core Sandbox controller—not SandboxClaim or an external layer—should own the lifecycle state machine ([issue #849](https://github.com/kubernetes-sigs/agent-sandbox/issues/849)). | Strong boundary evidence: Agenova must not own sandbox lifecycle. | High signal for scope boundary. |
| P17 | 2026-05-05 | Daytona operator integrating monitoring | Interactive, hourly JWTs make unattended quota monitoring fragile; requests a dedicated least-privilege API-key permission ([issue #4643](https://github.com/daytonaio/daytona/issues/4643)). | Shows identity/automation gaps, but native Daytona fix is appropriate. | High signal; low Agenova fit. |
| P18 | 2026 | Daytona automation user | External cron splits lifecycle ownership and creates failures invisible to Daytona audit, metrics, and webhooks ([issue #4661](https://github.com/daytonaio/daytona/issues/4661)). | Claim facts could correlate external scheduling, but lifecycle should remain native. | High signal; counterevidence against Agenova owning scheduling. |
| P19 | 2026-04-26 | Claude Code user/coding-agent operator | A read-oriented WebFetch allow rule also expands arbitrary sandbox network access, so `git push` can bypass intended approval; deny-list workaround is fragile ([issue #53511](https://github.com/anthropics/claude-code/issues/53511)). | Demonstrates need to separate semantic tool policy from network enforcement evidence. | High signal; issue closed as not planned. |
| P20 | 2026-08-07 | Claude Code Linux user/security reporter | Sandbox allowed credential-file reads and the agent could edit or invalidate its own policy without fail-closed behavior ([issue #84863](https://github.com/anthropics/claude-code/issues/84863)). | Outer RuntimeBackend can provide independent containment evidence. | High signal for defense in depth; vendor may fix quickly. |
| P21 | 2026 | Claude Code desktop user | Requests explicit local-session filesystem, network, credential and approval scopes rather than all-or-nothing autonomy ([issue #81121](https://github.com/anthropics/claude-code/issues/81121)). | Supports scoped-assignment UX; managed product remains the likely solution. | Medium-high. |
| P22 | 2026-07-11 | OpenTelemetry GenAI contributor/operator | Tool spans show what executed but not the invocation-time risk posture, preventing cross-vendor safety dashboards ([issue #373](https://github.com/open-telemetry/semantic-conventions-genai/issues/373)). | Map claim decision/evidence references to OTel; do not replace OTel. | Medium: proposal, not adopted convention. |
| P23 | 2026-06-03 | OpenTelemetry GenAI contributor/platform operator | Traces need opaque joins to external policy, approval, and execution-context records without embedding sensitive payloads ([issue #239](https://github.com/open-telemetry/semantic-conventions-genai/issues/239)). | Claim and decision IDs can be the producer-owned join targets. | Medium: active proposal, not finalized. |

The ledger exceeds the minimum 15 signals and covers at least six independent ecosystems: MCP Enterprise, AutoGen, kagent, Kubernetes Agent Sandbox, Daytona, Claude Code, and OpenTelemetry.

## Counterevidence and Existing Capability Map

| Source | What it already solves | Remaining boundary relevant to Agenova |
| --- | --- | --- |
| [AWS AgentCore overview](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/what-is-bedrock-agentcore.html) | Modular or integrated runtime, memory, gateway, identity, policy, observability, evaluation, sandbox tools; any framework/model. | AWS control plane and semantics; Agenova would need cross-provider value. |
| [AWS externally hosted workload identities](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/understanding-agent-identities.html) | Manual identities for self-hosted/hybrid agents, credential providers and centralized directory. | Self-hosting alone is not an unmet identity niche. |
| [AWS policy sessions](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/policy-session-based-temporal.html) | Principal-bound logical task/session, multi-hop identity propagation, history-aware tool policy, non-Runtime callers. | Agenova must add open portability, runtime/outcome evidence, or clearer assignment semantics. |
| [Microsoft Foundry Hosted Agents](https://learn.microsoft.com/en-us/azure/foundry/agents/concepts/hosted-agents) | Custom container, dedicated identity, per-session VM isolation, persistent files, tools, OTel, versioning. | Azure-managed environment; no proof of cross-runtime contract demand. |
| [Microsoft private networking](https://learn.microsoft.com/en-us/azure/foundry/agents/concepts/networking-options) | BYO VNet, private ingress/egress, on-prem access, customer data resources. | Data-control needs do not automatically require self-hosted Agenova. |
| [Google agent security/governance](https://cloud.google.com/blog/products/identity-security/whats-new-in-iam-security-governance-and-runtime-defense) | First-class Agent Identity, gateway, IAM policies, approval conditions, registry, VPC guardrails, runtime defense. | Google ecosystem and several preview capabilities; strong future competitor. |
| [OpenAI Codex safety](https://openai.com/index/running-codex-safely/) | Native sandbox, approval, network proxy, managed configuration, credential handling, OTel/compliance logs. | Single-product users need little from Agenova. |
| [Anthropic sandbox engineering](https://www.anthropic.com/engineering/claude-code-sandboxing) | OS-level filesystem/network sandbox and managed permission UX. | Product-specific gaps may be fixed natively. |
| [Kubernetes Agent Sandbox roadmap](https://github.com/kubernetes-sigs/agent-sandbox/blob/main/roadmap.md) | Portable backend, claim-time identity/network/storage, audit/telemetry, SDKs, lifecycle and UI are planned or in progress. | Assignment purpose, upstream principal/policy resolution, tool/model facts and outcome remain above sandbox, but overlap is growing. |
| [Kubernetes Agent Sandbox threat model](https://github.com/kubernetes-sigs/agent-sandbox/blob/main/docs/security/threat_model.md) | CRDs, managed network policy, disabled service-account token default, runtime-class composition, router controls. | Confirms substrate evidence and product boundary; Agenova cannot claim isolation itself. |
| [Daytona](https://github.com/daytonaio/daytona) | Dedicated isolation, network controls, secrets, organizations, lifecycle, audit, OTel, BYOC/self-host options. | Backend-neutral task authority is not its core abstraction, but many required controls exist. |
| [Agyn](https://github.com/agynio/platform) | Open K8s platform, any agent container, RBAC/SSO/audit, credential isolation, zero-trust network, Terraform. | Directly competes for self-hosted platform teams. |
| [LiteLLM Agent Control Plane](https://github.com/LiteLLM-Labs/litellm-agent-control-plane) | Unified API/UI and access across multiple agent runtimes, sessions, schedules and memory. | Strong counterexample to broad backend-neutral control-plane positioning. |
| [Lenny](https://github.com/lennylabs/lenny) | Specified runtime-neutral sessions, credentials, recursive delegation, policy, evidence and K8s isolation. | Design-stage but heavily overlaps Agenova's intended surface. |
| [Google AX](https://github.com/google/ax) | Self-hosted distributed runtime, event log, policy/audit coordination, recovery, agent/tool isolation, model/harness neutrality. | Agenova should not become a distributed agent executor. |
| [Permit MCP Gateway](https://docs.permit.io/permit-mcp-gateway/overview/) | Tool access, human-to-agent delegation, policy, consent and audit; hosted or self-hosted. | It explicitly does not own runtime/network containment; potential adapter, not a gap to rebuild. |
| [OpenTelemetry GenAI attributes](https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/) | Agent, conversation, model and tool telemetry vocabulary. | Governance decisions remain producer-owned; Agenova can expose join references. |
| [OpenID AuthZEN drafts](https://openid.net/openid-foundation-advances-authorization-for-the-agent-era-with-new-authzen-working-group-drafts/) | Interoperable access request/approval prerequisites and MCP authorization profile. | Use as a PDP/approval contract; do not create a new general authorization protocol. |
| [IETF WIMSE documents](https://datatracker.ietf.org/group/wimse/documents/) | Workload identity, agent applicability, delegation, execution context and authorization-evidence drafts. | Agenova can be an implementation/conformance consumer; standards are not stable yet. |
| [MCP work-contract discussion](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/3215) | Near-identical pre-authorization, execution receipt, work-order and causal-chain proposal. | Direct conceptual competition; limited engagement means neither proposal has adoption validation. |
| [Signet](https://github.com/Prismer-AI/signet), [Obsigna](https://github.com/agent-receipts/obsigna), [Agent Action Authorization Receipt Protocol](https://github.com/SamuraiWriter7/agent-action-authorization-receipt-protocol) | Open signed authorization/execution receipts, SDKs or proxies. | Crowded proposal space with modest public adoption signals; avoid receipt-led positioning. |

## Public Validation Targets

These targets are relevant because they have a concrete open issue, standard, or integration surface. They are not customers or endorsers.

| Target | Why high signal | Validation question |
| --- | --- | --- |
| [MCP Enterprise IG](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761) | Named enterprise contributors and cross-product pain catalog. | Would an opaque assignment/decision/evidence join reduce custom middleware, and who would emit it? |
| [Kubernetes Agent Sandbox #1154](https://github.com/kubernetes-sigs/agent-sandbox/issues/1154) and maintainers | Self-hosting/API-portability use case plus an upstream `SandboxClaim` collision. | What governance remains deliberately above Agent Sandbox, and would conformance tests be welcome? |
| [kagent #1270](https://github.com/kagent-dev/kagent/issues/1270) contributor/operators | Explicit shared/multi-tenant application authorization gap. | Is task-scoped effective authority useful after native RBAC, or is native RBAC sufficient? |
| [AutoGen #7405](https://github.com/microsoft/autogen/issues/7405) / [#7528](https://github.com/microsoft/autogen/issues/7528) | Concrete interception and delegation gaps. | Would a framework adapter consume a stable external contract or only expose native hooks? |
| [OpenTelemetry GenAI SIG](https://github.com/open-telemetry/semantic-conventions-genai) | Owns cross-vendor observability vocabulary and is discussing governance references. | Which opaque IDs/relationships can be standardized without embedding policy payloads? |
| [LiteLLM Agent Control Plane](https://github.com/LiteLLM-Labs/litellm-agent-control-plane) | Popular open runtime-neutral control plane; strongest direct substitution test. | What cannot its existing run/session model express, and would a separate assignment profile help? |
| [Agyn](https://github.com/agynio/platform) | Complete self-hosted K8s platform with identity and credential isolation. | Is cross-runtime evidence portability useful enough to justify an external contract? |
| [OpenID AuthZEN](https://openid.net/wg/authzen/) / [IETF WIMSE](https://datatracker.ietf.org/group/wimse/documents/) | Emerging authorization and workload-identity standards. | Can Agenova serve as a small implementation/conformance lab instead of defining overlapping primitives? |

## Search Coverage, Gaps, and Stopping Rationale

### Coverage

- Managed platforms: AWS, Azure, Google, OpenAI, Anthropic.
- Sandbox/runtime: Kubernetes Agent Sandbox, E2B, Daytona, Google AX, Agyn, LiteLLM, Lenny, Forge.
- Gateways/identity: Permit, Solo agentgateway, AWS AgentCore, Google Agent Gateway, AuthZEN, WIMSE.
- Observability/evidence: OpenTelemetry GenAI, MCP audit discussions, receipt projects.
- First-person sources: seven ecosystems and more than 20 coded signals.
- Counterevidence: vendor-complete paths, native framework fixes, project overlap, low-fit users, and weak adoption of independent receipt protocols.

### Remaining evidence gaps

- No private operator interviews or verified production deployment counts.
- No evidence that a team will own and maintain an Agenova adapter.
- No comparison based on running the competing products; capability mapping is documentation-based.
- No validated willingness to pay; this is an open-source adoption study, not market sizing.
- No regulatory legal analysis; compliance statements from community sources were treated as pain signals, not legal conclusions.

### Why research stopped here

Additional broad searches repeatedly returned new products and near-identical proposals but did not change the decision boundary. Capability saturation is well established. The unresolved question—whether external maintainers/operators will adopt a separate assignment contract—cannot be answered credibly with more desk research. It requires concrete public design interviews and an executable adapter profile.
