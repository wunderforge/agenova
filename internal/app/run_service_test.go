// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/operator"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestRunServiceSuccessOwnsLifecycleAndTeardownEvidence(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	launch := testLaunch(issued)
	workCalls := 0
	backend.terminateHook = func() {
		claim, ok := service.Claim(issued.Claim.ID)
		if !ok || claim.Phase != v1alpha1.ClaimPhaseSucceeded {
			t.Fatalf("claim at termination = %+v, %v; terminal outcome must be published first", claim, ok)
		}
	}

	result, err := service.Run(issued, launch, func() error {
		workCalls++
		claim, ok := service.Claim(issued.Claim.ID)
		if !ok || claim.Phase != v1alpha1.ClaimPhaseRunning {
			t.Fatalf("claim during work = %+v, %v; want Running", claim, ok)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if workCalls != 1 {
		t.Fatalf("work calls = %d, want 1", workCalls)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseSucceeded)
	assertEvents(t, result,
		EventBound, EventBackendReady, EventRunning, EventSucceeded,
		EventTerminateOK, EventCleanupOK,
	)
	assertTrace(t, backend, "allocate", "observe", "start", "terminate", "cleanup")

	// The caller's input and returned snapshots never alias authoritative state.
	issued.Claim.Phase = v1alpha1.ClaimPhaseFailed
	result.Claim.Phase = v1alpha1.ClaimPhaseFailed
	result.Claim.BackendIdentity.WorkerID = "mutated"
	result.Evidence.RuntimeEvents[0].Kind = "mutated"
	claim, ok := service.Claim(issued.Claim.ID)
	if !ok || claim.Phase != v1alpha1.ClaimPhaseSucceeded || claim.BackendIdentity.WorkerID != "worker-1" {
		t.Fatalf("authoritative claim was mutated: %+v, %v", claim, ok)
	}
	stored := service.current(claim.ID)
	assertEvents(t, stored,
		EventBound, EventBackendReady, EventRunning, EventSucceeded,
		EventTerminateOK, EventCleanupOK,
	)
}

func TestRunServiceClaimAuthorityReturnsCorrelatedDefensiveSnapshot(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	issued.EffectiveAuthority.Tools = []string{"git.read"}
	issued.EffectiveAuthority.ResourceScopes = []string{"repo:acme/payments"}
	if _, err := service.Run(issued, testLaunch(issued), func() error { return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}

	snapshot, ok := service.ClaimAuthority(issued.Claim.ID)
	if !ok {
		t.Fatal("ClaimAuthority did not return the stored issued snapshot")
	}
	if snapshot.Claim.AuthorityRef != snapshot.EffectiveAuthority.ID || snapshot.EffectiveAuthority.ID == "" {
		t.Fatalf("claim/authority correlation = claim %q authority %q", snapshot.Claim.AuthorityRef, snapshot.EffectiveAuthority.ID)
	}

	wantTool := snapshot.EffectiveAuthority.Tools[0]
	wantScope := snapshot.EffectiveAuthority.ResourceScopes[0]
	snapshot.Claim.AuthorityRef = "mutated"
	snapshot.EffectiveAuthority.ID = "mutated"
	snapshot.EffectiveAuthority.Tools[0] = "mutated"
	snapshot.EffectiveAuthority.ResourceScopes[0] = "mutated"

	again, ok := service.ClaimAuthority(issued.Claim.ID)
	if !ok || again.Claim.AuthorityRef != issued.Claim.AuthorityRef || again.EffectiveAuthority.ID != issued.EffectiveAuthority.ID {
		t.Fatalf("authoritative correlation was mutated: %+v, %v", again, ok)
	}
	if again.EffectiveAuthority.Tools[0] != wantTool || again.EffectiveAuthority.ResourceScopes[0] != wantScope {
		t.Fatalf("authoritative grant slices were mutated: %+v", again.EffectiveAuthority)
	}
}

func TestRunServiceAllocationFailureUsesNarrowPendingToFailedEdge(t *testing.T) {
	backend := newRecordingBackend()
	backend.allocateErr = errors.New("capacity exhausted")
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	workCalled := false

	result, err := service.Run(issued, testLaunch(issued), func() error {
		workCalled = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "capacity exhausted") {
		t.Fatalf("Run error = %v, want allocation cause", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
	if result.Claim.BackendIdentity != nil {
		t.Fatalf("allocation failure fabricated identity: %+v", result.Claim.BackendIdentity)
	}
	assertEvents(t, result, EventAllocationFailed)
	assertTrace(t, backend, "allocate")
	if workCalled {
		t.Fatal("work ran after allocation failure")
	}
}

func TestRunServiceStartFailureNeverPublishesRunningAndStillTearsDown(t *testing.T) {
	backend := newRecordingBackend()
	backend.startErr = errors.New("start refused")
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	workCalled := false

	result, err := service.Run(issued, testLaunch(issued), func() error {
		workCalled = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "start refused") {
		t.Fatalf("Run error = %v, want start cause", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
	assertEvents(t, result,
		EventBound, EventBackendReady, EventStartFailed,
		EventTerminateOK, EventCleanupOK,
	)
	assertTrace(t, backend, "allocate", "observe", "start", "terminate", "cleanup")
	if workCalled || hasEvent(result, EventRunning) {
		t.Fatal("start failure published Running or invoked work")
	}
}

func TestRunServiceWaitsForReadinessBeforeStart(t *testing.T) {
	backend := newRecordingBackend()
	backend.observation.Ready = false
	waits := 0
	service := newTestRunService(t, backend, RunServiceOptions{
		Wait: func(time.Duration) {
			waits++
			backend.observation.Ready = true
		},
	})
	issued := pendingIssuedState(time.Minute)

	result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseSucceeded)
	if waits != 1 {
		t.Fatalf("readiness waits = %d, want 1", waits)
	}
	assertTrace(t, backend, "allocate", "observe", "observe", "start", "terminate", "cleanup")
}

func TestRunServiceDrivesReferenceBackend(t *testing.T) {
	backend := operator.NewRuntime()
	if err := backend.AddTemplate(v1alpha1.AgentSandboxTemplate{
		Metadata: v1alpha1.ObjectMeta{Name: "engineer"},
		Spec:     v1alpha1.AgentSandboxTemplateSpec{Image: "example.local/agenova/engineer:test"},
	}); err != nil {
		t.Fatalf("AddTemplate: %v", err)
	}
	if err := backend.AddWarmPool(v1alpha1.SandboxWarmPool{
		Metadata: v1alpha1.ObjectMeta{Name: "engineer-pool"},
		Spec: v1alpha1.SandboxWarmPoolSpec{
			TemplateRef: "engineer",
			Replicas:    1,
		},
	}); err != nil {
		t.Fatalf("AddWarmPool: %v", err)
	}
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)

	result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseSucceeded)
	observation, err := backend.Observe(*result.Claim.BackendIdentity)
	if err != nil {
		t.Fatalf("Observe after run: %v", err)
	}
	if !observation.Released || !observation.Replaced || observation.Ready {
		t.Fatalf("reference cleanup evidence = %+v, want released/replaced/not ready", observation)
	}
}

func TestRunServiceAcceptsResolvedRuntimeTemplateDistinctFromAgentTemplate(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	launch := testLaunch(issued)
	launch.TemplateRef = "runtime-engineer-v2"

	result, err := service.Run(issued, launch, func() error { return nil })
	if err != nil {
		t.Fatalf("Run with separately resolved runtime template: %v", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseSucceeded)
	if backend.lastAllocate.TemplateRef != "runtime-engineer-v2" {
		t.Fatalf("backend template = %q, want runtime-engineer-v2", backend.lastAllocate.TemplateRef)
	}
}

func TestRunServiceRejectsLaunchForDifferentRuntimeProfile(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	launch := testLaunch(issued)
	launch.ProfileRef = "ungranted-profile"

	result, err := service.Run(issued, launch, func() error { return nil })
	if !errors.Is(err, ErrInvalidRun) || result != nil {
		t.Fatalf("Run = (%+v, %v), want pre-allocation ErrInvalidRun", result, err)
	}
	assertTrace(t, backend)
}

func TestRunServiceRejectsCredentialBearingLaunchInputBeforeAllocation(t *testing.T) {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "NPM_TOKEN", "GITLAB_TOKEN", "SSH_PRIVATE_KEY", "_auth", "_authToken", "client_secret", "AZURE_CLIENT_SECRET"} {
		t.Run(key, func(t *testing.T) {
			backend := newRecordingBackend()
			service := newTestRunService(t, backend, RunServiceOptions{})
			issued := pendingIssuedState(time.Minute)
			launch := testLaunch(issued)
			launch.Input[key] = "not-inspected"

			result, err := service.Run(issued, launch, func() error { return nil })
			if !errors.Is(err, ErrInvalidRun) || result != nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("Run = (%+v, %v), want pre-allocation credential rejection", result, err)
			}
			assertTrace(t, backend)
		})
	}
}

func TestRunServiceWorkFailurePublishesFailedBeforeTeardown(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)

	result, err := service.Run(issued, testLaunch(issued), func() error {
		return errors.New("agent failed")
	})
	if err == nil || !strings.Contains(err.Error(), "agent failed") {
		t.Fatalf("Run error = %v, want work cause", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
	assertEvents(t, result,
		EventBound, EventBackendReady, EventRunning, EventFailed,
		EventTerminateOK, EventCleanupOK,
	)
}

func TestRunServiceDeadlineAtEachNonTerminalPhase(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	t.Run("Pending before allocation", func(t *testing.T) {
		backend := newRecordingBackend()
		calls := 0
		now := func() time.Time {
			calls++
			if calls == 1 {
				return base
			}
			return base.Add(time.Minute)
		}
		service := newTestRunService(t, backend, RunServiceOptions{Now: now})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrRunDeadline) {
			t.Fatalf("Run error = %v, want ErrRunDeadline", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseExpired)
		assertEvents(t, result, EventExpired)
		assertTrace(t, backend)
	})

	t.Run("Pending after late allocation", func(t *testing.T) {
		backend := newRecordingBackend()
		clock := &manualClock{now: base}
		backend.allocateHook = func() { clock.Advance(2 * time.Minute) }
		service := newTestRunService(t, backend, RunServiceOptions{Now: clock.Now})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrRunDeadline) {
			t.Fatalf("Run error = %v, want ErrRunDeadline", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseExpired)
		if result.Claim.BackendIdentity != nil || hasEvent(result, EventBound) {
			t.Fatalf("late allocation advanced Pending claim: %+v", result.Claim)
		}
		assertEvents(t, result, EventExpired, EventTerminateOK, EventCleanupOK)
		assertTrace(t, backend, "allocate", "terminate", "cleanup")
	})

	t.Run("Pending after late allocation failure", func(t *testing.T) {
		backend := newRecordingBackend()
		clock := &manualClock{now: base}
		backend.allocateHook = func() { clock.Advance(2 * time.Minute) }
		backend.allocateErr = errors.New("late allocation failure")
		service := newTestRunService(t, backend, RunServiceOptions{Now: clock.Now})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrRunDeadline) || strings.Contains(err.Error(), "late allocation failure") {
			t.Fatalf("Run error = %v, want deadline to win over late backend failure", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseExpired)
		assertEvents(t, result, EventExpired)
		assertTrace(t, backend, "allocate")
	})

	t.Run("Bound after observation", func(t *testing.T) {
		backend := newRecordingBackend()
		clock := &manualClock{now: base}
		backend.observeHook = func() { clock.Advance(2 * time.Minute) }
		backend.observeErr = errors.New("late observation failure")
		service := newTestRunService(t, backend, RunServiceOptions{Now: clock.Now})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrRunDeadline) {
			t.Fatalf("Run error = %v, want ErrRunDeadline", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseExpired)
		if hasEvent(result, EventRunning) {
			t.Fatal("late observation published Running")
		}
		assertTrace(t, backend, "allocate", "observe", "terminate", "cleanup")
	})

	t.Run("Running after work", func(t *testing.T) {
		backend := newRecordingBackend()
		clock := &manualClock{now: base}
		service := newTestRunService(t, backend, RunServiceOptions{Now: clock.Now})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error {
			clock.Advance(2 * time.Minute)
			return nil
		})
		if !errors.Is(err, ErrRunDeadline) {
			t.Fatalf("Run error = %v, want ErrRunDeadline", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseExpired)
		if hasEvent(result, EventSucceeded) {
			t.Fatal("late work result rewrote Expired as Succeeded")
		}
		assertTrace(t, backend, "allocate", "observe", "start", "terminate", "cleanup")
	})
}

func TestRunServiceExpiresWhileWorkCallbackIsStillExecuting(t *testing.T) {
	backend := newRecordingBackend()
	deadline := make(chan time.Time, 1)
	service := newTestRunService(t, backend, RunServiceOptions{
		After: func(time.Duration) <-chan time.Time { return deadline },
	})
	issued := pendingIssuedState(time.Minute)
	workStarted := make(chan struct{})
	releaseWork := make(chan struct{})
	workFinished := make(chan struct{})
	resultDone := make(chan struct {
		state *v1alpha1.IssuedState
		err   error
	}, 1)

	go func() {
		state, err := service.Run(issued, testLaunch(issued), func() error {
			close(workStarted)
			<-releaseWork
			close(workFinished)
			return nil
		})
		resultDone <- struct {
			state *v1alpha1.IssuedState
			err   error
		}{state: state, err: err}
	}()
	<-workStarted
	deadline <- time.Unix(1_700_000_060, 0)
	result := <-resultDone
	if !errors.Is(result.err, ErrRunDeadline) {
		t.Fatalf("Run error = %v, want ErrRunDeadline", result.err)
	}
	assertPhase(t, result.state, v1alpha1.ClaimPhaseExpired)
	assertTrace(t, backend, "allocate", "observe", "start", "terminate", "cleanup")

	// The callback is still blocked, but expiry has released the serialized
	// run path so another independent claim can proceed.
	second := pendingIssuedStateNamed("after-expiry", time.Minute)
	secondIdentity := v1alpha1.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "worker-2"}
	backend.allocation = runtime.Allocation{ClaimID: second.Claim.ID, Identity: secondIdentity}
	backend.observation = runtime.Observation{ClaimID: second.Claim.ID, Identity: secondIdentity, Ready: true}
	backend.cleanup = runtime.CleanupResult{Identity: secondIdentity, Released: true}
	secondResult, err := service.Run(second, testLaunch(second), func() error { return nil })
	if err != nil {
		t.Fatalf("second Run while first callback remains blocked: %v", err)
	}
	assertPhase(t, secondResult, v1alpha1.ClaimPhaseSucceeded)

	// Expiry does not claim callback cancellation. Its eventual result is
	// ignored and cannot rewrite the already terminal claim.
	close(releaseWork)
	<-workFinished
	claim, ok := service.Claim(issued.Claim.ID)
	if !ok || claim.Phase != v1alpha1.ClaimPhaseExpired {
		t.Fatalf("claim after late work result = %+v, %v; want Expired", claim, ok)
	}
}

func TestRunServiceRejectsBackendIdentityAlreadyOwnedByAnotherClaim(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	first := pendingIssuedState(time.Minute)
	if _, err := service.Run(first, testLaunch(first), func() error { return nil }); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	second := pendingIssuedStateNamed("second", time.Minute)
	backend.allocation.ClaimID = second.Claim.ID
	backend.observation.ClaimID = second.Claim.ID
	result, err := service.Run(second, testLaunch(second), func() error { return nil })
	if !errors.Is(err, ErrInvalidRun) {
		t.Fatalf("second Run error = %v, want ErrInvalidRun", err)
	}
	assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
	if result.Claim.BackendIdentity != nil {
		t.Fatal("identity already owned by first claim was attached to second")
	}
	assertTrace(t, backend, "allocate", "observe", "start", "terminate", "cleanup", "allocate")
}

func TestRunServiceTeardownFailuresNeverRewriteOutcome(t *testing.T) {
	tests := []struct {
		name          string
		configure     func(*recordingBackend)
		expectedEvent string
	}{
		{
			name: "terminate",
			configure: func(backend *recordingBackend) {
				backend.terminateErr = errors.New("terminate failed")
			},
			expectedEvent: EventTerminateFailed,
		},
		{
			name: "cleanup",
			configure: func(backend *recordingBackend) {
				backend.cleanupErr = errors.New("cleanup failed")
			},
			expectedEvent: EventCleanupFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := newRecordingBackend()
			test.configure(backend)
			service := newTestRunService(t, backend, RunServiceOptions{})
			issued := pendingIssuedState(time.Minute)

			result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
			if err == nil {
				t.Fatal("Run should retain teardown failure")
			}
			assertPhase(t, result, v1alpha1.ClaimPhaseSucceeded)
			if !hasEvent(result, test.expectedEvent) {
				t.Fatalf("events = %v, want %s", eventKinds(result), test.expectedEvent)
			}
		})
	}
}

func TestRunServiceRejectsMismatchedBackendEvidence(t *testing.T) {
	t.Run("allocation", func(t *testing.T) {
		backend := newRecordingBackend()
		backend.allocation.ClaimID = "claim:other"
		service := newTestRunService(t, backend, RunServiceOptions{})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrInvalidRun) {
			t.Fatalf("Run error = %v, want ErrInvalidRun", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
		if result.Claim.BackendIdentity != nil {
			t.Fatal("mismatched allocation identity was attached")
		}
		assertTrace(t, backend, "allocate")
	})

	t.Run("observation", func(t *testing.T) {
		backend := newRecordingBackend()
		backend.observation.ClaimID = "claim:other"
		service := newTestRunService(t, backend, RunServiceOptions{})
		issued := pendingIssuedState(time.Minute)

		result, err := service.Run(issued, testLaunch(issued), func() error { return nil })
		if !errors.Is(err, ErrInvalidRun) {
			t.Fatalf("Run error = %v, want ErrInvalidRun", err)
		}
		assertPhase(t, result, v1alpha1.ClaimPhaseFailed)
		assertTrace(t, backend, "allocate", "observe", "terminate", "cleanup")
	})
}

func TestRunServiceRejectsInvalidAndLateTransitions(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)

	if _, err := service.Run(issued, testLaunch(issued), func() error { return nil }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	before := service.current(issued.Claim.ID)
	if err := service.transition(issued.Claim.ID, v1alpha1.ClaimPhaseFailed, EventFailed, nil); !errors.Is(err, ErrInvalidRun) {
		t.Fatalf("late transition error = %v, want ErrInvalidRun", err)
	}
	after := service.current(issued.Claim.ID)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("late transition changed state:\nbefore=%+v\nafter=%+v", before, after)
	}

	other := pendingIssuedState(time.Minute)
	other.Claim.ID = "claim:other"
	other.Evidence.ClaimID = other.Claim.ID
	if err := service.insert(other); err != nil {
		t.Fatalf("insert second claim: %v", err)
	}
	if err := service.transition(other.Claim.ID, v1alpha1.ClaimPhaseRunning, EventRunning, nil); !errors.Is(err, ErrInvalidRun) {
		t.Fatalf("Pending -> Running error = %v, want ErrInvalidRun", err)
	}
}

func TestRunServiceReaderIsSafeDuringRunningWork(t *testing.T) {
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	workEntered := make(chan struct{})
	releaseWork := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := service.Run(issued, testLaunch(issued), func() error {
			close(workEntered)
			<-releaseWork
			return nil
		})
		done <- err
	}()
	<-workEntered
	for i := 0; i < 100; i++ {
		claim, ok := service.Claim(issued.Claim.ID)
		if !ok || claim.Phase != v1alpha1.ClaimPhaseRunning {
			t.Fatalf("concurrent claim read = %+v, %v; want Running", claim, ok)
		}
		claim.Phase = v1alpha1.ClaimPhaseFailed
	}
	close(releaseWork)
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	claim, ok := service.Claim(issued.Claim.ID)
	if !ok || claim.Phase != v1alpha1.ClaimPhaseSucceeded {
		t.Fatalf("final claim = %+v, %v; want Succeeded", claim, ok)
	}
}

type recordingBackend struct {
	mu sync.Mutex

	trace        []string
	lastAllocate runtime.AllocateRequest
	allocation   runtime.Allocation
	observation  runtime.Observation
	cleanup      runtime.CleanupResult

	allocateErr  error
	observeErr   error
	startErr     error
	terminateErr error
	cleanupErr   error

	allocateHook  func()
	observeHook   func()
	terminateHook func()
}

func newRecordingBackend() *recordingBackend {
	identity := v1alpha1.SandboxClaimBackendIdentity{Backend: "test", WorkerID: "worker-1"}
	return &recordingBackend{
		allocation: runtime.Allocation{ClaimID: "claim:run:1", Identity: identity},
		observation: runtime.Observation{
			ClaimID:  "claim:run:1",
			Identity: identity,
			Ready:    true,
		},
		cleanup: runtime.CleanupResult{Identity: identity, Released: true},
	}
}

func (b *recordingBackend) Allocate(req runtime.AllocateRequest) (runtime.Allocation, error) {
	b.record("allocate")
	b.lastAllocate = req
	if b.allocateHook != nil {
		b.allocateHook()
	}
	return b.allocation, b.allocateErr
}

func (b *recordingBackend) Observe(v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	b.record("observe")
	if b.observeHook != nil {
		b.observeHook()
	}
	return b.observation, b.observeErr
}

func (b *recordingBackend) Start(v1alpha1.SandboxClaimBackendIdentity) error {
	b.record("start")
	return b.startErr
}

func (b *recordingBackend) Terminate(v1alpha1.SandboxClaimBackendIdentity) error {
	b.record("terminate")
	if b.terminateHook != nil {
		b.terminateHook()
	}
	return b.terminateErr
}

func (b *recordingBackend) Cleanup(v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	b.record("cleanup")
	return b.cleanup, b.cleanupErr
}

func (b *recordingBackend) record(operation string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trace = append(b.trace, operation)
}

func (b *recordingBackend) operations() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.trace...)
}

type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func newTestRunService(t *testing.T, backend runtime.RuntimeBackend, options RunServiceOptions) *RunService {
	t.Helper()
	if options.Now == nil {
		options.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	}
	if options.Wait == nil {
		options.Wait = func(time.Duration) {}
	}
	service, err := NewRunService(backend, options)
	if err != nil {
		t.Fatalf("NewRunService: %v", err)
	}
	return service
}

func pendingIssuedState(timeout time.Duration) *v1alpha1.IssuedState {
	policyRef := v1alpha1.PolicyReference{ID: "policy", Version: "1"}
	return &v1alpha1.IssuedState{
		RequestRef: "run",
		Principal: v1alpha1.Principal{
			Subject:               "user:team-a-engineer",
			Team:                  "team-a",
			AuthenticationContext: "upstream:test",
		},
		Action: v1alpha1.Action{
			Name:        "claim.create",
			Project:     "payments",
			TemplateRef: "engineer",
		},
		PolicyRef: policyRef,
		EffectiveAuthority: &v1alpha1.EffectiveAuthority{
			ID: "authority:run:1",
			Runtime: v1alpha1.EffectiveAuthorityRuntime{
				ProfileRef: "standard-isolated",
				Timeout:    v1alpha1.Duration(timeout),
			},
		},
		Claim: &v1alpha1.SandboxClaim{
			ID:           "claim:run:1",
			RequestRef:   "run",
			TemplateRef:  "engineer",
			AuthorityRef: "authority:run:1",
			Phase:        v1alpha1.ClaimPhasePending,
		},
		Decision: v1alpha1.Decision{
			ID:           "decision:run:1",
			PrincipalRef: "user:team-a-engineer",
			Action:       "claim.create",
			Result:       v1alpha1.DecisionResultAllow,
			PolicyRef:    policyRef,
			Reason:       "allowed by test policy",
		},
		Evidence: v1alpha1.Evidence{
			RequestRef:       "run",
			ClaimID:          "claim:run:1",
			DecisionIDs:      []string{"decision:run:1"},
			RuntimeEvents:    []v1alpha1.EvidenceRuntimeEvent{},
			ToolInvocations:  []v1alpha1.EvidenceToolInvocation{},
			ModelInvocations: []v1alpha1.EvidenceModelInvocation{},
		},
	}
}

func pendingIssuedStateNamed(name string, timeout time.Duration) *v1alpha1.IssuedState {
	state := pendingIssuedState(timeout)
	state.RequestRef = name
	state.Action.Project = name
	state.EffectiveAuthority.ID = "authority:" + name
	state.Claim.ID = "claim:" + name
	state.Claim.RequestRef = name
	state.Claim.AuthorityRef = state.EffectiveAuthority.ID
	state.Decision.ID = "decision:" + name
	state.Evidence.RequestRef = name
	state.Evidence.ClaimID = state.Claim.ID
	state.Evidence.DecisionIDs = []string{state.Decision.ID}
	return state
}

func testLaunch(issued *v1alpha1.IssuedState) ResolvedLaunch {
	return ResolvedLaunch{
		ProfileRef:  issued.EffectiveAuthority.Runtime.ProfileRef,
		TemplateRef: issued.Claim.TemplateRef,
		Input:       map[string]string{"objective": "run the test"},
	}
}

func assertPhase(t *testing.T, state *v1alpha1.IssuedState, want v1alpha1.ClaimPhase) {
	t.Helper()
	if state == nil || state.Claim == nil || state.Claim.Phase != want {
		t.Fatalf("claim = %+v, want phase %s", state, want)
	}
}

func assertEvents(t *testing.T, state *v1alpha1.IssuedState, want ...string) {
	t.Helper()
	if got := eventKinds(state); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func eventKinds(state *v1alpha1.IssuedState) []string {
	result := make([]string, 0, len(state.Evidence.RuntimeEvents))
	for _, event := range state.Evidence.RuntimeEvents {
		result = append(result, event.Kind)
	}
	return result
}

func hasEvent(state *v1alpha1.IssuedState, kind string) bool {
	for _, event := range state.Evidence.RuntimeEvents {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func assertTrace(t *testing.T, backend *recordingBackend, want ...string) {
	t.Helper()
	if got := backend.operations(); !reflect.DeepEqual(got, want) {
		t.Fatalf("backend trace = %v, want %v", got, want)
	}
}
