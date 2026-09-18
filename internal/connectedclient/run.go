// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package connectedclient sends canonical Work through the installed private
// API using a temporary loopback-only Kubernetes port-forward.
package connectedclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"gopkg.in/yaml.v3"
)

type Client struct {
	Context      string
	Namespace    string
	Executable   string
	PollInterval time.Duration
	// OpenTunnel is a test seam; production uses only the fixed private API port.
	OpenTunnel func(context.Context) (endpoint string, close func(), err error)
	HTTPClient *http.Client
}

const maxEvidenceBytes = 1 << 20

var forwardedPort = regexp.MustCompile(`^Forwarding from 127\.0\.0\.1:([0-9]+) -> 8081$`)

// Show fetches one canonical evidence view from the installed Work API after
// the process that submitted it has exited. History is still process-local.
func (c Client) Show(ref string) (evidence.View, error) {
	if !validRequestRef(ref) {
		return evidence.View{}, fmt.Errorf("provide one bounded request reference")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return evidence.View{}, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, nil, ref)
	if err != nil {
		return evidence.View{}, err
	}
	var view evidence.View
	if !decodeStrictJSON(response, &view) || !validEvidenceView(view, ref) {
		return evidence.View{}, fmt.Errorf("installed Work service returned invalid evidence")
	}
	return view, nil
}

// List reads only the installed service's bounded current-session records.
func (c Client) List() ([]evidence.View, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return nil, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	var views []evidence.View
	if !decodeStrictJSON(response, &views) || views == nil || len(views) > 32 {
		return nil, fmt.Errorf("installed Work service returned invalid list")
	}
	seenRefs := make(map[string]struct{}, len(views))
	seenClaims := make(map[string]struct{}, len(views))
	seenBackends := make(map[v0.SandboxClaimBackendIdentity]struct{}, len(views))
	seenFacts := make(map[string]struct{})
	seenSequences := make(map[uint64]struct{})
	invocationOwners := make(map[string]string)
	for _, view := range views {
		if !validRequestRef(view.RequestRef) || !validEvidenceView(view, view.RequestRef) {
			return nil, fmt.Errorf("installed Work service returned invalid list")
		}
		if _, exists := seenRefs[view.RequestRef]; exists {
			return nil, fmt.Errorf("installed Work service returned duplicate Work reference")
		}
		seenRefs[view.RequestRef] = struct{}{}
		if view.State != nil && view.State.Claim != nil {
			claim := view.State.Claim
			if _, exists := seenClaims[claim.ID]; exists {
				return nil, fmt.Errorf("installed Work service returned duplicate Claim identity")
			}
			seenClaims[claim.ID] = struct{}{}
			if claim.BackendIdentity != nil {
				if _, exists := seenBackends[*claim.BackendIdentity]; exists {
					return nil, fmt.Errorf("installed Work service returned duplicate worker identity")
				}
				seenBackends[*claim.BackendIdentity] = struct{}{}
			}
		}
		for _, fact := range view.Facts {
			if _, exists := seenFacts[fact.ID]; exists {
				return nil, fmt.Errorf("installed Work service returned duplicate Fact identity")
			}
			seenFacts[fact.ID] = struct{}{}
			if _, exists := seenSequences[fact.Sequence]; exists {
				return nil, fmt.Errorf("installed Work service returned duplicate Fact sequence")
			}
			seenSequences[fact.Sequence] = struct{}{}
			if fact.InvocationID != "" {
				if owner, exists := invocationOwners[fact.InvocationID]; exists && owner != view.RequestRef {
					return nil, fmt.Errorf("installed Work service returned duplicate invocation identity")
				}
				invocationOwners[fact.InvocationID] = view.RequestRef
			}
		}
	}
	return views, nil
}

