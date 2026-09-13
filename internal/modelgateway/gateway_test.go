// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelgateway

import (
	"fmt"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/gateway/gatewaytest"
	"github.com/wunderforge/agenova/internal/governance"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

// spyAdapter counts provider-side invocations so tests can prove the
// zero-call guarantee for Deny and ApprovalRequired.
type spyAdapter struct {
	calls []spyCall
}

type spyCall struct {
	invocationID string
	req          Request
}

func (s *spyAdapter) Invoke(invocationID string, req Request) error {
	s.calls = append(s.calls, spyCall{invocationID: invocationID, req: req})
	return nil
}

func newFixture(t *testing.T, opts ...Option) (*Gateway, *operator.Runtime, *facts.Store, *governance.Lineage, *spyAdapter) {
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
	spy := &spyAdapter{}
	base := []Option{WithAdapter(spy), WithIDSource(gateway.SequenceIDSource("inv-test"))}
	gw := NewGateway(r, lineage, store, append(base, opts...)...)
	return gw, r, store, lineage, spy
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

// teamARequest builds a valid governed model request from the frozen Team A
// engineer fixture: claim identity plus the granted model profile.
func teamARequest(t *testing.T) Request {
	t.Helper()

	authority := gatewaytest.LoadTeamAAuthority(t)
	return Request{
		ClaimID: authority.ClaimID,
		Profile: authority.ModelProfile,
	}
}

func mustInvoke(t *testing.T, gw *Gateway, req Request) gateway.Decision {
	t.Helper()
	decision, err := gw.Invoke(req)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	return decision
}

func TestGateway_AllowsRunningClaimWithFixtureProfile(t *testing.T) {
	gw, r, store, _, spy := newFixture(t)
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if decision.Result != gateway.ResultAllow {
		t.Fatalf("Result = %q (%s), want Allow", decision.Result, decision.Reason)
	}
	if decision.InvocationID == "" {
		t.Fatal("Allow decision must carry a gateway-assigned invocation id")
	}
	if len(spy.calls) != 1 {
		t.Fatalf("adapter calls = %d, want 1", len(spy.calls))
	}
	if spy.calls[0].invocationID != decision.InvocationID {
		t.Errorf("adapter call id = %q, decision id = %q; attempted call must correlate", spy.calls[0].invocationID, decision.InvocationID)
	}

	invocations := store.ModelInvocations(req.ClaimID)
	if len(invocations) != 1 {
		t.Fatalf("expected 1 recorded fact, got %d", len(invocations))
	}
	if invocations[0].InvocationID != decision.InvocationID {
		t.Errorf("fact invocation id = %q, decision id = %q; evidence must correlate", invocations[0].InvocationID, decision.InvocationID)
	}
	if invocations[0].Result != gateway.ResultAllow {
		t.Errorf("fact result = %q, want Allow", invocations[0].Result)
	}
	if invocations[0].ModelName != req.Profile {
		t.Errorf("fact profile = %q, want %q", invocations[0].ModelName, req.Profile)
	}
}

// TestGateway_IssuesInvocationIDBeforePolicyEvaluation proves the ordering the
// contract depends on, not just that an ID exists once Invoke returns. The ID
// source and the policy write into one shared log, so the policy can observe
// exactly what had been issued at the moment it ran. Asserting only on the
// returned decision cannot tell entry-time issuance apart from issuance after
// policy evaluation.
func TestGateway_IssuesInvocationIDBeforePolicyEvaluation(t *testing.T) {
	var issued []string
	var atPolicy []string
	policyRuns := 0

	ids := func() string {
		id := fmt.Sprintf("inv-order-%d", len(issued)+1)
		issued = append(issued, id)
		return id
	}
	policy := func(Request) gateway.Outcome {
		policyRuns++
		atPolicy = append([]string(nil), issued...)
		return gateway.Allowed()
	}

	gw, r, _, _, _ := newFixture(t, WithIDSource(ids), WithPolicy(policy))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if policyRuns != 1 {
		t.Fatalf("policy evaluations = %d, want 1", policyRuns)
	}
	if len(atPolicy) != 1 {
		t.Fatalf("ids issued when policy ran = %d, want 1: the trusted id must be issued at gateway entry, before policy evaluation", len(atPolicy))
	}
	if atPolicy[0] != decision.InvocationID {
		t.Errorf("id visible to policy = %q, decision id = %q; policy must see the same correlation identity the caller receives", atPolicy[0], decision.InvocationID)
	}
	if len(issued) != 1 {
		t.Errorf("ids issued for one attempt = %d, want 1", len(issued))
	}
}

// TestGateway_AllowCarriesPolicyReason keeps the Allow branch consistent with
// Deny and ApprovalRequired: the normalized policy outcome is what reaches the
// caller, so an authorization explanation is not dropped on the way out.
func TestGateway_AllowCarriesPolicyReason(t *testing.T) {
	const reason = "permitted by reference policy v1"
	policy := func(Request) gateway.Outcome {
		return gateway.Outcome{Result: gateway.ResultAllow, Reason: reason}
	}

	gw, r, _, _, spy := newFixture(t, WithPolicy(policy))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if decision.Result != gateway.ResultAllow {
		t.Fatalf("Result = %q, want Allow", decision.Result)
	}
	if decision.Reason != reason {
		t.Errorf("Reason = %q, want %q; an Allow must carry the policy's own reason", decision.Reason, reason)
	}
	if decision.Category != "" {
		t.Errorf("Category = %q, want empty for Allow", decision.Category)
	}
	if len(spy.calls) != 1 {
		t.Errorf("adapter calls = %d, want 1", len(spy.calls))
	}
}

func TestGateway_DoesNotAdoptCallerSuppliedIdentifier(t *testing.T) {
	gw, r, _, _, _ := newFixture(t)
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)
	req.CallerReference = "caller-chosen-id"

	decision := mustInvoke(t, gw, req)

	if decision.InvocationID == req.CallerReference {
		t.Fatalf("caller-supplied identifier %q was adopted as the trusted invocation id", req.CallerReference)
	}
	if decision.InvocationID != "inv-test-1" {
		t.Errorf("invocation id = %q, want gateway-issued inv-test-1", decision.InvocationID)
	}
}

func TestGateway_RejectsInvalidRequestsBeforeAdapter(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(t *testing.T, req *Request)
		wantCategory string
	}{
		{
			name:         "missing claim identity",
			mutate:       func(t *testing.T, req *Request) { req.ClaimID = "" },
			wantCategory: gateway.CategoryMissingClaimIdentity,
		},
		{
			name:         "missing profile",
			mutate:       func(t *testing.T, req *Request) { req.Profile = "" },
			wantCategory: gateway.CategoryIncompleteOperation,
		},
		{
			name:         "blank claim identity",
			mutate:       func(t *testing.T, req *Request) { req.ClaimID = "   " },
			wantCategory: gateway.CategoryMissingClaimIdentity,
		},
		{
			name:         "blank profile",
			mutate:       func(t *testing.T, req *Request) { req.Profile = "  " },
			wantCategory: gateway.CategoryIncompleteOperation,
		},
		{
			name: "secret-bearing parameter",
			mutate: func(t *testing.T, req *Request) {
				req.Parameters = map[string]string{gatewaytest.SecretParameterKey(t): "value"}
			},
			wantCategory: gateway.CategorySecretValue,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gw, r, store, _, spy := newFixture(t)
			req := teamARequest(t)
			runClaim(t, r, req.ClaimID)
			tc.mutate(t, &req)

			decision := mustInvoke(t, gw, req)

			if decision.Result != gateway.ResultDeny {
				t.Fatalf("Result = %q, want Deny", decision.Result)
			}
			if decision.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", decision.Category, tc.wantCategory)
			}
			if decision.InvocationID == "" {
				t.Error("rejected attempt must still carry an invocation id for evidence correlation")
			}
			if len(spy.calls) != 0 {
				t.Errorf("adapter calls = %d, want 0: rejection must happen before adapter invocation", len(spy.calls))
			}
			// Structural rejection happens before claim resolution, so the
			// asserted claim must not gain fabricated evidence.
			if got := len(store.ModelInvocations(req.ClaimID)); got != 0 {
				t.Errorf("persisted facts = %d, want 0: an unresolved claim must not be attributed a fact", got)
			}
		})
	}
}

