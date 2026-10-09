// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package memory

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
)

type StateReader interface {
	State(string) *v0.IssuedState
	ClaimDeadline(string) (time.Time, bool)
	// ObserveState calls back exactly once under the lifecycle read boundary
	// shared by terminal transitions. The callback must not reenter the reader.
	ObserveState(string, func(*v0.IssuedState))
}

type FactSink interface {
	// Append is a bounded local publication and must not call the state owner;
	// admission publishes decision/attempt while holding its lifecycle boundary.
	Append(facts.Fact) (facts.Fact, error)
}

type Service struct {
	states  StateReader
	facts   FactSink
	backend Backend
	routes  map[string]Namespace
	ids     gateway.IDSource
}

// New consumes operator-owned routes, not worker-submitted ownership values.
// A nil backend is explicitly unsupported; there is no local fallback.
func New(states StateReader, sink FactSink, backend Backend, routes []Namespace) (*Service, error) {
	if states == nil || sink == nil {
		return nil, ErrBinding
	}
	s := &Service{states: states, facts: sink, backend: backend, routes: make(map[string]Namespace), ids: gateway.RandomIDSource()}
	for _, route := range routes {
		if strings.TrimSpace(route.Scope) == "" || strings.TrimSpace(route.Team) == "" || strings.TrimSpace(route.Project) == "" {
			return nil, ErrBinding
		}
		if _, duplicate := s.routes[route.Scope]; duplicate {
			return nil, ErrBinding
		}
		s.routes[route.Scope] = route
	}
	return s, nil
}

// Session cannot be constructed from request content. Bind must be called by
// the trusted run composition with its run-owned context and allocated worker.
type Session struct {
	service  *Service
	ctx      context.Context
	state    *v0.IssuedState
	deadline time.Time
}

func (s *Service) Bind(ctx context.Context, claimID string, worker v0.SandboxClaimBackendIdentity) (*Session, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrBinding
	}
	state := s.states.State(claimID)
	deadline, ok := s.states.ClaimDeadline(claimID)
	if !active(state) || state.Claim.ID != claimID || *state.Claim.BackendIdentity != worker || !ok || !time.Now().Before(deadline) {
		return nil, ErrBinding
	}
	// Even a test/custom reader cannot retain mutable authority behind Bind.
	data, err := json.Marshal(state)
	if err != nil {
		return nil, ErrBinding
	}
	var bound v0.IssuedState
	if json.Unmarshal(data, &bound) != nil {
		return nil, ErrBinding
	}
	return &Session{service: s, ctx: ctx, state: &bound, deadline: deadline}, nil
}

func active(state *v0.IssuedState) bool {
	return state != nil && v0.ValidateIssuedState(state) == nil && state.Claim != nil && state.Claim.Phase == v0.ClaimPhaseRunning && state.Claim.BackendIdentity != nil && state.Principal.Team != ""
}

func (s *Session) current() bool {
	return s.matchesCurrent(s.service.states.State(s.state.Claim.ID))
}

func (s *Session) matchesCurrent(current *v0.IssuedState) bool {
	if !active(current) {
		return false
	}
	return current.RequestRef == s.state.RequestRef && current.Principal == s.state.Principal && current.Action == s.state.Action && current.PolicyRef == s.state.PolicyRef && current.Decision == s.state.Decision &&
		current.Claim.ID == s.state.Claim.ID && current.Claim.AuthorityRef == s.state.Claim.AuthorityRef && *current.Claim.BackendIdentity == *s.state.Claim.BackendIdentity &&
		sameAuthority(current.EffectiveAuthority, s.state.EffectiveAuthority)
}

func sameAuthority(a, b *v0.EffectiveAuthority) bool {
	return a != nil && b != nil && a.ID == b.ID && a.Runtime == b.Runtime && a.ModelProfile == b.ModelProfile && slices.Equal(a.Tools, b.Tools) && slices.Equal(a.ResourceScopes, b.ResourceScopes) && slices.Equal(a.MemoryScopes, b.MemoryScopes) && slices.Equal(a.MemoryOperations, b.MemoryOperations)
}

func (s *Session) Invoke(ctx context.Context, request Request) (Result, error) {
	result, err := s.invoke(ctx, request, "")
	if err == nil && result.Status == WriteUncertain {
		result.Retry = &WriteRetry{state: &writeRetryState{
			session: s, request: request, receiptID: result.InvocationID, usable: true, gate: make(chan struct{}, 1),
		}}
	}
	return result, err
}

// WriteRetry is an opaque, session-local continuation for an uncertain write.
// Copies share its disposition; neither copying nor wire decoding can revive it.
type WriteRetry struct{ state *writeRetryState }

type writeRetryState struct {
	gate      chan struct{}
	session   *Session
	request   Request
	receiptID string
	usable    bool
}

