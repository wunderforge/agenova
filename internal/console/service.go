// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package console composes the approved local/internal demo; it is not an auth
// system. Principal and runtime are configured by the operator, never the UI.
package console

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/gateway"
	"github.com/wunderforge/agenova/internal/modelgateway"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"github.com/wunderforge/agenova/internal/toolgateway"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"github.com/wunderforge/agenova/internal/workloadidentity"
	"github.com/wunderforge/agenova/internal/workstore"
)

var ErrConflict = errors.New("request already exists")

// The cap at Submit counts every record, including terminal ones, and records
// are retained as the operator's only queryable evidence. Waiting frees nothing.
var ErrCapacity = errors.New("local demo request capacity reached; completed requests are retained for evidence, so only a restart frees capacity")

type Executor interface {
	Execute(context.Context, v0.SandboxClaimBackendIdentity, workerprotocol.Task, workerprotocol.Handler) (string, error)
}

type record struct {
	sessionCheck func(context.Context) error
	view         evidence.View
	cancel       context.CancelFunc
	owner        v0.Principal
}

type Service struct {
	tools *toolbackend.Set
	// appendTool uses the durable journal; tagged acceptance probes can replace it.
	appendTool    func(facts.Fact) (facts.Fact, error)
	durable       DurableWork
	persisted     sync.Map
	storageFailed atomic.Bool
	access        AccessAuthorizer
	mu            sync.RWMutex
	executeMu     chan struct{}
	preset        app.ReferencePrincipalPreset
	setup         func() (Setup, error)
	setupFor      func(context.Context, v0.Principal, string, string) (Setup, error)
	prepare       func([]byte) (app.PreparedAssignment, error)
	prepareFor    func([]byte, v0.Principal) (app.PreparedAssignment, error)
	configure     func(*v0.AgentTemplate) (app.ResolvedLaunch, error)
	runner        *app.RunService
	executor      Executor
	provider      modelprovider.Client
	journal       *facts.Journal
	store         *facts.Store
	records       map[string]*record
	order         []string
	closed        bool
	wg            sync.WaitGroup
	workload      *workloadidentity.Service
}

// AccessAuthorizer checks current server-owned project roles; it cannot grant
// model/tool/runtime authority and does not accept identity from ClaimRequest.
type AccessAuthorizer interface {
	Authorize(context.Context, v0.Principal, string, string) error
}

type Options struct {
	ToolBackend *toolbackend.Set
	Durable     DurableWork
	Access      AccessAuthorizer
	// PrepareFor selects authenticated mode; reference Submit is then disabled.
	PrepareFor func([]byte, v0.Principal) (app.PreparedAssignment, error)
	Prepare    func([]byte) (app.PreparedAssignment, error)
	Configure  func(*v0.AgentTemplate) (app.ResolvedLaunch, error)
	Setup      func() (Setup, error)
	SetupFor   func(context.Context, v0.Principal, string, string) (Setup, error)
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
	if options.PrepareFor != nil {
		if options.Access == nil || options.Setup == nil || options.Configure == nil || options.Prepare != nil {
			return nil, errors.New("authenticated mode requires explicit setup, runtime configuration and access checks")
		}
	} else if _, err := app.NewReferencePrincipalSource(preset); err != nil {
		return nil, err
	}
	if options.Prepare == nil && options.PrepareFor == nil {
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
	s := &Service{tools: options.ToolBackend, durable: options.Durable, preset: preset, setup: options.Setup, setupFor: options.SetupFor, prepare: options.Prepare, prepareFor: options.PrepareFor, access: options.Access, configure: options.Configure, executor: executor, provider: provider, journal: facts.NewJournal(), store: facts.NewStore(), records: map[string]*record{}, order: []string{}, executeMu: make(chan struct{}, 1)}
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, errors.New("workload identity entropy unavailable")
	}
	workload, err := workloadidentity.New(key[:], nil)
	if err != nil {
		return nil, err
	}
	s.workload = workload
	runner, err := app.NewRunService(backend, app.RunServiceOptions{OnEvent: s.runtimeEvent})
	if err != nil {
		return nil, err
	}
	s.runner = runner
	if err := s.configureDurability(); err != nil {
		return nil, err
	}
	s.appendTool = s.journal.Append
	return s, nil
}