// A gateway with no provider adapter must not report success for an allowed
// invocation that was never attempted.
func TestGateway_UnconfiguredAdapterFailsClosed(t *testing.T) {
	r := operator.NewRuntime()
	if err := r.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-v1"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agent:dev", Command: []string{"agent"}},
	}); err != nil {
		t.Fatalf("add template: %v", err)
	}
	if err := r.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-pool"},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: "agent-v1", Replicas: 1},
	}); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	store := facts.NewStore()
	gw := NewGateway(r, governance.NewLineage(), store)
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision, err := gw.Invoke(req)
	if err == nil {
		t.Fatal("an allowed invocation with no adapter wired must report a configuration failure, not silent success")
	}
	if decision.Result != gateway.ResultAllow || decision.InvocationID == "" {
		t.Errorf("decision = %+v, want a correlatable Allow alongside the configuration error", decision)
	}
	if got := store.ModelInvocations(req.ClaimID); len(got) != 1 || got[0].Result != gateway.ResultAllow {
		t.Errorf("facts = %+v, want the single Allow decision fact", got)
	}
}

// A policy that omits Result must be denied, not recorded as an untyped result.
func TestGateway_UntypedPolicyOutcomeIsDenied(t *testing.T) {
	incomplete := func(Request) gateway.Outcome {
		return gateway.Outcome{Category: "profile-not-granted", Reason: "profile not in grant"}
	}
	gw, r, store, _, spy := newFixture(t, WithPolicy(incomplete))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if decision.Result != gateway.ResultDeny {
		t.Fatalf("Result = %q, want Deny", decision.Result)
	}
	if decision.Category != gateway.CategoryInvalidPolicyOutcome {
		t.Errorf("Category = %q, want %q", decision.Category, gateway.CategoryInvalidPolicyOutcome)
	}
	if len(spy.calls) != 0 {
		t.Errorf("adapter calls = %d, want 0", len(spy.calls))
	}
	if got := store.ModelInvocations(req.ClaimID); len(got) != 1 || got[0].Result != gateway.ResultDeny {
		t.Fatalf("want one persisted Deny fact, got %+v", got)
	}
}

