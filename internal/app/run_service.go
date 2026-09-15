// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"sync"
	"time"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/runtime"
)

// Runtime-event kinds emitted by RunService. They describe application
// lifecycle and resource operations without turning backend readiness or
// cleanup into a claim outcome.
const (
	EventAllocationFailed = "AllocationFailed"
	EventBound            = "Bound"
	EventBackendReady     = "BackendReady"
	EventStartFailed      = "StartFailed"
	EventRunning          = "Running"
	EventSucceeded        = "Succeeded"
	EventFailed           = "Failed"
	EventExpired          = "Expired"
	EventTerminateOK      = "TerminateSucceeded"
	EventTerminateFailed  = "TerminateFailed"
	EventCleanupOK        = "CleanupSucceeded"
	EventCleanupFailed    = "CleanupFailed"
)

var (
	// ErrRunDeadline marks an application deadline observed at an operation
	// boundary. RuntimeBackend does not promise in-flight cancellation.
	ErrRunDeadline = errors.New("application run deadline reached")
	// ErrInvalidRun marks an input or backend result that cannot be safely
	// correlated with the issued claim.
	ErrInvalidRun = errors.New("invalid application run")
)

// WorkFunc represents the application work performed after the backend has
// acknowledged Start and the authoritative claim has become Running.
type WorkFunc func() error

// ClaimReader is the application-owned lifecycle view used by governed
// interfaces. Implementations return defensive public claim snapshots; they
// never derive authority from backend readiness or resource existence.
type ClaimReader interface {
	Claim(claimID string) (v1alpha1.SandboxClaim, bool)
}

// ClaimAuthoritySnapshot is the minimum system-owned state a governed
// interface needs at invocation time. Claim and EffectiveAuthority come from
// one immutable issued-state snapshot; request intent and backend state are
// not authority sources.
type ClaimAuthoritySnapshot struct {
	Claim              v1alpha1.SandboxClaim
	EffectiveAuthority v1alpha1.EffectiveAuthority
}

// ClaimAuthorityReader extends the lifecycle view with the immutable
// effective authority issued for the same claim. Implementations must return
// defensive copies so gateway callers cannot mutate authoritative state.
type ClaimAuthorityReader interface {
	ClaimReader
	ClaimAuthority(claimID string) (ClaimAuthoritySnapshot, bool)
}

// RequireRunningClaim applies the shared lifecycle eligibility rule used by
// governed interfaces. It fails closed for unavailable, missing, mismatched,
// incomplete, unknown-phase, and non-Running snapshots.
func RequireRunningClaim(reader ClaimReader, claimID string) error {
	if reader == nil {
		return fmt.Errorf("authoritative claim reader is unavailable")
	}
	claim, ok := reader.Claim(claimID)
	if !ok {
		return fmt.Errorf("unknown claim: %s", claimID)
	}
	return RequireRunningClaimSnapshot(claim, claimID)
}

// RequireRunningClaimSnapshot applies the lifecycle eligibility rule to a
// claim already read as part of a larger atomic application snapshot.
func RequireRunningClaimSnapshot(claim v1alpha1.SandboxClaim, claimID string) error {
	if claim.ID == "" || claim.ID != claimID || claim.RequestRef == "" || claim.TemplateRef == "" || claim.AuthorityRef == "" {
		return fmt.Errorf("claim %q has an invalid authoritative snapshot", claimID)
	}
	switch claim.Phase {
	case v1alpha1.ClaimPhasePending, v1alpha1.ClaimPhaseBound, v1alpha1.ClaimPhaseSucceeded, v1alpha1.ClaimPhaseFailed, v1alpha1.ClaimPhaseExpired:
		return fmt.Errorf("claim %q is not running (phase: %s)", claimID, claim.Phase)
	case v1alpha1.ClaimPhaseRunning:
		if claim.BackendIdentity == nil || claim.BackendIdentity.Backend == "" || claim.BackendIdentity.WorkerID == "" {
			return fmt.Errorf("claim %q has an invalid authoritative snapshot", claimID)
		}
		return nil
	default:
		return fmt.Errorf("claim %q has an invalid authoritative phase %q", claimID, claim.Phase)
	}
}