func (c Client) RunFile(path string) (evidence.View, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return evidence.View{}, fmt.Errorf("read ClaimRequest: %w", err)
	}
	request, validationErr := v0.ParseClaimRequestYAML(data)
	if validationErr != nil {
		return evidence.View{}, validationErr
	}
	if !validRequestRef(request.Metadata.Name) {
		return evidence.View{}, errors.New("ClaimRequest reference is invalid")
	}
	var value map[string]any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return evidence.View{}, fmt.Errorf("decode ClaimRequest: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return evidence.View{}, fmt.Errorf("encode ClaimRequest: %w", err)
	}
	if _, validationErr := v0.ParseClaimRequestJSON(canonical); validationErr != nil {
		return evidence.View{}, validationErr
	}
	waitBudget := time.Duration(*request.Spec.Runtime.Timeout)
	if waitBudget > 28*time.Minute {
		waitBudget = 28 * time.Minute
	}
	// Submission may spend nearly three minutes in tunnel startup and the
	// installed service's synchronous registration/runtime setup before the
	// worker's runtime budget starts. Keep a separate margin for that phase.
	ctx, cancel := context.WithTimeout(context.Background(), waitBudget+4*time.Minute)
	defer cancel()
	endpoint, closeTunnel, err := c.openTunnel(ctx)
	if err != nil {
		return evidence.View{}, err
	}
	defer closeTunnel()
	response, err := c.call(ctx, endpoint, canonical, "")
	if err != nil {
		return evidence.View{}, err
	}
	view, err := decodeView(response, request.Metadata.Name)
	if err != nil {
		return evidence.View{}, err
	}
	if view.Outcome != nil {
		return view, nil
	}
	interval := c.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return view, fmt.Errorf("waiting for Work evidence: %w", ctx.Err())
		case <-time.After(interval):
		}
		response, err = c.call(ctx, endpoint, nil, request.Metadata.Name)
		if err != nil {
			return view, err
		}
		view, err = decodeView(response, request.Metadata.Name)
		if err != nil {
			return evidence.View{}, err
		}
		if view.Outcome != nil {
			return view, nil
		}
	}
}

func decodeView(data []byte, expectedRef string) (evidence.View, error) {
	var view evidence.View
	if !decodeStrictJSON(data, &view) {
		return evidence.View{}, errors.New("decode installed Work evidence: invalid JSON record")
	}
	if !validEvidenceView(view, expectedRef) {
		return evidence.View{}, errors.New("installed Work service returned incomplete or mismatched evidence")
	}
	return view, nil
}

func decodeStrictJSON(data []byte, value any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return false
	}
	var trailing json.RawMessage
	return decoder.Decode(&trailing) == io.EOF
}

