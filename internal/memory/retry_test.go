// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func writeRequest(s *Session) Request {
	return Request{ClaimID: s.state.Claim.ID, Operation: v0.MemoryWrite, Scope: "team-docs", Body: "private-retry-body-sentinel"}
}

func uncertainWrite(t *testing.T, s *Session) Result {
	t.Helper()
	result, err := s.Invoke(context.Background(), writeRequest(s))
	if err != nil || result.Status != WriteUncertain || result.Retry == nil {
		t.Fatalf("uncertain write: status=%s err=%v", result.Status, err)
	}
	return result
}

func TestUncertainWriteRetryPreservesReceiptAndImmutableRequest(t *testing.T) {
	for _, committed := range []bool{true, false} {
		t.Run(map[bool]string{true: "commit-ack-lost", false: "no-commit"}[committed], func(t *testing.T) {
			s, _, b, j := setup(t)
			rows := map[string]Record{}
			var ids []string
			b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
				ids = append(ids, in.InvocationID)
				if in.Body != "private-retry-body-sentinel" || in.ClaimID != s.state.Claim.ID || n != s.service.routes["team-docs"] {
					t.Fatal("retry changed its original immutable write")
				}
				if len(ids) == 1 && !committed {
					return Record{}, ErrWriteUncertain
				}
				stored, ok := rows[in.InvocationID]
				if !ok {
					stored = record(n, len(rows)+1, in.Body, in.ClaimID)
					rows[in.InvocationID] = stored
				}
				if len(ids) == 1 {
					return Record{}, ErrWriteUncertain
				}
				return stored, nil
			}
			request := writeRequest(s)
			first, err := s.Invoke(context.Background(), request)
			if err != nil || first.Status != WriteUncertain || first.Retry == nil || b.calls != 1 {
				t.Fatal("missing host retry continuation or automatic retry")
			}
			request.Body, request.Scope, request.ClaimID = "changed", "foreign", "victim"
			wire, _ := json.Marshal(first)
			if strings.Contains(string(wire), "retry") || strings.Contains(string(wire), "sentinel") {
				t.Fatal("private retry state escaped to worker JSON")
			}
			var decoded Result
			if json.Unmarshal(wire, &decoded) != nil || decoded.Retry != nil {
				t.Fatal("wire response revived host state")
			}
			copy := *first.Retry
			second, err := s.RetryWrite(context.Background(), &copy)
			if err != nil || second.Status != Written || second.Retry != nil || second.InvocationID == first.InvocationID || second.Reference == "" {
				t.Fatalf("retry: status=%s err=%v", second.Status, err)
			}
			if len(rows) != 1 || len(ids) != 2 || ids[0] != first.InvocationID || ids[1] != ids[0] {
				t.Fatal("retry bypassed the original receipt")
			}
			for _, handle := range []*WriteRetry{first.Retry, &copy} {
				if _, err := s.RetryWrite(context.Background(), handle); !errors.Is(err, ErrBinding) {
					t.Fatal("completed continuation was revived")
				}
			}
			if b.calls != 2 {
				t.Fatal("completed continuation called backend")
			}
			got := j.ForClaim(s.state.Claim.ID)
			if len(got) != 6 {
				t.Fatal("retry lost audit attempts")
			}
			for index, id := range []string{first.InvocationID, second.InvocationID} {
				for _, fact := range got[index*3 : index*3+3] {
					if fact.InvocationID != id || fact.ClaimID != s.state.Claim.ID {
						t.Fatal("retry corrupted journal correlation")
					}
				}
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "sentinel") {
				t.Fatal("retry body leaked into facts")
			}
		})
	}
}