// ResolvedLaunch is the trusted, backend-neutral output of runtime-profile
// resolution. ProfileRef proves which granted runtime profile was resolved;
// TemplateRef is the runtime template selected by that resolution and is not
// the claim's AgentTemplate reference. RunService supplies the ClaimID.
type ResolvedLaunch struct {
	ProfileRef  string
	TemplateRef string
	// Input is task data resolved by the trusted composition layer. Reserved
	// provider-credential fields are rejected before backend allocation.
	Input map[string]string
}

// RunServiceOptions contains the two seams needed for deterministic deadline
// and readiness tests. Zero values select wall-clock behavior.
type RunServiceOptions struct {
	Now          func() time.Time
	Wait         func(time.Duration)
	After        func(time.Duration) <-chan time.Time
	PollInterval time.Duration
}

// RunService owns the authoritative reference lifecycle for issued claims.
// Backend calls are serialized because RuntimeBackend has no general
// concurrency guarantee; claim reads remain concurrency-safe while work runs.
type RunService struct {
	backend runtime.RuntimeBackend
	now     func() time.Time
	wait    func(time.Duration)
	after   func(time.Duration) <-chan time.Time
	poll    time.Duration

	runMu         sync.Mutex
	mu            sync.RWMutex
	state         map[string]*v1alpha1.IssuedState
	identityOwner map[v1alpha1.SandboxClaimBackendIdentity]string
}

var _ ClaimReader = (*RunService)(nil)
var _ ClaimAuthorityReader = (*RunService)(nil)