func validEvidenceView(view evidence.View, ref string) bool {
	if view.Version != "agenova.evidence/v0" || view.RequestRef != ref || view.Request == nil ||
		view.Request.Metadata.Name != ref || v0.ValidateClaimRequest(view.Request) != nil || view.Facts == nil {
		return false
	}
	if view.State != nil && (view.State.RequestRef != ref || v0.ValidateIssuedState(view.State) != nil) {
		return false
	}
	if view.State != nil && (strings.TrimSpace(view.State.Principal.Team) == "" || strings.TrimSpace(view.State.Principal.AuthenticationContext) == "") {
		return false
	}
	if view.State != nil && view.State.Claim != nil {
		switch view.State.Claim.Phase {
		case v0.ClaimPhaseBound, v0.ClaimPhaseRunning, v0.ClaimPhaseSucceeded:
			if view.State.Claim.BackendIdentity == nil {
				return false
			}
		}
	}
	if view.State != nil && (view.State.Action.Name != "claim.create" || view.State.Action.Project != view.Request.Spec.ProjectRef || view.State.Action.TemplateRef != view.Request.Spec.TemplateRef) {
		return false
	}
	if view.State != nil && view.State.EffectiveAuthority != nil && !authorityWithinRequest(*view.State.EffectiveAuthority, view.Request) {
		return false
	}
	seenIDs := make(map[string]struct{}, len(view.Facts))
	type invocationStage struct {
		operation      string
		target         string
		providerTarget string
		stage          int
	}
	invocations := make(map[string]invocationStage)
	var lastSequence uint64
	receivedCount := 0
	var receivedSequence uint64
	runOutcomes := 0
	runOutcomeSeen := false
	resolutions := 0
	var resolutionSequence uint64
	boundRecorded := false
	authorityResolved := 0
	runningRecorded := false
	runtimeTerminal := false
	runtimeTerminalOperation := ""
	for _, fact := range view.Facts {
		// RunOutcome is appended after worker teardown. No further activity for
		// this Work can be part of a canonical terminal evidence view.
		if runOutcomeSeen {
			return false
		}
		if receivedCount == 0 && fact.Kind != "RequestReceived" {
			return false
		}
		if fact.Kind == "RequestResolution" && receivedCount != 1 {
			return false
		}
		if fact.Kind != "RequestReceived" && fact.Kind != "RequestResolution" && resolutions != 1 {
			return false
		}
		if fact.ID == "" || fact.Sequence <= lastSequence || fact.Timestamp.IsZero() || fact.Kind == "" || fact.RequestRef != ref {
			return false
		}
		if _, exists := seenIDs[fact.ID]; exists {
			return false
		}
		seenIDs[fact.ID] = struct{}{}
		// Journal sequence is global, so other Works can leave legitimate gaps.
		lastSequence = fact.Sequence
		if fact.ClaimID != "" && (view.State == nil || view.State.Claim == nil || fact.ClaimID != view.State.Claim.ID) {
			return false
		}
		if fact.Kind != "RequestReceived" && fact.Kind != "RequestResolution" && (view.State == nil || view.State.Claim == nil || fact.ClaimID != view.State.Claim.ID) {
			return false
		}
		if fact.InvocationID != "" && (view.State == nil || view.State.Claim == nil || fact.ClaimID != view.State.Claim.ID) {
			return false
		}
		if view.State != nil && view.State.Claim != nil && view.State.Claim.BackendIdentity == nil &&
			(fact.BackendIdentity != nil || fact.Kind == "WorkerActivity" || fact.InvocationID != "" || runtimeProvesAllocation(fact)) {
			return false
		}
		if fact.Kind == "RunOutcome" {
			runOutcomeSeen = true
			if view.State == nil || view.State.Claim == nil || fact.ClaimID != view.State.Claim.ID ||
				!validRunOutcomeOperation(fact.Operation, view.State.Claim.Phase) {
				return false
			}
			if view.Outcome != nil && view.State != nil && view.State.Decision.Result == v0.DecisionResultAllow {
				if fact.Operation != view.Outcome.Status || fact.Reason != view.Outcome.Failure {
					return false
				}
				runOutcomes++
			}
		}
		if fact.Kind == "RequestReceived" {
			if fact.ClaimID != "" || fact.InvocationID != "" || fact.Decision != nil || fact.Result != "" ||
				fact.PolicyRef != nil || fact.Authority != nil || len(fact.AuthorityChanges) != 0 ||
				fact.BackendIdentity != nil || fact.Operation != "" || fact.Target != "" || fact.ProviderStatus != "" {
				return false
			}
			receivedCount++
			receivedSequence = fact.Sequence
		}
		if fact.Kind == "RequestResolution" {
			if view.State == nil || fact.Decision == nil || fact.Result != view.State.Decision.Result || fact.PolicyRef == nil || *fact.PolicyRef != view.State.PolicyRef ||
				fact.ClaimID != "" || fact.InvocationID != "" || fact.Authority != nil || fact.BackendIdentity != nil || len(fact.AuthorityChanges) != 0 {
				return false
			}
			resolutions++
			resolutionSequence = fact.Sequence
		}
		if fact.Kind == "AuthorityResolved" {
			if view.State == nil || view.State.EffectiveAuthority == nil || fact.Authority == nil ||
				!sameAuthority(*fact.Authority, *view.State.EffectiveAuthority) {
				return false
			}
			authorityResolved++
			if authorityResolved != 1 {
				return false
			}
		}
		if fact.Kind == "Runtime" || fact.Kind == "WorkerActivity" || fact.InvocationID != "" {
			if authorityResolved != 1 {
				return false
			}
		}
		if fact.Kind == "Runtime" {
			if view.State == nil || view.State.Claim == nil || !validRuntimeTerminalOperation(fact.Operation, view.State.Claim.Phase) {
				return false
			}
			if runtimeTerminal && !isRuntimeTeardownOperation(fact.Operation) {
				return false
			}
			if fact.Operation == "Running" {
				if runningRecorded || !boundRecorded {
					return false
				}
				runningRecorded = true
			}
			if isRuntimeTerminalOperation(fact.Operation) {
				if runtimeTerminal {
					return false
				}
				runtimeTerminal = true
				runtimeTerminalOperation = fact.Operation
			}
		}
		if fact.Kind == "WorkerActivity" || fact.InvocationID != "" {
			if !runningRecorded || runtimeTerminal &&
				(fact.Kind != "ProviderOutcome" || fact.ProviderStatus != "Cancelled" ||
					(runtimeTerminalOperation != "Cancelled" && runtimeTerminalOperation != "Expired")) {
				return false
			}
		}
		if fact.Kind == "ToolDecision" && fact.Result == v0.DecisionResultAllow &&
			(view.State == nil || view.State.EffectiveAuthority == nil || !slices.Contains(view.State.EffectiveAuthority.Tools, fact.Target)) {
			return false
		}
		if fact.Kind == "ModelDecision" && fact.Result == v0.DecisionResultAllow &&
			(view.State == nil || view.State.EffectiveAuthority == nil || fact.Target == "" || fact.Target != view.State.EffectiveAuthority.ModelProfile) {
			return false
		}
		switch fact.Kind {
		case "ModelDecision", "ToolDecision":
			expected := "model.invoke"
			if fact.Kind == "ToolDecision" {
				expected = "tool.invoke"
			}
			if fact.InvocationID == "" || fact.Operation != expected || !validDecisionResult(fact.Result) ||
				fact.PolicyRef == nil || view.State == nil || *fact.PolicyRef != view.State.PolicyRef {
				return false
			}
			if _, exists := invocations[fact.InvocationID]; exists {
				return false
			}
			stage := 0
			if fact.Result == v0.DecisionResultAllow {
				stage = 1
			}
			invocations[fact.InvocationID] = invocationStage{operation: expected, target: fact.Target, stage: stage}
		case "ProviderAttempt", "ProviderOutcome":
			previous, exists := invocations[fact.InvocationID]
			if !exists || fact.InvocationID == "" || fact.Operation != previous.operation ||
				(previous.operation == "model.invoke" && fact.Target != previous.target) {
				return false
			}
			if fact.Kind == "ProviderAttempt" {
				if previous.stage != 1 || fact.ProviderStatus != "Attempted" || fact.Target == "" {
					return false
				}
				previous.providerTarget = fact.Target
				previous.stage = 2
			} else {
				if previous.stage != 2 || fact.Target != previous.providerTarget ||
					(fact.ProviderStatus != "Succeeded" && fact.ProviderStatus != "Failed" && fact.ProviderStatus != "Cancelled") {
					return false
				}
				previous.stage = 3
			}
			invocations[fact.InvocationID] = previous
		}
		if fact.Kind == "Runtime" && fact.Operation == "Bound" && fact.BackendIdentity != nil &&
			view.State != nil && view.State.Claim != nil && view.State.Claim.BackendIdentity != nil &&
			*fact.BackendIdentity == *view.State.Claim.BackendIdentity {
			boundRecorded = true
		}
		if fact.Decision != nil && (fact.Decision.ID == "" || fact.Decision.PrincipalRef == "" || fact.Decision.Action == "" || fact.Decision.Result == "" || fact.Decision.PolicyRef.ID == "" || fact.Decision.PolicyRef.Version == "") {
			return false
		}
		if fact.Decision != nil && !validDecisionResult(fact.Decision.Result) {
			return false
		}
		if fact.Decision != nil {
			if view.State == nil || *fact.Decision != view.State.Decision {
				return false
			}
			if fact.Result != "" && fact.Result != fact.Decision.Result {
				return false
			}
			if fact.PolicyRef != nil && *fact.PolicyRef != fact.Decision.PolicyRef {
				return false
			}
		}
		if fact.Result != "" && !validDecisionResult(fact.Result) {
			return false
		}
		if fact.PolicyRef != nil && (fact.PolicyRef.ID == "" || fact.PolicyRef.Version == "") {
			return false
		}
		if fact.PolicyRef != nil && view.State != nil && *fact.PolicyRef != view.State.PolicyRef {
			return false
		}
		if fact.Authority != nil && (fact.Authority.ID == "" || fact.Authority.Runtime.ProfileRef == "" || time.Duration(fact.Authority.Runtime.Timeout) <= 0) {
			return false
		}
		if fact.Authority != nil && (view.State == nil || view.State.Claim == nil || view.State.EffectiveAuthority == nil || fact.ClaimID != view.State.Claim.ID || fact.Authority.ID != view.State.Claim.AuthorityRef || !sameAuthority(*fact.Authority, *view.State.EffectiveAuthority)) {
			return false
		}
		if fact.BackendIdentity != nil && (fact.BackendIdentity.Backend == "" || fact.BackendIdentity.WorkerID == "") {
			return false
		}
		if fact.BackendIdentity != nil && (view.State == nil || view.State.Claim == nil || view.State.Claim.BackendIdentity == nil || *fact.BackendIdentity != *view.State.Claim.BackendIdentity) {
			return false
		}
	}
	if receivedCount != 1 || (view.State != nil && (resolutions != 1 || receivedSequence >= resolutionSequence)) {
		return false
	}
	if view.Outcome != nil {
		if view.State == nil || !validOutcomeState(view.Outcome.Status, view.State) {
			return false
		}
		if view.Outcome.Status != "Succeeded" && view.Outcome.Text != "" {
			return false
		}
		if view.State.Claim != nil && view.State.Claim.BackendIdentity != nil && !boundRecorded {
			return false
		}
		if view.State.Decision.Result == v0.DecisionResultAllow && runOutcomes != 1 {
			return false
		}
		if view.Outcome.Status == "Succeeded" && (authorityResolved != 1 || !runningRecorded) {
			return false
		}
		if view.Outcome.Model != nil && (view.Outcome.Model.InvocationID == "" || view.Outcome.Model.Model == "" || view.Outcome.Model.InputTokens < 0 || view.Outcome.Model.OutputTokens < 0) {
			return false
		}
		if view.Outcome.Model != nil && (view.State.EffectiveAuthority == nil || view.Outcome.Status != "Succeeded" || !hasSuccessfulModelInvocation(view.Facts, view.Outcome.Model.InvocationID, view.State.EffectiveAuthority.ModelProfile)) {
			return false
		}
		// Model is optional in the shared Work evidence contract: a successful
		// tool-only agent need not call a model. The installed kind/Ollama E2E
		// separately requires and correlates its real model invocation.
	}
	return true
}

