// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"testing"
	"time"
)

func TestRunServiceEvidenceFailureNeverLeavesActiveAuthorityOrLeaksWorker(t *testing.T) {
	for _, event := range []string{EventBound, EventBackendReady, EventRunning, EventSucceeded, EventFailed} {
		t.Run(event, func(t *testing.T) {
			backend := newRecordingBackend()
			sinkErr := errors.New("evidence unavailable")
			service := newTestRunService(t, backend, RunServiceOptions{OnEvent: func(state *v0.IssuedState, observed string) error {
				if observed == event {
					return sinkErr
				}
				return nil
			}})
			issued := pendingIssuedState(time.Minute)
			workCalls := 0
			final, err := service.Run(issued, testLaunch(issued), func(ctx context.Context) error {
				workCalls++
				if event == EventFailed {
					return errors.New("task failed")
				}
				return nil
			})
			if !errors.Is(err, sinkErr) {
				t.Fatalf("sink error lost: %v", err)
			}
			if final == nil || final.Claim.Phase == v0.ClaimPhaseRunning || final.Claim.Phase == v0.ClaimPhaseBound {
				t.Fatalf("active authority retained: %+v", final)
			}
			if err := RequireRunningClaim(service, issued.Claim.ID); err == nil {
				t.Fatal("completed claim remains eligible")
			}
			trace := backend.trace
			if len(trace) < 2 || trace[len(trace)-2] != "terminate" || trace[len(trace)-1] != "cleanup" {
				t.Fatalf("no teardown: %v", trace)
			}
			if event != EventSucceeded && event != EventFailed && workCalls != 0 {
				t.Fatal("work continued despite failed evidence sink")
			}
		})
	}
}

func TestRunContextCancellationBeforeAllocationMakesZeroBackendCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backend := newRecordingBackend()
	service := newTestRunService(t, backend, RunServiceOptions{})
	issued := pendingIssuedState(time.Minute)
	final, err := service.RunContext(ctx, issued, testLaunch(issued), func(ctx context.Context) error { t.Fatal("cancelled work ran"); return nil })
	if !errors.Is(err, context.Canceled) || final.Claim.Phase != v0.ClaimPhaseFailed || len(backend.trace) != 0 {
		t.Fatalf("cancelled allocation: final=%+v err=%v trace=%v", final, err, backend.trace)
	}
}
