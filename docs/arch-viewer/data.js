window.ARCH = {
  headline: "Agenova is the governance layer for agent work.",
  value: [
    "Multi-agent applications keep owning reasoning, prompts, tools, and task semantics.",
    "Agenova owns claim graphs, authorization, gateways, audit facts, and stable runtime contracts.",
    "Sandbox lifecycle, warm pools, storage, and low-level isolation are delegated behind backend adapters."
  ],
  ownership: [
    { key: "app", label: "Application-owned", body: "Reasoning, prompt assembly, tool choice, task semantics, and memory strategy." },
    { key: "core", label: "Agenova core", body: "Stable product semantics, authorization, gateways, facts, and governance." },
    { key: "interface", label: "Agenova interface", body: "RuntimeBackend and adapters that isolate Agenova APIs from backend-specific resource shapes." },
    { key: "dependency", label: "Runtime dependency", body: "Kubernetes SIG Apps Agent Sandbox and Kubernetes substrate capabilities." },
    { key: "future", label: "Reserved future", body: "Memory, checkpoint, cloud control plane, and other later-phase surfaces." }
  ],
  regions: [
    { id: "appRegion", title: "Application Layer", subtitle: "Multi-agent apps, not Agenova", x: 28, y: 40, w: 265, h: 650, kind: "app" },
    { id: "coreRegion", title: "Agenova Core", subtitle: "Governance and facts we build", x: 326, y: 40, w: 470, h: 665, kind: "core" },
    { id: "depRegion", title: "Execution Substrate", subtitle: "Backends we depend on", x: 830, y: 40, w: 300, h: 665, kind: "dependency" }
  ],
  nodes: [
    {
      id: "application",
      name: "Multi-Agent App",
      kind: "app",
      x: 62,
      y: 116,
      w: 195,
      h: 72,
      desc: "The application may run many cooperating agents. It owns task logic, reasoning loops, prompt construction, tool choice, and application memory policy.",
      note: "Agenova does not become an agent framework or workflow DAG engine.",
      example: "agents:\n  researcher: plan_research(topic)\n  coder: implement_patch(task)\n  reviewer: review_evidence(diff)"
    },
    {
      id: "agentfleet",
      name: "Agent Workers A/B/C",
      kind: "app",
      x: 62,
      y: 230,
      w: 195,
      h: 72,
      desc: "A multi-agent app can request many concurrent worker runs. Each run becomes its own claim and can be governed independently.",
      note: "Multi-agent support is represented as multiple claims, claim relationships, and shared governance, not as Agenova owning agent reasoning.",
      example: "for agent in [researcher, coder, reviewer]:\n  agenova.run(agent, parentClaim=sessionClaim)"
    },
    {
      id: "contract",
      name: "Stable Runtime Contract",
      kind: "core",
      x: 372,
      y: 104,
      w: 380,
      h: 72,
      desc: "The application-facing contract for one agent worker run, independent of any Kubernetes or Agent Sandbox CRD shape.",
      note: "This is the API surface applications should depend on.",
      example: "runtime.createClaim({\n  pool: \"python-agent-pool\",\n  input: { task: \"research-run\" }\n})"
    },
    {
      id: "claim",
      name: "SandboxClaim",
      kind: "core",
      x: 370,
      y: 210,
      w: 170,
      h: 70,
      desc: "One agent worker run / sandbox execution lease.",
      note: "It is not one tool call. Terminal claim outcome stays separate from sandbox cleanup evidence.",
      example: "apiVersion: sandbox.agenova.io/v1alpha1\nkind: SandboxClaim\nspec:\n  poolRef: python-agent-pool\n  input:\n    task: research-run"
    },
    {
      id: "auth",
      name: "Claim Authorization",
      kind: "core",
      x: 580,
      y: 210,
      w: 170,
      h: 70,
      desc: "Access decisions are anchored to the active claim, not to an idle sandbox or network location.",
      note: "Warm idle sandboxes must not hold standing authority.",
      example: "allow(tool_call) if\n  claim.phase == \"Running\" and\n  policy.permits(claim.id, tool.name)"
    },
    {
      id: "toolgateway",
      name: "Tool Gateway",
      kind: "core",
      x: 370,
      y: 320,
      w: 170,
      h: 70,
      desc: "Mediates external tool access and keeps upstream credentials outside sandboxes.",
      note: "Records ToolInvocation facts in a later phase.",
      example: "ToolInvocation:\n  claimRef: claim-a\n  tool: github.create_issue\n  decision: allowed\n  inputDigest: sha256:..."
    },
    {
      id: "modelgateway",
      name: "Model Gateway",
      kind: "core",
      x: 580,
      y: 320,
      w: 170,
      h: 70,
      desc: "Mediates model access, audit, and cost attribution without owning prompt assembly.",
      note: "Records ModelInvocation facts in a later phase.",
      example: "ModelInvocation:\n  claimRef: claim-a\n  model: gpt-5\n  inputTokens: 1842\n  outputTokens: 611"
    },
    {
      id: "facts",
      name: "Runtime Facts",
      kind: "core",
      x: 370,
      y: 430,
      w: 170,
      h: 70,
      desc: "Append-only facts under a claim: runtime events, tool calls, model calls, and future session grouping.",
      note: "Facts are Agenova's durable audit and debugging layer.",
      example: "RuntimeEvent:\n  claimRef: claim-a\n  type: SandboxBound\n  sandboxId: pool-1"
    },
    {
      id: "governance",
      name: "Claim Graph",
      kind: "core",
      x: 580,
      y: 430,
      w: 170,
      h: 70,
      desc: "Represents parent/child and peer relationships among claims so multi-agent runs can be authorized, audited, and bounded as a group.",
      note: "This is how Agenova supports multi-agent workloads without owning agent orchestration.",
      example: "claims:\n  claim-root:\n    children: [claim-research, claim-code]\n  claim-review:\n    dependsOn: [claim-code]"
    },
    {
      id: "memory",
      name: "Memory / Checkpoints",
      kind: "future",
      x: 350,
      y: 535,
      w: 170,
      h: 70,
      desc: "Reserved interfaces for state continuity, checkpointing, and rollback semantics.",
      note: "Future phase; not part of the current implementation.",
      example: "checkpoint:\n  session: session-42\n  headFact: fact-109\n  rollbackTo: fact-088"
    },
    {
      id: "futurebackend",
      name: "Future Backend",
      kind: "interface",
      x: 610,
      y: 540,
      w: 140,
      h: 64,
      desc: "A future backend can replace Agent Sandbox when it satisfies the same RuntimeBackend contract.",
      note: "This is the pluggability guarantee: customers or Agenova can choose another sandbox substrate without application-facing API changes.",
      example: "RuntimeBackend\n  +-- InMemoryBackend\n  +-- AgentSandboxAdapter\n  +-- FutureBackendAdapter"
    },
    {
      id: "backend",
      name: "RuntimeBackend",
      kind: "interface",
      x: 390,
      y: 620,
      w: 150,
      h: 64,
      desc: "RuntimeBackend is the small internal boundary for template, warm pool, claim lifecycle, status, and cleanup evidence.",
      note: "The in-memory backend is the reference contract target.",
      example: "type RuntimeBackend interface {\n  AddClaim(claim)\n  BindClaim(name)\n  StartClaim(name)\n  Claim(name)\n}"
    },
    {
      id: "adapter",
      name: "AS Adapter",
      kind: "interface",
      x: 600,
      y: 620,
      w: 150,
      h: 64,
      desc: "AgentSandboxAdapter maps Agenova runtime semantics to Kubernetes SIG Apps Agent Sandbox.",
      note: "Protects applications and Agenova governance from upstream API churn.",
      example: "Agenova SandboxClaim\n  -> Agent Sandbox acquisition\n  -> status translation\n  -> Agenova RuntimeEvent"
    },
    {
      id: "agentsandbox",
      name: "K8s Agent Sandbox",
      kind: "dependency",
      x: 875,
      y: 160,
      w: 210,
      h: 80,
      desc: "Kubernetes SIG Apps Agent Sandbox is the first execution substrate to evaluate for sandbox lifecycle, warm pools, claims, and stateful singleton workloads.",
      note: "Capabilities must be verified before they are treated as supported.",
      example: "verify:\n  acquisition: one claim == one lease\n  warmPool: replacement evidence\n  status: maps to Agenova phases"
    },
    {
      id: "substrate",
      name: "Kubernetes Substrate",
      kind: "dependency",
      x: 875,
      y: 320,
      w: 210,
      h: 80,
      desc: "Cluster and runtime primitives used for scheduling, placement, storage, network policy, and stronger isolation options.",
      note: "Ordinary Pod isolation is not enough for mutually hostile agents.",
      example: "placement:\n  runtimeClassName: kata\n  nodeSelector:\n    pool: isolated-agents"
    },
    {
      id: "external",
      name: "External Systems",
      kind: "dependency",
      x: 875,
      y: 480,
      w: 210,
      h: 80,
      desc: "Tool APIs, model providers, databases, and enterprise systems reached through Agenova gateways.",
      note: "Their credentials stay behind gateways, not inside sandboxes.",
      example: "sandbox env:\n  AGENOVA_CLAIM_TOKEN: scoped\n  GITHUB_TOKEN: absent"
    }
  ],
  edges: [
    ["application", "agentfleet", "spawns agents"],
    ["agentfleet", "contract", "requests worker runs"],
    ["contract", "claim", "creates lease"],
    ["claim", "auth", "anchors policy"],
    ["claim", "toolgateway", "authorizes tool use"],
    ["claim", "modelgateway", "authorizes model use"],
    ["toolgateway", "facts", "records"],
    ["modelgateway", "facts", "records"],
    ["claim", "facts", "emits runtime events"],
    ["claim", "governance", "may govern child claims"],
    ["agentfleet", "governance", "multi-agent claim graph"],
    ["facts", "memory", "feeds later state"],
    ["claim", "backend", "lifecycle contract"],
    ["backend", "adapter", "selects backend"],
    ["backend", "futurebackend", "optional backend"],
    ["adapter", "agentsandbox", "translates to"],
    ["agentsandbox", "substrate", "runs on"],
    ["toolgateway", "external", "mediates"],
    ["modelgateway", "external", "mediates"]
  ],
  runPath: [
    { title: "1. Multi-agent app asks for work", body: "An application may launch many agents. Each worker run enters through the stable Agenova runtime contract.", subjects: ["application", "agentfleet", "contract"] },
    { title: "2. Agenova creates claim graph", body: "Each worker run gets a SandboxClaim; related agents can be linked through parent/child or peer claim relationships.", subjects: ["claim", "governance", "auth", "facts"] },
    { title: "3. Backend provides execution", body: "RuntimeBackend maps the claim to the selected backend: Agent Sandbox now, or another adapter later.", subjects: ["backend", "adapter", "futurebackend", "agentsandbox"] },
    { title: "4. Gateways mediate access", body: "Tool and model calls go through Agenova-owned gateways; external credentials remain outside sandboxes.", subjects: ["toolgateway", "modelgateway", "external"] },
    { title: "5. Outcome stays clean", body: "Claim outcome remains Succeeded, Failed, or Expired; sandbox replacement is recorded as resource evidence.", subjects: ["claim", "facts", "agentsandbox"] }
  ],
  ownershipRows: [
    { area: "Application reasoning and multi-agent orchestration", owner: "Application", why: "Agenova governs runs; it does not choose prompts, tools, or agent plans." },
    { area: "Stable claim contract and claim graph", owner: "Agenova core", why: "Preserves one-worker-run semantics and multi-agent governance across backends." },
    { area: "Tool and model access boundaries", owner: "Agenova core", why: "Credentials, policy, audit, and cost attribution live here." },
    { area: "Runtime facts", owner: "Agenova core", why: "Claim-anchored facts are the audit/debugging layer." },
    { area: "RuntimeBackend and adapters", owner: "Agenova interface", why: "Backends can change without changing application APIs." },
    { area: "Future backend adapters", owner: "Agenova interface", why: "If Agent Sandbox cannot carry required semantics, another backend can implement the same contract." },
    { area: "Sandbox lifecycle and warm pools", owner: "Runtime dependency", why: "Prefer Agent Sandbox if it satisfies the contract." },
    { area: "runtimeClass, node pools, stronger isolation", owner: "Runtime dependency", why: "Provided by Kubernetes/runtime substrate and verified by adapter spike." },
    { area: "Memory, checkpoints, rollback", owner: "Reserved future", why: "Agenova-owned interfaces later; not current scope." }
  ],
  phases: [
    { name: "Now", status: "Phase 1 target", scope: "RuntimeBackend boundary, reference in-memory backend, and Agent Sandbox adapter spike." },
    { name: "Next verification", status: "Required", scope: "Can Agent Sandbox acquisition semantics represent one Agenova SandboxClaim lease?" },
    { name: "Later", status: "Planned", scope: "Tool Gateway, Model Gateway, facts, parent/child governance, memory, checkpoints, and cloud shapes." }
  ]
};
