// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
)

type reader struct {
	mu       sync.Mutex
	state    *v0.IssuedState
	deadline time.Time
}

func (r *reader) State(string) *v0.IssuedState {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, _ := json.Marshal(r.state)
	var state *v0.IssuedState
	_ = json.Unmarshal(data, &state)
	return state
}

func (r *reader) ClaimDeadline(string) (time.Time, bool) { return r.deadline, !r.deadline.IsZero() }

type spy struct {
	mu     sync.Mutex
	calls  int
	write  func(context.Context, Namespace, WriteInput) (Record, error)
	search func(context.Context, Namespace, SearchInput) ([]Record, error)
}

func (b *spy) Write(ctx context.Context, n Namespace, in WriteInput) (Record, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	if b.write != nil {
		return b.write(ctx, n, in)
	}
	return record(n, 1, in.Body, in.ClaimID), nil
}

func (b *spy) Search(ctx context.Context, n Namespace, in SearchInput) ([]Record, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	if b.search != nil {
		return b.search(ctx, n, in)
	}
	return []Record{record(n, 1, "private stored observation", "claim:previous")}, nil
}

func record(n Namespace, index int, body, source string) Record {
	return Record{Namespace: n, Reference: fmt.Sprintf("memory:%032x", index), Body: body, SourceClaim: source, CreatedAt: time.Unix(int64(100-index), 0)}
}

