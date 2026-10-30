// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package evidence defines one transport-independent query representation.
package evidence

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"strings"
)

type View struct {
	AccessAdvice *AccessAdvice    `json:"accessAdvice,omitempty"`
	Version      string           `json:"version"`
	RequestRef   string           `json:"requestRef"`
	Request      *v0.ClaimRequest `json:"request"`
	State        *v0.IssuedState  `json:"state,omitempty"`
	Facts        []facts.Fact     `json:"facts"`
	TraceID      string           `json:"traceId,omitempty"`
	Recovery     *Recovery        `json:"recovery,omitempty"`
	Diagnosis    *Diagnosis       `json:"diagnosis,omitempty"`
	Outcome      *Outcome         `json:"outcome,omitempty"`
}

// AccessAdvice is accepted preflight provenance, not a Work authority or a
// worker model invocation. Hashes avoid storing raw planner prompt/response.
type AccessAdvice struct {
	ID                    string `json:"id"`
	Mode                  string `json:"mode"`
	Engine                string `json:"engine"`
	IntentDigest          string `json:"intentDigest"`
	GuideDigest           string `json:"guideDigest"`
	GuideVersion          string `json:"guideVersion"`
	ProfileID             string `json:"profileID"`
	AcceptedRequestDigest string `json:"acceptedRequestDigest"`
	PlannerModel          string `json:"plannerModel,omitempty"`
	PlannerInputTokens    int    `json:"plannerInputTokens,omitempty"`
	PlannerOutputTokens   int    `json:"plannerOutputTokens,omitempty"`
}

type Outcome struct {
	Completion *Completion  `json:"completion,omitempty"`
	Status     string       `json:"status"`
	Text       string       `json:"text,omitempty"`
	Failure    string       `json:"failure,omitempty"`
	Model      *ModelResult `json:"model,omitempty"`
}

// Completion is the Agent's reported goal outcome, separate from execution.
// It cannot establish authority or independently verified network enforcement.
type Completion struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Target string `json:"target,omitempty"`
}

// ModelResult is observed provider metadata, never an authorization input.
type ModelResult struct {
	InvocationID string `json:"invocationId"`
	Model        string `json:"model"`
	ResponseID   string `json:"responseId,omitempty"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
}

func Clone(view View) View {
	data, _ := json.Marshal(view)
	var copy View
	_ = json.Unmarshal(data, &copy)
	if copy.Facts == nil {
		copy.Facts = []facts.Fact{}
	}
	return copy
}

// Recovery records interruption without resurrecting authority or inventing outcome.
type Recovery struct {
	Code    string `json:"code"`
	Cleanup string `json:"cleanup"`
}
type Diagnosis struct {
	Stage string `json:"stage"`
	Code  string `json:"code"`
}

// Decorate derives only diagnostic projections from committed observations.
func Decorate(v *View) {
	pending := map[string]bool{}
	for _, f := range v.Facts {
		if f.Kind == "ProviderAttempt" {
			pending[f.InvocationID] = true
		}
		if f.Kind == "ProviderOutcome" {
			delete(pending, f.InvocationID)
		}
		if f.Kind == "RunOutcome" && f.ReasonCode != "" {
			stage := "execution"
			if strings.HasPrefix(f.ReasonCode, "provider-") {
				stage = "provider"
			}
			if strings.HasPrefix(f.ReasonCode, "runtime-") {
				stage = "runtime"
			}
			v.Diagnosis = &Diagnosis{Stage: stage, Code: f.ReasonCode}
		}
		if f.Kind == "ProviderOutcome" && f.ProviderStatus == "Failed" {
			v.Diagnosis = &Diagnosis{Stage: "provider", Code: "provider-failed"}
		}
		if f.Kind == "Runtime" && (f.Operation == "AllocationFailed" || f.Operation == "StartFailed") {
			v.Diagnosis = &Diagnosis{Stage: "runtime", Code: "runtime-" + f.Operation}
		}
	}
	if v.State != nil && v.State.Decision.Result == v0.DecisionResultDeny {
		v.Diagnosis = &Diagnosis{Stage: "admission", Code: "policy-denied"}
	}
	if v.Recovery != nil {
		v.Diagnosis = &Diagnosis{Stage: "recovery", Code: "service-restarted"}
		if len(pending) > 0 {
			v.Diagnosis.Code = "provider-outcome-unknown"
		}
	}
}