// NewRunService constructs an application lifecycle owner over one backend.
func NewRunService(backend runtime.RuntimeBackend, options RunServiceOptions) (*RunService, error) {
	if backend == nil {
		return nil, fmt.Errorf("%w: runtime backend is required", ErrInvalidRun)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	wait := options.Wait
	if wait == nil {
		wait = time.Sleep
	}
	after := options.After
	if after == nil {
		after = time.After
	}
	poll := options.PollInterval
	if poll <= 0 {
		poll = 10 * time.Millisecond
	}
	return &RunService{
		backend:       backend,
		now:           now,
		wait:          wait,
		after:         after,
		poll:          poll,
		state:         make(map[string]*v1alpha1.IssuedState),
		identityOwner: make(map[v1alpha1.SandboxClaimBackendIdentity]string),
	}, nil
}

// Claim returns a defensive public claim snapshot. It is the authoritative
// Running-only read boundary consumed by Ticket #32; backend observation is
// deliberately not exposed as claim state.
func (s *RunService) Claim(claimID string) (v1alpha1.SandboxClaim, bool) {
	if s == nil {
		return v1alpha1.SandboxClaim{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.state[claimID]
	if !ok || state.Claim == nil {
		return v1alpha1.SandboxClaim{}, false
	}
	return cloneClaim(*state.Claim), true
}

// ClaimAuthority returns one defensive claim-plus-authority snapshot from
// the application state owner. A stored but malformed snapshot remains
// observable to gateway validation instead of being disguised as unknown.
func (s *RunService) ClaimAuthority(claimID string) (ClaimAuthoritySnapshot, bool) {
	if s == nil {
		return ClaimAuthoritySnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.state[claimID]
	if !ok {
		return ClaimAuthoritySnapshot{}, false
	}
	var snapshot ClaimAuthoritySnapshot
	if state.Claim != nil {
		snapshot.Claim = cloneClaim(*state.Claim)
	}
	if state.EffectiveAuthority != nil {
		snapshot.EffectiveAuthority = cloneEffectiveAuthority(*state.EffectiveAuthority)
	}
	return snapshot, true
}

// Run consumes one accepted Allow/Pending issued snapshot and drives the
// backend-neutral lifecycle. The returned state is a defensive copy of the
// same authoritative evidence model exposed during the run.
func (s *RunService) Run(issued *v1alpha1.IssuedState, launch ResolvedLaunch, work WorkFunc) (*v1alpha1.IssuedState, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: run service is required", ErrInvalidRun)
	}
	if work == nil {
		return nil, fmt.Errorf("%w: work callback is required", ErrInvalidRun)
	}
	// Snapshot caller-owned launch input before validation and before waiting
	// behind runMu. The same immutable map is then forwarded to allocation.
	launch.Input = cloneStringMap(launch.Input)
	initial, err := validateInitialRun(issued, launch)
	if err != nil {
		return nil, err
	}

	s.runMu.Lock()
	defer s.runMu.Unlock()

	claimID := initial.Claim.ID
	if err := s.insert(initial); err != nil {
		return nil, err
	}
	deadline := s.now().Add(time.Duration(initial.EffectiveAuthority.Runtime.Timeout))
	if s.deadlineReached(deadline) {
		return s.finishWithoutIdentity(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
	}

	allocateRequest := runtime.AllocateRequest{
		ClaimID:     claimID,
		TemplateRef: launch.TemplateRef,
		Input:       cloneStringMap(launch.Input),
	}
	allocation, allocateErr := s.backend.Allocate(allocateRequest)
	if s.deadlineReached(deadline) {
		state, transitionErr := s.finishWithoutIdentity(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
		if allocateErr == nil && validateAllocation(claimID, allocation) == nil {
			if reserveErr := s.reserveIdentity(claimID, allocation.Identity); reserveErr == nil {
				return s.teardown(state, allocation.Identity, transitionErr)
			}
		}
		return state, transitionErr
	}
	if allocateErr != nil {
		return s.finishWithoutIdentity(claimID, v1alpha1.ClaimPhaseFailed, EventAllocationFailed, fmt.Errorf("allocate runtime: %w", allocateErr))
	}
	if err := validateAllocation(claimID, allocation); err != nil {
		return s.finishWithoutIdentity(claimID, v1alpha1.ClaimPhaseFailed, EventAllocationFailed, err)
	}
	identity := allocation.Identity
	if err := s.reserveIdentity(claimID, identity); err != nil {
		return s.finishWithoutIdentity(claimID, v1alpha1.ClaimPhaseFailed, EventAllocationFailed, err)
	}
	if err := s.transition(claimID, v1alpha1.ClaimPhaseBound, EventBound, &identity); err != nil {
		return s.current(claimID), err
	}

	for {
		if s.deadlineReached(deadline) {
			state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
			return s.teardown(state, identity, transitionErr)
		}
		observation, observeErr := s.backend.Observe(identity)
		if s.deadlineReached(deadline) {
			state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
			return s.teardown(state, identity, transitionErr)
		}
		if observeErr != nil {
			state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseFailed, EventFailed, fmt.Errorf("observe runtime: %w", observeErr))
			return s.teardown(state, identity, transitionErr)
		}
		if err := validateObservation(claimID, identity, observation); err != nil {
			state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseFailed, EventFailed, err)
			return s.teardown(state, identity, transitionErr)
		}
		if observation.Ready {
			if err := s.recordEvent(claimID, EventBackendReady); err != nil {
				return s.current(claimID), err
			}
			break
		}
		s.wait(s.poll)
	}

	if s.deadlineReached(deadline) {
		state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
		return s.teardown(state, identity, transitionErr)
	}
	startErr := s.backend.Start(identity)
	if s.deadlineReached(deadline) {
		state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
		return s.teardown(state, identity, transitionErr)
	}
	if startErr != nil {
		state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseFailed, EventStartFailed, fmt.Errorf("start runtime: %w", startErr))
		return s.teardown(state, identity, transitionErr)
	}
	if err := s.transition(claimID, v1alpha1.ClaimPhaseRunning, EventRunning, nil); err != nil {
		return s.current(claimID), err
	}

	workDone := make(chan error, 1)
	go func() {
		workDone <- work()
	}()
	remaining := deadline.Sub(s.now())
	if remaining <= 0 {
		state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
		return s.teardown(state, identity, transitionErr)
	}
	var workErr error
	select {
	case workErr = <-workDone:
	case <-s.after(remaining):
		state, transitionErr := s.finish(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, ErrRunDeadline)
		return s.teardown(state, identity, transitionErr)
	}
	var outcomeErr error
	switch {
	case s.deadlineReached(deadline):
		outcomeErr = ErrRunDeadline
		if err := s.transition(claimID, v1alpha1.ClaimPhaseExpired, EventExpired, nil); err != nil {
			outcomeErr = errors.Join(outcomeErr, err)
		}
	case workErr != nil:
		outcomeErr = fmt.Errorf("application work: %w", workErr)
		if err := s.transition(claimID, v1alpha1.ClaimPhaseFailed, EventFailed, nil); err != nil {
			outcomeErr = errors.Join(outcomeErr, err)
		}
	default:
		if err := s.transition(claimID, v1alpha1.ClaimPhaseSucceeded, EventSucceeded, nil); err != nil {
			outcomeErr = err
		}
	}

	return s.teardown(s.current(claimID), identity, outcomeErr)
}

func validateInitialRun(issued *v1alpha1.IssuedState, launch ResolvedLaunch) (*v1alpha1.IssuedState, error) {
	if validationErr := v1alpha1.ValidateIssuedState(issued); validationErr != nil {
		return nil, fmt.Errorf("%w: issued state: %v", ErrInvalidRun, validationErr)
	}
	if issued.Decision.Result != v1alpha1.DecisionResultAllow || issued.Claim == nil || issued.EffectiveAuthority == nil {
		return nil, fmt.Errorf("%w: run requires an Allow issued state", ErrInvalidRun)
	}
	if issued.Claim.Phase != v1alpha1.ClaimPhasePending || issued.Claim.BackendIdentity != nil {
		return nil, fmt.Errorf("%w: run requires an unbound Pending claim", ErrInvalidRun)
	}
	if launch.ProfileRef == "" || launch.ProfileRef != issued.EffectiveAuthority.Runtime.ProfileRef {
		return nil, fmt.Errorf("%w: launch profile %q does not match granted runtime profile %q", ErrInvalidRun, launch.ProfileRef, issued.EffectiveAuthority.Runtime.ProfileRef)
	}
	if launch.TemplateRef == "" {
		return nil, fmt.Errorf("%w: resolved runtime template is required", ErrInvalidRun)
	}
	if key, found := v1alpha1.FindReservedCredentialFieldName(launch.Input); found {
		return nil, fmt.Errorf("%w: launch input %q is credential-bearing; provider credentials stay behind gateway adapters", ErrInvalidRun, key)
	}
	return cloneIssuedState(issued), nil
}

func validateAllocation(claimID string, allocation runtime.Allocation) error {
	if allocation.ClaimID != claimID {
		return fmt.Errorf("%w: allocation claim %q does not match %q", ErrInvalidRun, allocation.ClaimID, claimID)
	}
	if allocation.Identity.Backend == "" || allocation.Identity.WorkerID == "" {
		return fmt.Errorf("%w: allocation returned an incomplete backend identity", ErrInvalidRun)
	}
	return nil
}

func validateObservation(claimID string, identity v1alpha1.SandboxClaimBackendIdentity, observation runtime.Observation) error {
	if observation.ClaimID != claimID {
		return fmt.Errorf("%w: observation claim %q does not match %q", ErrInvalidRun, observation.ClaimID, claimID)
	}
	if observation.Identity != identity {
		return fmt.Errorf("%w: observation backend identity does not match the claim binding", ErrInvalidRun)
	}
	return nil
}

func (s *RunService) insert(state *v1alpha1.IssuedState) error {
	claimID := state.Claim.ID
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.state[claimID]; exists {
		return fmt.Errorf("%w: claim %q already exists", ErrInvalidRun, claimID)
	}
	s.state[claimID] = cloneIssuedState(state)
	return nil
}

func (s *RunService) reserveIdentity(claimID string, identity v1alpha1.SandboxClaimBackendIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner, exists := s.identityOwner[identity]; exists && owner != claimID {
		return fmt.Errorf("%w: backend identity %s/%s is already bound to claim %q", ErrInvalidRun, identity.Backend, identity.WorkerID, owner)
	}
	s.identityOwner[identity] = claimID
	return nil
}

func (s *RunService) deadlineReached(deadline time.Time) bool {
	return !s.now().Before(deadline)
}

func (s *RunService) finishWithoutIdentity(claimID string, phase v1alpha1.ClaimPhase, event string, cause error) (*v1alpha1.IssuedState, error) {
	return s.finish(claimID, phase, event, cause)
}

func (s *RunService) finish(claimID string, phase v1alpha1.ClaimPhase, event string, cause error) (*v1alpha1.IssuedState, error) {
	if err := s.transition(claimID, phase, event, nil); err != nil {
		return s.current(claimID), errors.Join(cause, err)
	}
	return s.current(claimID), cause
}

func (s *RunService) teardown(state *v1alpha1.IssuedState, identity v1alpha1.SandboxClaimBackendIdentity, cause error) (*v1alpha1.IssuedState, error) {
	claimID := state.Claim.ID
	var teardownErrs []error
	if err := s.backend.Terminate(identity); err != nil {
		teardownErrs = append(teardownErrs, fmt.Errorf("terminate runtime: %w", err))
		if recordErr := s.recordEvent(claimID, EventTerminateFailed); recordErr != nil {
			teardownErrs = append(teardownErrs, recordErr)
		}
	} else {
		if recordErr := s.recordEvent(claimID, EventTerminateOK); recordErr != nil {
			teardownErrs = append(teardownErrs, recordErr)
		}
	}
	cleanup, err := s.backend.Cleanup(identity)
	if err != nil {
		teardownErrs = append(teardownErrs, fmt.Errorf("cleanup runtime: %w", err))
		if recordErr := s.recordEvent(claimID, EventCleanupFailed); recordErr != nil {
			teardownErrs = append(teardownErrs, recordErr)
		}
	} else if cleanup.Identity != identity || !cleanup.Released {
		teardownErrs = append(teardownErrs, fmt.Errorf("%w: cleanup returned uncorrelated or unreleased evidence", ErrInvalidRun))
		if recordErr := s.recordEvent(claimID, EventCleanupFailed); recordErr != nil {
			teardownErrs = append(teardownErrs, recordErr)
		}
	} else {
		if recordErr := s.recordEvent(claimID, EventCleanupOK); recordErr != nil {
			teardownErrs = append(teardownErrs, recordErr)
		}
	}
	return s.current(claimID), errors.Join(append([]error{cause}, teardownErrs...)...)
}

func (s *RunService) transition(claimID string, to v1alpha1.ClaimPhase, event string, identity *v1alpha1.SandboxClaimBackendIdentity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.state[claimID]
	if !ok || current.Claim == nil {
		return fmt.Errorf("%w: unknown claim %q", ErrInvalidRun, claimID)
	}
	if !allowedTransition(current.Claim.Phase, to) {
		return fmt.Errorf("%w: invalid claim transition %s -> %s", ErrInvalidRun, current.Claim.Phase, to)
	}
	next := cloneIssuedState(current)
	if to == v1alpha1.ClaimPhaseBound {
		if identity == nil || identity.Backend == "" || identity.WorkerID == "" {
			return fmt.Errorf("%w: Bound requires a complete backend identity", ErrInvalidRun)
		}
		bound := *identity
		next.Claim.BackendIdentity = &bound
	} else if identity != nil {
		return fmt.Errorf("%w: backend identity may only be attached while publishing Bound", ErrInvalidRun)
	}
	next.Claim.Phase = to
	next.Evidence.RuntimeEvents = append(next.Evidence.RuntimeEvents, v1alpha1.EvidenceRuntimeEvent{Kind: event})
	if validationErr := v1alpha1.ValidateIssuedState(next); validationErr != nil {
		return fmt.Errorf("%w: transition produced invalid issued state: %v", ErrInvalidRun, validationErr)
	}
	s.state[claimID] = next
	return nil
}

func (s *RunService) recordEvent(claimID, event string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.state[claimID]
	if !ok {
		return fmt.Errorf("%w: unknown claim %q", ErrInvalidRun, claimID)
	}
	next := cloneIssuedState(current)
	next.Evidence.RuntimeEvents = append(next.Evidence.RuntimeEvents, v1alpha1.EvidenceRuntimeEvent{Kind: event})
	s.state[claimID] = next
	return nil
}

func (s *RunService) current(claimID string) *v1alpha1.IssuedState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneIssuedState(s.state[claimID])
}

func allowedTransition(from, to v1alpha1.ClaimPhase) bool {
	switch from {
	case v1alpha1.ClaimPhasePending:
		return to == v1alpha1.ClaimPhaseBound || to == v1alpha1.ClaimPhaseFailed || to == v1alpha1.ClaimPhaseExpired
	case v1alpha1.ClaimPhaseBound:
		return to == v1alpha1.ClaimPhaseRunning || to == v1alpha1.ClaimPhaseFailed || to == v1alpha1.ClaimPhaseExpired
	case v1alpha1.ClaimPhaseRunning:
		return to == v1alpha1.ClaimPhaseSucceeded || to == v1alpha1.ClaimPhaseFailed || to == v1alpha1.ClaimPhaseExpired
	default:
		return false
	}
}

func cloneIssuedState(source *v1alpha1.IssuedState) *v1alpha1.IssuedState {
	if source == nil {
		return nil
	}
	copy := *source
	if source.EffectiveAuthority != nil {
		authority := cloneEffectiveAuthority(*source.EffectiveAuthority)
		copy.EffectiveAuthority = &authority
	}
	if source.Claim != nil {
		claim := cloneClaim(*source.Claim)
		copy.Claim = &claim
	}
	copy.Evidence.DecisionIDs = append([]string{}, source.Evidence.DecisionIDs...)
	copy.Evidence.RuntimeEvents = append([]v1alpha1.EvidenceRuntimeEvent{}, source.Evidence.RuntimeEvents...)
	copy.Evidence.ToolInvocations = append([]v1alpha1.EvidenceToolInvocation{}, source.Evidence.ToolInvocations...)
	copy.Evidence.ModelInvocations = append([]v1alpha1.EvidenceModelInvocation{}, source.Evidence.ModelInvocations...)
	return &copy
}

func cloneEffectiveAuthority(source v1alpha1.EffectiveAuthority) v1alpha1.EffectiveAuthority {
	copy := source
	copy.Tools = append([]string(nil), source.Tools...)
	copy.ResourceScopes = append([]string(nil), source.ResourceScopes...)
	copy.MemoryScopes = append([]string(nil), source.MemoryScopes...)
	return copy
}

func cloneClaim(source v1alpha1.SandboxClaim) v1alpha1.SandboxClaim {
	copy := source
	if source.BackendIdentity != nil {
		identity := *source.BackendIdentity
		copy.BackendIdentity = &identity
	}
	return copy
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