func runtimeProvesAllocation(fact facts.Fact) bool {
	if fact.Kind != "Runtime" {
		return false
	}
	switch fact.Operation {
	case "Bound", "BackendReady", "Running", "StartFailed", "Succeeded", "CleanupSucceeded", "CleanupFailed", "TerminateSucceeded", "TerminateFailed":
		return true
	default:
		return false
	}
}

func validRunOutcomeOperation(operation string, phase v0.ClaimPhase) bool {
	switch phase {
	case v0.ClaimPhaseSucceeded:
		return operation == "Succeeded"
	case v0.ClaimPhaseFailed:
		return operation == "Failed" || operation == "Cancelled"
	case v0.ClaimPhaseExpired:
		return operation == "Expired"
	default:
		return false
	}
}

func isRuntimeTerminalOperation(operation string) bool {
	switch operation {
	case "AllocationFailed", "StartFailed", "Succeeded", "Failed", "Expired", "Cancelled":
		return true
	default:
		return false
	}
}

func isRuntimeTeardownOperation(operation string) bool {
	switch operation {
	case "TerminateSucceeded", "TerminateFailed", "CleanupSucceeded", "CleanupFailed":
		return true
	default:
		return false
	}
}

func validRuntimeTerminalOperation(operation string, phase v0.ClaimPhase) bool {
	if !isRuntimeTerminalOperation(operation) {
		switch operation {
		case "Pending", "Bound", "BackendReady", "Running", "TerminateSucceeded", "TerminateFailed", "CleanupSucceeded", "CleanupFailed":
			return true
		default:
			return false
		}
	}
	switch operation {
	case "Succeeded":
		return phase == v0.ClaimPhaseSucceeded
	case "Expired":
		return phase == v0.ClaimPhaseExpired
	default:
		return phase == v0.ClaimPhaseFailed
	}
}

