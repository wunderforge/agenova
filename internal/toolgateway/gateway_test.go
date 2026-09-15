// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package toolgateway

import (
	"testing"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/governance"
)

type claimReader struct {
	claims map[string]v1alpha1.SandboxClaim
}

func (r *claimReader) Claim(claimID string) (v1alpha1.SandboxClaim, bool) {
	claim, ok := r.claims[claimID]
	return claim, ok
}

func newFixture() (*Gateway, *claimReader, *facts.Store, *governance.Lineage) {
	claims := &claimReader{claims: make(map[string]v1alpha1.SandboxClaim)}
	store := facts.NewStore()
	lineage := governance.NewLineage()
	return NewGateway(claims, lineage, store), claims, store, lineage
}

func TestGatewayAllowsOnlyAuthoritativeRunningClaim(t *testing.T) {
	gw, claims, store, _ := newFixture()
	claims.claims["run-1"] = claimSnapshot("run-1", v1alpha1.ClaimPhaseRunning)

	if err := gw.Authorize(Request{ClaimID: "run-1", ToolName: "web-search"}); err != nil {
		t.Fatalf("running claim should be authorized: %v", err)
	}
	invocations := store.ToolInvocations("run-1")
	if len(invocations) != 1 || invocations[0].ToolName != "web-search" {
		t.Fatalf("tool facts = %+v, want one web-search invocation", invocations)
	}
}

func TestGatewayDeniesEveryNonRunningAuthoritativePhase(t *testing.T) {
	phases := []v1alpha1.ClaimPhase{
		v1alpha1.ClaimPhasePending,
		v1alpha1.ClaimPhaseBound,
		v1alpha1.ClaimPhaseSucceeded,
		v1alpha1.ClaimPhaseFailed,
		v1alpha1.ClaimPhaseExpired,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			gw, claims, store, _ := newFixture()
			claims.claims["claim-1"] = claimSnapshot("claim-1", phase)

			if err := gw.Authorize(Request{ClaimID: "claim-1", ToolName: "web-search"}); err == nil {
				t.Fatalf("%s claim should be denied", phase)
			}
			if got := store.ToolInvocations("claim-1"); len(got) != 0 {
				t.Fatalf("denied %s claim recorded facts: %+v", phase, got)
			}
		})
	}
}

func TestGatewayDeniesTerminalClaimWhileWorkerIdentityStillExists(t *testing.T) {
	gw, claims, store, _ := newFixture()
	// The backend binding intentionally remains present. Resource existence
	// cannot prolong the authoritative terminal claim's eligibility.
	claims.claims["terminal"] = claimSnapshot("terminal", v1alpha1.ClaimPhaseSucceeded)

	if err := gw.Authorize(Request{ClaimID: "terminal", ToolName: "web-search"}); err == nil {
		t.Fatal("terminal claim with a worker identity should be denied")
	}
	if got := store.ToolInvocations("terminal"); len(got) != 0 {
		t.Fatalf("denied terminal claim recorded facts: %+v", got)
	}
}

func TestGatewayFailsClosedForMissingAndMalformedSnapshots(t *testing.T) {
	tests := []struct {
		name  string
		claim v1alpha1.SandboxClaim
		add   bool
	}{
		{name: "missing"},
		{name: "mismatched id", add: true, claim: claimSnapshot("other", v1alpha1.ClaimPhaseRunning)},
		{name: "blank request ref", add: true, claim: func() v1alpha1.SandboxClaim {
			claim := claimSnapshot("claim-1", v1alpha1.ClaimPhaseRunning)
			claim.RequestRef = ""
			return claim
		}()},
		{name: "running without identity", add: true, claim: func() v1alpha1.SandboxClaim {
			claim := claimSnapshot("claim-1", v1alpha1.ClaimPhaseRunning)
			claim.BackendIdentity = nil
			return claim
		}()},
		{name: "unknown phase", add: true, claim: claimSnapshot("claim-1", v1alpha1.ClaimPhase("Unknown"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gw, claims, store, _ := newFixture()
			if test.add {
				claims.claims["claim-1"] = test.claim
			}
			if err := gw.Authorize(Request{ClaimID: "claim-1", ToolName: "web-search"}); err == nil {
				t.Fatal("missing or malformed snapshot should be denied")
			}
			if got := store.ToolInvocations("claim-1"); len(got) != 0 {
				t.Fatalf("denied request recorded facts: %+v", got)
			}
		})
	}
}

func TestGatewayPreservesExperimentalParentRunningRule(t *testing.T) {
	gw, claims, store, lineage := newFixture()
	claims.claims["parent"] = claimSnapshot("parent", v1alpha1.ClaimPhaseRunning)
	claims.claims["child"] = claimSnapshot("child", v1alpha1.ClaimPhaseRunning)
	if err := lineage.RegisterChild("parent", "child"); err != nil {
		t.Fatalf("RegisterChild: %v", err)
	}

	if err := gw.Authorize(Request{ClaimID: "child", ToolName: "web-search"}); err != nil {
		t.Fatalf("running child with running parent should be allowed: %v", err)
	}
	claims.claims["parent"] = claimSnapshot("parent", v1alpha1.ClaimPhaseSucceeded)
	if err := gw.Authorize(Request{ClaimID: "child", ToolName: "web-search"}); err == nil {
		t.Fatal("running child with terminal parent should be denied")
	}
	if got := store.ToolInvocations("child"); len(got) != 1 {
		t.Fatalf("child facts = %d, want only the allowed invocation", len(got))
	}
}

func TestGatewayValidatesRequestAndDependencies(t *testing.T) {
	gw, claims, _, _ := newFixture()
	claims.claims["run-1"] = claimSnapshot("run-1", v1alpha1.ClaimPhaseRunning)
	if err := gw.Authorize(Request{ToolName: "web-search"}); err == nil {
		t.Fatal("blank claim id should be rejected")
	}
	if err := gw.Authorize(Request{ClaimID: "run-1"}); err == nil {
		t.Fatal("blank tool name should be rejected")
	}
	if err := NewGateway(nil, nil, facts.NewStore()).Authorize(Request{ClaimID: "run-1", ToolName: "web-search"}); err == nil {
		t.Fatal("missing authoritative reader should deny")
	}
	if err := NewGateway(claims, nil, nil).Authorize(Request{ClaimID: "run-1", ToolName: "web-search"}); err == nil {
		t.Fatal("missing fact store should deny")
	}
}

func claimSnapshot(id string, phase v1alpha1.ClaimPhase) v1alpha1.SandboxClaim {
	return v1alpha1.SandboxClaim{
		ID:           id,
		RequestRef:   "request:" + id,
		TemplateRef:  "engineer",
		AuthorityRef: "authority:" + id,
		Phase:        phase,
		BackendIdentity: &v1alpha1.SandboxClaimBackendIdentity{
			Backend:  "reference",
			WorkerID: "worker:" + id,
		},
	}
}