// Submit accepts only the strict canonical JSON surface. The task must contain
// an objective for this example worker; it cannot contain an asserted identity.
func (s *Service) Submit(data []byte) (evidence.View, error) {
	if s.prepareFor != nil {
		return evidence.View{}, errors.New("authenticated submission required")
	}
	return s.submit(data, v0.Principal{}, s.prepare, nil, submissionKey{})
}

// SubmitFor accepts only a principal supplied by the trusted authentication boundary.
func (s *Service) SubmitFor(data []byte, principal v0.Principal) (evidence.View, error) {
	return s.submitForSession(data, principal, nil)
}

func (s *Service) submitForSession(data []byte, principal v0.Principal, sessionCheck func(context.Context) error) (evidence.View, error) {
	if sessionCheck != nil && sessionCheck(context.Background()) != nil {
		return evidence.View{}, authentication.ErrForbidden
	}
	if s.prepareFor == nil || s.access == nil || principal.Subject == "" || principal.Team == "" || principal.AuthenticationContext != "oidc" {
		return evidence.View{}, errors.New("authenticated submission unavailable")
	}
	request, err := v0.ParseClaimRequestJSON(data)
	if err != nil {
		return evidence.View{}, err
	}
	if err := s.access.Authorize(context.Background(), principal, "work.submit", request.Spec.ProjectRef); err != nil {
		return evidence.View{}, authentication.ErrForbidden
	}
	key := submissionKey{}
	if s.durable != nil {
		if s.storageFailed.Load() {
			return evidence.View{}, workstore.ErrUnavailable
		}
		key = submissionKey{name: request.Metadata.Name, digest: workstore.Digest(request)}
		view, existing, err := s.durable.Existing(context.Background(), principal, key.name, key.digest)
		if err != nil {
			return evidence.View{}, err
		}
		if existing {
			return view, nil
		}
		request.Metadata.Name = workstore.Reference(principal, key.name)
		data, err = json.Marshal(request)
		if err != nil {
			return evidence.View{}, err
		}
	}
	return s.submit(data, principal, func(data []byte) (app.PreparedAssignment, error) {
		p, err := s.prepareFor(data, principal)
		if err == nil && (p.Admission.Principal != principal || (p.Issued != nil && p.Issued.Principal != principal)) {
			return app.PreparedAssignment{}, errors.New("prepared identity mismatch")
		}
		return p, err
	}, sessionCheck, key)
}

