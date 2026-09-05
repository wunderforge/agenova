// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/governance"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

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

	if err := gw.Authorize(Request{ClaimID: "run-1", ModelName: "claude-sonnet-4-6"}); err != nil {
		t.Fatalf("running claim should be authorized: %v", err)
	}

	invocations := store.ModelInvocations("run-1")
	if len(invocations) != 1 {
		t.Fatalf("expected 1 recorded fact, got %d", len(invocations))
	}
	if invocations[0].ModelName != "claude-sonnet-4-6" {
		t.Errorf("ModelName = %q, want claude-sonnet-4-6", invocations[0].ModelName)
	}
}

// --- negative authorization tests ---

func TestGateway_DeniesUnknownClaim(t *testing.T) {
	gw, _, store, _ := newFixture(t)

	if err := gw.Authorize(Request{ClaimID: "nonexistent", ModelName: "claude-sonnet-4-6"}); err == nil {
		t.Fatal("unknown claim should be denied")
	}
	if len(store.ModelInvocations("nonexistent")) != 0 {
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

	if err := gw.Authorize(Request{ClaimID: "pending-claim", ModelName: "claude-sonnet-4-6"}); err == nil {
		t.Fatal("pending (unbound) claim should be denied")
	}
	if len(store.ModelInvocations("pending-claim")) != 0 {
		t.Error("denied request should not record a fact")
	}
}

func TestGateway_DeniesTerminalClaim(t *testing.T) {
	gw, r, store, _ := newFixture(t)
	runClaim(t, r, "term-claim")

	if err := r.SucceedClaim("term-claim"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "term-claim", ModelName: "claude-sonnet-4-6"}); err == nil {
		t.Fatal("succeeded (terminal) claim should be denied")
	}
	if len(store.ModelInvocations("term-claim")) != 0 {
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

	// Allowed while parent is Running.
	if err := gw.Authorize(Request{ClaimID: "child-claim", ModelName: "claude-haiku-4-5"}); err != nil {
		t.Fatalf("child with running parent should be authorized: %v", err)
	}

	// Terminate the parent.
	if err := r.SucceedClaim("parent-claim"); err != nil {
		t.Fatalf("succeed parent: %v", err)
	}

	// Child is still Running but parent is terminal -> denied (out-of-scope).
	if err := gw.Authorize(Request{ClaimID: "child-claim", ModelName: "claude-haiku-4-5"}); err == nil {
		t.Fatal("child claim with terminal parent should be denied")
	}

	// Only the first call should be recorded.
	if len(store.ModelInvocations("child-claim")) != 1 {
		t.Errorf("expected 1 recorded fact, got %d", len(store.ModelInvocations("child-claim")))
	}
}

func TestGateway_RejectsEmptyClaimID(t *testing.T) {
	gw, _, _, _ := newFixture(t)

	if err := gw.Authorize(Request{ClaimID: "", ModelName: "claude-sonnet-4-6"}); err == nil {
		t.Fatal("empty claim id should be rejected")
	}
}

func TestGateway_RejectsEmptyModelName(t *testing.T) {
	gw, r, _, _ := newFixture(t)
	runClaim(t, r, "run-1")

	if err := gw.Authorize(Request{ClaimID: "run-1", ModelName: ""}); err == nil {
		t.Fatal("empty model name should be rejected")
	}
}