func authorityWithinRequest(granted v0.EffectiveAuthority, request *v0.ClaimRequest) bool {
	wanted := request.Spec.RequestedAccess
	for _, pair := range []struct{ granted, requested []string }{
		{granted.Tools, wanted.Tools},
		{granted.ResourceScopes, wanted.ResourceScopes},
		{granted.MemoryScopes, wanted.MemoryScopes},
	} {
		for _, value := range pair.granted {
			if !slices.Contains(pair.requested, value) {
				return false
			}
		}
	}
	return granted.ModelProfile == wanted.ModelProfile && granted.Runtime.ProfileRef == request.Spec.Runtime.ProfileRef &&
		time.Duration(granted.Runtime.Timeout) <= time.Duration(*request.Spec.Runtime.Timeout)
}

func sameAuthority(a, b v0.EffectiveAuthority) bool {
	return a.ID == b.ID && slices.Equal(a.Tools, b.Tools) && slices.Equal(a.ResourceScopes, b.ResourceScopes) &&
		a.ModelProfile == b.ModelProfile && slices.Equal(a.MemoryScopes, b.MemoryScopes) && a.Runtime == b.Runtime
}

func hasSuccessfulModelInvocation(recorded []facts.Fact, invocationID, grantedProfile string) bool {
	if grantedProfile == "" {
		return false
	}
	stage := 0
	for _, fact := range recorded {
		if fact.InvocationID != invocationID || fact.Operation != "model.invoke" {
			continue
		}
		switch fact.Kind {
		case "ModelDecision":
			if stage != 0 || fact.Result != v0.DecisionResultAllow || fact.Target != grantedProfile {
				return false
			}
			stage = 1
		case "ProviderAttempt":
			if stage != 1 || fact.Target != grantedProfile || fact.ProviderStatus != "Attempted" {
				return false
			}
			stage = 2
		case "ProviderOutcome":
			if stage != 2 || fact.ProviderStatus != "Succeeded" || fact.Target != grantedProfile {
				return false
			}
			stage = 3
		}
	}
	return stage == 3
}