func TestRetryContinuationCannotBeForgedOrMovedToAnotherSession(t *testing.T) {
	s, _, b, _ := setup(t)
	b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, ErrWriteUncertain }
	first := uncertainWrite(t, s)
	other, _, foreign, _ := setup(t)
	if _, err := other.RetryWrite(context.Background(), first.Retry); !errors.Is(err, ErrBinding) || foreign.calls != 0 {
		t.Fatal("foreign session reused receipt")
	}
	rebound, err := s.service.Bind(context.Background(), s.state.Claim.ID, *s.state.Claim.BackendIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rebound.RetryWrite(context.Background(), first.Retry); !errors.Is(err, ErrBinding) {
		t.Fatal("rebound session restored private continuation")
	}
	var forged WriteRetry
	if json.Unmarshal([]byte(`{"receiptID":"worker-chosen","usable":true}`), &forged) != nil {
		t.Fatal("test decode failed")
	}
	for _, handle := range []*WriteRetry{nil, {}, &forged} {
		if _, err := s.RetryWrite(context.Background(), handle); !errors.Is(err, ErrBinding) {
			t.Fatal("forged continuation accepted")
		}
	}
	if b.calls != 1 {
		t.Fatal("invalid continuation reached backend")
	}
}

func TestRetryRechecksAuthorityAndContextsBeforeBackend(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Session, *reader) context.Context
	}{
		{"terminal", func(_ *Session, r *reader) context.Context {
			r.state.Claim.Phase = v0.ClaimPhaseSucceeded
			return context.Background()
		}},
		{"grant-revoked", func(_ *Session, r *reader) context.Context {
			r.state.EffectiveAuthority.MemoryOperations = nil
			return context.Background()
		}},
		{"scope-revoked", func(_ *Session, r *reader) context.Context {
			r.state.EffectiveAuthority.MemoryScopes = nil
			return context.Background()
		}},
		{"worker-changed", func(_ *Session, r *reader) context.Context {
			r.state.Claim.BackendIdentity.WorkerID = "foreign"
			return context.Background()
		}},
		{"deadline", func(s *Session, _ *reader) context.Context {
			s.deadline = time.Now().Add(-time.Second)
			return context.Background()
		}},
		{"ownership", func(s *Session, _ *reader) context.Context {
			n := s.service.routes["team-docs"]
			n.Team = "foreign"
			s.service.routes["team-docs"] = n
			return context.Background()
		}},
		{"caller-cancelled", func(_ *Session, _ *reader) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}},
		{"run-cancelled", func(s *Session, _ *reader) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			s.ctx = ctx
			return context.Background()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, b, j := setup(t)
			b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, ErrWriteUncertain }
			first := uncertainWrite(t, s)
			ctx := tc.change(s, r)
			result, err := s.RetryWrite(ctx, first.Retry)
			if ctx.Err() != nil || s.ctx.Err() != nil {
				if !errors.Is(err, context.Canceled) || b.calls != 1 || len(j.ForClaim(s.state.Claim.ID)) != 3 {
					t.Fatal("pre-admission cancellation dispatched or recorded an invocation")
				}
				return
			}
			if err != nil || (result.Status != Denied && result.Status != Cancelled) || b.calls != 1 {
				t.Fatal("retry bypassed admission")
			}
			got := j.ForClaim(s.state.Claim.ID)
			if len(got) != 4 || got[3].Kind != "MemoryDecision" || got[3].Result != v0.DecisionResultDeny || got[3].InvocationID == first.InvocationID {
				t.Fatal("retry denial lost isolated audit record")
			}
		})
	}
}

func TestRetryTransientFaultsKeepSameReceiptWithoutAutomaticRetries(t *testing.T) {
	s, _, b, j := setup(t)
	var ids []string
	b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
		ids = append(ids, in.InvocationID)
		switch len(ids) {
		case 1, 3:
			return Record{}, ErrWriteUncertain
		case 2:
			return Record{}, ErrUnavailable
		default:
			return record(n, 1, in.Body, in.ClaimID), nil
		}
	}
	first := uncertainWrite(t, s)
	for index, want := range []Status{Unavailable, WriteUncertain, Written} {
		result, err := s.RetryWrite(context.Background(), first.Retry)
		if err != nil || result.Status != want || b.calls != index+2 || (want != Written && result.Retry != first.Retry) {
			t.Fatal("retry hid fault, lost state or automatically replayed")
		}
	}
	for _, id := range ids {
		if id != first.InvocationID {
			t.Fatal("transient failure minted new receipt")
		}
	}
	if len(j.ForClaim(s.state.Claim.ID)) != 12 {
		t.Fatal("missing transient attempt facts")
	}
}

