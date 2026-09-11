// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"errors"
	"testing"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/runtime/contracttest"
)

const (
	testTemplate = "python-agent-v1"
	testPool     = "python-agent-pool"
)

// TestRuntimeBackendContract runs the reusable reduced-contract suite against
// the reference runtime with the control hooks the suite requires.
func TestRuntimeBackendContract(t *testing.T) {
	contracttest.Run(t, func(t *testing.T) contracttest.Fixture {
		r := newConfiguredRuntime(t)
		return contracttest.Fixture{
			Backend:       r,
			TemplateRef:   testTemplate,
			HoldReadiness: r.HoldReadiness,
			ReleaseReadiness: func(id v1alpha1.SandboxClaimBackendIdentity) {
				if err := r.MarkReady(id); err != nil {
					t.Fatalf("release readiness: %v", err)
				}
			},
			Started: r.Started,
			FailNextStart: func(id v1alpha1.SandboxClaimBackendIdentity, err error) {
				injectWorkerFailure(t, r, id, "start", err)
			},
			FailNextTerminate: func(id v1alpha1.SandboxClaimBackendIdentity, err error) {
				injectWorkerFailure(t, r, id, "terminate", err)
			},
			FailNextCleanup: func(id v1alpha1.SandboxClaimBackendIdentity, err error) {
				injectWorkerFailure(t, r, id, "cleanup", err)
			},
		}
	})
}

// TestRuntimeFilesystemContract proves the reference model's backend-neutral
// filesystem semantics. It does not claim native-process isolation.
func TestRuntimeFilesystemContract(t *testing.T) {
	contracttest.RunFilesystem(t, func(t *testing.T) contracttest.FilesystemFixture {
		r := newConfiguredRuntime(t)
		return contracttest.FilesystemFixture{
			Backend:          r,
			TemplateRef:      testTemplate,
			PrepareTaskFile:  r.prepareTaskFile,
			WriteTaskFile:    r.writeTaskFile,
			ReadTaskFile:     r.readTaskFile,
			ExportTaskFile:   r.exportTaskFile,
			ReadRuntimeFile:  r.readRuntimeFile,
			WriteRuntimeFile: r.writeRuntimeFile,
			OutsideSentinel:  r.outsideFilesystemSentinel,
			RuntimeSentinel:  r.runtimeFilesystemSentinel,
			FailNextTerminate: func(id v1alpha1.SandboxClaimBackendIdentity, err error) {
				injectWorkerFailure(t, r, id, "terminate", err)
			},
		}
	})
}

func newConfiguredRuntime(t *testing.T) *Runtime {
	t.Helper()

	r := NewRuntime()
	template := v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: testTemplate},
		Spec: v1alpha1.AgentSandboxTemplateSpec{
			Image:   "example.local/agenova/python-agent:dev",
			Command: []string{"python", "/app/agent.py"},
		},
	}
	if err := r.AddTemplate(template); err != nil {
		t.Fatalf("add template: %v", err)
	}

	pool := v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: testPool},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: testTemplate,
			Replicas:    1,
		},
	}
	if err := r.AddWarmPool(pool); err != nil {
		t.Fatalf("add warm pool: %v", err)
	}
	return r
}

// --- reduced contract: reference-specific behavior ---

func TestAllocate_rejectsUnknownTemplateAndAmbiguousPool(t *testing.T) {
	r := newConfiguredRuntime(t)

	if _, err := r.Allocate(runtime.AllocateRequest{ClaimID: "c", TemplateRef: "missing"}); err == nil {
		t.Fatal("unknown template should be rejected")
	}

	if err := r.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "second-pool"},
		Spec:     v1alpha1.SandboxWarmPoolSpec{TemplateRef: testTemplate, Replicas: 1},
	}); err != nil {
		t.Fatalf("add second pool: %v", err)
	}
	if _, err := r.Allocate(runtime.AllocateRequest{ClaimID: "c", TemplateRef: testTemplate}); err == nil {
		t.Fatal("two pools for one template must be rejected as ambiguous, not silently picked")
	}
}

func TestAllocate_failsExplicitlyWithoutCapacity(t *testing.T) {
	r := newConfiguredRuntime(t)
	if _, err := r.Allocate(runtime.AllocateRequest{ClaimID: "first", TemplateRef: testTemplate}); err != nil {
		t.Fatalf("first allocate: %v", err)
	}
	_, err := r.Allocate(runtime.AllocateRequest{ClaimID: "second", TemplateRef: testTemplate})
	if err == nil {
		t.Fatal("allocation without idle capacity must fail explicitly")
	}
	if _, err := r.Observe(v1alpha1.SandboxClaimBackendIdentity{Backend: BackendName, WorkerID: "python-agent-pool-2"}); !errors.Is(err, runtime.ErrUnknownIdentity) {
		t.Fatalf("no identity may be fabricated for a failed allocation, got %v", err)
	}
}

