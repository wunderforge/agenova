// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"reflect"
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
}

func (b *networkRecordingBackend) ActivateNetwork(id v1alpha1.SandboxClaimBackendIdentity) (runtime.NetworkEvidence, error) {
	b.activations++
	state := b.service.State(b.lastAllocate.ClaimID)
	if state.Claim.Phase != v1alpha1.ClaimPhaseRunning || !hasEvent(state, EventNetworkActivationAttempt) {
		return runtime.NetworkEvidence{}, errors.New("activation before durable Running/attempt")
	}
	e, err := b.recordingBackend.ActivateNetwork(id)
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