func setup(t *testing.T) (*Session, *reader, *spy, *facts.Journal) {
	t.Helper()
	data, err := os.ReadFile("../../harness/fixtures/contract/v0/inputs/issued-state/valid-team-a-engineer.json")
	if err != nil {
		t.Fatal(err)
	}
	state, validationErr := v0.ParseSystemIssuedState(data)
	if validationErr != nil {
		t.Fatal(validationErr)
	}
	state.EffectiveAuthority.MemoryOperations = []string{v0.MemoryRead, v0.MemoryWrite}
	r := &reader{state: state, deadline: time.Now().Add(time.Minute)}
	b := &spy{}
	j := facts.NewJournal()
	if err := j.RegisterRequest(state.RequestRef, state.Principal); err != nil {
		t.Fatal(err)
	}
	if err := j.BindClaim(state.RequestRef, *state.Claim); err != nil {
		t.Fatal(err)
	}
	s, err := New(r, j, b, []Namespace{{Team: "team-a", Project: "payments", Scope: "team-docs"}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.Bind(context.Background(), state.Claim.ID, *state.Claim.BackendIdentity)
	if err != nil {
		t.Fatal(err)
	}
	return session, r, b, j
}

func searchRequest(s *Session) Request {
	return Request{ClaimID: s.state.Claim.ID, Operation: v0.MemoryRead, Scope: "team-docs", Query: "private-query-sentinel"}
}

func TestSuccessfulWriteSearchEmptyAndMetadataPrivacy(t *testing.T) {
	s, _, b, j := setup(t)
	r := searchRequest(s)
	result, err := s.Invoke(context.Background(), r)
	if err != nil || result.Status != Found || len(result.Entries) != 1 {
		t.Fatalf("search: %+v, %v", result, err)
	}
	r.Operation, r.Query, r.Body = v0.MemoryWrite, "", "private-write-sentinel"
	written, err := s.Invoke(context.Background(), r)
	if err != nil || written.Status != Written || written.Reference == "" || len(written.Entries) != 0 {
		t.Fatalf("write: %+v, %v", written, err)
	}
	b.search = func(context.Context, Namespace, SearchInput) ([]Record, error) { return nil, nil }
	empty, err := s.Invoke(context.Background(), searchRequest(s))
	if err != nil || empty.Status != Empty || empty.InvocationID == written.InvocationID {
		t.Fatalf("empty: %+v, %v", empty, err)
	}
	got := j.ForClaim(s.state.Claim.ID)
	if len(got) != 9 || b.calls != 3 {
		t.Fatalf("facts=%d calls=%d", len(got), b.calls)
	}
	for i := 0; i < len(got); i += 3 {
		if got[i].Kind != "MemoryDecision" || got[i+1].Kind != "ProviderAttempt" || got[i+2].Kind != "ProviderOutcome" || got[i].InvocationID != got[i+2].InvocationID {
			t.Fatalf("bad correlation: %+v", got[i:i+3])
		}
	}
	encoded, _ := json.Marshal(got)
	for _, sentinel := range []string{r.Body, "private stored observation", "private-query-sentinel"} {
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("private content in facts: %s", sentinel)
		}
	}
}

func TestDenialsNeverCallBackendOrAttributeVictim(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Session, *reader, *Request)
	}{
		{"foreign target", func(_ *Session, _ *reader, r *Request) { r.ClaimID = "claim:victim" }},
		{"unknown claim", func(_ *Session, r *reader, _ *Request) { r.state = nil }},
		{"no operations", func(s *Session, r *reader, _ *Request) {
			s.state.EffectiveAuthority.MemoryOperations = nil
			r.state.EffectiveAuthority.MemoryOperations = nil
		}},
		{"no scope", func(s *Session, r *reader, _ *Request) {
			s.state.EffectiveAuthority.MemoryScopes = nil
			r.state.EffectiveAuthority.MemoryScopes = nil
		}},
		{"foreign scope", func(_ *Session, _ *reader, r *Request) { r.Scope = "private-untrusted-scope-sentinel" }},
		{"cross team", func(s *Session, _ *reader, _ *Request) {
			s.service.routes["team-docs"] = Namespace{Team: "team-b", Project: "payments", Scope: "team-docs"}
		}},
		{"cross project", func(s *Session, _ *reader, _ *Request) {
			s.service.routes["team-docs"] = Namespace{Team: "team-a", Project: "foreign", Scope: "team-docs"}
		}},
		{"read only write", func(s *Session, r *reader, req *Request) {
			s.state.EffectiveAuthority.MemoryOperations = []string{v0.MemoryRead}
			r.state.EffectiveAuthority.MemoryOperations = []string{v0.MemoryRead}
			req.Operation, req.Query, req.Body = v0.MemoryWrite, "", "body"
		}},
		{"changed worker", func(_ *Session, r *reader, _ *Request) { r.state.Claim.BackendIdentity.WorkerID = "foreign" }},
		{"changed issued grant", func(_ *Session, r *reader, _ *Request) { r.state.EffectiveAuthority.MemoryScopes = []string{"other"} }},
		{"expired deadline", func(s *Session, _ *reader, _ *Request) { s.deadline = time.Now().Add(-time.Second) }},
		{"unknown operation", func(_ *Session, _ *reader, r *Request) { r.Operation = "private-operation-sentinel" }},
		{"blank query", func(_ *Session, _ *reader, r *Request) { r.Query = "  " }},
		{"long query", func(_ *Session, _ *reader, r *Request) { r.Query = strings.Repeat("x", MaxQueryBytes+1) }},
		{"invalid utf8", func(_ *Session, _ *reader, r *Request) { r.Query = string([]byte{255}) }},
		{"negative limit", func(_ *Session, _ *reader, r *Request) { r.Limit = -1 }},
		{"large limit", func(_ *Session, _ *reader, r *Request) { r.Limit = MaxLimit + 1 }},
		{"extra write body", func(_ *Session, _ *reader, r *Request) { r.Body = "body" }},
		{"long body", func(_ *Session, _ *reader, r *Request) {
			r.Operation, r.Query, r.Body = v0.MemoryWrite, "", strings.Repeat("x", MaxBodyBytes+1)
		}},
	}
	for _, phase := range []v0.ClaimPhase{v0.ClaimPhasePending, v0.ClaimPhaseBound, v0.ClaimPhaseSucceeded, v0.ClaimPhaseFailed, v0.ClaimPhaseExpired} {
		tests = append(tests, struct {
			name   string
			change func(*Session, *reader, *Request)
		}{string(phase), func(_ *Session, r *reader, _ *Request) { r.state.Claim.Phase = phase }})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, reader, b, j := setup(t)
			r := searchRequest(s)
			test.change(s, reader, &r)
			result, err := s.Invoke(context.Background(), r)
			if err != nil || result.Status != Denied || b.calls != 0 {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, b.calls)
			}
			got := j.ForRequest(s.state.RequestRef)
			if len(got) != 1 || got[0].Kind != "MemoryDecision" || got[0].Result != v0.DecisionResultDeny || got[0].ClaimID != s.state.Claim.ID || len(j.ForClaim("claim:victim")) != 0 {
				t.Fatalf("bad denial facts: %+v", got)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "private-untrusted") || strings.Contains(string(encoded), "private-operation") {
				t.Fatal("untrusted denial target leaked")
			}
		})
	}
}

