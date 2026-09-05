// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package contracttest provides a reusable suite that exercises the
// RuntimeBackend lifecycle contract. Import this package from any backend's
// test file and call Run to verify behavioral parity with the in-memory
// reference implementation.
package contracttest

import (
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// Run exercises the RuntimeBackend lifecycle contract against any backend.
// newBackendFn must return a fresh, pre-configured backend instance (with the
// test template and pool already registered) for each subtest.
func Run(t *testing.T, newBackendFn func(t *testing.T) runtime.RuntimeBackend) {
	t.Helper()

	t.Run("succeeded claim auto-replaces sandbox without growing active pool", func(t *testing.T) {
		testSucceededClaimAutoReplacesSandbox(t, newBackendFn(t))
	})
	t.Run("failed claim auto-replaces sandbox and leaves pool claimable", func(t *testing.T) {
		testFailedClaimAutoReplacesSandboxAndLeavesPoolClaimable(t, newBackendFn(t))
	})
	t.Run("bound claim can fail before start and replaces sandbox", func(t *testing.T) {
		testBoundClaimCanFailBeforeStartAndReplacesSandbox(t, newBackendFn(t))
	})
	t.Run("pending claim can expire without binding sandbox", func(t *testing.T) {
		testPendingClaimCanExpireWithoutBindingSandbox(t, newBackendFn(t))
	})
	t.Run("duplicate claim is rejected", func(t *testing.T) {
		testDuplicateClaimIsRejected(t, newBackendFn(t))
	})
	t.Run("invalid transitions remain rejected", func(t *testing.T) {
		testInvalidTransitionsRemainRejected(t, newBackendFn(t))
	})
}

func testSucceededClaimAutoReplacesSandbox(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "research-run")
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})

	if err := backend.BindClaim("research-run"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, backend, "research-run", v1alpha1.ClaimPhaseBound)
	if bound.Status.SandboxID == "" {
		t.Fatal("bound claim should record sandbox id")
	}
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{BoundClaims: 1})

	if err := backend.StartClaim("research-run"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	assertClaimPhase(t, backend, "research-run", v1alpha1.ClaimPhaseRunning)
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{RunningClaims: 1})

	if err := backend.SucceedClaim("research-run"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	succeeded := assertClaimPhase(t, backend, "research-run", v1alpha1.ClaimPhaseSucceeded)
	if succeeded.Status.SandboxID != bound.Status.SandboxID {
		t.Fatalf("succeeded claim should keep original sandbox id, got %q want %q", succeeded.Status.SandboxID, bound.Status.SandboxID)
	}
	if !succeeded.Status.SandboxReplaced {
		t.Fatal("succeeded claim should record the sandbox-replaced resource fact")
	}
	// Pool must stay at replicas=1 with a fresh idle sandbox after replacement.
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})

	// Binding another claim proves the replacement sandbox is idle and usable,
	// and that the original sandbox was not reused.
	addPendingClaim(t, backend, "research-run-2")
	if err := backend.BindClaim("research-run-2"); err != nil {
		t.Fatalf("pool should be claimable after succeeded replacement: %v", err)
	}
	next := assertClaimPhase(t, backend, "research-run-2", v1alpha1.ClaimPhaseBound)
	if next.Status.SandboxID == "" {
		t.Fatal("replacement sandbox should bind to the next claim")
	}
	if next.Status.SandboxID == bound.Status.SandboxID {
		t.Fatal("succeeded sandbox should not be reused for the next claim")
	}
}

func testFailedClaimAutoReplacesSandboxAndLeavesPoolClaimable(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "fail-once")

	if err := backend.BindClaim("fail-once"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, backend, "fail-once", v1alpha1.ClaimPhaseBound)
	if err := backend.StartClaim("fail-once"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := backend.FailClaim("fail-once", "agent exited 1"); err != nil {
		t.Fatalf("fail claim: %v", err)
	}

	failed := assertClaimPhase(t, backend, "fail-once", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "agent exited 1" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})

	addPendingClaim(t, backend, "after-failure")
	if err := backend.BindClaim("after-failure"); err != nil {
		t.Fatalf("pool should remain claimable after failed claim replacement: %v", err)
	}
	afterFailure := assertClaimPhase(t, backend, "after-failure", v1alpha1.ClaimPhaseBound)
	if afterFailure.Status.SandboxID == "" {
		t.Fatal("replacement sandbox should bind to the next claim")
	}
	if afterFailure.Status.SandboxID == bound.Status.SandboxID {
		t.Fatal("failed sandbox should not be reused for the next claim")
	}
}

