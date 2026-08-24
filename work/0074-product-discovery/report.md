# Product Discovery: Where Agenova Can Still Matter

**Research date:** 2026-08-25
**Decision status:** evidence-backed recommendation, not a PRD change
**Ticket:** [#74](https://github.com/wunderforge/agenova/issues/74)

## Executive Answer

The underlying problem is real: teams struggle to preserve who authorized an agent action, what authority was actually available, how that authority narrows across delegation, what happened at runtime, and how to reconstruct the result across tools and systems. Public evidence from the MCP Enterprise Interest Group, AutoGen, kagent, Kubernetes Agent Sandbox, Daytona, Claude Code, OpenTelemetry, NIST, IETF, and OpenID communities consistently exposes parts of this gap.

That does **not** validate Agenova as a broad “agent governance platform.” By August 2026, AWS AgentCore, Microsoft Foundry, Google Agent Platform, OpenAI Codex, Anthropic, Agyn, LiteLLM Agent Control Plane, Lenny, Permit, Solo agentgateway, and others already cover large combinations of runtime isolation, identity, policy, credential brokering, tool gateways, observability, audit, templates, and deployment. AWS even supports policy sessions scoped to a logical task and manually created workload identities for agents hosted outside AgentCore.

The credible remaining wedge is narrower:

> **Agenova can be an open, backend-neutral assignment contract and conformance kit that binds one trusted request, resolved authority, runtime evidence, governed invocations, and outcome across infrastructure the operator already chose.**

This is a **hypothesis with medium technical confidence and low market confidence**. Public sources show the interoperability problem and repeated reinvention. They do not yet show that platform teams will adopt another independent contract instead of selecting one cloud/platform, extending their framework, or waiting for standards to converge.

The project should therefore continue as a focused open-source validation project, not present itself as an established standard or a complete enterprise platform. Its next success criterion should be external adoption evidence, not feature count.

## What the Evidence Supports

### Supported with high confidence

1. **Capability is not the same as authority.** MCP standardizes tool connectivity and OAuth at a transport boundary, while its own community is actively debating agent delegation, audit context, signed authorization, and work receipts. The [MCP Enterprise pain-points catalog](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761) records identity loss, invisible MCP connections, non-portable middleware, incomplete audit chains, and inconsistent on-behalf-of handling from practitioners at Okta, Blue Shield of California, EmpowerID, Saxo Bank, and other organizations.
2. **Application policy and substrate isolation are separate controls.** A Claude Code user demonstrated that allowing a read-oriented `WebFetch` permission could also expand arbitrary sandbox network access, enabling operations such as `git push` without the intended approval ([issue #53511](https://github.com/anthropics/claude-code/issues/53511)). Kubernetes Agent Sandbox explicitly states that it composes with isolation runtimes and cluster controls rather than implementing isolation itself ([threat model](https://github.com/kubernetes-sigs/agent-sandbox/blob/main/docs/security/threat_model.md)).
3. **Self-hosted and portable execution has a real constituency.** Kubernetes Agent Sandbox users have requested an E2B-compatible API specifically for self-hosting, compliance, data sovereignty, and migration without framework rewrites ([issue #1154](https://github.com/kubernetes-sigs/agent-sandbox/issues/1154)). Agyn, LiteLLM Agent Control Plane, Google AX, Forge, and other open projects explicitly target self-hosted or runtime-neutral execution.
4. **The standards surface is moving quickly.** NIST is reviewing a project on software and AI-agent identity and authorization ([NCCoE project](https://www.nccoe.nist.gov/projects/software-and-ai-agent-identity-and-authorization)); IETF WIMSE has active agent identity, delegation, execution-context, and signed-authorization-evidence drafts ([document list](https://datatracker.ietf.org/group/wimse/documents/)); OpenID AuthZEN has working drafts for access requests, approvals, and MCP tool authorization ([announcement](https://openid.net/openid-foundation-advances-authorization-for-the-agent-era-with-new-authzen-working-group-drafts/)). Agenova should integrate or test these efforts, not invent a competing identity or general authorization protocol.

### Supported only as a hypothesis

1. A separate assignment contract will be adopted across multiple runtimes.
2. Platform teams prefer a small contract/conformance layer over one integrated control plane.
3. A `ClaimRequest` plus claim evidence is the right shared abstraction rather than a session, conversation, work order, execution, or vendor-specific run.
4. Operators will write and maintain RuntimeBackend adapters.
5. Reviewers value one cross-system evidence view enough to require it from agent platforms.

No public evidence found establishes purchase intent, deployment volume, or willingness to standardize on Agenova's schema.

## The Market Is Crowded in Every Broad Category

| Category | Existing coverage | Consequence for Agenova |
| --- | --- | --- |
| Managed agent platform | [AWS AgentCore](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/what-is-bedrock-agentcore.html), [Microsoft Foundry Hosted Agents](https://learn.microsoft.com/en-us/azure/foundry/agents/concepts/hosted-agents), and [Google Agent Platform](https://cloud.google.com/blog/products/identity-security/whats-new-in-iam-security-governance-and-runtime-defense) combine custom images/frameworks with runtime, identity, private networking, gateway, policy, telemetry, and audit. | Do not compete on “all production controls for agents.” |
| Managed coding-agent security | [OpenAI Codex](https://openai.com/index/running-codex-safely/) and [Claude Code sandboxing](https://www.anthropic.com/engineering/claude-code-sandboxing) already provide native sandbox, approvals, credential/network controls, and telemetry for their products. | A single-vendor coding-agent user is low fit. |
| Open/self-hosted runtime | [Google AX](https://github.com/google/ax), [Agyn](https://github.com/agynio/platform), [LiteLLM Agent Control Plane](https://github.com/LiteLLM-Labs/litellm-agent-control-plane), [Lenny](https://github.com/lennylabs/lenny), and [Forge](https://github.com/initializ/forge) cover runtime-neutral APIs, sessions, policy, credential isolation, delegation, audit, or Kubernetes deployment in different combinations. | Do not build another broad self-hosted agent platform. |
| Sandbox lifecycle | [Kubernetes Agent Sandbox](https://agent-sandbox.sigs.k8s.io/), [E2B](https://e2b.dev/), and [Daytona](https://github.com/daytonaio/daytona) own allocation, isolation, files, networking, warm state, and lifecycle. | Keep lifecycle inside RuntimeBackend adapters. |
| Tool authorization/gateway | [Permit MCP Gateway](https://docs.permit.io/permit-mcp-gateway/overview/), [Solo agentgateway](https://docs.solo.io/agentgateway/latest/mcp/), [AWS AgentCore Gateway](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/gateway-core-concepts.html), and Google Agent Gateway enforce identity and tool policy. | Tool Gateway should be a contract and adapter surface, not a new gateway product. |
| Agent identity and credentials | AWS supports externally hosted workload identities and user-agent-bound access tokens ([identity docs](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/understanding-agent-identities.html)); Google and Azure provide first-class or dedicated agent identities. | Consume upstream identity and short-lived credentials; do not build an IdP. |
| Observability | AgentCore, Foundry, Google, LangSmith, and OpenTelemetry GenAI conventions already trace agents, models, and tools. OTel is adding causality and governance references ([issue #239](https://github.com/open-telemetry/semantic-conventions-genai/issues/239)). | Map claim facts to OTel; do not invent a general telemetry vocabulary. |
| Authorization/receipt protocols | MCP has an almost exact “work contract” discussion ([#3215](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/3215)); small projects such as [Signet](https://github.com/Prismer-AI/signet), [Obsigna](https://github.com/agent-receipts/obsigna), and [Agent Action Authorization Receipt Protocol](https://github.com/SamuraiWriter7/agent-action-authorization-receipt-protocol) already propose signed authority and execution records. | “Authorization plus receipts” is not a unique claim; interoperability and practical conformance must be demonstrated. |

An especially important counterexample is [AWS AgentCore policy sessions](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/policy-session-based-temporal.html): a customer-supplied session can represent one logical task, binds to the authenticated principal, carries history across multiple tool calls and hops, and supports task-scoped temporal constraints. Agenova's differentiation cannot simply be “authority that lasts for one task rather than one tool call.” It must be openness, backend neutrality, a clearer assignment/outcome contract, and evidence portability—and those benefits still require validation.

## Ranked Users and Problems

### 1. Platform/security team with heterogeneous, self-hosted agent execution

**Profile:** already operates Kubernetes, VMs, on-premises or multiple clouds; has at least two agent frameworks/runtimes or expects migration; already owns IAM, policy, gateway, and observability components.

**Job:** define one bounded assignment and reconstruct its authority, runtime placement, governed calls, and outcome without replacing every existing component.

**Why this is the best candidate:** MCP practitioners report identity and audit context disappearing across product boundaries; Kubernetes Agent Sandbox users want API portability; self-hosted projects demonstrate demand for operator-controlled infrastructure. Agenova's non-owning adapter model is most useful where no single vendor is allowed to become the complete control plane.

**Fit:** high architectural fit; **medium-low demand confidence**. Agyn, LiteLLM, Lenny, and cloud services may be “good enough,” and operating another control plane has real cost.

### 2. Runtime/framework/gateway maintainer needing a governance interoperability profile

**Profile:** maintains an agent runtime, framework, sandbox, or gateway and does not want to own every policy/evidence concern.

**Job:** implement a small adapter and pass deterministic conformance tests showing how requests, authority, lifecycle, calls, and evidence map to the product.

**Evidence:** AutoGen lacks a standard pre-tool interception point ([#7405](https://github.com/microsoft/autogen/issues/7405)) and permission attenuation across delegation ([#7528](https://github.com/microsoft/autogen/issues/7528)); Kubernetes Agent Sandbox is developing portable backends and claim-time identity/network attachment ([roadmap](https://github.com/kubernetes-sigs/agent-sandbox/blob/main/roadmap.md)); OTel lacks settled governance joins and causal links.

**Fit:** high as an open-source distribution path; **medium demand confidence**. Maintainers may prefer native extension points or emerging standards instead of an Agenova-specific adapter.

### 3. Regulated platform reviewer who must reconstruct a task

**Profile:** security, compliance, platform risk, or incident-response reviewer in healthcare, financial services, government, or a multi-entity enterprise.

**Job:** answer who authorized the work, which policy/version resolved it, which runtime carried it, which side effects were observed, and whether authority ended.

**Evidence:** Blue Shield of California contributed multi-step PHI chain and audit gaps; Saxo Bank reported building non-portable middleware; Okta described invisible MCP connections in the [MCP Enterprise catalog](https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2761).

**Fit:** strong problem fit; **low product confidence**. The evidence validates the need, but not Agenova as the buying or adoption surface. Large organizations may select AWS/Google/Azure or standards-aligned products instead.

### Low-fit audiences

- Individual developers or small teams using one managed coding agent.
- Teams that only need an isolated code sandbox.
- AWS-, Azure-, or Google-standardized organizations satisfied with the native platform.
- Users seeking a workflow DAG engine, agent framework, chat UI, or memory product.
- Read-only diagnostic agents whose existing identity and logs are sufficient.
- Teams that cannot enforce gateway/network bypass prevention; a contract without an enforcement path would provide misleading assurance.

## The Self-Hosted Kubernetes Niche

There is a niche, but it is narrower than “people who run agents on Kubernetes.” Public signals show three motivations:

1. **Control of data and compute:** users cite compliance, data sovereignty, air-gapped environments, or a requirement to keep workloads in an existing cluster.
2. **Migration and API portability:** the Kubernetes Agent Sandbox E2B-compatibility request seeks migration without rewriting agent code.
3. **Existing platform investment:** K8s operators already have RBAC, workload identity, network policy, admission controls, telemetry, and deployment processes they want to reuse.

However, this is also a highly contested niche. Agent Sandbox is adding identity and policy attachment at claim time; Agyn provides a complete zero-trust self-hosted platform; LiteLLM provides a runtime control plane; and Lenny specifies an even broader runtime-neutral platform. “Self-hosted on K8s” is therefore a deployment qualifier, not a differentiated product position.

The better qualifying question is:

> Does the team need the **same assignment/evidence meaning across more than one runtime or enforcement product**, while keeping those products independently replaceable?

If the answer is no, Agenova likely adds unnecessary operational complexity.

## VM, Physical Host, and Nested Sandbox

The user's model is architecturally valid: a RuntimeBackend may allocate a VM, physical host, container, or Kubernetes sandbox; the runnable artifact inside it may then start Codex, Claude Code, E2B-compatible tooling, a browser sandbox, or another isolation mechanism. Agenova can govern the outer assignment while the inner product owns its own execution boundary.

This composition does **not automatically create stronger security**. It is useful only when the layers have independent responsibilities and the outer boundary can prevent bypass. For example:

- outer claim: trusted principal, task, resolved authority, timeout, backend identity, evidence correlation;
- outer substrate: filesystem/network/workload isolation and revocation;
- inner agent product: reasoning loop, tool-specific approvals, local process sandbox;
- gateway: credentials and external side effects.

If the same credentials, writable policy, host filesystem, or unrestricted network are visible through both layers, nesting mostly adds complexity. The Claude Code filesystem report ([#84863](https://github.com/anthropics/claude-code/issues/84863)) is a concrete reminder that an app-reported sandbox state is not sufficient substrate evidence.

Therefore nested execution should be documented as a supported adapter topology, not marketed as “double isolation” or a unique Agenova feature.

## Recommended Product Direction

### Keep

- One declarative request for one bounded worker assignment.
- Trusted upstream principal; requested access is never authority.
- Deterministic resolution of effective authority before backend allocation.
- Backend-neutral runtime interface and visible capability gaps.
- Claim-level lifecycle, parent/child narrowing, append-only facts, and a shared evidence view.
- A small executable reference implementation and real-backend proof.

### Clarify

- Agenova is a **governance interoperability contract plus reference implementation**, not the source of identity, the sandbox, the tool gateway, or the telemetry platform.
- Platform/runtime maintainers and heterogeneous self-hosted teams are the initial users; generic “agent developers” are not the primary target.
- A claim is a logical work assignment, not a conversation and not Kubernetes Agent Sandbox's resource claim. Resolve the public name (`WorkClaim`, `RuntimeClaim`, or another term) before external adoption to avoid collision with upstream `SandboxClaim`.
- “Standard” means an intended interoperable profile, not an adopted industry standard. Until external implementations exist, use “open contract” or “reference contract.”

### Deprioritize

- Building production Tool, Model, or Memory gateways.
- Broad policy authoring, IdP, workflow orchestration, durable multi-agent scheduling, and runtime lifecycle features.
- A large React console beyond proving the shared evidence contract.
- Cryptographic receipt schemes before a consumer requires them; the space already has many proposals and little demonstrated adoption.

### Integrate rather than invent

- Identity/delegation: WIMSE, SPIFFE, OAuth/OBO, AuthZEN, or upstream verified principal references.
- Policy decision interface: adapter to OPA/Cedar/AuthZEN-style PDPs; keep the MVP static evaluator as a reference only.
- Tool enforcement: MCP gateway or existing gateway adapters.
- Telemetry: OpenTelemetry GenAI attributes and opaque governance references.
- Runtime evidence: Kubernetes Agent Sandbox, VM/local, E2B/Daytona, or vendor runtime adapters.

### MVP adjustment to consider in a separate decision Ticket

Keep the existing vertical slice, but judge it by **portability and conformance**, not platform breadth:

1. Submit the same backend-neutral assignment to the reference backend and one real backend.
2. Resolve the same allowed/denied authority semantics before allocation.
3. Route one governed tool action through an existing or minimal gateway boundary.
4. Produce the same evidence representation with explicit backend capability differences.
5. Publish an adapter profile and executable conformance tests.

This remains compatible with the current architecture contract. Changing target users, public terminology, or deliverable priority requires a separate maintainer decision and PRD update.

## Positioning and Community Strategy

### Recommended concise position

> Agenova is an open contract and conformance kit for one governed agent assignment across runtimes. It binds trusted intent and temporary authority to runtime, tool/model facts, outcome, and backend evidence—without replacing your sandbox, gateway, identity provider, or agent framework.

### Do not claim

- “the standard runtime boundary” without adoption;
- stronger isolation merely because two sandboxes are nested;
- production security from an in-memory implementation;
- a complete enterprise governance platform;
- unique tool authorization or audit receipts.

### High-signal public communities and potential collaborators

These are validation targets, not endorsements:

1. **MCP Enterprise Interest Group** — test whether an assignment ID plus authority/evidence joins solve catalogued multi-product gaps.
2. **Kubernetes Agent Sandbox maintainers and #1154 participants** — test whether a governance profile belongs above the sandbox API and whether claim terminology conflicts.
3. **kagent #1270 contributors/operators** — test whether application RBAC users also need task-scoped authority and cross-runtime evidence, or whether native kagent RBAC is sufficient.
4. **AutoGen #7405/#7528 contributors and maintainers** — test whether a small adapter/conformance suite is preferable to framework-specific guardrail hooks.
5. **OpenTelemetry GenAI SIG** — align evidence correlation with existing conventions; do not ask OTel to carry Agenova policy payloads.
6. **LiteLLM Agent Control Plane and Agyn maintainers** — direct competitive/partner test: can Agenova add a contract they would not naturally implement themselves?
7. **OpenID AuthZEN and IETF WIMSE communities** — use their authorization/identity work and identify what remains at the assignment/runtime-evidence layer.

The best open-source growth path is not broad promotion to AI developers. It is two or three credible adapter collaborations that prove the contract works without Agenova owning the runtime.

## Next Validation: Evidence Before More Scope

Run five public, reproducible design interviews or issue discussions with maintainers/operators from at least three ecosystems. Use a concrete one-page schema and demo, not the full vision.

Ask:

1. Show the last incident or review where you could not connect a user/task to agent actions and runtime evidence.
2. Which system owns that relationship today, and why is it insufficient?
3. Would you consume an external assignment ID, resolved-authority object, and evidence envelope? Where would it enter your stack?
4. Which fields would be impossible or dangerous to supply?
5. Would conformance tests reduce integration work, or create another compatibility burden?

### Continue criteria

Within the next validation cycle, obtain all of:

- three independent practitioners confirming a recent, concrete cross-system reconstruction or authority problem;
- two maintainers willing to review an adapter profile;
- one external prototype or committed adapter experiment;
- no requirement for Agenova to own the sandbox, gateway, or IdP to be useful.

### Narrow or stop criteria

- interest remains conceptual (“good idea”) without a concrete integration point;
- every target says its selected cloud/runtime already owns the problem;
- adoption requires replacing the target's policy, gateway, runtime, and telemetry stack;
- the contract duplicates an emerging MCP, WIMSE, AuthZEN, or OTel standard without adding conformance value;
- no external adapter appears after the reference implementation is usable.

If these criteria fail, the responsible outcome is to keep Agenova as an educational reference/demo or contribute the useful contract tests and adapters upstream, rather than continue expanding it as an independent platform.

## Research Limits

- This research uses public documentation, GitHub issues/discussions, standards work, and selected public forum threads current to 2026-08-25. It does not include private deployment data or interviews.
- Vendor documentation proves capability, not customer success. GitHub and forum posts prove that a person stated a problem, not its prevalence.
- Several authorization/receipt proposals are authored by people promoting their own projects. They establish active solution-building and competitive density, not independent demand.
- The MCP Enterprise catalog is unusually valuable because contributors and organizations are named, but it is still a draft community artifact, not a finalized standard or market survey.
- Repository stars are weak adoption signals. They were used only to distinguish broad developer attention from claims of production use.

The evidence supports continuing a narrow validation effort. It does not yet support claiming product-market fit, a new standard, or a defensible standalone market.
