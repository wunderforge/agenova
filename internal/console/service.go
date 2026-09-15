// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package console composes the approved local/internal demo; it is not an auth
// system. Principal and runtime are configured by the operator, never the UI.
package console

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

var ErrConflict = errors.New("request already exists")
var ErrCapacity = errors.New("local demo request capacity reached; restart only after active work completes")

type Executor interface {
	Execute(context.Context, v0.SandboxClaimBackendIdentity, workerprotocol.Task, workerprotocol.Handler) (string, error)
}

type record struct {
	view   evidence.View
	cancel context.CancelFunc
}

type Service struct {
	mu        sync.RWMutex
	executeMu chan struct{}
	preset    app.ReferencePrincipalPreset
	runner    *app.RunService
	executor  Executor
	provider  modelprovider.Client
	journal   *facts.Journal
	store     *facts.Store
	records   map[string]*record
	order     []string
	closed    bool
	wg        sync.WaitGroup
}

func NewService(backend runtime.RuntimeBackend, executor Executor, provider modelprovider.Client, preset app.ReferencePrincipalPreset) (*Service, error) {
	if backend == nil || executor == nil || provider == nil {
		return nil, errors.New("runtime, executor and provider are required")
	}
	if _, err := app.NewReferencePrincipalSource(preset); err != nil {
		return nil, err
	}
	s := &Service{preset: preset, executor: executor, provider: provider, journal: facts.NewJournal(), store: facts.NewStore(), records: map[string]*record{}, order: []string{}, executeMu: make(chan struct{}, 1)}
	runner, err := app.NewRunService(backend, app.RunServiceOptions{OnEvent: s.runtimeEvent})
	if err != nil {
		return nil, err
	}
	s.runner = runner
	return s, nil
}

