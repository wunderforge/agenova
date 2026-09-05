// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package e2e contains the backend-neutral multi-agent governance reference scenario.
// It runs against the in-memory backend only and does not require a cluster.
package e2e

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
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
	toolGW := toolgateway.NewGateway(r, lineage, store)
	modelGW := modelgateway.NewGateway(r, lineage, store)

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
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "orchestrator", ToolName: "plan"}); err != nil {
		t.Fatalf("orchestrator tool auth: %v", err)
	}
	if err := modelGW.Authorize(modelgateway.Request{ClaimID: "orchestrator", ModelName: "claude-opus-4-8"}); err != nil {
		t.Fatalf("orchestrator model auth: %v", err)
	}

	// Worker-a does research.
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "worker-a", ToolName: "web-search"}); err != nil {
		t.Fatalf("worker-a tool auth: %v", err)
	}
	if err := modelGW.Authorize(modelgateway.Request{ClaimID: "worker-a", ModelName: "claude-sonnet-4-6"}); err != nil {
		t.Fatalf("worker-a model auth: %v", err)
	}

	// Worker-b executes code.
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "worker-b", ToolName: "code-exec"}); err != nil {
		t.Fatalf("worker-b tool auth: %v", err)
	}
	if err := modelGW.Authorize(modelgateway.Request{ClaimID: "worker-b", ModelName: "claude-haiku-4-5"}); err != nil {
		t.Fatalf("worker-b model auth: %v", err)
	}

	// --- Step 3: verify facts are isolated per claim ---

	orchTools := store.ToolInvocations("orchestrator")
	if len(orchTools) != 1 || orchTools[0].ToolName != "plan" {
		t.Errorf("orchestrator tools: want [{plan}], got %v", orchTools)
	}

	aTools := store.ToolInvocations("worker-a")
	if len(aTools) != 1 || aTools[0].ToolName != "web-search" {
		t.Errorf("worker-a tools: want [{web-search}], got %v", aTools)
	}

	bTools := store.ToolInvocations("worker-b")
	if len(bTools) != 1 || bTools[0].ToolName != "code-exec" {
		t.Errorf("worker-b tools: want [{code-exec}], got %v", bTools)
	}

	orchModels := store.ModelInvocations("orchestrator")
	if len(orchModels) != 1 || orchModels[0].ModelName != "claude-opus-4-8" {
		t.Errorf("orchestrator models: want [{claude-opus-4-8}], got %v", orchModels)
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
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "worker-a", ToolName: "web-search"}); err == nil {
		t.Fatal("worker-a should be denied after orchestrator terminates")
	}
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "worker-b", ToolName: "code-exec"}); err == nil {
		t.Fatal("worker-b should be denied after orchestrator terminates")
	}

	// Model gateway must also deny them.
	if err := modelGW.Authorize(modelgateway.Request{ClaimID: "worker-a", ModelName: "claude-sonnet-4-6"}); err == nil {
		t.Fatal("worker-a model call should be denied after orchestrator terminates")
	}

	// Fact counts must not grow after denial.
	if len(store.ToolInvocations("worker-a")) != 1 {
		t.Errorf("worker-a tool facts should remain at 1 after denial, got %d", len(store.ToolInvocations("worker-a")))
	}

	t.Log("scope enforcement: workers denied after orchestrator terminates")
}

// TestMultiAgentReference_WorkerWithoutParentIsIndependent verifies that a
// claim with no registered parent is governed by its own Running state only.
func TestMultiAgentReference_WorkerWithoutParentIsIndependent(t *testing.T) {
	r := newSharedRuntime(t)
	store := facts.NewStore()
	lineage := governance.NewLineage()
	toolGW := toolgateway.NewGateway(r, lineage, store)

	addRunningClaim(t, r, "standalone")

	// No parent registered — allowed while Running.
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "standalone", ToolName: "file-read"}); err != nil {
		t.Fatalf("standalone claim should be authorized: %v", err)
	}

	if err := r.SucceedClaim("standalone"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	// After termination, denied.
	if err := toolGW.Authorize(toolgateway.Request{ClaimID: "standalone", ToolName: "file-read"}); err == nil {
		t.Fatal("terminal standalone claim should be denied")
	}
}
