// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

type networkRecordingBackend struct {
	*recordingBackend
	service                  *RunService
	activations, revocations int
	mutate                   func(*runtime.NetworkEvidence)
	activationErr            error
	revocationErr            error
	activationHook           func(context.Context) error
	ignoreActivationContext  bool
}

func (b *networkRecordingBackend) ActivateNetwork(ctx context.Context, id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	b.activations++
	state := b.service.State(b.lastAllocate.ClaimID)
	if state.Claim.Phase != v1alpha1.ClaimPhaseRunning || !hasEvent(state, EventNetworkActivationAttempt) {
		return runtime.NetworkEvidence{}, errors.New("activation before durable Running/attempt")
	}
	if b.activationHook != nil {
		if err := b.activationHook(ctx); err != nil {
			return runtime.NetworkEvidence{}, err
		}
	}
	backendContext := ctx
	if b.ignoreActivationContext {
		backendContext = context.Background()
	}
	e, err := b.recordingBackend.ActivateNetwork(backendContext, id)
	if b.mutate != nil {
		b.mutate(&e)
	}
	return e, errors.Join(err, b.activationErr)
}

func (b *networkRecordingBackend) RevokeNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	b.revocations++
	e, err := b.recordingBackend.RevokeNetwork(id)
	return e, errors.Join(err, b.revocationErr)
}

