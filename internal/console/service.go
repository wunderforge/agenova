// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package console composes the approved local/internal demo; it is not an auth
// system. Principal and runtime are configured by the operator, never the UI.
package console

import (
	"context"
	"encoding/json"
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
	"github.com/wunderforge/agenova/internal/toolgateway"
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
	setup     func() (Setup, error)
	prepare   func([]byte) (app.PreparedAssignment, error)
	configure func(*v0.AgentTemplate) (app.ResolvedLaunch, error)
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

type Options struct {
	Prepare   func([]byte) (app.PreparedAssignment, error)
	Configure func(*v0.AgentTemplate) (app.ResolvedLaunch, error)
	Setup     func() (Setup, error)
}

// SubmissionError exposes an operator-actionable, bounded diagnosis without
// returning raw Kubernetes, provider, or policy-loader error text to callers.
type SubmissionError struct {
	Code    string
	Message string
	Cause   error
}

func (e *SubmissionError) Error() string { return e.Message }
func (e *SubmissionError) Unwrap() error { return e.Cause }

func NewService(backend runtime.RuntimeBackend, executor Executor, provider modelprovider.Client, preset app.ReferencePrincipalPreset) (*Service, error) {
	return NewServiceWithOptions(backend, executor, provider, preset, Options{})
}

func NewServiceWithOptions(backend runtime.RuntimeBackend, executor Executor, provider modelprovider.Client, preset app.ReferencePrincipalPreset, options Options) (*Service, error) {
	if backend == nil || executor == nil || provider == nil {
		return nil, errors.New("runtime, executor and provider are required")
	}
	if _, err := app.NewReferencePrincipalSource(preset); err != nil {
		return nil, err
	}
	if options.Prepare == nil {
		options.Prepare = func(data []byte) (app.PreparedAssignment, error) { return app.PrepareReferenceAssignment(data, preset) }
	}
	if options.Configure == nil {
		options.Configure = func(*v0.AgentTemplate) (app.ResolvedLaunch, error) {
			return app.ResolvedLaunch{TemplateRef: app.ReferenceRuntimeTemplateRef}, nil
		}
	}
	if options.Setup == nil {
		options.Setup = func() (Setup, error) {
			source, err := app.NewReferencePrincipalSource(preset)
			if err != nil {
				return Setup{}, err
			}
			return Setup{Principal: source.Principal(), Template: app.ReferenceTemplate(), Policy: app.ReferencePolicy(), Capabilities: map[string]string{"taskSubmission": "ready", "runtime": "configured", "model": "configured", "tool": "mock", "memory": "notConnected"}, Installation: InstallationIdentity{Kind: "local-demo"}}, nil
		}
	}
	s := &Service{preset: preset, setup: options.Setup, prepare: options.Prepare, configure: options.Configure, executor: executor, provider: provider, journal: facts.NewJournal(), store: facts.NewStore(), records: map[string]*record{}, order: []string{}, executeMu: make(chan struct{}, 1)}
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
	prepared, err := s.prepare(data)
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
	launch, err := s.configure(prepared.Template)
	if err != nil {
		// A failed template installation is a terminal, queryable Work result.
		// The request has already been registered in the journal, so deleting
		// only its record would permanently poison this reference until restart.
		reason := fmt.Sprintf("runtime template configuration failed: %v", err)
		view.State.Claim.Phase = v0.ClaimPhaseFailed
		view.Outcome = &evidence.Outcome{Status: "Failed", Failure: reason}
		_, _ = s.journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: ref, ClaimID: view.State.Claim.ID, Operation: "Failed", ReasonCode: "runtime-template-configuration-failed", Reason: reason})
		s.mu.Lock()
		s.records[ref] = &record{view: evidence.Clone(view)}
		s.order = append(s.order, ref)
		s.mu.Unlock()
		return s.QueryRequest(ref)
	}
	launch.ProfileRef = prepared.Issued.EffectiveAuthority.Runtime.ProfileRef
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
	go func() { defer s.wg.Done(); defer cancel(); s.run(ctx, prepared, launch, objective) }()
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