func TestBindingAndRouteCopies(t *testing.T) {
	s, r, _, j := setup(t)
	for _, worker := range []v0.SandboxClaimBackendIdentity{{}, {Backend: "reference", WorkerID: "foreign"}} {
		if _, err := s.service.Bind(context.Background(), s.state.Claim.ID, worker); !errors.Is(err, ErrBinding) {
			t.Fatal("foreign binding accepted")
		}
	}
	if _, err := s.service.Bind(context.Background(), "unknown", *s.state.Claim.BackendIdentity); !errors.Is(err, ErrBinding) {
		t.Fatal("unknown binding accepted")
	}
	if _, err := (&Session{}).Invoke(context.Background(), searchRequest(s)); !errors.Is(err, ErrBinding) {
		t.Fatal("forged session accepted")
	}
	routes := []Namespace{{Team: "team-a", Project: "payments", Scope: "team-docs"}}
	service, err := New(r, j, nil, routes)
	if err != nil {
		t.Fatal(err)
	}
	routes[0].Team = "foreign"
	if service.routes["team-docs"].Team != "team-a" {
		t.Fatal("caller can mutate captured route")
	}
	if _, err := New(r, j, nil, append(routes, routes[0])); !errors.Is(err, ErrBinding) {
		t.Fatal("duplicate route accepted")
	}
	r.state.EffectiveAuthority.MemoryOperations[0] = v0.MemoryWrite
	if s.state.EffectiveAuthority.MemoryOperations[0] != v0.MemoryRead {
		t.Fatal("reader can mutate bound authority")
	}
}

func TestTypedBackendFaultsWithoutRawErrorsOrRetries(t *testing.T) {
	for _, test := range []struct {
		err    error
		status Status
	}{{ErrUnsupported, Unsupported}, {ErrUnavailable, Unavailable}, {context.DeadlineExceeded, Timeout}, {context.Canceled, Cancelled}, {errors.New("private-error-sentinel"), Failed}, {fmt.Errorf("private-error-sentinel: %w", ErrWriteUncertain), WriteUncertain}} {
		t.Run(string(test.status), func(t *testing.T) {
			s, _, b, j := setup(t)
			b.write = func(context.Context, Namespace, WriteInput) (Record, error) { return Record{}, test.err }
			r := searchRequest(s)
			r.Operation, r.Query, r.Body = v0.MemoryWrite, "", "body"
			result, err := s.Invoke(context.Background(), r)
			if err != nil || result.Status != test.status || b.calls != 1 {
				t.Fatalf("%+v, %v calls=%d", result, err, b.calls)
			}
			encoded, _ := json.Marshal(j.ForClaim(s.state.Claim.ID))
			if strings.Contains(string(encoded), "private-error-sentinel") {
				t.Fatal("backend error leaked")
			}
		})
	}
	s, _, b, _ := setup(t)
	s.service.backend = nil
	result, err := s.Invoke(context.Background(), searchRequest(s))
	if err != nil || result.Status != Unsupported || b.calls != 0 {
		t.Fatalf("missing backend: %+v %v", result, err)
	}
}

func TestDeadlineAndRunCancellationWithholdReadData(t *testing.T) {
	s, _, b, _ := setup(t)
	s.deadline = time.Now().Add(40 * time.Millisecond)
	b.search = func(ctx context.Context, n Namespace, in SearchInput) ([]Record, error) {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.After(s.deadline) || time.Until(deadline) > CallTimeout || in.Limit != DefaultLimit {
			t.Fatal("unbounded call")
		}
		<-ctx.Done()
		return []Record{record(n, 1, "late-private-body", "previous")}, nil
	}
	result, err := s.Invoke(context.Background(), searchRequest(s))
	if err != nil || result.Status != Timeout || len(result.Entries) != 0 {
		t.Fatalf("late read: %+v %v", result, err)
	}
	s, _, b, _ = setup(t)
	runCtx, cancel := context.WithCancel(context.Background())
	s.ctx = runCtx
	b.search = func(ctx context.Context, n Namespace, _ SearchInput) ([]Record, error) {
		cancel()
		<-ctx.Done()
		return []Record{record(n, 1, "late-private-body", "previous")}, nil
	}
	result, err = s.Invoke(context.Background(), searchRequest(s))
	if err != nil || result.Status != Cancelled || len(result.Entries) != 0 {
		t.Fatalf("revoked read: %+v %v", result, err)
	}
	result, err = s.Invoke(context.Background(), searchRequest(s))
	if err != nil || result.Status != Cancelled || b.calls != 1 {
		t.Fatalf("post cancel: %+v %v", result, err)
	}
}

type failingSink struct {
	j    *facts.Journal
	kind string
}

func (s failingSink) Append(f facts.Fact) (facts.Fact, error) {
	if f.Kind == s.kind {
		return facts.Fact{}, errors.New("private-evidence-error")
	}
	return s.j.Append(f)
}

