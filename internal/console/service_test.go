// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type verticalBackend struct {
	calls        atomic.Int32
	claim        string
	identity     v0.SandboxClaimBackendIdentity
	cleanupError bool
}

func (b *verticalBackend) Allocate(r runtime.AllocateRequest) (runtime.Allocation, error) {
	b.calls.Add(1)
	b.claim = r.ClaimID
	b.identity = v0.SandboxClaimBackendIdentity{Backend: "test", WorkerID: r.ClaimID}
	return runtime.Allocation{ClaimID: r.ClaimID, Identity: b.identity}, nil
}
func (b *verticalBackend) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	b.calls.Add(1)
	return runtime.Observation{ClaimID: b.claim, Identity: id, Ready: true}, nil
}
func (b *verticalBackend) Start(v0.SandboxClaimBackendIdentity) error     { b.calls.Add(1); return nil }
func (b *verticalBackend) Terminate(v0.SandboxClaimBackendIdentity) error { b.calls.Add(1); return nil }
func (b *verticalBackend) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	b.calls.Add(1)
	if b.cleanupError {
		return runtime.CleanupResult{}, errors.New("private cleanup details")
	}
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}

type verticalExecutor struct {
	foreign   bool
	ungranted bool
	calls     atomic.Int32
}

func (e *verticalExecutor) Execute(ctx context.Context, id v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handle workerprotocol.Handler) (string, error) {
	e.calls.Add(1)
	op := workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: task.Objective}
	if e.foreign {
		op.ClaimID = "claim:other"
	}
	if e.ungranted {
		op.Profile = "not-granted"
	}
	reply, err := handle(ctx, op)
	if err != nil {
		return "", err
	}
	if !reply.Allowed {
		return "", errors.New("model denied")
	}
	return reply.Text, nil
}

type verticalProvider struct {
	calls   atomic.Int32
	fail    bool
	entered chan struct{}
	release chan struct{}
}