func (s *Service) submit(data []byte, owner v0.Principal, prepare func([]byte) (app.PreparedAssignment, error), sessionCheck func(context.Context) error, key submissionKey) (evidence.View, error) {
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
	if len(s.records) >= 32 && s.durable == nil {
		s.mu.Unlock()
		return evidence.View{}, ErrCapacity
	}
	// Reserve before issuance to prevent duplicate submissions racing to allocate.
	reservation := &record{owner: owner, sessionCheck: sessionCheck}
	s.records[request.Metadata.Name] = reservation
	s.wg.Add(1)
	s.mu.Unlock()
	runOwned := false
	defer func() {
		// Every return below either replaces this reservation with a queryable
		// record or abandons the submission. A surviving reservation never
		// enters order, so List cannot show it and QueryRequest reports it as
		// not found, while it still holds a capacity slot and shadows its own
		// reference until restart. Identity decides, not a flag: the terminal
		// non-Allow record and the template-configuration record replace it and
		// must survive. Release before Done so Close never returns with one left.
		s.mu.Lock()
		if s.records[request.Metadata.Name] == reservation {
			delete(s.records, request.Metadata.Name)
		}
		s.mu.Unlock()
		if !runOwned {
			s.wg.Done()
		}
	}()
	prepared, err := prepare(data)
	if err == nil {
		err = s.checkWorkCatalog(prepared)
	}
	if err != nil {
		// The reservation is released by the deferred cleanup above.
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
		if err := s.persistAdmission(owner, key, view); err != nil {
			return evidence.View{}, err
		}
		s.mu.Lock()
		s.records[ref] = &record{view: evidence.Clone(view), owner: owner, sessionCheck: sessionCheck}
		s.order = append(s.order, ref)
		s.mu.Unlock()
		return s.QueryRequest(ref)
	}
	if err := s.persistAdmission(owner, key, view); err != nil {
		return evidence.View{}, err
	}
	if err = s.journal.BindClaim(ref, *prepared.Issued.Claim); err != nil {
		return evidence.View{}, err
	}
	launch, err := s.configure(prepared.Template)
	if err != nil {
		// A failed template installation is a terminal, queryable Work result.
		// The request has already been registered in the journal, so deleting
		// only its record would permanently poison this reference until restart.
		// Adapter errors may contain raw kubectl output or provider details. The
		// shared evidence API exposes a stable code, never that underlying text.
		reason := "runtime template configuration failed; check the installed runtime and operator logs"
		view.State.Claim.Phase = v0.ClaimPhaseFailed
		view.Outcome = &evidence.Outcome{Status: "Failed", Failure: reason}
		// Attribution is already established. In durable mode a storage failure
		// latches storageFailed; persistFinished below must then refuse success.
		// No worker was allocated on this configuration-failure path.
		_, _ = s.journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: ref, ClaimID: view.State.Claim.ID, Operation: "Failed", ReasonCode: "runtime-template-configuration-failed", Reason: reason})
		s.mu.Lock()
		s.records[ref] = &record{view: evidence.Clone(view), owner: owner, sessionCheck: sessionCheck}
		s.order = append(s.order, ref)
		s.mu.Unlock()
		if err := s.persistFinished(ref); err != nil {
			return evidence.View{}, err
		}
		return s.QueryRequest(ref)
	}
	launch.ProfileRef = prepared.Issued.EffectiveAuthority.Runtime.ProfileRef
	details := []string{}
	resolutionCode := "admitted-template-ceiling"
	for _, change := range prepared.Changes {
		if change.ReasonCode == "outside-policy-constraint" || change.ReasonCode == "policy-timeout-cap" {
			resolutionCode = "admitted-policy-constraint"
		}
		if change.Effective == "" {
			if change.ReasonCode == "outside-policy-constraint" {
				details = append(details, change.Requested+" excluded by the Policy constraint.")
			} else {
				details = append(details, change.Requested+" excluded by the template ceiling.")
			}
		} else {
			details = append(details, change.Field+" capped from "+change.Requested+" to "+change.Effective+".")
		}
	}
	if _, err = s.journal.Append(facts.Fact{Kind: "AuthorityResolved", RequestRef: ref, ClaimID: prepared.Issued.Claim.ID, Authority: prepared.Issued.EffectiveAuthority, AuthorityChanges: prepared.Changes, PolicyRef: &d.PolicyRef, ReasonCode: resolutionCode, Reason: strings.Join(details, " ")}); err != nil {
		return evidence.View{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(prepared.Issued.EffectiveAuthority.Runtime.Timeout))
	s.mu.Lock()
	s.records[ref] = &record{view: evidence.Clone(view), cancel: cancel, owner: owner, sessionCheck: sessionCheck}
	s.order = append(s.order, ref)
	if s.closed {
		cancel()
	}
	s.mu.Unlock()
	if owner.Subject != "" {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if s.authorizeRun(ctx, ref) != nil {
						cancel()
						return
					}
				}
			}
		}()
	}
	runOwned = true
	go func() { defer s.wg.Done(); defer cancel(); s.run(ctx, prepared, launch, objective) }()
	return s.QueryRequest(ref)
}