// RetryWrite repeats all admission checks and preserves the original immutable
// body/scope and receipt ID. It never retries automatically or accepts a new body.
func (s *Session) RetryWrite(ctx context.Context, retry *WriteRetry) (Result, error) {
	if s == nil || s.ctx == nil || ctx == nil || retry == nil || retry.state == nil || retry.state.session != s {
		return Result{}, ErrBinding
	}
	state := retry.state
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := s.ctx.Err(); err != nil {
		return Result{}, err
	}
	// Waiting is before admission: cancellation starts no audit invocation and
	// cannot force a second caller to wait for another backend acknowledgement.
	select {
	case state.gate <- struct{}{}:
		defer func() { <-state.gate }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-s.ctx.Done():
		return Result{}, s.ctx.Err()
	}
	// Acquisition can win when cancellation and gate release are both ready.
	// Admission starts only after checking both contexts at this handoff.
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := s.ctx.Err(); err != nil {
		return Result{}, err
	}
	if !state.usable {
		return Result{}, ErrBinding
	}
	result, err := s.invoke(ctx, state.request, state.receiptID)
	if err != nil || result.Status == Written {
		// Known completion or evidence failure must not be replayed to conceal
		// missing facts. A remaining backend fault does not resolve uncertainty.
		state.usable = false
	} else {
		result.Retry = retry
	}
	return result, err
}

func (s *Session) invoke(ctx context.Context, request Request, receiptID string) (Result, error) {
	if s == nil || s.service == nil || s.state == nil || s.ctx == nil || ctx == nil {
		return Result{}, ErrBinding
	}
	id := s.service.ids()
	if receiptID == "" {
		receiptID = id
	}
	result := Result{InvocationID: id, Status: Denied, ReasonCode: "memory-denied"}
	operation := "memory.invalid"
	if request.Operation == v0.MemoryRead || request.Operation == v0.MemoryWrite {
		operation = "memory." + request.Operation
	}
	target := ""
	route, routed := s.service.routes[request.Scope]
	if routed {
		target = route.Scope
	}
	if request.Operation == v0.MemoryRead && request.Limit == 0 {
		request.Limit = DefaultLimit
	}
	deadline := time.Now().Add(CallTimeout)
	if s.deadline.Before(deadline) {
		deadline = s.deadline
	}
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	defer cancel()
	var fact facts.Fact
	var admissionErr error
	observed := false
	// Admission and its complete decision/attempt publication share the state
	// owner's read lock. A terminal transition can win before this boundary or
	// follow it, but cannot split it. Backend IO runs after releasing the lock.
	s.service.states.ObserveState(s.state.Claim.ID, func(current *v0.IssuedState) {
		observed = true
		switch {
		case request.ClaimID != s.state.Claim.ID:
			result.ReasonCode = "memory-context-mismatch"
		case !s.matchesCurrent(current) || !time.Now().Before(s.deadline):
			result.ReasonCode = "memory-claim-inactive"
		case s.ctx.Err() != nil || ctx.Err() != nil:
			result.Status, result.ReasonCode = Cancelled, "memory-context-cancelled"
		case !validRequest(request):
			result.ReasonCode = "memory-invalid-input"
		case !slices.Contains(s.state.EffectiveAuthority.MemoryOperations, request.Operation):
			result.ReasonCode = "memory-operation-not-granted"
		case !slices.Contains(s.state.EffectiveAuthority.MemoryScopes, request.Scope):
			result.ReasonCode = "memory-scope-not-granted"
		case !routed || route.Team != s.state.Principal.Team || route.Project != s.state.Action.Project:
			result.ReasonCode = "memory-ownership-denied"
		default:
			result.Status, result.ReasonCode = "", "memory-allowed"
		}
		fact = facts.Fact{Kind: "MemoryDecision", RequestRef: s.state.RequestRef, ClaimID: s.state.Claim.ID, InvocationID: id, PolicyRef: &s.state.PolicyRef, Operation: operation, Target: target, Result: v0.DecisionResultDeny, ReasonCode: result.ReasonCode}
		if result.Status == "" {
			fact.Result = v0.DecisionResultAllow
		}
		if _, admissionErr = s.service.facts.Append(fact); admissionErr != nil || fact.Result != v0.DecisionResultAllow {
			return
		}
		fact.Kind, fact.Result, fact.ProviderStatus = "ProviderAttempt", "", "Attempted"
		_, admissionErr = s.service.facts.Append(fact)
	})
	if !observed {
		return Result{}, ErrBinding
	}
	if admissionErr != nil {
		return Result{}, ErrEvidence
	}
	if result.Status != "" {
		return result, nil
	}
	started := time.Now()
	var records []Record
	var err error
	if callCtx.Err() != nil {
		err = callCtx.Err()
	} else if s.ctx.Err() != nil {
		// AfterFunc propagates cancellation asynchronously. A cancelled run
		// cannot dispatch while that callback or terminal publication is pending.
		err = s.ctx.Err()
	} else if !s.current() {
		err = context.Canceled
	} else if s.service.backend == nil {
		err = ErrUnsupported
	} else if request.Operation == v0.MemoryWrite {
		var record Record
		record, err = s.service.backend.Write(callCtx, route, WriteInput{InvocationID: receiptID, ClaimID: s.state.Claim.ID, Body: request.Body})
		if err == nil {
			records = []Record{record}
		}
	} else {
		records, err = s.service.backend.Search(callCtx, route, SearchInput{InvocationID: id, Query: request.Query, Limit: request.Limit})
	}
	result.Status, result.ReasonCode = statusFor(err, request.Operation), "memory-completed"
	if err == nil {
		result = s.reply(result, request, route, records)
	}
	// A known commit stays Written; revocation cannot promise rollback. Read
	// content is never delivered when the admitted context has ended.
	if request.Operation == v0.MemoryRead && (callCtx.Err() != nil || s.ctx.Err() != nil || !time.Now().Before(deadline) || !s.current()) {
		result.Entries, result.Truncated = nil, false
		result.Status = Cancelled
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) || !time.Now().Before(deadline) {
			result.Status = Timeout
		}
	}
	if result.Status != Written && result.Status != Found && result.Status != Empty {
		result.ReasonCode = "memory-" + strings.ToLower(string(result.Status))
		if result.Status == WriteUncertain {
			result.ReasonCode = "memory-write-uncertain"
		}
	}
	metadata := &facts.MemoryMetadata{Status: string(result.Status), DurationMilliseconds: int(time.Since(started).Milliseconds()), Truncated: result.Truncated}
	if result.Reference != "" {
		metadata.References = []string{result.Reference}
	}
	for _, entry := range result.Entries {
		metadata.References = append(metadata.References, entry.Reference)
	}
	metadata.Count = len(metadata.References)
	fact.Kind, fact.ProviderStatus, fact.ReasonCode, fact.Memory = "ProviderOutcome", "Failed", result.ReasonCode, metadata
	if result.Status == Written || result.Status == Found || result.Status == Empty {
		fact.ProviderStatus = "Succeeded"
	} else if result.Status == Cancelled {
		fact.ProviderStatus = "Cancelled"
	}
	if _, err := s.service.facts.Append(fact); err != nil {
		return Result{}, ErrEvidence
	}
	return result, nil
}