func (p *verticalProvider) Complete(ctx context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	p.calls.Add(1)
	if p.entered != nil {
		close(p.entered)
		select {
		case <-ctx.Done():
			return modelprovider.Result{}, ctx.Err()
		case <-p.release:
		}
	}
	if p.fail {
		return modelprovider.Result{}, errors.New("private provider credential failure details")
	}
	return modelprovider.Result{Text: "Answer: " + r.Prompt, Model: "local-test", ResponseID: "response-1", InputTokens: 10, OutputTokens: 20}, nil
}
func verticalRequest(t *testing.T, name string) []byte {
	t.Helper()
	timeout := v0.Duration(45 * time.Minute)
	r := v0.ClaimRequest{APIVersion: v0.ClaimRequestAPIVersion, Kind: v0.ClaimRequestKind, Metadata: v0.ObjectMeta{Name: name}, Spec: v0.ClaimRequestSpec{TemplateRef: "engineer", ProjectRef: "payments", Task: &v0.ClaimRequestTask{Type: "repository-change", Input: map[string]any{"objective": "Explain bounded retry", "repository": "acme/payments"}}, RequestedAccess: v0.ClaimRequestedAccess{Tools: []string{"git.read", "shell.exec"}, ResourceScopes: []string{"repo:acme/payments"}, ModelProfile: "approved-coding-model"}, Runtime: &v0.ClaimRuntimeRequirements{ProfileRef: "standard-isolated", Timeout: &timeout}}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func awaitVertical(t *testing.T, s *Service, ref string) evidence.View {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		view, err := s.QueryRequest(ref)
		if err != nil {
			t.Fatal(err)
		}
		if view.Outcome != nil {
			return view
		}
		if time.Now().After(deadline) {
			t.Fatalf("no outcome: %+v", view)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestVerticalServiceSameClaimResultNarrowingAndFacts(t *testing.T) {
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "test-one")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "test-one")
	if v.State.Claim.Phase != v0.ClaimPhaseSucceeded || v.Outcome.Text != "Answer: Explain bounded retry" || v.Outcome.Model == nil || p.calls.Load() != 1 {
		t.Fatalf("result: %+v", v)
	}
	a := v.State.EffectiveAuthority
	if len(a.Tools) != 1 || a.Tools[0] != "git.read" || time.Duration(a.Runtime.Timeout) != 30*time.Minute {
		t.Fatalf("not narrowed: %+v", a)
	}
	claimID := v.State.Claim.ID
	inv := v.Outcome.Model.InvocationID
	found := map[string]int{}
	for _, f := range v.Facts {
		if f.ClaimID != "" && f.ClaimID != claimID {
			t.Fatal("cross-claim fact")
		}
		found[f.Kind]++
		if strings.HasPrefix(f.Kind, "Provider") || f.Kind == "ModelDecision" {
			if f.InvocationID != inv {
				t.Fatal("invocation drift")
			}
		}
	}
	for _, kind := range []string{"RequestResolution", "AuthorityResolved", "ModelDecision", "ProviderAttempt", "ProviderOutcome", "RunOutcome"} {
		if found[kind] != 1 {
			t.Fatalf("%s facts=%d", kind, found[kind])
		}
	}
	if _, err = s.QueryClaim(claimID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(verticalRequest(t, "test-one")); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate request allowed")
	}
	v.State.EffectiveAuthority.Tools[0] = "shell.exec"
	v.Facts[0].Kind = "corrupted"
	fresh, _ := s.QueryRequest("test-one")
	if fresh.State.EffectiveAuthority.Tools[0] != "git.read" || fresh.Facts[0].Kind == "corrupted" {
		t.Fatal("query mutates authoritative state")
	}
}
func TestVerticalServiceDenialAndForeignClaimMakeZeroProviderCalls(t *testing.T) {
	for _, tc := range []struct {
		name               string
		preset             app.ReferencePrincipalPreset
		foreign, ungranted bool
	}{{"denial", app.ReferencePrincipalTeamB, false, false}, {"foreign", app.ReferencePrincipalTeamA, true, false}, {"ungranted", app.ReferencePrincipalTeamA, false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			b, e, p := &verticalBackend{}, &verticalExecutor{foreign: tc.foreign, ungranted: tc.ungranted}, &verticalProvider{}
			s, err := NewService(b, e, p, tc.preset)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err = s.Submit(verticalRequest(t, tc.name)); err != nil {
				t.Fatal(err)
			}
			v := awaitVertical(t, s, tc.name)
			if p.calls.Load() != 0 {
				t.Fatal("denial reached provider")
			}
			if tc.preset == app.ReferencePrincipalTeamB {
				if v.State.Claim != nil || b.calls.Load() != 0 || e.calls.Load() != 0 {
					t.Fatal("preclaim denial allocated")
				}
				for _, f := range v.Facts {
					if f.ClaimID != "" {
						t.Fatal("fabricated denied claim")
					}
				}
			}
			for _, f := range v.Facts {
				if f.Kind == "ProviderAttempt" {
					t.Fatal("denial fabricated attempted provider call")
				}
			}
		})
	}
}
func TestVerticalServiceProviderFailureIsNotPermissionDenial(t *testing.T) {
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{fail: true}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "failure")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "failure")
	if v.State.Claim.Phase != v0.ClaimPhaseFailed || v.Outcome.Failure == "" {
		t.Fatal("failure presented as success")
	}
	var allowed, failed bool
	for _, f := range v.Facts {
		if f.Kind == "ModelDecision" {
			allowed = f.Result == v0.DecisionResultAllow
		}
		if f.Kind == "ProviderOutcome" {
			failed = f.ProviderStatus == "Failed"
		}
	}
	if !allowed || !failed {
		t.Fatal("provider failure conflated with denial")
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "private") {
		t.Fatal("private error leaked")
	}
}
func TestVerticalServicePollingShowsActualRunningThenTerminal(t *testing.T) {
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{entered: make(chan struct{}), release: make(chan struct{})}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "poll")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("provider not reached")
	}
	v, _ := s.QueryRequest("poll")
	if v.State.Claim.Phase != v0.ClaimPhaseRunning || v.Outcome != nil {
		t.Fatal("poll fabricated terminal")
	}
	close(p.release)
	v = awaitVertical(t, s, "poll")
	if v.State.Claim.Phase != v0.ClaimPhaseSucceeded {
		t.Fatal("no terminal outcome")
	}
}

func TestVerticalCleanupFailurePreservesSuccessfulTaskResult(t *testing.T) {
	b, e, p := &verticalBackend{cleanupError: true}, &verticalExecutor{}, &verticalProvider{}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "cleanup-failed")); err != nil {
		t.Fatal(err)
	}
	v := awaitVertical(t, s, "cleanup-failed")
	if v.State.Claim.Phase != v0.ClaimPhaseSucceeded || v.Outcome.Text == "" || v.Outcome.Model == nil || v.Outcome.Failure == "" {
		t.Fatalf("successful work lost on cleanup failure: %+v", v.Outcome)
	}
}

func TestVerticalCloseCancelsActiveAndQueuedWorkBeforeFurtherAllocation(t *testing.T) {
	b, e, p := &verticalBackend{}, &verticalExecutor{}, &verticalProvider{entered: make(chan struct{}), release: make(chan struct{})}
	s, err := NewService(b, e, p, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Submit(verticalRequest(t, "active")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("provider not reached")
	}
	if _, err = s.Submit(verticalRequest(t, "queued")); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { s.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not cancel active work")
	}
	if p.calls.Load() != 1 || e.calls.Load() != 1 || b.calls.Load() != 5 {
		t.Fatalf("queued work escaped shutdown: provider=%d executor=%d backend=%d", p.calls.Load(), e.calls.Load(), b.calls.Load())
	}
	v, _ := s.QueryRequest("queued")
	if v.Outcome == nil || v.Outcome.Status != "Cancelled" || v.State.Claim.BackendIdentity != nil {
		t.Fatal("queued cancellation invented runtime execution")
	}
	if _, err = s.Submit(verticalRequest(t, "after-close")); err == nil {
		t.Fatal("submission after shutdown accepted")
	}
}
