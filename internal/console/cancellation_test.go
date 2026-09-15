// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/modelprovider"
)

func TestQueuedDeadlineExpiresBeforeActiveWorkerFinishes(t *testing.T) {
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{entered: make(chan struct{}), release: make(chan struct{})}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "active-expiry")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("provider not entered")
	}
	var request v0.ClaimRequest
	if err := json.Unmarshal(verticalRequest(t, "queued-expiry"), &request); err != nil {
		t.Fatal(err)
	}
	limit := v0.Duration(20 * time.Millisecond)
	request.Spec.Runtime.Timeout = &limit
	data, _ := json.Marshal(request)
	if _, err = s.Submit(data); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "queued-expiry")
	if v.State.Claim.Phase != v0.ClaimPhaseExpired || v.Outcome.Status != "Expired" || v.State.Claim.BackendIdentity != nil {
		t.Fatalf("queued deadline not canonical: %+v", v)
	}
	if p.calls.Load() != 1 || e.calls.Load() != 1 || b.calls.Load() != 3 {
		t.Fatal("queued expiry touched backend/provider")
	}
	active, _ := s.QueryRequest("active-expiry")
	if active.State.Claim.Phase != v0.ClaimPhaseRunning || active.Outcome != nil {
		t.Fatal("expiry changed active work")
	}
}

// Provider acknowledges cancellation but keeps a bounded observation in flight.
// Close must join that observation before publishing RunOutcome or returning.
type cancellationProvider struct{ entered, cancelled, release, finished chan struct{} }

func (p *cancellationProvider) Complete(ctx context.Context, _ modelprovider.Request) (modelprovider.Result, error) {
	close(p.entered)
	<-ctx.Done()
	close(p.cancelled)
	<-p.release
	close(p.finished)
	return modelprovider.Result{}, ctx.Err()
}
func TestCloseJoinsProviderBeforeCleanupAndFinalOutcome(t *testing.T) {
	p := &cancellationProvider{make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})}
	b := &verticalBackend{}
	s, err := NewService(b, &verticalExecutor{}, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(verticalRequest(t, "close-join")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("provider not entered")
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-p.cancelled:
	case <-time.After(time.Second):
		t.Fatal("provider not cancelled")
	}
	select {
	case <-done:
		t.Fatal("Close abandoned provider")
	default:
	}
	v, _ := s.QueryRequest("close-join")
	if v.Outcome != nil {
		t.Fatal("published final outcome before observation ended")
	}
	if b.calls.Load() != 3 {
		t.Fatal("cleanup ran before provider joined")
	}
	close(p.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish after join")
	}
	v, _ = s.QueryRequest("close-join")
	providerEnd, cleanup, final := uint64(0), uint64(0), uint64(0)
	for _, f := range v.Facts {
		if f.Kind == "ProviderOutcome" {
			providerEnd = f.Sequence
		}
		if f.Kind == "Runtime" && f.Operation == "CleanupSucceeded" {
			cleanup = f.Sequence
		}
		if f.Kind == "RunOutcome" {
			final = f.Sequence
		}
	}
	if providerEnd == 0 || providerEnd >= cleanup || cleanup >= final {
		t.Fatalf("wrong final ordering: %d %d %d", providerEnd, cleanup, final)
	}
}
