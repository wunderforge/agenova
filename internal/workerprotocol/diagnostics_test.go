// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticsIsOptInBoundAndDoesNotReplaceFinalModelText(t *testing.T) {
	task := Task{ClaimID: "c", ModelToken: "m", Objective: "goal", ModelProfile: "p", Mode: ReAct, Diagnostics: true, Discovery: true}
	for _, test := range []struct {
		name    string
		enabled bool
		token   string
		count   int
		result  string
		ok      bool
	}{
		{"valid", true, "m", 1, "No evidence.", true}, {"legacy", false, "m", 1, "No evidence.", false}, {"wrong-token", true, "other", 1, "No evidence.", false}, {"spam", true, "m", 4, "No evidence.", false}, {"fabricated-final", true, "m", 1, "invented", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var in, out bytes.Buffer
			e := json.NewEncoder(&in)
			current := task
			current.Diagnostics = test.enabled
			e.Encode(Message{Operation: &Operation{Kind: "model", AgentProtocol: DiagnosticsProtocol, ClaimID: "c", WorkloadToken: "m", Profile: "p", Prompt: PromptPrefix(task)}})
			for i := 0; i < test.count; i++ {
				e.Encode(Message{Operation: &Operation{Kind: "feedback", ClaimID: "c", WorkloadToken: test.token, DiagnosticCode: "finish-accepted", Input: "Agent explanation"}})
			}
			e.Encode(Message{Result: test.result})
			got, err := Exchange(context.Background(), &in, &out, current, func(context.Context, Operation) (Reply, error) {
				return Reply{Allowed: true, Text: `{"action":"finish","answer":"No evidence.","completion":"incomplete","reason":"insufficient-evidence","decisionSummary":"No source returned."}`}, nil
			})
			if (err == nil) != test.ok || test.ok && got != test.result {
				t.Fatalf("binding failed: %s %v", got, err)
			}
		})
	}
}
func TestDiagnosticObjectGrammarKeepsLegacyShapeAndArgumentLimits(t *testing.T) {
	tool := Tool{Operation: "web.search", ResourceScope: "web:public-pages", Description: "search", Parameter: "query", InputMode: "text", MaxBytes: 512}
	legacy, _ := ResearchSchema([]Tool{tool}, false)
	if strings.Contains(string(legacy), "decisionSummary") {
		t.Fatal("legacy changed")
	}
	diagnostic, _ := DiagnosticsSchema([]Tool{tool}, false)
	out, err := GatewayResearchSchema(diagnostic, []Tool{tool})
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		AnyOf      []any                      `json:"anyOf"`
	}
	json.Unmarshal(out, &shape)
	if shape.Type != "object" || len(shape.AnyOf) != 0 || len(shape.Required) != 9 || len(shape.Properties) != 9 || !bytes.Contains(shape.Properties["input"], []byte(`512`)) || !bytes.Contains(shape.Properties["action"], []byte(`"tool"`)) {
		t.Fatal("diagnostic provider schema must require one complete bounded action object")
	}
	finishing, _ := DiagnosticsSchema([]Tool{tool}, true)
	last, err := GatewayResearchSchema(finishing, []Tool{tool})
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(last, &shape)
	if bytes.Contains(shape.Properties["action"], []byte(`"tool"`)) {
		t.Fatal("finishing schema re-enabled a tool action")
	}
	if _, err := ParseAgentAction(`{"action":"finish","answer":"A","completion":"completed","decisionSummary":"actual explanation"}`, nil, DiscoveryProtocol); err == nil {
		t.Fatal("legacy parser accepts new content")
	}
}