func testBoundClaimCanFailBeforeStartAndReplacesSandbox(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "lost-before-start")

	if err := backend.BindClaim("lost-before-start"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}

	if err := backend.FailClaim("lost-before-start", "sandbox lost before start"); err != nil {
		t.Fatalf("fail bound claim: %v", err)
	}

	failed := assertClaimPhase(t, backend, "lost-before-start", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "sandbox lost before start" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("bound-failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})
}

func testPendingClaimCanExpireWithoutBindingSandbox(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "expire-once")

	if err := backend.ExpireClaim("expire-once", "ttl elapsed"); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	expired := assertClaimPhase(t, backend, "expire-once", v1alpha1.ClaimPhaseExpired)
	if expired.Status.SandboxID != "" {
		t.Fatalf("expired pending claim should not bind a sandbox, got %q", expired.Status.SandboxID)
	}
	if expired.Status.SandboxReplaced {
		t.Fatal("expired pending claim never bound a sandbox, so none should be replaced")
	}
	assertPoolStatus(t, backend, v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})
}

func testDuplicateClaimIsRejected(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "duplicate")

	err := backend.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "duplicate"},
		Spec: runtime.BackendClaimSpec{
			PoolRef: "python-agent-pool",
		},
	})
	if err == nil {
		t.Fatal("expected duplicate claim to be rejected")
	}
}

func testInvalidTransitionsRemainRejected(t *testing.T, backend runtime.RuntimeBackend) {
	t.Helper()

	addPendingClaim(t, backend, "invalid")

	if err := backend.StartClaim("invalid"); err == nil {
		t.Fatal("pending claim should not start before bind")
	}
	if err := backend.SucceedClaim("invalid"); err == nil {
		t.Fatal("pending claim should not succeed")
	}
	if err := backend.FailClaim("invalid", "no sandbox yet"); err == nil {
		t.Fatal("pending claim should not fail; it expires instead")
	}
	if err := backend.BindClaim("invalid"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	if err := backend.SucceedClaim("invalid"); err == nil {
		t.Fatal("bound claim should not succeed before running")
	}
	if err := backend.StartClaim("invalid"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := backend.ExpireClaim("invalid", "too late"); err == nil {
		t.Fatal("running claim should not expire through pending transition")
	}
	if err := backend.SucceedClaim("invalid"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}
	if err := backend.SucceedClaim("invalid"); err == nil {
		t.Fatal("terminal claim should not transition again")
	}
}

// --- helpers ---

func addPendingClaim(t *testing.T, backend runtime.RuntimeBackend, name string) {
	t.Helper()

	claim := runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: name},
		Spec: runtime.BackendClaimSpec{
			PoolRef: "python-agent-pool",
		},
	}
	if err := backend.AddClaim(claim); err != nil {
		t.Fatalf("add claim: %v", err)
	}
}

func assertClaimPhase(t *testing.T, backend runtime.RuntimeBackend, name string, phase v1alpha1.ClaimPhase) runtime.BackendClaim {
	t.Helper()

	claim, ok := backend.Claim(name)
	if !ok {
		t.Fatalf("claim not found: %s", name)
	}
	if claim.Status.Phase != phase {
		t.Fatalf("claim phase = %s, want %s", claim.Status.Phase, phase)
	}
	return claim
}

func assertPoolStatus(t *testing.T, backend runtime.RuntimeBackend, want v1alpha1.SandboxWarmPoolStatus) {
	t.Helper()

	got, ok := backend.PoolStatus("python-agent-pool")
	if !ok {
		t.Fatalf("pool not found: python-agent-pool")
	}
	if got != want {
		t.Fatalf("pool status = %+v, want %+v", got, want)
	}
}