func TestWorkNetworkLifecycleOrdersDurableGrantAndRevocation(t *testing.T) {
	b := &networkRecordingBackend{recordingBackend: newRecordingBackend()}
	s := newTestRunService(t, b, RunServiceOptions{})
	b.service = s
	issued := pendingIssuedState(time.Minute)
	state, err := s.Run(issued, testLaunch(issued), func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{EventBound, EventBackendReady, EventRunning, EventNetworkActivationAttempt, EventNetworkActive, EventSucceeded, EventNetworkRevocationAttempt, EventNetworkRevoked, EventTerminateOK, EventCleanupOK}
	if got := eventKinds(state); !reflect.DeepEqual(got, want) {
		t.Fatalf("network lifecycle %v want %v", got, want)
	}
	if b.activations != 1 || b.revocations != 1 || b.lastAllocate.Network.ExpiresAt.IsZero() || len(b.lastAllocate.Network.Profiles) != 0 {
		t.Fatalf("not default offline: %+v", b)
	}
}

func TestWorkNetworkMismatchedBackendEvidenceNeverDispatches(t *testing.T) {
	for _, name := range []string{"expanded-grant", "extended-expiry", "missing-level", "wrong-identity"} {
		t.Run(name, func(t *testing.T) {
			b := &networkRecordingBackend{recordingBackend: newRecordingBackend()}
			b.mutate = func(e *runtime.NetworkEvidence) {
				switch name {
				case "expanded-grant":
					e.Profiles = []string{"public-web"}
				case "extended-expiry":
					e.ExpiresAt = e.ExpiresAt.Add(time.Hour)
				case "missing-level":
					e.EvidenceLevel = ""
				case "wrong-identity":
					e.Identity.WorkerID = "other"
				}
			}
			s := newTestRunService(t, b, RunServiceOptions{})
			b.service = s
			issued := pendingIssuedState(time.Minute)
			called := false
			state, err := s.Run(issued, testLaunch(issued), func(context.Context) error { called = true; return nil })
			if err == nil || called || b.revocations != 1 || hasEvent(state, EventNetworkActive) || state.Claim.Phase != v1alpha1.ClaimPhaseFailed {
				t.Fatalf("mismatched evidence accepted: %v %+v", err, state)
			}
		})
	}
}

func TestWorkNetworkFailedAttemptWritePreventsGrantButNotRevocation(t *testing.T) {
	for _, event := range []string{EventNetworkActivationAttempt, EventNetworkRevocationAttempt} {
		t.Run(event, func(t *testing.T) {
			b := &networkRecordingBackend{recordingBackend: newRecordingBackend()}
			s := newTestRunService(t, b, RunServiceOptions{OnEvent: func(_ *v1alpha1.IssuedState, kind string) error {
				if kind == event {
					return errors.New("storage down")
				}
				return nil
			}})
			b.service = s
			issued := pendingIssuedState(time.Minute)
			_, err := s.Run(issued, testLaunch(issued), func(context.Context) error { return nil })
			if err == nil || b.revocations != 1 {
				t.Fatalf("safety side effect skipped: %v %+v", err, b)
			}
			if event == EventNetworkActivationAttempt && b.activations != 0 {
				t.Fatal("failed durable attempt permitted network grant")
			}
		})
	}
}

// Explicit old-only adapter: hiding NetworkBackend proves no implicit offline
// fallback occurs even when no public profile is requested.
type networkUnsupportedBackend struct{ b *recordingBackend }

func (b networkUnsupportedBackend) Allocate(r runtime.AllocateRequest) (runtime.Allocation, error) {
	return b.b.Allocate(r)
}
func (b networkUnsupportedBackend) Observe(id v1alpha1.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	return b.b.Observe(id)
}
func (b networkUnsupportedBackend) Start(id v1alpha1.SandboxClaimBackendIdentity) error {
	return b.b.Start(id)
}
func (b networkUnsupportedBackend) Terminate(id v1alpha1.SandboxClaimBackendIdentity) error {
	return b.b.Terminate(id)
}
func (b networkUnsupportedBackend) Cleanup(id v1alpha1.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	return b.b.Cleanup(id)
}

func TestWorkNetworkUnsupportedOfflineBackendDoesNotAllocate(t *testing.T) {
	b := newRecordingBackend()
	s := newTestRunService(t, networkUnsupportedBackend{b: b}, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	state, err := s.Run(issued, testLaunch(issued), func(context.Context) error { t.Fatal("unsupported work ran"); return nil })
	if !errors.Is(err, runtime.ErrUnsupported) || len(b.operations()) != 0 || state.Claim.BackendIdentity != nil {
		t.Fatalf("unsupported allocated: %+v %v", state, err)
	}
}

func TestWorkNetworkRechecksInterruptionAfterDurableWrites(t *testing.T) {
	for _, event := range []string{EventRunning, EventNetworkActivationAttempt, EventNetworkActive} {
		for _, interruption := range []string{"cancel", "runtime-deadline", "caller-deadline"} {
			t.Run(event+"/"+interruption, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if interruption == "caller-deadline" {
					var timeoutCancel context.CancelFunc
					ctx, timeoutCancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer timeoutCancel()
				}
				now := time.Now()
				initialNow := now
				triggered := false
				b := &networkRecordingBackend{recordingBackend: newRecordingBackend()}
				s := newTestRunService(t, b, RunServiceOptions{
					Now: func() time.Time { return now },
					OnEvent: func(_ *v1alpha1.IssuedState, kind string) error {
						if kind != event {
							return nil
						}
						triggered = true
						switch interruption {
						case "cancel":
							cancel()
						case "runtime-deadline":
							now = initialNow.Add(2 * time.Minute)
						case "caller-deadline":
							<-ctx.Done()
						}
						return nil // write eventually succeeds AFTER eligibility changed
					},
				})
				b.service = s
				issued := pendingIssuedState(time.Minute)
				issued.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
				workCalls := 0
				state, err := s.RunContext(ctx, issued, testLaunch(issued), func(context.Context) error { workCalls++; return nil })
				wantPhase, wantEvent, wantErr := v1alpha1.ClaimPhaseFailed, "Cancelled", context.Canceled
				if interruption != "cancel" {
					wantPhase, wantEvent, wantErr = v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline
					if interruption == "caller-deadline" {
						wantErr = context.DeadlineExceeded
					}
				}
				if !triggered || !errors.Is(err, wantErr) || state.Claim.Phase != wantPhase || !hasEvent(state, wantEvent) || workCalls != 0 {
					t.Fatalf("persisted interruption dispatched work or lost outcome: triggered=%v err=%v phase=%s work=%d events=%v", triggered, err, state.Claim.Phase, workCalls, eventKinds(state))
				}
				wantActivations := 0
				if event == EventNetworkActive {
					wantActivations = 1
				}
				if b.activations != wantActivations || b.revocations != 1 || !hasEvent(state, EventNetworkRevoked) || !hasEvent(state, EventCleanupOK) {
					t.Fatalf("interrupted grant/release incorrect: activations=%d want=%d revoke=%d events=%v", b.activations, wantActivations, b.revocations, eventKinds(state))
				}
				if wantActivations == 0 && hasEvent(state, EventNetworkActive) {
					t.Fatal("pre-grant interruption transiently activated networking")
				}
			})
		}
	}
}

func TestWorkNetworkCancellationDuringActivationNeverDispatchesAndAlwaysReleases(t *testing.T) {
	for _, interruption := range []string{"cancel", "caller-deadline", "runtime-deadline"} {
		for _, backendResult := range []string{"context-error", "ambiguous-error", "success-after-interruption"} {
			t.Run(interruption+"/"+backendResult, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				runtimeTimeout := time.Minute
				if interruption == "caller-deadline" {
					var deadlineCancel context.CancelFunc
					ctx, deadlineCancel = context.WithTimeout(context.Background(), 150*time.Millisecond)
					defer deadlineCancel()
				}
				if interruption == "runtime-deadline" {
					runtimeTimeout = 150 * time.Millisecond
				}
				type contextMarker struct{}
				ctx = context.WithValue(ctx, contextMarker{}, "caller-context")
				entered := make(chan struct{})
				b := &networkRecordingBackend{recordingBackend: newRecordingBackend(), ignoreActivationContext: backendResult == "success-after-interruption"}
				b.activationHook = func(activationCtx context.Context) error {
					if activationCtx.Value(contextMarker{}) != "caller-context" {
						t.Error("caller context values lost")
					}
					limit, ok := activationCtx.Deadline()
					if !ok || limit.After(b.lastAllocate.Network.ExpiresAt.Add(time.Millisecond)) {
						t.Error("activation is not bounded by the Work expiry")
					}
					close(entered)
					<-activationCtx.Done() // Side effect is genuinely blocked when interrupted.
					switch backendResult {
					case "context-error":
						return activationCtx.Err()
					case "ambiguous-error":
						return errors.New("backend result ambiguous after interruption")
					default:
						return nil // Even a backend returning success must not start Work.
					}
				}
				s := newTestRunService(t, b, RunServiceOptions{Now: time.Now})
				b.service = s
				issued := pendingIssuedState(runtimeTimeout)
				issued.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
				cancelledAfterEntry := make(chan struct{})
				if interruption == "cancel" {
					go func() { <-entered; cancel(); close(cancelledAfterEntry) }()
				} else {
					close(cancelledAfterEntry)
				}
				workCalls := 0
				state, err := s.RunContext(ctx, issued, testLaunch(issued), func(context.Context) error { workCalls++; return nil })
				<-cancelledAfterEntry
				wantPhase, wantEvent, wantErr := v1alpha1.ClaimPhaseFailed, "Cancelled", context.Canceled
				if interruption != "cancel" {
					wantPhase, wantEvent, wantErr = v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline
					if interruption == "caller-deadline" {
						wantErr = context.DeadlineExceeded
					}
				}
				// The equal effective deadline can be observed by the Work timer
				// just before the caller timer publishes Err; both are expiry,
				// never cancellation or a generic activation failure.
				validError := errors.Is(err, wantErr) || interruption == "caller-deadline" && errors.Is(err, ErrRunDeadline)
				if !validError || state.Claim.Phase != wantPhase || !hasEvent(state, wantEvent) || workCalls != 0 || b.activations != 1 || b.revocations != 1 {
					t.Fatalf("activation interruption lost: err=%v phase=%s work=%d activations=%d revocations=%d events=%v", err, state.Claim.Phase, workCalls, b.activations, b.revocations, eventKinds(state))
				}
				if hasEvent(state, EventNetworkActive) || hasEvent(state, EventNetworkActivationFailed) || !hasEvent(state, EventNetworkRevoked) || !hasEvent(state, EventCleanupOK) {
					t.Fatalf("interruption fabricated a grant/failure or skipped release: %v", eventKinds(state))
				}
			})
		}
	}
}