func (s *Service) run(ctx context.Context, p app.PreparedAssignment, launch app.ResolvedLaunch, objective string) {
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
	final, runErr := s.runner.RunContext(ctx, p.Issued, launch, func(ctx context.Context) error {
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
		mock := &mockReadAdapter{ctx: ctx, service: s, ref: ref, claimID: claimID, policy: p.Issued.PolicyRef, results: map[string]workerprotocol.Reply{}}
		toolGW := toolgateway.NewGateway(s.runner, nil, s.store, toolgateway.WithAdapter(mock), toolgateway.WithObserver(func(req toolgateway.Request, d gateway.Decision) error {
			code, reason := string(d.Category), d.Reason
			if code == "" && d.Result == v0.DecisionResultAllow {
				code = "within-effective-authority"
				reason = "Allowed mock tool within active claim authority."
			}
			_, err := s.journal.Append(facts.Fact{Kind: "ToolDecision", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Result: d.Result, ReasonCode: code, Reason: reason, PolicyRef: &p.Issued.PolicyRef, Operation: "tool.invoke", Target: req.Tool + "." + req.Action})
			return err
		}))
		scope := ""
		for _, tool := range snapshot.EffectiveAuthority.Tools {
			if tool == "git.read" && len(snapshot.EffectiveAuthority.ResourceScopes) > 0 {
				scope = snapshot.EffectiveAuthority.ResourceScopes[0]
			}
		}
		turn := 0
		readFiles := map[string]bool{}
		step := func(operation string) error {
			_, err := s.journal.Append(facts.Fact{Kind: "WorkerActivity", RequestRef: ref, ClaimID: claimID, Operation: operation, Target: fmt.Sprintf("Turn %d", turn), ReasonCode: "agent-action-observed"})
			return err
		}
		text, err := s.executor.Execute(ctx, *snapshot.Claim.BackendIdentity, workerprotocol.Task{ClaimID: claimID, Objective: objective, ModelProfile: snapshot.EffectiveAuthority.ModelProfile, Mode: workerprotocol.ReAct, ResourceScope: scope}, func(callCtx context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			if op.ClaimID != claimID || callCtx.Err() != nil {
				return workerprotocol.Reply{}, errors.New("worker session binding or context rejected")
			}
			if err := app.RequireRunningClaim(s.runner, claimID); err != nil {
				return workerprotocol.Reply{}, err
			}
			if op.Kind == "tool" {
				if op.Tool != "git.read" {
					return workerprotocol.Reply{}, errors.New("unsupported demo tool")
				}
				d, err := toolGW.Invoke(toolgateway.Request{ClaimID: claimID, Tool: "git", Action: "read", ResourceScope: op.ResourceScope, Parameters: map[string]string{"file": op.Input}})
				if err != nil {
					return workerprotocol.Reply{}, errors.New("tool execution failed; inspect evidence")
				}
				if d.Result != v0.DecisionResultAllow {
					return workerprotocol.Reply{Allowed: false, Error: "tool access denied"}, nil
				}
				if err := step("ObservationReceived"); err != nil {
					return workerprotocol.Reply{}, err
				}
				reply := mock.results[d.InvocationID]
				if reply.Allowed && reply.Error == "" && reply.Text != "" {
					readFiles[op.Input] = true
				}
				return reply, nil
			}
			if op.Kind != "model" {
				return workerprotocol.Reply{}, errors.New("unsupported operation")
			}
			turn++
			if turn > workerprotocol.MaxTurns {
				return workerprotocol.Reply{}, workerprotocol.ErrTurnLimit
			}
			if err := step("TurnStarted"); err != nil {
				return workerprotocol.Reply{}, err
			}
			// Trusted demo-edge state chooses the output format. The gateway and
			// RuntimeBackend remain agent/provider agnostic; no prompt matching.
			if scope == "" || turn == workerprotocol.MaxTurns || len(readFiles) == 3 {
				adapter.outputSchema = json.RawMessage(workerprotocol.FinishSchema)
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
			if err := step("ActionReceived"); err != nil {
				return workerprotocol.Reply{}, err
			}
			// Public action shape, not model reasoning or raw model output.
			action, parseErr := workerprotocol.ParseAction(result.Text)
			code, reason := "agent-action-valid", "The model selected a valid final-answer action."
			if parseErr != nil {
				code, reason = workerprotocol.ActionIssue(result.Text)
			} else if action.Action == "tool" {
				code, reason = "agent-action-tool", "The model selected a tool-read action."
			}
			if _, err := s.journal.Append(facts.Fact{Kind: "WorkerActivity", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Operation: "ActionValidated", Target: fmt.Sprintf("Turn %d", turn), ReasonCode: code, Reason: reason}); err != nil {
				return workerprotocol.Reply{}, err
			}
			return workerprotocol.Reply{Allowed: true, Text: result.Text}, nil
		})
		if err != nil {
			return err
		}
		if ctx.Err() != nil || observed == nil || strings.TrimSpace(text) == "" {
			return errors.New("no active task-dependent model result")
		}
		modelText = text
		if err := step("FinalAnswer"); err != nil {
			return err
		}
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
	code := "run-" + status
	if runErr != nil {
		code, outcome.Failure = runFailure(runErr, s.journal.ForRequest(ref))
	}
	_, _ = s.journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: ref, ClaimID: claimID, Operation: status, ReasonCode: code, Reason: outcome.Failure})
	s.mu.Lock()
	if final != nil {
		s.records[ref].view.State = final
	}
	s.records[ref].view.Outcome = outcome
	s.mu.Unlock()
}

// Classify only trusted categories/facts. Never return arbitrary backend,
// provider, model output or operating-system error text to the browser.
func runFailure(err error, recorded []facts.Fact) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "run-cancelled", "The work was cancelled before execution completed."
	case errors.Is(err, app.ErrRunDeadline), errors.Is(err, context.DeadlineExceeded):
		return "run-deadline", "The work exceeded its execution time limit."
	case errors.Is(err, workerprotocol.ErrTurnLimit):
		turns := 0
		for _, f := range recorded {
			if f.Kind == "WorkerActivity" && f.Operation == "TurnStarted" {
				turns++
			}
		}
		for i := len(recorded) - 1; i >= 0; i-- {
			if recorded[i].Operation == "ActionValidated" {
				if turns >= workerprotocol.MaxTurns && recorded[i].ReasonCode == "agent-action-invalid" {
					return "agent-invalid-action-limit", "The agent exhausted its model-turn limit while retrying an invalid tool/finish response format."
				}
				break
			}
		}
		return "agent-turn-limit", "The agent reached its model-turn limit without a final answer."
	case errors.Is(err, workerprotocol.ErrNoFinalResult):
		return "agent-no-final-result", "The agent exited without returning a final answer."
	case errors.Is(err, workerprotocol.ErrInvalidFinalResult):
		return "agent-invalid-final-result", "The agent returned a result that did not match its governed model and tool evidence."
	}
	var sawProvider, sawDecision bool
	for i := len(recorded) - 1; i >= 0; i-- {
		f := recorded[i]
		if f.Kind == "ProviderOutcome" && !sawProvider {
			sawProvider = true
			if f.ProviderStatus == "Failed" {
				return "provider-failed", "The last governed provider call failed. Open the failed call record for context."
			}
		}
		if (f.Kind == "ModelDecision" || f.Kind == "ToolDecision") && !sawDecision {
			sawDecision = true
			if f.Result == v0.DecisionResultDeny {
				return "invocation-denied", "The last governed request was denied; the agent did not complete."
			}
		}
	}
	for _, f := range recorded {
		if f.Operation == "Failed" || f.Operation == "StartFailed" || f.Operation == "AllocateFailed" {
			return "execution-failed", "Execution failed; no more specific failure reason was recorded."
		}
	}
	for _, f := range recorded {
		if f.Operation == "CleanupFailed" || f.Operation == "TerminateFailed" {
			return "runtime-cleanup-failed", "Runtime stop or cleanup failed. Inspect the cleanup records."
		}
	}
	return "execution-failed", "Execution failed; no more specific failure reason was recorded."
}

type completionAdapter struct {
	ctx          context.Context
	service      *Service
	ref, claimID string
	policy       v0.PolicyReference
	mu           sync.Mutex
	results      map[string]modelprovider.Result
	outputSchema json.RawMessage
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
	result, providerErr := a.service.provider.Complete(a.ctx, modelprovider.Request{Profile: req.Profile, Prompt: req.Parameters["prompt"], OutputSchema: a.outputSchema})
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
	reason, reasonCode := "", ""
	if status == "Failed" {
		reasonCode, reason = "model-provider-failed", "The model provider call failed before a usable response was available."
	}
	if status == "Cancelled" {
		reasonCode, reason = "model-call-cancelled", "The model call was cancelled or its claim became inactive."
	}
	if _, err := a.service.journal.Append(facts.Fact{Kind: "ProviderOutcome", RequestRef: a.ref, ClaimID: a.claimID, InvocationID: id, PolicyRef: &a.policy, Operation: "model.invoke", Target: req.Profile, ProviderStatus: status, Reason: reason, ReasonCode: reasonCode}); err != nil {
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