func (s *Service) runtimeEvent(state *v0.IssuedState, event string) error {
	fact := facts.Fact{Kind: "Runtime", RequestRef: state.RequestRef, ClaimID: state.Claim.ID, BackendIdentity: state.Claim.BackendIdentity, Operation: event, ReasonCode: "runtime-" + event, PolicyRef: &state.PolicyRef}
	var err error
	if s.durable == nil {
		_, err = s.journal.Append(fact)
	} else {
		_, err = s.journal.AppendWithCommit(fact, func(f facts.Fact) error { return s.storageResult(s.durable.Append(context.Background(), f, state)) })
	}
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
	toolEvidenceFailed := false
	final, runErr := s.runner.RunContext(ctx, p.Issued, launch, func(ctx context.Context) error {
		snapshot, ok := s.runner.ClaimAuthority(claimID)
		if !ok || snapshot.Claim.BackendIdentity == nil {
			return errors.New("running worker binding is missing")
		}
		deadline, ok := app.WorkDeadline(ctx)
		if !ok {
			return errors.New("running claim deadline is missing")
		}
		workerID := snapshot.Claim.BackendIdentity.WorkerID
		modelToken, err := s.workload.Issue(claimID, workerID, "model-gateway", deadline)
		if err != nil {
			return errors.New("model workload identity could not be issued")
		}
		toolToken, err := s.workload.Issue(claimID, workerID, "tool-gateway", deadline)
		if err != nil {
			return errors.New("tool workload identity could not be issued")
		}
		adapter := &completionAdapter{ctx: ctx, service: s, ref: ref, claimID: claimID, policy: p.Issued.PolicyRef}
		gw := modelgateway.NewGateway(s.runner, nil, s.store, modelgateway.WithWorkloadIdentity(s.workload), modelgateway.WithAdapter(adapter), modelgateway.WithObserver(func(req modelgateway.Request, d gateway.Decision) error {
			code, reason := string(d.Category), d.Reason
			if code == "" && d.Result == v0.DecisionResultAllow {
				code = "within-effective-authority"
				reason = "Allowed within the active claim's effective authority."
			}
			_, err := s.journal.Append(facts.Fact{Kind: "ModelDecision", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Result: d.Result, ReasonCode: code, Reason: reason, PolicyRef: &p.Issued.PolicyRef, Operation: "model.invoke", Target: req.Profile})
			return err
		}))
		mock := &mockReadAdapter{ctx: ctx, service: s, ref: ref, claimID: claimID, policy: p.Issued.PolicyRef, results: map[string]workerprotocol.Reply{}}

		var selected toolgateway.Adapter = mock
		results := mock.results
		if s.tools != nil {
			configured := &providerToolAdapter{ctx: ctx, claims: s.runner, appendFact: s.appendTool, ref: ref, claimID: claimID, tools: s.tools, results: map[string]workerprotocol.Reply{}}
			selected, results = configured, configured.results
		}
		toolGW := toolgateway.NewGateway(s.runner, nil, s.store, toolgateway.WithWorkloadIdentity(s.workload), toolgateway.WithAdapter(selected), toolgateway.WithObserver(func(req toolgateway.Request, d gateway.Decision) error {
			code, reason := string(d.Category), d.Reason
			if code == "" && d.Result == v0.DecisionResultAllow {
				code = "within-effective-authority"
				reason = "Allowed tool within active claim authority."
			}
			if _, err := s.appendTool(facts.Fact{Kind: "ToolDecision", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Result: d.Result, ReasonCode: code, Reason: reason, PolicyRef: &p.Issued.PolicyRef, Operation: "tool.invoke", Target: req.Tool + "." + req.Action}); err != nil {
				return errToolEvidence
			}
			return nil
		}))
		workTools := s.workCatalog(&snapshot.EffectiveAuthority)
		workSchema, err := workerprotocol.ActionSchema(workTools)
		if err != nil {
			return err
		}
		adapter.outputSchema = json.RawMessage(workSchema)
		turn := 0
		readInputs := map[string]bool{}
		step := func(operation string) error {
			_, err := s.journal.Append(facts.Fact{Kind: "WorkerActivity", RequestRef: ref, ClaimID: claimID, Operation: operation, Target: fmt.Sprintf("Turn %d", turn), ReasonCode: "agent-action-observed"})
			return err
		}
		text, err := s.executor.Execute(ctx, *snapshot.Claim.BackendIdentity, workerprotocol.Task{ClaimID: claimID, ModelToken: modelToken, ToolToken: toolToken, Objective: objective, ModelProfile: snapshot.EffectiveAuthority.ModelProfile, Mode: workerprotocol.ReAct, Tools: workTools}, func(callCtx context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			if op.ClaimID != claimID || callCtx.Err() != nil {
				return workerprotocol.Reply{}, errors.New("worker session binding or context rejected")
			}
			if err := s.authorizeRun(ctx, ref); err != nil {
				return workerprotocol.Reply{}, err
			}
			if err := app.RequireRunningClaim(s.runner, claimID); err != nil {
				return workerprotocol.Reply{}, err
			}
			if op.Kind == "tool" {
				parameter, err := s.toolParameter(op)
				if err != nil {
					return workerprotocol.Reply{}, err
				}
				tool, action := toolbackend.SplitOperation(op.Tool)
				d, err := toolGW.InvokeAuthenticated(toolgateway.Request{ClaimID: op.ClaimID, Tool: tool, Action: action, ResourceScope: op.ResourceScope, Parameters: map[string]string{parameter: op.Input}}, op.WorkloadToken)
				if errors.Is(err, errToolEvidence) {
					toolEvidenceFailed = true
					return workerprotocol.Reply{}, errToolEvidence
				}
				if err != nil {
					return workerprotocol.Reply{}, errors.New("tool execution failed; inspect evidence")
				}
				if d.Result != v0.DecisionResultAllow {
					return workerprotocol.Reply{Allowed: false, Error: "tool access denied"}, nil
				}
				if err := step("ObservationReceived"); err != nil {
					return workerprotocol.Reply{}, err
				}
				reply := results[d.InvocationID]
				if reply.Allowed && reply.Error == "" && reply.Text != "" {
					readInputs[op.Tool+"\x00"+op.ResourceScope+"\x00"+op.Input] = true
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
			if len(workTools) == 0 || turn == workerprotocol.MaxTurns || len(readInputs) == workerprotocol.ToolInputs(workTools) {
				adapter.outputSchema = json.RawMessage(workerprotocol.FinishSchema)
			}
			d, err := gw.InvokeAuthenticated(modelgateway.Request{ClaimID: op.ClaimID, Profile: op.Profile, Parameters: map[string]string{"prompt": op.Prompt}}, op.WorkloadToken)
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
			action, parseErr := workerprotocol.ParseAction(result.Text, workTools)
			code, reason := "agent-action-valid", "The model selected a valid final-answer action."
			if parseErr != nil {
				code, reason = workerprotocol.ActionIssue(result.Text, workTools)
			} else if action.Action == "tool" {
				code, reason = "agent-action-tool", "The model selected an available tool action."
			}
			if _, err := s.journal.Append(facts.Fact{Kind: "WorkerActivity", RequestRef: ref, ClaimID: claimID, InvocationID: d.InvocationID, Operation: "ActionValidated", Target: fmt.Sprintf("Turn %d", turn), ReasonCode: code, Reason: reason}); err != nil {
				return workerprotocol.Reply{}, err
			}
			return workerprotocol.Reply{Allowed: true, Text: result.Text}, nil
		})
		// A lost tool record fails the Work even if the worker carried on.
		if toolEvidenceFailed {
			return errToolEvidence
		}
		if err != nil {
			return err
		}
		if ctx.Err() != nil || observed == nil || strings.TrimSpace(text) == "" {
			return errors.New("no active task-dependent model result")
		}
		if s.durable != nil {
			if err := s.storageResult(s.durable.StageResult(context.Background(), ref, evidence.Outcome{Status: "Succeeded", Text: text, Model: observed})); err != nil {
				return err
			}
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
		if toolEvidenceFailed {
			code, outcome.Failure = "tool-evidence-failed", "A governed tool call could not be fully recorded. If a provider attempt is recorded, treat its external effect as unknown."
		}
	}
	// Attribution is already established. Unlike the reference-only journal,
	// durable writes may fail: its commit callback latches storageFailed and
	// persistFinished refuses publication. Never panic or replay a provider.
	_, _ = s.journal.Append(facts.Fact{Kind: "RunOutcome", RequestRef: ref, ClaimID: claimID, Operation: status, ReasonCode: code, Reason: outcome.Failure})
	s.mu.Lock()
	// A missing key yields a nil *record. Submit installs the record before
	// starting this goroutine and no path deletes an installed record, so it is
	// normally present; guard anyway, because a nil dereference here happens on
	// a goroutine where the panic is unrecoverable and would take down the
	// console together with every other request's evidence. The RunOutcome fact
	// above is already in the journal, so the outcome is not lost.
	if r := s.records[ref]; r != nil {
		if final != nil {
			r.view.State = final
		}
		r.view.Outcome = outcome
	}
	s.mu.Unlock()
	_ = s.persistFinished(ref)
}

// toolParameter checks a call against the installed routes before the
// Gateway, so an argument the provider would refuse never leaves an allowed
// invocation without an outcome. It does not decide authority: an installed
// route outside the claim's grant still reaches the Gateway and is denied
// there with evidence.
func (s *Service) toolParameter(op workerprotocol.Operation) (string, error) {
	if s.tools == nil {
		if op.Tool != "git.read" {
			return "", errors.New("unsupported demo tool")
		}
		return "file", nil
	}
	catalog := s.tools.Catalog()
	parameter, ok := catalog.Parameter(op.Tool, op.ResourceScope)
	if !ok {
		return "", toolbackend.ErrArguments
	}
	if err := catalog.Validate(op.Tool, op.ResourceScope, map[string]string{parameter: op.Input}); err != nil {
		return "", err
	}
	return parameter, nil
}

// checkWorkCatalog refuses an allowed Work whose catalog the worker protocol
// cannot carry, before a claim is journalled or a worker allocated; run would
// otherwise fail it only after allocation.
func (s *Service) checkWorkCatalog(p app.PreparedAssignment) error {
	if p.Admission.Decision.Result != v0.DecisionResultAllow || p.Issued == nil || p.Issued.EffectiveAuthority == nil {
		return nil
	}
	if _, err := workerprotocol.ActionSchema(s.workCatalog(p.Issued.EffectiveAuthority)); err != nil {
		return &SubmissionError{Code: "tool_catalog_unsupported", Message: "Granted tools and resource scopes exceed the worker tool catalog limits; narrow the template or the requested resource scopes.", Cause: err}
	}
	return nil
}

// workCatalog intersects the installed catalog with the claim's effective
// authority. Without a configured backend, a git.read grant selects the
// explicitly labelled synthetic fixture instead; nothing else is advertised.
func (s *Service) workCatalog(authority *v0.EffectiveAuthority) []workerprotocol.Tool {
	if authority == nil {
		return nil
	}
	if s.tools == nil {
		return mockCatalog(authority)
	}
	entries := s.tools.Catalog().Intersect(authority.Tools, authority.ResourceScopes).Entries()
	tools := make([]workerprotocol.Tool, 0, len(entries))
	for _, entry := range entries {
		tools = append(tools, workerprotocol.Tool{Operation: entry.Operation, Description: entry.Description, ResourceScope: entry.ResourceScope, Parameter: entry.Parameter, AllowedValues: entry.AllowedValues})
	}
	return tools
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
	if err := a.service.authorizeRun(a.ctx, a.ref); err != nil {
		return err
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
	// Runtime transitions update state and append their journal fact under one
	// lifecycle lock. Observe both projections within that lock so active Work
	// never exposes a Bound/Running state without its matching Runtime fact.
	if view.State != nil && view.State.Claim != nil {
		s.runner.ObserveState(view.State.Claim.ID, func(state *v0.IssuedState) {
			view.Facts = s.journal.ForRequest(ref)
			if state != nil {
				view.State = state
			}
		})
	} else {
		view.Facts = s.journal.ForRequest(ref)
	}
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