// Submit accepts only the strict canonical JSON surface. The task must contain
// an objective for this example worker; it cannot contain an asserted identity.
func (s *Service) Submit(data []byte) (evidence.View, error) {
	request, validationErr := v0.ParseClaimRequestJSON(data)
	if validationErr != nil {
		return evidence.View{}, validationErr
	}
	objective, ok := request.Spec.Task.Input["objective"].(string)
	if !ok || strings.TrimSpace(objective) == "" || len(objective) > 64<<10 {
		return evidence.View{}, errors.New("demo task requires a bounded objective string")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return evidence.View{}, errors.New("local demo is stopping")
	}
	if _, ok := s.records[request.Metadata.Name]; ok {
		s.mu.Unlock()
		return evidence.View{}, ErrConflict
	}
	if len(s.records) >= 32 {
		s.mu.Unlock()
		return evidence.View{}, ErrCapacity
	}
	// Reserve before issuance to prevent duplicate submissions racing to allocate.
	s.records[request.Metadata.Name] = &record{}
	s.wg.Add(1)
	s.mu.Unlock()
	runOwned := false
	defer func() {
		if !runOwned {
			s.wg.Done()
		}
	}()
	prepared, err := app.PrepareReferenceAssignment(data, s.preset)
	if err != nil {
		s.mu.Lock()
		delete(s.records, request.Metadata.Name)
		s.mu.Unlock()
		return evidence.View{}, err
	}
	ref := request.Metadata.Name
	if err = s.journal.RegisterRequest(ref, prepared.Admission.Principal); err != nil {
		return evidence.View{}, err
	}
	if _, err = s.journal.Append(facts.Fact{Kind: "RequestReceived", RequestRef: ref}); err != nil {
		return evidence.View{}, err
	}
	d := prepared.Admission.Decision
	if _, err = s.journal.Append(facts.Fact{Kind: "RequestResolution", RequestRef: ref, Decision: &d, Result: d.Result, ReasonCode: "assignment-" + strings.ToLower(string(d.Result)), PolicyRef: &d.PolicyRef}); err != nil {
		return evidence.View{}, err
	}
	view := evidence.View{Version: "agenova.evidence/v0", RequestRef: ref, Request: request, State: prepared.Issued, Facts: []facts.Fact{}}
	if d.Result != v0.DecisionResultAllow {
		view.Outcome = &evidence.Outcome{Status: string(d.Result)}
		s.mu.Lock()
		s.records[ref] = &record{view: evidence.Clone(view)}
		s.order = append(s.order, ref)
		s.mu.Unlock()
		return s.QueryRequest(ref)
	}
	if err = s.journal.BindClaim(ref, *prepared.Issued.Claim); err != nil {
		return evidence.View{}, err
	}
	details := []string{}
	for _, change := range prepared.Changes {
		if change.Effective == "" {
			details = append(details, change.Requested+" excluded by the template ceiling.")
		} else {
			details = append(details, change.Field+" capped from "+change.Requested+" to "+change.Effective+".")
		}
	}
	if _, err = s.journal.Append(facts.Fact{Kind: "AuthorityResolved", RequestRef: ref, ClaimID: prepared.Issued.Claim.ID, Authority: prepared.Issued.EffectiveAuthority, AuthorityChanges: prepared.Changes, PolicyRef: &d.PolicyRef, ReasonCode: "admitted-template-ceiling", Reason: strings.Join(details, " ")}); err != nil {
		return evidence.View{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(prepared.Issued.EffectiveAuthority.Runtime.Timeout))
	s.mu.Lock()
	s.records[ref] = &record{view: evidence.Clone(view), cancel: cancel}
	s.order = append(s.order, ref)
	if s.closed {
		cancel()
	}
	s.mu.Unlock()
	runOwned = true
	go func() { defer s.wg.Done(); defer cancel(); s.run(ctx, prepared, objective) }()
	return s.QueryRequest(ref)
}

func (s *Service) runtimeEvent(state *v0.IssuedState, event string) error {
	_, err := s.journal.Append(facts.Fact{Kind: "Runtime", RequestRef: state.RequestRef, ClaimID: state.Claim.ID, BackendIdentity: state.Claim.BackendIdentity, Operation: event, ReasonCode: "runtime-" + event, PolicyRef: &state.PolicyRef})
	if err != nil {
		return err
	}
	if state.Claim.Phase == v0.ClaimPhaseSucceeded || state.Claim.Phase == v0.ClaimPhaseFailed || state.Claim.Phase == v0.ClaimPhaseExpired {
		s.mu.RLock()
		r := s.records[state.RequestRef]
		var cancel context.CancelFunc
		if r != nil {
			cancel = r.cancel
		}
		s.mu.RUnlock()
		if cancel != nil {
			cancel()
		}
	}
	return nil
}

func (s *Service) run(ctx context.Context, p app.PreparedAssignment, objective string) {
	ref, claimID := p.Request.Metadata.Name, p.Issued.Claim.ID
	select {
	case s.executeMu <- struct{}{}:
		defer func() { <-s.executeMu }()
	case <-ctx.Done():
		// RunContext publishes canonical Failed/Expired without acquiring the
		// backend lock or allocating an identity for this queued claim.
	}
	var observed *evidence.ModelResult
	var modelText string
	final, runErr := s.runner.RunContext(ctx, p.Issued, app.ResolvedLaunch{ProfileRef: p.Issued.EffectiveAuthority.Runtime.ProfileRef, TemplateRef: app.ReferenceRuntimeTemplateRef}, func(ctx context.Context) error {
		snapshot, ok := s.runner.ClaimAuthority(claimID)
		if !ok || snapshot.Claim.BackendIdentity == nil {
			return errors.New("running worker binding is missing")
		}
		adapter := &completionAdapter{ctx: ctx, service: s, ref: ref, claimID: claimID, policy: p.Issued.PolicyRef}
		gw := modelgateway.NewGateway(s.runner, nil, s.store, modelgateway.WithAdapter(adapter), modelgateway.WithObserver(func(req modelgateway.Request, d gateway.Decision) error {
			code, reason := string(d.Category), d.Reason
			if code == "" && d.Result == v0.DecisionResultAllow {
				code = "within-effective-authority"
				reason = "Allowed within the active claim's effective authority."
			}
			_, err := s.journal.Append(facts.Fact{Kind: "ModelDecision", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Result: d.Result, ReasonCode: code, Reason: reason, PolicyRef: &p.Issued.PolicyRef, Operation: "model.invoke", Target: req.Profile})
			return err
		}))
		text, err := s.executor.Execute(ctx, *snapshot.Claim.BackendIdentity, workerprotocol.Task{ClaimID: claimID, Objective: objective, ModelProfile: snapshot.EffectiveAuthority.ModelProfile}, func(callCtx context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			if op.ClaimID != claimID || callCtx.Err() != nil {
				return workerprotocol.Reply{}, errors.New("worker session binding or context rejected")
			}
			if op.Kind != "model" {
				return workerprotocol.Reply{}, errors.New("this demo worker supports model operations only")
			}
			d, err := gw.Invoke(modelgateway.Request{ClaimID: claimID, Profile: op.Profile, Parameters: map[string]string{"prompt": op.Prompt}})
			if err != nil {
				return workerprotocol.Reply{}, errors.New("model execution failed; see evidence")
			}
			if d.Result != v0.DecisionResultAllow {
				return workerprotocol.Reply{Allowed: false, Error: "model access was not allowed"}, nil
			}
			result, ok := adapter.result(d.InvocationID)
			if !ok {
				return workerprotocol.Reply{}, errors.New("model response is unavailable")
			}
			observed = &evidence.ModelResult{InvocationID: d.InvocationID, Model: result.Model, ResponseID: result.ResponseID, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens}
			return workerprotocol.Reply{Allowed: true, Text: result.Text}, nil
		})
		if err != nil {
			return err
		}
		if ctx.Err() != nil || observed == nil || strings.TrimSpace(text) == "" {
			return errors.New("no active task-dependent model result")
		}
		modelText = text
		return nil
	})
	status := "Failed"
	if final != nil && final.Claim != nil {
		status = string(final.Claim.Phase)
	}
	if errors.Is(runErr, context.Canceled) {
		status = "Cancelled"
	}
	outcome := &evidence.Outcome{Status: status}
	if final != nil && final.Claim != nil && final.Claim.Phase == v0.ClaimPhaseSucceeded {
		outcome.Text = modelText
		outcome.Model = observed
	}
	if runErr != nil {
		outcome.Failure = "Execution or cleanup failed; inspect the recorded activity."
		if status == "Succeeded" {
			outcome.Failure = "Task completed, but runtime stop or cleanup failed; inspect activity."
		}
	}
	_, _ = s.journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: ref, ClaimID: claimID, Operation: status, ReasonCode: "run-" + status})
	s.mu.Lock()
	if final != nil {
		s.records[ref].view.State = final
	}
	s.records[ref].view.Outcome = outcome
	s.mu.Unlock()
}

