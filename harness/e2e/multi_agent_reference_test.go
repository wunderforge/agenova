// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package e2e contains the backend-neutral multi-agent governance reference scenario.
// It runs against the in-memory backend only and does not require a cluster.
package e2e

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/governance"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/toolgateway"
)

// newSharedRuntime creates the shared in-memory runtime for reference tests.
// It pre-registers one template and one pool with enough replicas for all sub-claims.
func newSharedRuntime(t *testing.T) *operator.Runtime {
	t.Helper()
	r := operator.NewRuntime()
	if err := r.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-v1"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agent:dev", Command: []string{"agent"}},
	}); err != nil {
		t.Fatalf("add template: %v", err)
	}
	if err := r.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-pool"},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: "agent-v1", Replicas: 5},
	}); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	return r
}

func addRunningClaim(t *testing.T, r *operator.Runtime, name string) {
	t.Helper()
	if err := r.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: name},
		Spec:     runtime.BackendClaimSpec{PoolRef: "agent-pool"},
	}); err != nil {
		t.Fatalf("add claim %q: %v", name, err)
	}
	if err := r.BindClaim(name); err != nil {
		t.Fatalf("bind claim %q: %v", name, err)
	}
	if err := r.StartClaim(name); err != nil {
		t.Fatalf("start claim %q: %v", name, err)
	}
}

// referenceToolAdapter and referenceModelAdapter stand in for provider
// adapters so the scenario's evidence describes calls that were actually
// attempted. A gateway with no adapter wired fails closed by design.
type referenceToolAdapter struct{ attempts int }

func (a *referenceToolAdapter) Invoke(string, toolgateway.Request) error {
	a.attempts++
	return nil
}

type referenceModelAdapter struct{ attempts int }

func (a *referenceModelAdapter) Invoke(string, modelgateway.Request) error {
	a.attempts++
	return nil
}

func toolRequest(claimID, tool, action string) toolgateway.Request {
	return toolgateway.Request{
		ClaimID:       claimID,
		Tool:          tool,
		Action:        action,
		ResourceScope: "repo:acme/payments",
	}
}

func modelRequest(claimID, profile string) modelgateway.Request {
	return modelgateway.Request{ClaimID: claimID, Profile: profile}
}

func mustAllowTool(t *testing.T, gw *toolgateway.Gateway, req toolgateway.Request) {
	t.Helper()
	decision, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("tool invoke %s: %v", req.ClaimID, err)
	}
	if decision.Result != gateway.ResultAllow {
		t.Fatalf("tool invoke %s: Result = %q (%s), want Allow", req.ClaimID, decision.Result, decision.Reason)
	}
}

func mustAllowModel(t *testing.T, gw *modelgateway.Gateway, req modelgateway.Request) {
	t.Helper()
	decision, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("model invoke %s: %v", req.ClaimID, err)
	}
	if decision.Result != gateway.ResultAllow {
		t.Fatalf("model invoke %s: Result = %q (%s), want Allow", req.ClaimID, decision.Result, decision.Reason)
	}
}

func mustDenyTool(t *testing.T, gw *toolgateway.Gateway, req toolgateway.Request, why string) {
	t.Helper()
	decision, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("tool invoke %s: %v", req.ClaimID, err)
	}
	if decision.Result != gateway.ResultDeny {
		t.Fatalf("%s: Result = %q, want Deny", why, decision.Result)
	}
}

func mustDenyModel(t *testing.T, gw *modelgateway.Gateway, req modelgateway.Request, why string) {
	t.Helper()
	decision, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("model invoke %s: %v", req.ClaimID, err)
	}
	if decision.Result != gateway.ResultDeny {
		t.Fatalf("%s: Result = %q, want Deny", why, decision.Result)
	}
}

