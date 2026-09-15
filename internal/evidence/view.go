// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package evidence defines one transport-independent query representation.
package evidence

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
)

type View struct {
	Version    string           `json:"version"`
	RequestRef string           `json:"requestRef"`
	Request    *v0.ClaimRequest `json:"request"`
	State      *v0.IssuedState  `json:"state,omitempty"`
	Facts      []facts.Fact     `json:"facts"`
	Outcome    *Outcome         `json:"outcome,omitempty"`
}

type Outcome struct {
	Status  string       `json:"status"`
	Text    string       `json:"text,omitempty"`
	Failure string       `json:"failure,omitempty"`
	Model   *ModelResult `json:"model,omitempty"`
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