// The legacy phase path and the reduced Allocate path share the pools, so the
// same ClaimID must be rejected in both directions.
func TestDuplicateClaimAcrossPaths_isRejectedBothWays(t *testing.T) {
	t.Run("allocate then legacy AddClaim", func(t *testing.T) {
		r := newConfiguredRuntime(t)
		if _, err := r.Allocate(runtime.AllocateRequest{ClaimID: "shared", TemplateRef: testTemplate}); err != nil {
			t.Fatalf("allocate: %v", err)
		}
		err := r.AddClaim(runtime.BackendClaim{
			Metadata: v1alpha1.ObjectMeta{Name: "shared"},
			Spec:     runtime.BackendClaimSpec{PoolRef: testPool},
		})
		if err == nil {
			t.Fatal("legacy AddClaim must reject a ClaimID already allocated through the contract")
		}
	})
	t.Run("legacy AddClaim then allocate", func(t *testing.T) {
		r := newConfiguredRuntime(t)
		addPendingClaim(t, r, "shared")
		if _, err := r.Allocate(runtime.AllocateRequest{ClaimID: "shared", TemplateRef: testTemplate}); err == nil {
			t.Fatal("Allocate must reject a ClaimID already present in the legacy claim path")
		}
	})
}

func TestTerminate_neverChangesLegacyClaimPhase(t *testing.T) {
	r := newConfiguredRuntime(t)
	alloc, err := r.Allocate(runtime.AllocateRequest{ClaimID: "iso", TemplateRef: testTemplate})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if err := r.Start(alloc.Identity); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := r.Terminate(alloc.Identity); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if _, ok := r.Claim("iso"); ok {
		t.Fatal("reduced-contract allocation must not create a legacy BackendClaim")
	}
	res, err := r.Cleanup(alloc.Identity)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if !res.Released || !res.Replaced {
		t.Fatalf("memory cleanup should release and replace, got %+v", res)
	}
	status, ok := r.PoolStatus(testPool)
	if !ok || status.IdleSandboxes != 1 || status.ReplacedSandboxes != 1 {
		t.Fatalf("pool should hold one fresh idle worker after cleanup, got %+v", status)
	}
}

// --- legacy reference phase behavior (moved from the old reusable suite) ---
//
// These cases pin the in-memory reference runtime's phase state machine and
// pool replacement semantics. They are reference-specific regressions, not
// contract requirements for every backend.

func TestReferencePoolLifecycle_succeededClaimAutoReplacesSandbox(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "research-run")
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})

	if err := r.BindClaim("research-run"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, r, "research-run", v1alpha1.ClaimPhaseBound)
	if bound.Status.SandboxID == "" {
		t.Fatal("bound claim should record sandbox id")
	}
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{BoundClaims: 1})

	if err := r.StartClaim("research-run"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	assertClaimPhase(t, r, "research-run", v1alpha1.ClaimPhaseRunning)
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{RunningClaims: 1})

	if err := r.SucceedClaim("research-run"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}

	succeeded := assertClaimPhase(t, r, "research-run", v1alpha1.ClaimPhaseSucceeded)
	if succeeded.Status.SandboxID != bound.Status.SandboxID {
		t.Fatalf("succeeded claim should keep original sandbox id, got %q want %q", succeeded.Status.SandboxID, bound.Status.SandboxID)
	}
	if !succeeded.Status.SandboxReplaced {
		t.Fatal("succeeded claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})

	addPendingClaim(t, r, "research-run-2")
	if err := r.BindClaim("research-run-2"); err != nil {
		t.Fatalf("pool should be claimable after succeeded replacement: %v", err)
	}
	next := assertClaimPhase(t, r, "research-run-2", v1alpha1.ClaimPhaseBound)
	if next.Status.SandboxID == "" {
		t.Fatal("replacement sandbox should bind to the next claim")
	}
	if next.Status.SandboxID == bound.Status.SandboxID {
		t.Fatal("succeeded sandbox should not be reused for the next claim")
	}
}

func TestReferencePoolLifecycle_failedClaimAutoReplacesSandboxAndLeavesPoolClaimable(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "fail-once")

	if err := r.BindClaim("fail-once"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	bound := assertClaimPhase(t, r, "fail-once", v1alpha1.ClaimPhaseBound)
	if err := r.StartClaim("fail-once"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := r.FailClaim("fail-once", "agent exited 1"); err != nil {
		t.Fatalf("fail claim: %v", err)
	}

	failed := assertClaimPhase(t, r, "fail-once", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "agent exited 1" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})

	addPendingClaim(t, r, "after-failure")
	if err := r.BindClaim("after-failure"); err != nil {
		t.Fatalf("pool should remain claimable after failed claim replacement: %v", err)
	}
	afterFailure := assertClaimPhase(t, r, "after-failure", v1alpha1.ClaimPhaseBound)
	if afterFailure.Status.SandboxID == "" {
		t.Fatal("replacement sandbox should bind to the next claim")
	}
	if afterFailure.Status.SandboxID == bound.Status.SandboxID {
		t.Fatal("failed sandbox should not be reused for the next claim")
	}
}

func TestReferencePoolLifecycle_boundClaimCanFailBeforeStartAndReplacesSandbox(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "lost-before-start")

	if err := r.BindClaim("lost-before-start"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}

	if err := r.FailClaim("lost-before-start", "sandbox lost before start"); err != nil {
		t.Fatalf("fail bound claim: %v", err)
	}

	failed := assertClaimPhase(t, r, "lost-before-start", v1alpha1.ClaimPhaseFailed)
	if failed.Status.Error != "sandbox lost before start" {
		t.Fatalf("expected failure summary, got %q", failed.Status.Error)
	}
	if !failed.Status.SandboxReplaced {
		t.Fatal("bound-failed claim should record the sandbox-replaced resource fact")
	}
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{
		IdleSandboxes:     1,
		ReplacedSandboxes: 1,
	})
}