// A nil option value must keep the safe default instead of leaving the gateway
// in a state that panics on the first allowed request.
func TestGateway_NilOptionsKeepSafeDefaults(t *testing.T) {
	r := operator.NewRuntime()
	if err := r.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-v1"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agent:dev", Command: []string{"agent"}},
	}); err != nil {
		t.Fatalf("add template: %v", err)
	}
	if err := r.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "agent-pool"},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: "agent-v1", Replicas: 1},
	}); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	gw := NewGateway(r, governance.NewLineage(), facts.NewStore(),
		WithAdapter(nil), WithIDSource(nil), WithPolicy(nil))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision, err := gw.Invoke(req)
	if err == nil {
		t.Fatal("a nil adapter must fall back to the fail-closed default, not report success")
	}
	if decision.InvocationID == "" {
		t.Error("a nil ID source must fall back to the default, still issuing an id")
	}
	if decision.Result != gateway.ResultAllow {
		t.Errorf("Result = %q, want Allow from the default policy", decision.Result)
	}
}

func TestGateway_ApprovalRequiredDoesNotInvokeAdapterOrGrant(t *testing.T) {
	approvalPolicy := func(Request) gateway.Outcome {
		return gateway.Outcome{
			Result: gateway.ResultApprovalRequired,
			Reason: "operation routed for human approval",
		}
	}
	gw, r, store, _, spy := newFixture(t, WithPolicy(approvalPolicy))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if decision.Result != gateway.ResultApprovalRequired {
		t.Fatalf("Result = %q, want ApprovalRequired", decision.Result)
	}
	if decision.InvocationID == "" {
		t.Error("invocation id must be assigned before policy evaluation")
	}
	if len(spy.calls) != 0 {
		t.Errorf("adapter calls = %d, want 0: ApprovalRequired must not reach the provider", len(spy.calls))
	}
	invocations := store.ModelInvocations(req.ClaimID)
	if len(invocations) != 1 || invocations[0].Result != gateway.ResultApprovalRequired {
		t.Fatalf("want exactly 1 persisted ApprovalRequired fact (granting nothing), got %+v", invocations)
	}
	if invocations[0].InvocationID != decision.InvocationID {
		t.Errorf("fact invocation id = %q, decision id = %q", invocations[0].InvocationID, decision.InvocationID)
	}
}

func TestGateway_PolicyDenyDoesNotInvokeAdapter(t *testing.T) {
	denyPolicy := func(Request) gateway.Outcome {
		return gateway.Outcome{
			Result:   gateway.ResultDeny,
			Category: "policy-denied",
			Reason:   "test policy denies everything",
		}
	}
	gw, r, store, _, spy := newFixture(t, WithPolicy(denyPolicy))
	req := teamARequest(t)
	runClaim(t, r, req.ClaimID)

	decision := mustInvoke(t, gw, req)

	if decision.Result != gateway.ResultDeny {
		t.Fatalf("Result = %q, want Deny", decision.Result)
	}
	if len(spy.calls) != 0 {
		t.Errorf("adapter calls = %d, want 0 for Deny", len(spy.calls))
	}
	invocations := store.ModelInvocations(req.ClaimID)
	if len(invocations) != 1 || invocations[0].Result != gateway.ResultDeny {
		t.Fatalf("want exactly 1 persisted Deny fact, got %+v", invocations)
	}
}