func TestRetryEvidenceFailureDisablesAllCopies(t *testing.T) {
	for _, kind := range []string{"MemoryDecision", "ProviderAttempt", "ProviderOutcome"} {
		t.Run(kind, func(t *testing.T) {
			s, _, b, j := setup(t)
			b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, ErrWriteUncertain }
			first := uncertainWrite(t, s)
			copy := *first.Retry
			b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
				return record(n, 1, in.Body, in.ClaimID), nil
			}
			s.service.facts = failingSink{j, kind}
			result, err := s.RetryWrite(context.Background(), first.Retry)
			if !errors.Is(err, ErrEvidence) || result.Retry != nil {
				t.Fatal("retry evidence failure was hidden")
			}
			s.service.facts = j
			calls := b.calls
			if (kind == "ProviderOutcome" && calls != 2) || (kind != "ProviderOutcome" && calls != 1) {
				t.Fatal("evidence failure dispatch count incorrect")
			}
			if _, err := s.RetryWrite(context.Background(), &copy); !errors.Is(err, ErrBinding) || b.calls != calls {
				t.Fatal("evidence failure replayed to conceal missing facts")
			}
		})
	}
}

func TestConcurrentRetryCopiesResolveOnlyOnce(t *testing.T) {
	s, _, b, _ := setup(t)
	b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, ErrWriteUncertain }
	first := uncertainWrite(t, s)
	b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
		return record(n, 1, in.Body, in.ClaimID), nil
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := *first.Retry
			result, err := s.RetryWrite(context.Background(), &copy)
			if err != nil && !errors.Is(err, ErrBinding) {
				t.Error(err)
			}
			if err == nil && result.Status != Written {
				t.Error("concurrent retry did not resolve")
			}
		}()
	}
	wg.Wait()
	if b.calls != 2 {
		t.Fatal("concurrent copies dispatched duplicate resolution")
	}
}

func TestWaitingRetryCanCancelBeforeAdmission(t *testing.T) {
	for _, runCancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "caller-cancelled", true: "run-cancelled"}[runCancelled], func(t *testing.T) {
			s, _, b, j := setup(t)
			runCtx, cancelRun := context.WithCancel(context.Background())
			defer cancelRun()
			s.ctx = runCtx
			b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, ErrWriteUncertain }
			first := uncertainWrite(t, s)
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
				close(started)
				<-release
				return record(n, 1, in.Body, in.ClaimID), nil
			}
			go func() {
				defer close(finished)
				result, err := s.RetryWrite(context.Background(), first.Retry)
				if err != nil || result.Status != Written {
					t.Error("active retry lost known completion")
				}
			}()
			defer func() { close(release); <-finished }()
			<-started
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			waiting := make(chan struct{})
			go func() {
				close(waiting)
				_, err := s.RetryWrite(ctx, first.Retry)
				done <- err
			}()
			<-waiting
			select {
			case <-done:
				t.Fatal("queued retry returned before cancellation or acknowledgement")
			case <-time.After(10 * time.Millisecond):
			}
			if runCancelled {
				cancelRun()
			} else {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Error("queued cancellation was not reported")
				}
			case <-time.After(time.Second):
				t.Error("queued retry ignored cancellation")
			}
			b.mu.Lock()
			calls := b.calls
			b.mu.Unlock()
			if calls != 2 || len(j.ForClaim(s.state.Claim.ID)) != 5 {
				t.Error("queued cancellation dispatched or fabricated an invocation")
			}
		})
	}
}