func validOutcomeState(status string, state *v0.IssuedState) bool {
	if state.Decision.Result != v0.DecisionResultAllow {
		return state.Claim == nil && status == string(state.Decision.Result)
	}
	if state.Claim == nil {
		return false
	}
	switch state.Claim.Phase {
	case v0.ClaimPhaseSucceeded, v0.ClaimPhaseFailed, v0.ClaimPhaseExpired:
		return status == string(state.Claim.Phase) || state.Claim.Phase == v0.ClaimPhaseFailed && status == "Cancelled"
	default:
		return false
	}
}

func validDecisionResult(result v0.DecisionResult) bool {
	switch result {
	case v0.DecisionResultAllow, v0.DecisionResultDeny, v0.DecisionResultApprovalRequired:
		return true
	default:
		return false
	}
}

func validRequestRef(ref string) bool {
	if strings.TrimSpace(ref) == "" || len(ref) > 256 || strings.ContainsAny(ref, "/\\") {
		return false
	}
	for _, ch := range ref {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func (c Client) call(ctx context.Context, endpoint string, input []byte, ref string) ([]byte, error) {
	method, path := http.MethodPost, "/api/requests"
	if input == nil {
		method = http.MethodGet
		if ref != "" {
			if !validRequestRef(ref) {
				return nil, errors.New("Work reference is invalid")
			}
			path = "/api/requests/" + url.PathEscape(ref) + "/evidence"
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+path, bytes.NewReader(input))
	if err != nil {
		return nil, errors.New("installed Work API request is invalid")
	}
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout:   4 * time.Minute,
			Transport: &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("installed Work API is unavailable; check Platform status and local tunnel")
	}
	defer response.Body.Close()
	limit := maxEvidenceBytes
	if input == nil && ref == "" {
		// A bounded current-session list may contain up to 32 individual views.
		limit = 32*maxEvidenceBytes + 4096 // 32 bounded views plus JSON framing.
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("installed Work API response is unavailable or too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Only stable, locally defined diagnostics may reach the operator. Never
		// echo arbitrary API/provider/kubectl error bodies into the CLI.
		var problem struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(data, &problem) == nil {
			if message, ok := safeAPIDiagnostic[problem.Code]; ok {
				return nil, fmt.Errorf("%s (HTTP %d)", message, response.StatusCode)
			}
		}
		return nil, fmt.Errorf("installed Work API rejected request (HTTP %d)", response.StatusCode)
	}
	return data, nil
}

var safeAPIDiagnostic = map[string]string{
	"active_policy_unavailable":   "Active PolicyBundle is unavailable; register or repair the active policy.",
	"active_policy_invalid":       "Active PolicyBundle is invalid; register a valid policy version.",
	"agent_template_unavailable":  "AgentTemplate is unavailable; register the requested template.",
	"assignment_unavailable":      "Assignment could not be resolved; check the registered template and active policy.",
	"authority_missing":           "Issued effective authority is missing; inspect policy and template configuration.",
	"model_profile_unavailable":   "Granted model profile is not installed; update the Platform model configuration.",
	"runtime_profile_unavailable": "Granted runtime profile is not installed; update the Platform runtime configuration.",
	"tool_unsupported":            "Granted tool is not supported by the installed Tool Gateway; narrow the template or install a compatible gateway.",
	"memory_unsupported":          "Granted memory scope is not supported by the installed Memory Interface; narrow the template or install a compatible interface.",
	"request_conflict":            "This Work reference already exists; choose a new request name.",
	"capacity_reached":            "The installed service has reached its current-session Work limit.",
	"not_found":                   "Work was not found in the installed service.",
}

func (c Client) openTunnel(ctx context.Context) (string, func(), error) {
	if c.Context == "" || c.Namespace == "" || c.Namespace == "default" {
		return "", nil, errors.New("installed Kubernetes target is unavailable")
	}
	var endpoint string
	var closeTunnel func()
	var err error
	if c.OpenTunnel != nil {
		endpoint, closeTunnel, err = c.OpenTunnel(ctx)
	} else {
		endpoint, closeTunnel, err = c.startPortForward(ctx)
	}
	if err != nil {
		return "", nil, err
	}
	if closeTunnel == nil {
		closeTunnel = func() {}
	}
	parsed, parseErr := url.Parse(endpoint)
	if parseErr != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || endpoint != "http://127.0.0.1:"+parsed.Port() {
		closeTunnel()
		return "", nil, errors.New("installed Work API tunnel returned a non-loopback endpoint")
	}
	return endpoint, closeTunnel, nil
}

func (c Client) startPortForward(ctx context.Context) (string, func(), error) {
	path := c.Executable
	if path == "" {
		path = "kubectl"
	}
	forwardCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(forwardCtx, path, c.portForwardArgs()...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start")
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return "", nil, errors.New("installed Work API tunnel could not start; check kubectl installation")
	}
	go io.Copy(io.Discard, stderr)
	portReady := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		announced := false
		for scanner.Scan() {
			if match := forwardedPort.FindStringSubmatch(strings.TrimSpace(scanner.Text())); match != nil && !announced {
				port, err := strconv.Atoi(match[1])
				if err == nil && port > 0 && port <= 65535 {
					portReady <- port
					announced = true
				}
			}
		}
		if !announced {
			portReady <- 0
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case port := <-portReady:
		if port > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", port), stop, nil
		}
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
	stop()
	return "", nil, errors.New("installed Work API tunnel is unavailable; check Platform status and Kubernetes port-forward RBAC")
}

func (c Client) portForwardArgs() []string {
	return []string{"--context", c.Context, "--namespace", c.Namespace, "port-forward", "deployment/agenova-control-plane", ":8081", "--address", "127.0.0.1"}
}