// Simulate the real runtime's context-lifetime enforcement without manufacturing
// packet evidence or racing the reference backend's state maps. Only the
// runtime-owned lease flag is touched by the watcher; journaling stays in Run.
type lifetimeNetworkBackend struct {
	*recordingBackend
	mu        sync.Mutex
	stop      chan struct{}
	revoked   chan struct{}
	watchDone chan struct{}
	once      sync.Once
}

func (b *lifetimeNetworkBackend) ActivateNetwork(ctx context.Context, id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	e, err := b.recordingBackend.ActivateNetwork(ctx, id)
	if err != nil {
		return e, err
	}
	b.mu.Lock()
	b.stop = make(chan struct{})
	stop := b.stop
	b.mu.Unlock()
	go func() {
		defer close(b.watchDone)
		select {
		case <-ctx.Done():
			// Runtime performs the safety action independently of the app/DB
			// lock. Authoritative evidence is still recorded by normal teardown.
			close(b.revoked)
		case <-stop:
		}
	}()
	return e, nil
}

func (b *lifetimeNetworkBackend) RevokeNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	b.mu.Lock()
	stop := b.stop
	b.mu.Unlock()
	if stop != nil {
		b.once.Do(func() { close(stop) })
	}
	return b.recordingBackend.RevokeNetwork(id)
}