type completionAdapter struct {
	ctx          context.Context
	service      *Service
	ref, claimID string
	policy       v0.PolicyReference
	mu           sync.Mutex
	results      map[string]modelprovider.Result
}

func (a *completionAdapter) Invoke(id string, req modelgateway.Request) error {
	if req.ClaimID != a.claimID || a.ctx.Err() != nil {
		return errors.New("model session is inactive")
	}
	if err := app.RequireRunningClaim(a.service.runner, a.claimID); err != nil {
		return err
	}
	if _, err := a.service.journal.Append(facts.Fact{Kind: "ProviderAttempt", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, PolicyRef: &a.policy, Operation: "model.invoke", Target: req.Profile, ProviderStatus: "Attempted"}); err != nil {
		return err
	}
	result, providerErr := a.service.provider.Complete(a.ctx, modelprovider.Request{Profile: req.Profile, Prompt: req.Parameters["prompt"]})
	status := "Succeeded"
	if providerErr != nil {
		status = "Failed"
	}
	if a.ctx.Err() != nil {
		status = "Cancelled"
		providerErr = a.ctx.Err()
	}
	if err := app.RequireRunningClaim(a.service.runner, a.claimID); err != nil {
		status = "Cancelled"
		providerErr = err
	}
	if _, err := a.service.journal.Append(facts.Fact{Kind: "ProviderOutcome", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, PolicyRef: &a.policy, Operation: "model.invoke", Target: req.Profile, ProviderStatus: status}); err != nil {
		return err
	}
	if providerErr != nil {
		return errors.New("provider did not complete successfully")
	}
	a.mu.Lock()
	if a.results == nil {
		a.results = map[string]modelprovider.Result{}
	}
	a.results[id] = result
	a.mu.Unlock()
	return nil
}
func (a *completionAdapter) result(id string) (modelprovider.Result, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.results[id]
	return r, ok
}

func (s *Service) QueryRequest(ref string) (evidence.View, error) {
	s.mu.RLock()
	r, ok := s.records[ref]
	if !ok || r.view.Request == nil {
		s.mu.RUnlock()
		return evidence.View{}, fmt.Errorf("request not found")
	}
	view := evidence.Clone(r.view)
	s.mu.RUnlock()
	if view.State != nil && view.State.Claim != nil {
		if state := s.runner.State(view.State.Claim.ID); state != nil {
			view.State = state
		}
	}
	view.Facts = s.journal.ForRequest(ref)
	return view, nil
}
func (s *Service) QueryClaim(id string) (evidence.View, error) {
	s.mu.RLock()
	ref := ""
	for key, r := range s.records {
		if r.view.State != nil && r.view.State.Claim != nil && r.view.State.Claim.ID == id {
			ref = key
			break
		}
	}
	s.mu.RUnlock()
	if ref == "" {
		return evidence.View{}, errors.New("claim not found")
	}
	return s.QueryRequest(ref)
}
func (s *Service) List() []evidence.View {
	s.mu.RLock()
	order := append([]string{}, s.order...)
	s.mu.RUnlock()
	views := []evidence.View{}
	for _, ref := range order {
		if view, err := s.QueryRequest(ref); err == nil {
			views = append(views, view)
		}
	}
	return views
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for _, r := range s.records {
		if r.cancel != nil {
			r.cancel()
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
}
