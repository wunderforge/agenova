// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolgateway

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/governance"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

// newFixture returns a gateway backed by the in-memory reference runtime.
// The backend has one template and pool pre-configured.
func newFixture(t *testing.T) (*Gateway, *operator.Runtime, *facts.Store, *governance.Lineage) {
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
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: "agent-v1", Replicas: 3},
	}); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}

	store := facts.NewStore()
	lineage := governance.NewLineage()
	gw := NewGateway(r, lineage, store)
	return gw, r, store, lineage
}

// runClaim advances a claim through Pending -> Bound -> Running.
func runClaim(t *testing.T, r *operator.Runtime, name string) {
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

func TestGateway_AllowsRunningClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)
	runClaim(t, r, "run-1")

	if err := gw.Authorize(Request{ClaimID: "run-1", ToolName: "web-search"}); err != nil {
		t.Fatalf("running claim should be authorized: %v", err)
	}

	invocations := store.ToolInvocations("run-1")
	if len(invocations) != 1 {
		t.Fatalf("expected 1 recorded fact, got %d", len(invocations))
	}
	if invocations[0].ToolName != "web-search" {
		t.Errorf("ToolName = %q, want web-search", invocations[0].ToolName)
	}
}

func TestGateway_RecordsMultipleInvocations(t *testing.T) {
	gw, r, store, _ := newFixture(t)
	runClaim(t, r, "run-1")

	tools := []string{"web-search", "code-exec", "file-read"}
	for _, tool := range tools {
		if err := gw.Authorize(Request{ClaimID: "run-1", ToolName: tool}); err != nil {
			t.Fatalf("authorize %q: %v", tool, err)
		}
	}

	if len(store.ToolInvocations("run-1")) != len(tools) {
		t.Errorf("expected %d recorded facts, got %d", len(tools), len(store.ToolInvocations("run-1")))
	}
}

// --- negative authorization tests ---

func TestGateway_DeniesUnknownClaim(t *testing.T) {
	gw, _, store, _ := newFixture(t)

	if err := gw.Authorize(Request{ClaimID: "nonexistent", ToolName: "web-search"}); err == nil {
		t.Fatal("unknown claim should be denied")
	}
	if len(store.ToolInvocations("nonexistent")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesPendingClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)

	if err := r.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "pending-claim"},
		Spec:     runtime.BackendClaimSpec{PoolRef: "agent-pool"},
	}); err != nil {
		t.Fatalf("add claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "pending-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("pending (unbound) claim should be denied")
	}
	if len(store.ToolInvocations("pending-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesBoundClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)

	if err := r.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "bound-claim"},
		Spec:     runtime.BackendClaimSpec{PoolRef: "agent-pool"},
	}); err != nil {
		t.Fatalf("add claim: %v", err)
	}
	if err := r.BindClaim("bound-claim"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "bound-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("bound (not yet running) claim should be denied")
	}
	if len(store.ToolInvocations("bound-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesSucceededClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)
	runClaim(t, r, "term-claim")

	if err := r.SucceedClaim("term-claim"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "term-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("succeeded (terminal) claim should be denied")
	}
	if len(store.ToolInvocations("term-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesFailedClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)
	runClaim(t, r, "fail-claim")

	if err := r.FailClaim("fail-claim", "agent error"); err != nil {
		t.Fatalf("fail claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "fail-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("failed (terminal) claim should be denied")
	}
	if len(store.ToolInvocations("fail-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesExpiredClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)

	if err := r.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "expired-claim"},
		Spec:     runtime.BackendClaimSpec{PoolRef: "agent-pool"},
	}); err != nil {
		t.Fatalf("add claim: %v", err)
	}
	if err := r.ExpireClaim("expired-claim", "ttl elapsed"); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "expired-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("expired (terminal) claim should be denied")
	}
	if len(store.ToolInvocations("expired-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesChildWithTerminalParent(t *testing.T) {
	gw, r, store, lineage := newFixture(t)
	runClaim(t, r, "parent-claim")
	runClaim(t, r, "child-claim")

	if err := lineage.RegisterChild("parent-claim", "child-claim"); err != nil {
		t.Fatalf("register child: %v", err)
	}

	// Child is Running - should be allowed while parent is Running.
	if err := gw.Authorize(Request{ClaimID: "child-claim", ToolName: "web-search"}); err != nil {
		t.Fatalf("child with running parent should be authorized: %v", err)
	}

	// Terminate the parent.
	if err := r.SucceedClaim("parent-claim"); err != nil {
		t.Fatalf("succeed parent: %v", err)
	}

	// Child is still Running but parent is Succeeded (terminal) -> denied (out-of-scope).
	if err := gw.Authorize(Request{ClaimID: "child-claim", ToolName: "web-search"}); err == nil {
		t.Fatal("child claim with terminal parent should be denied (out of parent scope)")
	}

	// Only the first authorized call should be recorded.
	if len(store.ToolInvocations("child-claim")) != 1 {
		t.Errorf("expected exactly 1 recorded fact for child-claim, got %d", len(store.ToolInvocations("child-claim")))
	}
}

func TestGateway_AllowsChildWithRunningParent(t *testing.T) {
	gw, r, store, lineage := newFixture(t)
	runClaim(t, r, "parent-claim")
	runClaim(t, r, "child-claim")

	if err := lineage.RegisterChild("parent-claim", "child-claim"); err != nil {
		t.Fatalf("register child: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "child-claim", ToolName: "child-tool"}); err != nil {
		t.Fatalf("child with running parent should be authorized: %v", err)
	}

	invocations := store.ToolInvocations("child-claim")
	if len(invocations) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(invocations))
	}
	if invocations[0].ClaimID != "child-claim" {
		t.Errorf("fact ClaimID = %q, want child-claim", invocations[0].ClaimID)
	}

	// Parent's facts should be empty (child invocations are not attributed to parent).
	if len(store.ToolInvocations("parent-claim")) != 0 {
		t.Error("child invocation should not be attributed to parent claim")
	}
}

func TestGateway_RejectsEmptyClaimID(t *testing.T) {
	gw, _, _, _ := newFixture(t)

	if err := gw.Authorize(Request{ClaimID: "", ToolName: "web-search"}); err == nil {
		t.Fatal("empty claim id should be rejected")
	}
}

func TestGateway_RejectsEmptyToolName(t *testing.T) {
	gw, r, _, _ := newFixture(t)
	runClaim(t, r, "run-1")

	if err := gw.Authorize(Request{ClaimID: "run-1", ToolName: ""}); err == nil {
		t.Fatal("empty tool name should be rejected")
	}
}