// TestMultiAgentReference exercises the multi-agent governance reference scenario:
//
//  1. An orchestrator (parent) claim runs and delegates to two worker (child) claims.
//  2. All three invoke tools and models through the respective gateways.
//  3. Facts are recorded under the correct claim IDs (not cross-attributed).
//  4. Lineage reflects parent/child relationships.
//  5. After the parent terminates, the child claims are denied further tool access
//     (child-out-of-scope), proving governance scope enforcement.
func TestMultiAgentReference(t *testing.T) {
	r := newSharedRuntime(t)
	store := facts.NewStore()
	lineage := governance.NewLineage()
	toolAdapter := &referenceToolAdapter{}
	modelAdapter := &referenceModelAdapter{}
	toolGW := toolgateway.NewGateway(r, lineage, store, toolgateway.WithAdapter(toolAdapter))
	modelGW := modelgateway.NewGateway(r, lineage, store, modelgateway.WithAdapter(modelAdapter))

	// --- Step 1: start all three claims ---
	addRunningClaim(t, r, "orchestrator")
	addRunningClaim(t, r, "worker-a")
	addRunningClaim(t, r, "worker-b")

	// Register child lineage.
	if err := lineage.RegisterChild("orchestrator", "worker-a"); err != nil {
		t.Fatalf("register worker-a: %v", err)
	}
	if err := lineage.RegisterChild("orchestrator", "worker-b"); err != nil {
		t.Fatalf("register worker-b: %v", err)
	}

	t.Log("lineage: orchestrator -> [worker-a, worker-b]")

	// --- Step 2: each claim invokes through the gateways ---

	// Orchestrator coordinates via tool.
	mustAllowTool(t, toolGW, toolRequest("orchestrator", "plan", "compose"))
	mustAllowModel(t, modelGW, modelRequest("orchestrator", "approved-planning-model"))

	// Worker-a does research.
	mustAllowTool(t, toolGW, toolRequest("worker-a", "web", "search"))
	mustAllowModel(t, modelGW, modelRequest("worker-a", "approved-research-model"))

	// Worker-b executes code.
	mustAllowTool(t, toolGW, toolRequest("worker-b", "code", "exec"))
	mustAllowModel(t, modelGW, modelRequest("worker-b", "approved-coding-model"))

	// --- Step 3: verify facts are isolated per claim ---

	orchTools := store.ToolInvocations("orchestrator")
	if len(orchTools) != 1 || orchTools[0].ToolName != "plan.compose" {
		t.Errorf("orchestrator tools: want [{plan.compose}], got %v", orchTools)
	}
	if len(orchTools) == 1 && orchTools[0].InvocationID == "" {
		t.Error("orchestrator tool fact must carry the gateway-assigned invocation id")
	}

	aTools := store.ToolInvocations("worker-a")
	if len(aTools) != 1 || aTools[0].ToolName != "web.search" {
		t.Errorf("worker-a tools: want [{web.search}], got %v", aTools)
	}

	bTools := store.ToolInvocations("worker-b")
	if len(bTools) != 1 || bTools[0].ToolName != "code.exec" {
		t.Errorf("worker-b tools: want [{code.exec}], got %v", bTools)
	}

	orchModels := store.ModelInvocations("orchestrator")
	if len(orchModels) != 1 || orchModels[0].ModelName != "approved-planning-model" {
		t.Errorf("orchestrator models: want [{approved-planning-model}], got %v", orchModels)
	}

	if toolAdapter.attempts != 3 || modelAdapter.attempts != 3 {
		t.Errorf("adapter attempts = tool %d / model %d, want 3 each: recorded evidence must match attempted calls", toolAdapter.attempts, modelAdapter.attempts)
	}

	t.Log("facts: tool and model invocations recorded under correct claim IDs")

	// --- Step 4: verify lineage ---

	if !lineage.IsChildOf("orchestrator", "worker-a") {
		t.Error("worker-a should be a child of orchestrator")
	}
	if !lineage.IsChildOf("orchestrator", "worker-b") {
		t.Error("worker-b should be a child of orchestrator")
	}

	children := lineage.Children("orchestrator")
	if len(children) != 2 {
		t.Errorf("orchestrator should have 2 children, got %d", len(children))
	}

	if p, ok := lineage.Parent("worker-a"); !ok || p != "orchestrator" {
		t.Errorf("worker-a parent = %q, want orchestrator", p)
	}

	t.Log("lineage: parent/child relationships verified")

	// --- Step 5: terminate orchestrator; workers become out-of-scope ---

	if err := r.SucceedClaim("orchestrator"); err != nil {
		t.Fatalf("succeed orchestrator: %v", err)
	}

	// worker-a and worker-b are still Running, but their parent is Succeeded.
	// Tool gateway must deny them (child-out-of-scope).
	mustDenyTool(t, toolGW, toolRequest("worker-a", "web", "search"), "worker-a after orchestrator terminates")
	mustDenyTool(t, toolGW, toolRequest("worker-b", "code", "exec"), "worker-b after orchestrator terminates")

	// Model gateway must also deny them.
	mustDenyModel(t, modelGW, modelRequest("worker-a", "approved-research-model"), "worker-a model call after orchestrator terminates")

	// Denied attempts append inspectable non-Allow facts; no new Allow appears.
	workerAFacts := store.ToolInvocations("worker-a")
	if len(workerAFacts) != 2 {
		t.Fatalf("worker-a should have 1 Allow + 1 Deny fact after denial, got %d", len(workerAFacts))
	}
	if workerAFacts[0].Result != gateway.ResultAllow || workerAFacts[1].Result != gateway.ResultDeny {
		t.Errorf("worker-a fact results = [%s, %s], want [Allow, Deny]", workerAFacts[0].Result, workerAFacts[1].Result)
	}

	t.Log("scope enforcement: workers denied after orchestrator terminates")
}

// TestMultiAgentReference_WorkerWithoutParentIsIndependent verifies that a
// claim with no registered parent is governed by its own Running state only.
func TestMultiAgentReference_WorkerWithoutParentIsIndependent(t *testing.T) {
	r := newSharedRuntime(t)
	store := facts.NewStore()
	lineage := governance.NewLineage()
	toolGW := toolgateway.NewGateway(r, lineage, store, toolgateway.WithAdapter(&referenceToolAdapter{}))

	addRunningClaim(t, r, "standalone")

	// No parent registered — allowed while Running.
	mustAllowTool(t, toolGW, toolRequest("standalone", "file", "read"))

	if err := r.SucceedClaim("standalone"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	// After termination, denied.
	mustDenyTool(t, toolGW, toolRequest("standalone", "file", "read"), "terminal standalone claim")
}