func validRequest(r Request) bool {
	if strings.TrimSpace(r.Scope) == "" {
		return false
	}
	if r.Operation == v0.MemoryWrite {
		return validText(r.Body, MaxBodyBytes) && r.Query == "" && r.Limit == 0
	}
	return r.Operation == v0.MemoryRead && validText(r.Query, MaxQueryBytes) && r.Body == "" && r.Limit >= 1 && r.Limit <= MaxLimit
}

func validText(text string, limit int) bool {
	return len(text) <= limit && utf8.ValidString(text) && strings.TrimSpace(text) != "" && !strings.ContainsRune(text, 0)
}

func statusFor(err error, operation string) Status {
	switch {
	case err == nil:
		return Empty
	case operation == v0.MemoryWrite && errors.Is(err, ErrWriteUncertain):
		return WriteUncertain
	case errors.Is(err, context.DeadlineExceeded):
		return Timeout
	case errors.Is(err, context.Canceled):
		return Cancelled
	case errors.Is(err, ErrUnsupported):
		return Unsupported
	case errors.Is(err, ErrUnavailable):
		return Unavailable
	default:
		return Failed
	}
}

func (s *Session) reply(result Result, request Request, namespace Namespace, records []Record) Result {
	invalid := func() Result {
		return Result{InvocationID: result.InvocationID, Status: Failed, ReasonCode: "memory-invalid-response"}
	}
	limit := request.Limit
	if request.Operation == v0.MemoryWrite {
		limit = 1
		if len(records) != 1 {
			return invalid()
		}
	}
	if len(records) > limit {
		return invalid()
	}
	seen := make(map[string]bool, len(records))
	for i, record := range records {
		if record.Namespace != namespace || !facts.ValidMemoryReference(record.Reference) || seen[record.Reference] || !validText(record.Body, MaxBodyBytes) || record.CreatedAt.IsZero() || strings.TrimSpace(record.SourceClaim) == "" {
			return invalid()
		}
		if request.Operation == v0.MemoryWrite && (record.Body != request.Body || record.SourceClaim != s.state.Claim.ID) {
			return invalid()
		}
		if i > 0 && (records[i-1].CreatedAt.Before(record.CreatedAt) || (records[i-1].CreatedAt.Equal(record.CreatedAt) && records[i-1].Reference < record.Reference)) {
			return invalid()
		}
		seen[record.Reference] = true
	}
	if request.Operation == v0.MemoryWrite {
		result.Status, result.Reference = Written, records[0].Reference
		return result
	}
	if len(records) == 0 {
		result.Status = Empty
		return result
	}
	result.Status = Found
	for _, record := range records {
		result.Entries = append(result.Entries, Entry{Reference: record.Reference, Body: record.Body})
		encoded, _ := json.Marshal(result)
		if len(encoded) > MaxReplyBytes {
			result.Entries = result.Entries[:len(result.Entries)-1]
			result.Truncated = true
			break
		}
	}
	return result
}