func TestGateway_DeniesInactiveClaims(t *testing.T) {
	cases := []struct {
		name         string
		prepare      func(t *testing.T, r *operator.Runtime, claimID string)
		wantCategory string
		wantFacts    int
	}{
		{
			// The claim never resolves, so no claim-scoped fact may be created.
			name:         "unknown claim",
			prepare:      func(t *testing.T, r *operator.Runtime, claimID string) {},
			wantCategory: gateway.CategoryUnknownClaim,
			wantFacts:    0,
		},
		{
			name: "pending claim",
			prepare: func(t *testing.T, r *operator.Runtime, claimID string) {
				if err := r.AddClaim(runtime.BackendClaim{
					Metadata: v1alpha1.ObjectMeta{Name: claimID},
					Spec:     runtime.BackendClaimSpec{PoolRef: "agent-pool"},
				}); err != nil {
					t.Fatalf("add claim: %v", err)
				}
			},
			wantCategory: gateway.CategoryClaimNotActive,
			wantFacts:    1,
		},
		{
			name: "terminal claim",
			prepare: func(t *testing.T, r *operator.Runtime, claimID string) {
				runClaim(t, r, claimID)
				if err := r.SucceedClaim(claimID); err != nil {
					t.Fatalf("succeed claim: %v", err)
				}
			},
			wantCategory: gateway.CategoryClaimNotActive,
			wantFacts:    1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gw, r, store, _, spy := newFixture(t)
			req := teamARequest(t)
			tc.prepare(t, r, req.ClaimID)

			decision := mustInvoke(t, gw, req)

			if decision.Result != gateway.ResultDeny {
				t.Fatalf("Result = %q, want Deny", decision.Result)
			}
			if decision.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", decision.Category, tc.wantCategory)
			}
			if len(spy.calls) != 0 {
				t.Errorf("adapter calls = %d, want 0", len(spy.calls))
			}
			invocations := store.ModelInvocations(req.ClaimID)
			if len(invocations) != tc.wantFacts {
				t.Fatalf("persisted facts = %d, want %d", len(invocations), tc.wantFacts)
			}
			if tc.wantFacts == 1 {
				if invocations[0].Result != gateway.ResultDeny {
					t.Errorf("fact result = %q, want Deny", invocations[0].Result)
				}
				if invocations[0].InvocationID != decision.InvocationID {
					t.Errorf("fact invocation id = %q, decision id = %q", invocations[0].InvocationID, decision.InvocationID)
				}
			}
		})
	}
}

func TestGateway_DeniesChildWithTerminalParent(t *testing.T) {
	gw, r, store, lineage, spy := newFixture(t)
	req := teamARequest(t)
	parentID := "parent-" + req.ClaimID
	runClaim(t, r, parentID)
	runClaim(t, r, req.ClaimID)

	if err := lineage.RegisterChild(parentID, req.ClaimID); err != nil {
		t.Fatalf("register child: %v", err)
	}

	if decision := mustInvoke(t, gw, req); decision.Result != gateway.ResultAllow {
		t.Fatalf("child with running parent: Result = %q (%s), want Allow", decision.Result, decision.Reason)
	}

	if err := r.SucceedClaim(parentID); err != nil {
		t.Fatalf("succeed parent: %v", err)
	}

	decision := mustInvoke(t, gw, req)
	if decision.Result != gateway.ResultDeny {
		t.Fatalf("child with terminal parent: Result = %q, want Deny", decision.Result)
	}
	if decision.Category != gateway.CategoryOutOfParentScope {
		t.Errorf("Category = %q, want %q", decision.Category, gateway.CategoryOutOfParentScope)
	}
	if len(spy.calls) != 1 {
		t.Errorf("adapter calls = %d, want exactly the 1 allowed attempt", len(spy.calls))
	}
	invocations := store.ModelInvocations(req.ClaimID)
	if len(invocations) != 2 {
		t.Fatalf("expected Allow then Deny facts, got %d", len(invocations))
	}
	if invocations[0].Result != gateway.ResultAllow || invocations[1].Result != gateway.ResultDeny {
		t.Errorf("fact results = [%s, %s], want [Allow, Deny]", invocations[0].Result, invocations[1].Result)
	}
}