func TestEvidenceFailureStopsDispatchOrFailsCommittedWork(t *testing.T) {
	for _, kind := range []string{"MemoryDecision", "ProviderAttempt", "ProviderOutcome"} {
		t.Run(kind, func(t *testing.T) {
			s, _, b, j := setup(t)
			s.service.facts = failingSink{j, kind}
			r := searchRequest(s)
			r.Operation, r.Query, r.Body = v0.MemoryWrite, "", "private-body"
			result, err := s.Invoke(context.Background(), r)
			wantCalls := 0
			if kind == "ProviderOutcome" {
				wantCalls = 1
			}
			if !errors.Is(err, ErrEvidence) || result.InvocationID != "" || b.calls != wantCalls {
				t.Fatalf("%+v %v calls=%d", result, err, b.calls)
			}
		})
	}
}

func TestReplyBoundsAndMisrouting(t *testing.T) {
	s, _, b, _ := setup(t)
	b.search = func(_ context.Context, n Namespace, _ SearchInput) ([]Record, error) {
		records := make([]Record, MaxLimit)
		for i := range records {
			records[i] = record(n, i+1, strings.Repeat("\x01", MaxBodyBytes), "previous")
		}
		return records, nil
	}
	r := searchRequest(s)
	r.Limit = MaxLimit
	result, err := s.Invoke(context.Background(), r)
	// JSON escaping can make even a single valid record exceed the reply cap.
	if err != nil || result.Status != Found || !result.Truncated || len(result.Entries) != 0 {
		t.Fatalf("oversized first record: %+v %v", result, err)
	}
	b.search = func(_ context.Context, n Namespace, _ SearchInput) ([]Record, error) {
		return []Record{record(n, 1, strings.Repeat("x", MaxBodyBytes), "previous"), record(n, 2, strings.Repeat("x", MaxBodyBytes), "previous"), record(n, 3, strings.Repeat("x", MaxBodyBytes), "previous"), record(n, 4, strings.Repeat("x", MaxBodyBytes), "previous")}, nil
	}
	result, err = s.Invoke(context.Background(), r)
	encoded, _ := json.Marshal(result)
	if err != nil || result.Status != Found || !result.Truncated || len(result.Entries) != 3 || len(encoded) > MaxReplyBytes {
		t.Fatalf("bounded reply: status=%s entries=%d bytes=%d err=%v", result.Status, len(result.Entries), len(encoded), err)
	}
	b.search = func(_ context.Context, n Namespace, _ SearchInput) ([]Record, error) {
		n.Team = "foreign"
		return []Record{record(n, 1, "private", "previous")}, nil
	}
	result, err = s.Invoke(context.Background(), r)
	if err != nil || result.Status != Failed || len(result.Entries) != 0 {
		t.Fatalf("misrouting: %+v %v", result, err)
	}
}

func TestConcurrentCallsKeepIndependentInvocationFacts(t *testing.T) {
	s, _, b, j := setup(t)
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := s.Invoke(context.Background(), searchRequest(s))
			if err != nil || result.Status != Found {
				t.Errorf("%+v %v", result, err)
				return
			}
			ids <- result.InvocationID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatal("invocation identity reused")
		}
		seen[id] = true
	}
	if b.calls != 20 || len(seen) != 20 || len(j.ForClaim(s.state.Claim.ID)) != 60 {
		t.Fatal("concurrent invocation lost correlation")
	}
}

func TestKnownWriteCommitIsNotReportedAsRollback(t *testing.T) {
	s, _, b, _ := setup(t)
	runCtx, cancel := context.WithCancel(context.Background())
	s.ctx = runCtx
	b.write = func(_ context.Context, n Namespace, in WriteInput) (Record, error) {
		cancel()
		return record(n, 1, in.Body, in.ClaimID), nil
	}
	r := searchRequest(s)
	r.Operation, r.Query, r.Body = v0.MemoryWrite, "", "known committed data"
	result, err := s.Invoke(context.Background(), r)
	if err != nil || result.Status != Written || result.Reference == "" || b.calls != 1 {
		t.Fatalf("known commit: %+v %v", result, err)
	}
}

func TestEmptyNonMemoryGrantsAndCallerDeadline(t *testing.T) {
	s, r, b, _ := setup(t)
	r.state.EffectiveAuthority.Tools = []string{}
	r.state.EffectiveAuthority.ResourceScopes = []string{}
	s.state.EffectiveAuthority.Tools = nil
	s.state.EffectiveAuthority.ResourceScopes = nil
	b.search = func(ctx context.Context, _ Namespace, _ SearchInput) ([]Record, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("caller deadline expanded")
		}
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := s.Invoke(ctx, searchRequest(s))
	if err != nil || result.Status != Empty {
		t.Fatalf("empty equivalent grants denied: %+v %v", result, err)
	}
}
