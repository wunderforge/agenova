window.AGENOVA_STATUS = {
  updated: "2026-06-15",
  phase: "phase-1-agent-sandbox-adapter-spike",
  step: "Phase 0 semantic baseline complete; Phase 1 Agent Sandbox adapter spike scaffolded",
  subjects: {
    template: { status: "done", evidence: "api/v1alpha1/types.go; internal/operator/runtime_test.go", note: "Local image/command template shape only; real CRD generation belongs to Phase 1." },
    warmpool: { status: "done", evidence: "internal/sandbox/pool.go; internal/operator/runtime_test.go", note: "Maintains idle sandbox count and replaces consumed sandboxes." },
    claim: { status: "done", evidence: "internal/operator/runtime_test.go", note: "Terminal phases are Succeeded, Failed, and Expired; sandbox replacement is resource evidence." },
    operator: { status: "partial", evidence: "internal/operator/runtime.go; docs/phases/phase-1-agent-sandbox-adapter-spike/backend-capability-matrix.md", note: "Local in-memory runtime remains the reference backend; Phase 1 evaluates Agent Sandbox through an adapter before native controller work." },
    toolgateway: { status: "planned", evidence: "harness/phase-0-foundation-alpha/scenarios/smoke-tool-gateway-secret-boundary", note: "Static boundary fixture only; real access mediation belongs to a later phase." },
    modelgateway: { status: "reserved", note: "Concept reserved; no Phase 0 implementation." },
    runtimeevent: { status: "reserved", note: "Future append-only runtime facts under a claim." },
    toolinvocation: { status: "reserved", note: "Future fact for one concrete tool call inside a claim." },
    modelinvocation: { status: "reserved", note: "Future fact for one concrete model call inside a claim." },
    session: { status: "reserved", note: "Future grouping object for related claims and facts." },
    memory: { status: "reserved", note: "Future read/write interfaces; platform does not decide prompt assembly." },
    checkpoint: { status: "reserved", note: "Future high-water mark over append-only state facts." },
    agenovaruntime: { status: "future", note: "Core product shape; Phase 0 has only a local runtime model." },
    cloudbyoc: { status: "future", note: "Managed control plane with customer-side runtime plane; not Phase 0 scope." },
    cloudmanaged: { status: "future", note: "Fully managed cloud runtime path; not Phase 0 scope." }
  }
};
