// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package agentsandbox

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestLegacyStartClaim_doesNotGrantRunningFromReadiness(t *testing.T) {
	k := newFakeKube()
	k.bindOnApply = "legacy-worker"
	k.readyOnApply = true
	a := newTestAdapter(k)
	if err := a.AddClaim(runtime.BackendClaim{Metadata: v1alpha1.ObjectMeta{Name: "legacy"}, Spec: runtime.BackendClaimSpec{PoolRef: "my-pool"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.StartClaim("legacy"); err == nil || errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("pending claim must fail its phase check before unsupported: %v", err)
	}
	if err := a.BindClaim("legacy"); err != nil {
		t.Fatal(err)
	}
	before, _ := a.Claim("legacy")
	if err := a.StartClaim("legacy"); !errors.Is(err, runtime.ErrUnsupported) {
		t.Fatalf("ready worker must not imply work start: %v", err)
	}
	after, _ := a.Claim("legacy")
	if after.Status.Phase != v1alpha1.ClaimPhaseBound || after.Status != before.Status || k.delets != 0 {
		t.Fatalf("unsupported start changed legacy claim or resources: %+v", after)
	}
	if err := a.SucceedClaim("legacy"); err == nil {
		t.Fatal("readiness must not enable legacy success")
	}
}

// Preserve the old expiry bookkeeping regression without presenting it as
// worker-release or application-run evidence in the reduced integration gate.
func TestLegacyExpirePendingClaim_bookkeepingOnly(t *testing.T) {
	k := newFakeKube()
	a := newTestAdapter(k)
	if err := a.AddClaim(runtime.BackendClaim{Metadata: v1alpha1.ObjectMeta{Name: "expiry"}, Spec: runtime.BackendClaimSpec{PoolRef: "my-pool"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.ExpireClaim("expiry", "ttl elapsed"); err != nil {
		t.Fatal(err)
	}
	claim, ok := a.Claim("expiry")
	if !ok || claim.Status.Phase != v1alpha1.ClaimPhaseExpired || claim.Status.SandboxID != "" || claim.Status.SandboxReplaced {
		t.Fatalf("legacy expiry bookkeeping changed: %+v", claim)
	}
}

// Unit tests cover adapter state machine logic without a running cluster.
// Integration tests against a real kind cluster live in harness/integration/agentsandbox/.

func TestResourceName(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if got, want := resourceName("claim", "research-run"), "agenova-claim-research-run"; got != want {
		t.Fatalf("existing safe resource name changed: got %q, want %q", got, want)
	}
	if got, want := resourceName("tmpl", "engineer"), "agenova-tmpl-engineer"; got != want {
		t.Fatalf("existing safe template name changed: got %q, want %q", got, want)
	}
	if got, want := resourceName("pool", "engineer-pool"), "agenova-pool-engineer-pool"; got != want {
		t.Fatalf("existing safe pool name changed: got %q, want %q", got, want)
	}
	for _, original := range []string{
		"research-run",
		"claim:fix-payment-timeout:issuance:0123456789abcdef0123456789abcdef",
		"A/B_C:D",
		strings.Repeat("long-claim-", 20),
	} {
		got := resourceName("claim", original)
		if len(got) > 63 || !valid.MatchString(got) || got != resourceName("claim", original) {
			t.Fatalf("resourceName(%q) = %q is not a stable DNS label", original, got)
		}
	}
	if first, second := resourceName("claim", "A:B"), resourceName("claim", "A-B"); first == second {
		t.Fatalf("distinct identities collapsed to the same name: %q", first)
	}
	unsafeName := resourceName("claim", "A:B")
	safeAlias := strings.TrimPrefix(unsafeName, "agenova-claim-")
	if got := resourceName("claim", safeAlias); got == unsafeName {
		t.Fatalf("reserved mapped name aliases safe identity %q", safeAlias)
	}
}

func TestRequirePhase_notFound(t *testing.T) {
	a := New("test-context", "default")
	err := a.requirePhase("missing", v1alpha1.ClaimPhasePending)
	if err == nil {
		t.Fatal("expected error for missing claim")
	}
}

func TestRequirePhase_wrongPhase(t *testing.T) {
	a := New("test-context", "default")
	a.claims["c1"] = claimEntry{phase: v1alpha1.ClaimPhaseBound}

	err := a.requirePhase("c1", v1alpha1.ClaimPhasePending)
	if err == nil {
		t.Fatal("expected error: claim is Bound, not Pending")
	}
}

func TestRequirePhase_match(t *testing.T) {
	a := New("test-context", "default")
	a.claims["c1"] = claimEntry{phase: v1alpha1.ClaimPhaseBound}

	if err := a.requirePhase("c1", v1alpha1.ClaimPhaseBound, v1alpha1.ClaimPhaseRunning); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClaim_returnsLocalState(t *testing.T) {
	a := New("test-context", "default")
	a.claims["run1"] = claimEntry{
		phase:     v1alpha1.ClaimPhaseSucceeded,
		sandboxID: "agenova-claim-run1-sandbox",
		replaced:  true,
	}

	got, ok := a.Claim("run1")
	if !ok {
		t.Fatal("claim not found")
	}
	if got.Status.Phase != v1alpha1.ClaimPhaseSucceeded {
		t.Fatalf("phase = %s, want Succeeded", got.Status.Phase)
	}
	if !got.Status.SandboxReplaced {
		t.Fatal("expected SandboxReplaced = true")
	}
	if got.Status.SandboxID != "agenova-claim-run1-sandbox" {
		t.Fatalf("sandboxID = %q, want %q", got.Status.SandboxID, "agenova-claim-run1-sandbox")
	}
}

func TestClaim_notFound(t *testing.T) {
	a := New("test-context", "default")
	_, ok := a.Claim("does-not-exist")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestPoolStatus_notFound(t *testing.T) {
	a := New("test-context", "default")
	_, ok := a.PoolStatus("missing-pool")
	if ok {
		t.Fatal("expected pool not found")
	}
}

func TestHasCondition(t *testing.T) {
	conditions := []upstreamCondition{
		{Type: "Ready", Status: "True", Reason: "SandboxReady"},
		{Type: "Degraded", Status: "False"},
	}
	if !hasCondition(conditions, "Ready", "True") {
		t.Fatal("expected Ready=True to be found")
	}
	if hasCondition(conditions, "Ready", "False") {
		t.Fatal("expected Ready=False not to be found")
	}
	if hasCondition(conditions, "Missing", "True") {
		t.Fatal("expected Missing condition not to be found")
	}
}

func TestAddClaim_duplicateRejected(t *testing.T) {
	a := New("test-context", "default")
	// Pre-populate pool and template refs so validation passes.
	a.poolRefs["tmpl-v1"] = "agenova-tmpl-tmpl-v1"
	a.pools["my-pool"] = poolEntry{
		upstreamTemplateName: "agenova-tmpl-tmpl-v1",
		upstreamPoolName:     "agenova-pool-my-pool",
		replicas:             1,
	}
	a.claims["dup"] = claimEntry{phase: v1alpha1.ClaimPhasePending}

	err := a.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "dup"},
		Spec:     runtime.BackendClaimSpec{PoolRef: "my-pool"},
	})
	if err == nil {
		t.Fatal("expected duplicate claim to be rejected")
	}
}
