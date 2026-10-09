// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
)

// A valid context scheduler delays AfterFunc delivery until the test releases
// it. Err/Done reflect cancellation immediately, without scheduler sleeps.
type delayedRunContext struct {
	parent  context.Context
	release <-chan struct{}
}

func (c delayedRunContext) Deadline() (time.Time, bool) { return c.parent.Deadline() }
func (c delayedRunContext) Done() <-chan struct{}       { return c.parent.Done() }
func (c delayedRunContext) Err() error                  { return c.parent.Err() }
func (delayedRunContext) Value(any) any                 { return nil }
func (c delayedRunContext) AfterFunc(f func()) func() bool {
	stop := make(chan struct{})
	var ended atomic.Bool
	go func() {
		select {
		case <-c.Done():
		case <-stop:
			return
		}
		select {
		case <-c.release:
		case <-stop:
			return
		}
		if !ended.Swap(true) {
			f()
		}
	}()
	return func() bool {
		if ended.Swap(true) {
			return false
		}
		close(stop)
		return true
	}
}

type cancellingAdmissionSink struct {
	*facts.Journal
	kind   string
	cancel context.CancelFunc
}

func (s cancellingAdmissionSink) Append(f facts.Fact) (facts.Fact, error) {
	if f.Kind == s.kind {
		s.cancel()
	}
	return s.Journal.Append(f)
}

func TestMemoryAdmissionRechecksRunCancellationBeforeDispatch(t *testing.T) {
	for _, kind := range []string{"MemoryDecision", "ProviderAttempt"} {
		for _, operation := range []string{v0.MemoryRead, v0.MemoryWrite} {
			t.Run(kind+"/"+operation, func(t *testing.T) {
				s, r, b, journal := setup(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				release := make(chan struct{})
				defer close(release)
				session, err := s.service.Bind(delayedRunContext{ctx, release}, s.state.Claim.ID, *s.state.Claim.BackendIdentity)
				if err != nil {
					t.Fatal(err)
				}
				s.service.facts = cancellingAdmissionSink{journal, kind, cancel}
				request := searchRequest(session)
				if operation == v0.MemoryWrite {
					request.Operation, request.Query, request.Body = operation, "", "private-cancelled-write"
				}
				result, err := session.Invoke(context.Background(), request)
				if err != nil || result.Status != Cancelled || b.calls != 0 || len(result.Entries) != 0 || result.Reference != "" {
					t.Fatalf("cancelled run dispatched before AfterFunc delivery: status=%s calls=%d err=%v", result.Status, b.calls, err)
				}
				if !active(r.State(session.state.Claim.ID)) || ctx.Err() == nil {
					t.Fatal("fixture must keep canonical state Running while its run context is cancelled")
				}
				got := journal.ForClaim(session.state.Claim.ID)
				if len(got) != 3 || got[2].Memory.Status != string(Cancelled) {
					t.Fatal("cancelled admitted invocation did not close its correlated evidence")
				}
			})
		}
	}
}
