window.AGENOVA_STATUS = {
  updated: "2026-06-15",
  phase: "phase-2-deployable-runtime",
  step: "RuntimeBackend verified; Agent Sandbox v0.4.6 installed; SpikeAdapter e2e lifecycle verified with documented semantic gaps",
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
    adapter: { status: "spike", evidence: "internal/runtime/agentsandbox; docs/evidence/phase-2/claim-lifecycle-e2e", note: "SpikeAdapter maps Agenova RuntimeBackend calls to upstream Agent Sandbox CRDs. E2E verified on kind, with terminal phase durability gaps documented." },
    futurebackend: { status: "reserved", note: "Pluggability guarantee: any backend satisfying RuntimeBackend can replace Agent Sandbox without application-facing API changes." },
    agentsandbox: { status: "verified-spike", evidence: "docs/evidence/phase-2/upstream-agent-sandbox-install-or-blocker; docs/evidence/phase-2/claim-lifecycle-e2e", note: "Agent Sandbox v0.4.6 installed on kind; claim acquisition and warm-pool replenishment verified, explicit terminal phases remain an adapter gap." },
    substrate: { status: "partial", note: "Kubernetes kind substrate verified for controller and pod lifecycle. runtimeClass, placement, and stronger isolation are not yet tested." },
    external: { status: "reserved", note: "External credentials stay behind gateways; sandbox env must not contain upstream tokens." }
  }
};
