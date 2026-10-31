// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"errors"
)

type researchProperties struct {
	actionProperties
	Completion stringSchema `json:"completion"`
	Reason     stringSchema `json:"reason"`
	Target     stringSchema `json:"target"`
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
	empty := stringSchema{Type: "string", Enum: []string{""}}
	finish := base
	finish.Properties.Action = stringSchema{Type: "string", Enum: []string{"finish"}}
	finish.Properties.Tool = empty
	finish.Properties.Resource = empty
	finish.Properties.Input = empty
	finish.Properties.Completion = stringSchema{Type: "string", Enum: []string{"completed", "partial", "incomplete"}}
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
		callRaw, err := ActionSchema(selected)
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
		call.Properties.Answer = empty
		call.Properties.Completion = empty
		call.Properties.Reason = empty
		call.Properties.Target = empty
		variants = append(variants, call)
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
