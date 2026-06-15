window.AGENOVA_STATUS = {
  updated: "2026-06-15",
  phase: "phase-1-agent-sandbox-adapter-spike",
  step: "RuntimeBackend interface defined; in-memory backend verified; Phase 1 Agent Sandbox adapter spike next",
  subjects: {
    contract: { status: "done", evidence: "internal/runtime/backend.go; internal/operator/runtime_test.go", note: "RuntimeBackend interface defines the stable application-facing contract. Swapping backends does not change this API." },
    claim: { status: "done", evidence: "internal/operator/runtime_test.go", note: "Terminal phases are Succeeded, Failed, and Expired; sandbox replacement is resource evidence, not a claim phase." },
    auth: { status: "reserved", note: "Future: access decisions anchored to active claim, not idle sandbox." },
    toolgateway: { status: "planned", evidence: "harness/phase-0-foundation-alpha/scenarios/smoke-tool-gateway-secret-boundary", note: "Static boundary fixture only; real access mediation belongs to a later phase." },
    modelgateway: { status: "reserved", note: "Concept reserved; no current implementation." },
    facts: { status: "reserved", note: "Future append-only RuntimeEvent, ToolInvocation, and ModelInvocation facts under a claim." },
    governance: { status: "reserved", note: "Future claim graph for parent/child and peer relationships in multi-agent runs." },
    memory: { status: "reserved", note: "Reserved interfaces for state continuity, checkpointing, and rollback. Future phase." },
    backend: { status: "done", evidence: "internal/runtime/backend.go; internal/operator/runtime_test.go", note: "RuntimeBackend interface extracted. InMemoryBackend (internal/operator/runtime.go) is the reference implementation and passes the contract test suite." },
    adapter: { status: "next", evidence: "docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md", note: "AgentSandboxAdapter is the Phase 1 spike target. Not yet implemented; capability matrix must be verified first." },
    futurebackend: { status: "reserved", note: "Pluggability guarantee: any backend satisfying RuntimeBackend can replace Agent Sandbox without application-facing API changes." },
    agentsandbox: { status: "next", evidence: "docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md", note: "First execution substrate to evaluate. Capabilities must be verified before treating them as supported." },
    substrate: { status: "reserved", note: "Kubernetes cluster primitives used for placement, runtimeClass, and stronger isolation. Verified by adapter spike." },
    external: { status: "reserved", note: "External credentials stay behind gateways; sandbox env must not contain upstream tokens." }
  }
};
