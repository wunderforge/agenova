window.ARCH = {
  domains: [
    { id: "runtime", name: "Runtime and Sandbox Management", tier: "Architecture commitment", x: 20, y: 60, w: 300, h: 520 },
    { id: "access", name: "Tool and Model Access Boundaries", tier: "Architecture commitment", x: 350, y: 60, w: 300, h: 520 },
    { id: "facts", name: "Facts and Sessions", tier: "Architecture commitment", x: 680, y: 60, w: 440, h: 250 },
    { id: "reserved", name: "Reserved Interfaces", tier: "Interface reservation", x: 680, y: 330, w: 440, h: 250 },
    { id: "deployment", name: "Future Deployment Shapes", tier: "Future architecture constraint", x: 20, y: 610, w: 1100, h: 130 }
  ],
  nodes: [
    { id: "template", name: "AgentSandboxTemplate", domain: "runtime", x: 55, y: 115, w: 220, h: 54, desc: "Worker image and command shape for the local Phase 0 model.", example: "image: example.local/agenova/python-agent:dev\ncommand: [\"python\", \"/app/agent.py\"]" },
    { id: "warmpool", name: "SandboxWarmPool", domain: "runtime", x: 55, y: 210, w: 220, h: 54, desc: "Maintains idle sandboxes for a template.", example: "templateRef: python-agent-v1\nreplicas: 2" },
    { id: "claim", name: "SandboxClaim", domain: "runtime", x: 55, y: 305, w: 220, h: 54, desc: "One agent worker run / sandbox execution lease. Not one tool call.", example: "poolRef: python-agent-pool\ninput:\n  task: research-run" },
    { id: "operator", name: "Runtime / Operator", domain: "runtime", x: 55, y: 400, w: 220, h: 54, desc: "Local in-memory lifecycle model in Phase 0; Kubernetes runtime reconciliation later.", example: "Pending -> Bound -> Running -> Succeeded\nPending -> Bound -> Running -> Failed\nPending -> Bound -> Failed\nPending -> Expired" },
    { id: "toolgateway", name: "Tool Gateway", domain: "access", x: 390, y: 150, w: 220, h: 54, desc: "Future boundary for tool access and upstream credentials.", example: "External credentials stay behind the gateway." },
    { id: "modelgateway", name: "Model Gateway", domain: "access", x: 390, y: 290, w: 220, h: 54, desc: "Future boundary for model access, audit, and cost attribution.", example: "Records ModelInvocation facts later." },
    { id: "runtimeevent", name: "RuntimeEvent", domain: "facts", x: 720, y: 115, w: 180, h: 54, desc: "Future append-only runtime fact under a claim.", example: "claim bound, worker started, sandbox replaced" },
    { id: "toolinvocation", name: "ToolInvocation", domain: "facts", x: 910, y: 115, w: 180, h: 54, desc: "Future fact for one concrete tool call inside a claim.", example: "tool name, input digest, output digest, policy decision" },
    { id: "modelinvocation", name: "ModelInvocation", domain: "facts", x: 910, y: 205, w: 180, h: 54, desc: "Future fact for one concrete model call inside a claim.", example: "model, token counts, cost, latency" },
    { id: "session", name: "Session", domain: "reserved", x: 720, y: 385, w: 180, h: 54, desc: "Future concrete resource for the committed session-level fact grouping concept.", example: "session id and ordered fact references" },
    { id: "memory", name: "MemoryReader/Writer", domain: "reserved", x: 910, y: 385, w: 180, h: 54, desc: "Future memory interfaces. The platform provides interfaces and metadata, not prompt assembly.", example: "query records; append records" },
    { id: "checkpoint", name: "Checkpoint", domain: "reserved", x: 815, y: 485, w: 180, h: 54, desc: "Future high-water mark over append-only state facts.", example: "move head pointer for rollback" },
    { id: "agenovaruntime", name: "Agenova Runtime", domain: "deployment", x: 55, y: 665, w: 220, h: 54, desc: "Core deployable runtime for customer-managed infrastructure.", example: "Self-managed runtime plane." },
    { id: "cloudbyoc", name: "Agenova Cloud BYOC", domain: "deployment", x: 390, y: 665, w: 220, h: 54, desc: "Future managed control plane with runtime plane in the customer's cloud account, VPC, or cluster.", example: "Control Plane != Runtime Plane cluster." },
    { id: "cloudmanaged", name: "Cloud Fully Managed", domain: "deployment", x: 720, y: 665, w: 220, h: 54, desc: "Future fully managed control plane and runtime plane behind standard Agenova APIs.", example: "Users consume APIs, not clusters." }
  ],
  edges: [
    ["template", "warmpool", "creates warm sandboxes"],
    ["warmpool", "claim", "binds lease"],
    ["claim", "operator", "lifecycle"],
    ["claim", "toolgateway", "uses tools through"],
    ["claim", "modelgateway", "uses models through"],
    ["toolgateway", "toolinvocation", "records"],
    ["modelgateway", "modelinvocation", "records"],
    ["operator", "runtimeevent", "records"],
    ["runtimeevent", "session", "groups later"],
    ["session", "memory", "queries later"],
    ["session", "checkpoint", "versions later"],
    ["agenovaruntime", "cloudbyoc", "evolves to"],
    ["cloudbyoc", "cloudmanaged", "also supports"]
  ],
  flows: [
    { title: "Prepare pool", body: "A template and warm pool define available worker capacity.", subjects: ["template", "warmpool"] },
    { title: "Create claim", body: "A claim requests one sandbox lease for an agent worker run.", subjects: ["claim"] },
    { title: "Run worker", body: "The runtime moves the claim through bound and running states.", subjects: ["operator", "claim"] },
    { title: "Reach terminal state", body: "The claim remains Succeeded, Failed, or Expired.", subjects: ["claim"] },
    { title: "Replace sandbox", body: "Sandbox replacement is recorded as resource evidence, not claim phase.", subjects: ["warmpool", "runtimeevent"] }
  ],
  phases: [
    { name: "Phase 0", status: "done", scope: "Local foundation alpha and harness discipline." },
    { name: "Phase 1", status: "next", scope: "RuntimeBackend boundary and Kubernetes SIG Apps Agent Sandbox adapter spike." },
    { name: "Phase 2", status: "planned", scope: "Tool Gateway behavior and ToolInvocation facts." },
    { name: "Phase 3", status: "planned", scope: "Model Gateway behavior and ModelInvocation facts." },
    { name: "Future", status: "reserved", scope: "Agenova Cloud BYOC and Fully Managed deployment shapes without assuming one shared cluster." }
  ]
};
