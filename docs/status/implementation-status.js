window.AGENOVA_STATUS = {
  updated: "2026-06-15",
  phase: "phase-3-governance-runtime",
  step: "Tool Gateway MVP, Model Gateway MVP, claim lineage, facts store, and multi-agent reference scenario implemented on in-memory backend. All Phase 3 evidence gates pass.",
  subjects: {
    contract: { status: "done", evidence: "internal/runtime/backend.go; internal/operator/runtime_test.go", note: "RuntimeBackend interface defines the stable application-facing contract. Swapping backends does not change this API." },
    claim: { status: "done", evidence: "internal/operator/runtime_test.go", note: "Terminal phases are Succeeded, Failed, and Expired; sandbox replacement is resource evidence, not a claim phase." },
    auth: { status: "done", evidence: "docs/evidence/phase-3/authorization-negative", note: "Claim-anchored access control: unbound, terminal, unknown, and child-out-of-scope claims are denied by both gateways." },
    toolgateway: { status: "done", evidence: "docs/evidence/phase-3/tool-gateway; internal/toolgateway/gateway.go", note: "Tool Gateway MVP authorizes by active Running claim, enforces parent scope for child claims, and records ToolInvocation facts." },
    modelgateway: { status: "done", evidence: "docs/evidence/phase-3/model-gateway; internal/modelgateway/gateway.go", note: "Model Gateway MVP authorizes by active Running claim, enforces parent scope for child claims, and records ModelInvocation facts." },
    facts: { status: "done", evidence: "docs/evidence/phase-3/facts-query; internal/facts/store.go", note: "In-memory append-only store for ToolInvocation, ModelInvocation, and RuntimeEvent facts. Queried by claim ID. ToolInvocation and ModelInvocation are api/v1alpha1 types." },
    governance: { status: "done", evidence: "docs/evidence/phase-3/claim-lineage; internal/governance/lineage.go", note: "Claim lineage tracks parent/child relationships for multi-agent governance. Not a DAG engine. Gateways use lineage to enforce child-out-of-scope denial." },
    multiagent: { status: "done-reference-blocker-k8s", evidence: "docs/evidence/phase-3/multi-agent-reference; docs/evidence/phase-3/multi-agent-kubernetes-or-blocker", note: "Reference scenario: orchestrator + two worker claims, lineage registered, facts isolated per claim, workers denied after orchestrator terminates. Kubernetes-backed interception is a documented follow-up blocker." },
    memory: { status: "reserved", note: "Reserved interfaces for state continuity, checkpointing, and rollback. Future phase." },
    backend: { status: "done", evidence: "internal/runtime/backend.go; internal/operator/runtime_test.go", note: "RuntimeBackend interface extracted. InMemoryBackend (internal/operator/runtime.go) is the reference implementation and passes the contract test suite." },
    adapter: { status: "spike", evidence: "internal/runtime/agentsandbox; docs/evidence/phase-2/claim-lifecycle-e2e", note: "SpikeAdapter maps Agenova RuntimeBackend calls to upstream Agent Sandbox CRDs. E2E verified on kind, with terminal phase durability gaps documented." },
    futurebackend: { status: "reserved", note: "Pluggability guarantee: any backend satisfying RuntimeBackend can replace Agent Sandbox without application-facing API changes." },
    agentsandbox: { status: "verified-spike", evidence: "docs/evidence/phase-2/upstream-agent-sandbox-install-or-blocker; docs/evidence/phase-2/claim-lifecycle-e2e", note: "Agent Sandbox v0.4.6 installed on kind; claim acquisition and warm-pool replenishment verified, explicit terminal phases remain an adapter gap." },
    substrate: { status: "partial", note: "Kubernetes kind substrate verified for controller and pod lifecycle. runtimeClass, placement, and stronger isolation are not yet tested." },
    external: { status: "reserved", note: "External credentials stay behind gateways; sandbox env must not contain upstream tokens." }
  }
};