func TestReferencePoolLifecycle_pendingClaimCanExpireWithoutBindingSandbox(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "expire-once")

	if err := r.ExpireClaim("expire-once", "ttl elapsed"); err != nil {
		t.Fatalf("expire claim: %v", err)
	}

	expired := assertClaimPhase(t, r, "expire-once", v1alpha1.ClaimPhaseExpired)
	if expired.Status.SandboxID != "" {
		t.Fatalf("expired pending claim should not bind a sandbox, got %q", expired.Status.SandboxID)
	}
	if expired.Status.SandboxReplaced {
		t.Fatal("expired pending claim never bound a sandbox, so none should be replaced")
	}
	assertPoolStatus(t, r, v1alpha1.SandboxWarmPoolStatus{IdleSandboxes: 1})
}

func TestReferencePoolLifecycle_duplicateClaimIsRejected(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "duplicate")

	err := r.AddClaim(runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: "duplicate"},
		Spec: runtime.BackendClaimSpec{
			PoolRef: testPool,
		},
	})
	if err == nil {
		t.Fatal("expected duplicate claim to be rejected")
	}
}

func TestReferencePoolLifecycle_invalidTransitionsRemainRejected(t *testing.T) {
	r := newConfiguredRuntime(t)

	addPendingClaim(t, r, "invalid")

	if err := r.StartClaim("invalid"); err == nil {
		t.Fatal("pending claim should not start before bind")
	}
	if err := r.SucceedClaim("invalid"); err == nil {
		t.Fatal("pending claim should not succeed")
	}
	if err := r.FailClaim("invalid", "no sandbox yet"); err == nil {
		t.Fatal("pending claim should not fail; it expires instead")
	}
	if err := r.BindClaim("invalid"); err != nil {
		t.Fatalf("bind claim: %v", err)
	}
	if err := r.SucceedClaim("invalid"); err == nil {
		t.Fatal("bound claim should not succeed before running")
	}
	if err := r.StartClaim("invalid"); err != nil {
		t.Fatalf("start claim: %v", err)
	}
	if err := r.ExpireClaim("invalid", "too late"); err == nil {
		t.Fatal("running claim should not expire through pending transition")
	}
	if err := r.SucceedClaim("invalid"); err != nil {
		t.Fatalf("succeed claim: %v", err)
	}
	if err := r.SucceedClaim("invalid"); err == nil {
		t.Fatal("terminal claim should not transition again")
	}
}

// --- helpers ---

func addPendingClaim(t *testing.T, r *Runtime, name string) {
	t.Helper()

	claim := runtime.BackendClaim{
		Metadata: v1alpha1.ObjectMeta{Name: name},
		Spec: runtime.BackendClaimSpec{
			PoolRef: testPool,
		},
	}
	if err := r.AddClaim(claim); err != nil {
		t.Fatalf("add claim: %v", err)
	}
}

func assertClaimPhase(t *testing.T, r *Runtime, name string, phase v1alpha1.ClaimPhase) runtime.BackendClaim {
	t.Helper()

	claim, ok := r.Claim(name)
	if !ok {
		t.Fatalf("claim not found: %s", name)
	}
	if claim.Status.Phase != phase {
		t.Fatalf("claim phase = %s, want %s", claim.Status.Phase, phase)
	}
	return claim
}

func assertPoolStatus(t *testing.T, r *Runtime, want v1alpha1.SandboxWarmPoolStatus) {
	t.Helper()

	got, ok := r.PoolStatus(testPool)
	if !ok {
		t.Fatalf("pool not found: %s", testPool)
	}
	if got != want {
		t.Fatalf("pool status = %+v, want %+v", got, want)
	}
}