func TestWorkNetworkCancellationRevokesBeforeBlockedActiveWriteReturns(t *testing.T) {
	for _, writeErr := range []bool{false, true} {
		t.Run(map[bool]string{false: "write-success", true: "write-failure"}[writeErr], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered, unblock := make(chan struct{}), make(chan struct{})
			var releaseWrite sync.Once
			defer releaseWrite.Do(func() { close(unblock) })
			b := &lifetimeNetworkBackend{recordingBackend: newRecordingBackend(), revoked: make(chan struct{}), watchDone: make(chan struct{})}
			s := newTestRunService(t, b, RunServiceOptions{Now: time.Now, OnEvent: func(_ *v1alpha1.IssuedState, event string) error {
				if event == EventNetworkActive {
					close(entered)
					<-unblock
					if writeErr {
						return errors.New("blocked persistence eventually failed")
					}
				}
				return nil
			}})
			issued := pendingIssuedState(time.Minute)
			issued.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
			type result struct {
				state *v1alpha1.IssuedState
				err   error
				work  int
			}
			done := make(chan result, 1)
			go func() {
				calls := 0
				state, err := s.RunContext(ctx, issued, testLaunch(issued), func(context.Context) error { calls++; return nil })
				done <- result{state, err, calls}
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("Active persistence was not reached")
			}
			// Activation must retain the context: successful return alone must
			// not cancel a lease that is still eligible to run.
			select {
			case <-b.revoked:
				t.Fatal("activation context ended before cancellation")
			default:
			}
			cancel()
			select {
			case <-b.revoked:
			case <-time.After(2 * time.Second):
				t.Fatal("runtime safety action waited for blocked persistence")
			}
			select {
			case <-done:
				t.Fatal("blocked write unexpectedly returned")
			default:
			}
			releaseWrite.Do(func() { close(unblock) })
			select {
			case got := <-done:
				if !errors.Is(got.err, context.Canceled) || got.state.Claim.Phase != v1alpha1.ClaimPhaseFailed || !hasEvent(got.state, "Cancelled") || hasEvent(got.state, EventNetworkActivationFailed) || !hasEvent(got.state, EventNetworkRevoked) || !hasEvent(got.state, EventCleanupOK) || got.work != 0 {
					t.Fatalf("cancelled persistence lost lifecycle: err=%v work=%d events=%v", got.err, got.work, eventKinds(got.state))
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled Work did not finish after releasing persistence")
			}
			<-b.watchDone
		})
	}
}
