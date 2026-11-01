// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type researchProperties struct {
	actionProperties
	DecisionSummary *stringSchema `json:"decisionSummary,omitempty"`
	Completion      stringSchema  `json:"completion"`
	Reason          stringSchema  `json:"reason"`
	Target          stringSchema  `json:"target"`
}
type researchVariant struct {
	Type                 string             `json:"type"`
	Properties           researchProperties `json:"properties"`
	Required             []string           `json:"required"`
	AdditionalProperties bool               `json:"additionalProperties"`
}

// GatewayResearchSchema is an optional local decoder profile for research-v1.
// It narrows formatting to tool/finish shapes already accepted by the parser;
// it does not grant tools, introduce an action or adapt the execution contract.
// Other providers keep the existing flat schema unless explicitly wrapped.
func GatewayResearchSchema(raw json.RawMessage, catalog []Tool) (json.RawMessage, error) {
	var base researchVariant
	if json.Unmarshal(raw, &base) != nil || base.Type != "object" || base.Properties.Completion.Type != "string" {
		return nil, errors.New("expected research action schema")
	}
	if base.Properties.DecisionSummary != nil {
		return diagnosticObjectSchema(base, catalog)
	}
	empty := stringSchema{Type: "string", Enum: []string{""}}
	finish := base
	finish.Properties.Action = stringSchema{Type: "string", Enum: []string{"finish"}}
	finish.Properties.Tool = empty
	finish.Properties.Resource = empty
	finish.Properties.Input = empty
	finish.Properties.Completion = stringSchema{Type: "string", Enum: []string{"completed", "partial", "incomplete"}}
	finish.Properties.Answer.MaxLength = 1200
	finish.Properties.Target.MaxLength = 128
	variants := []researchVariant{finish}
	if len(base.Properties.Tool.Enum) > 1 && len(base.Properties.Action.Enum) > 1 {
		// ResearchSchema permits the native diagnostic URL as well as tool
		// inputs. Gateway-only calls need the installed route's original input
		// enum, otherwise a small model repeatedly invents a URL for a dataset.
		var selected []Tool
		for _, tool := range catalog {
			if containsSchemaValue(base.Properties.Tool.Enum, tool.Operation) && containsSchemaValue(base.Properties.Resource.Enum, tool.ResourceScope) {
				selected = append(selected, tool)
			}
		}
		if len(selected) == 0 {
			return nil, ErrToolCatalog
		}
		for _, route := range selected {
			callRaw, err := ActionSchema([]Tool{route})
			if err != nil {
				return nil, err
			}
			var callShape actionSchema
			if json.Unmarshal(callRaw, &callShape) != nil {
				return nil, ErrToolCatalog
			}
			call := base
			call.Properties.actionProperties = callShape.Properties
			call.Properties.Action = stringSchema{Type: "string", Enum: []string{"tool"}}
			call.Properties.Tool.Enum = nonemptySchemaValues(call.Properties.Tool.Enum)
			call.Properties.Resource.Enum = nonemptySchemaValues(call.Properties.Resource.Enum)
			if len(call.Properties.Input.Enum) > 0 {
				call.Properties.Input.Enum = nonemptySchemaValues(call.Properties.Input.Enum)
			}
			if len(call.Properties.Input.Enum) == 0 {
				call.Properties.Input.MaxLength = route.MaxBytes
			}
			call.Properties.Answer = empty
			call.Properties.Completion = empty
			call.Properties.Reason = empty
			call.Properties.Target = empty
			variants = append(variants, call)
		}
	}
	// Do not mix properties and anyOf at the same level: llama.cpp does not
	// support that combination. Normal validation still checks tool/scope pairing.
	out, err := json.Marshal(struct {
		AnyOf []researchVariant `json:"anyOf"`
	}{variants})
	if len(out) > MaxSchemaBytes {
		return nil, ErrToolCatalog
	}
	return out, err
}

// A single required-field object works with both constrained local decoders and
// provider tool schemas. Small cloud models can omit the action or mix fields
// when required fields exist only inside anyOf branches. Action semantics and
// operation-specific input bounds are still checked by the Agent and Gateway.
func diagnosticObjectSchema(base researchVariant, catalog []Tool) (json.RawMessage, error) {
	allowTool := containsSchemaValue(base.Properties.Action.Enum, "tool")
	base.Properties.Action = stringSchema{Type: "string", Enum: []string{"finish"}}
	tools, resources := []string{""}, []string{""}
	maxInput := 0
	fixedOnly := true
	inputs := []string{""}
	var inputHints []string
	for _, tool := range catalog {
		if !allowTool || !containsSchemaValue(base.Properties.Tool.Enum, tool.Operation) || !containsSchemaValue(base.Properties.Resource.Enum, tool.ResourceScope) {
			continue
		}
		if !containsSchemaValue(tools, tool.Operation) {
			tools = append(tools, tool.Operation)
		}
		if !containsSchemaValue(resources, tool.ResourceScope) {
			resources = append(resources, tool.ResourceScope)
		}
		maxInput = max(maxInput, tool.MaxBytes)
		if tool.InputMode == "text" {
			fixedOnly = false
			inputHints = append(inputHints, tool.Operation+": nonempty raw text or URL")
		} else {
			values, _ := json.Marshal(tool.AllowedValues)
			inputHints = append(inputHints, fmt.Sprintf("%s: raw JSON string from %s (do not include %s=)", tool.Operation, values, tool.Parameter))
		}
		for _, value := range tool.AllowedValues {
			maxInput = max(maxInput, len(value))
			if !containsSchemaValue(inputs, value) {
				inputs = append(inputs, value)
			}
		}
	}
	if len(tools) > 1 {
		base.Properties.Action.Enum = []string{"tool", "finish"}
	}
	base.Properties.Tool = stringSchema{Type: "string", Enum: tools}
	base.Properties.Resource = stringSchema{Type: "string", Enum: resources}
	base.Properties.Input = stringSchema{Type: "string", MaxLength: maxInput}
	base.Properties.Input.Description = "Tool-specific raw input value, never a key=value wrapper. " + strings.Join(inputHints, "; ")
	if fixedOnly {
		base.Properties.Input.Enum = inputs
	}
	if len(tools) == 1 {
		base.Properties.Input.Enum = []string{""}
	}
	base.Properties.Answer.MaxLength = 700
	base.Properties.Target.MaxLength = 128
	base.Properties.DecisionSummary.MaxLength = 100
	base.Required = []string{"action", "tool", "resource", "input", "answer", "completion", "reason", "target", "decisionSummary"}
	base.AdditionalProperties = false
	out, err := json.Marshal(base)
	if len(out) > MaxSchemaBytes {
		base.Properties.Input.Description = ""
		out, err = json.Marshal(base)
	}
	if len(out) > MaxSchemaBytes && len(tools) > 1 {
		// Match ActionSchema's bounded fallback for large fixed catalogs. The
		// strict parser still validates the exact route and input value.
		base.Properties.Input.Enum = nil
		out, err = json.Marshal(base)
	}
	if len(out) > MaxSchemaBytes {
		return nil, ErrToolCatalog
	}
	return out, err
}

func containsSchemaValue(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func nonemptySchemaValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
