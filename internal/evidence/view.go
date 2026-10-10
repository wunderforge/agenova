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
	Version           string           `json:"version"`
	RequestRef        string           `json:"requestRef"`
	Request           *v0.ClaimRequest `json:"request"`
	State             *v0.IssuedState  `json:"state,omitempty"`
	Facts             []facts.Fact     `json:"facts"`
	Outcome           *Outcome         `json:"outcome,omitempty"`
	ContentRedactions []string         `json:"contentRedactions,omitempty"`
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
	if view.Request != nil && view.Request.Spec.Task != nil && view.Request.Spec.Task.Input != nil && copy.Request != nil && copy.Request.Spec.Task != nil && copy.Request.Spec.Task.Input == nil {
		copy.Request.Spec.Task.Input = map[string]any{}
	}
	return copy
}

const TaskInputRedaction = "request.spec.task.input"
const OutcomeTextRedaction = "outcome.text"

func MemoryRequested(request *v0.ClaimRequest) bool {
	return request != nil && (len(request.Spec.RequestedAccess.MemoryScopes) > 0 || len(request.Spec.RequestedAccess.MemoryOperations) > 0)
}

// ProjectPublic preserves authority evidence without making private task content
// available to submission acknowledgements, queries or lists.
func ProjectPublic(view View) View {
	copy := Clone(view)
	if MemoryRequested(copy.Request) {
		if copy.Request.Spec.Task != nil {
			copy.Request.Spec.Task.Input = map[string]any{}
		}
		if copy.Outcome != nil {
			copy.Outcome.Text = ""
		}
		copy.ContentRedactions = []string{TaskInputRedaction, OutcomeTextRedaction}
	}
	return copy
}

func ValidContentProjection(view View) bool {
	if !MemoryRequested(view.Request) {
		return len(view.ContentRedactions) == 0
	}
	if len(view.ContentRedactions) != 2 || view.Request.Spec.Task == nil || view.Request.Spec.Task.Input == nil || len(view.Request.Spec.Task.Input) != 0 ||
		(view.Outcome != nil && view.Outcome.Text != "") {
		return false
	}
	paths := map[string]bool{}
	for _, path := range view.ContentRedactions {
		if (path != TaskInputRedaction && path != OutcomeTextRedaction) || paths[path] {
			return false
		}
		paths[path] = true
	}
	return true
}

// MarshalPublic is the shared HTTP/CLI serialization boundary.
func MarshalPublic(value any) ([]byte, error) {
	switch v := value.(type) {
	case View:
		return marshalPublicView(v)
	case *View:
		if v == nil {
			return json.Marshal(nil)
		}
		return marshalPublicView(*v)
	case []View:
		items := make([]json.RawMessage, 0, len(v))
		for _, view := range v {
			data, err := marshalPublicView(view)
			if err != nil {
				return nil, err
			}
			items = append(items, data)
		}
		return json.Marshal(items)
	default:
		return json.Marshal(value)
	}
}

// The canonical submission schema remains unchanged; only evidence forces an
// empty input object on the wire instead of its usual omitempty behavior.
func marshalPublicView(view View) ([]byte, error) {
	public := ProjectPublic(view)
	data, err := json.Marshal(public)
	if err != nil || !MemoryRequested(public.Request) || public.Request.Spec.Task == nil {
		return data, err
	}
	var root, request, spec, task map[string]json.RawMessage
	if err = json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(root["request"], &request); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(request["spec"], &spec); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(spec["task"], &task); err != nil {
		return nil, err
	}
	task["input"] = json.RawMessage(`{}`)
	spec["task"], err = json.Marshal(task)
	if err != nil {
		return nil, err
	}
	request["spec"], err = json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	root["request"], err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(root)
}
