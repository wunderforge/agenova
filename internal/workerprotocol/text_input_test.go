// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTextSearchGrammarAndHostValidation(t *testing.T) {
	tools := append(catalog(), Tool{Operation: "web.search", ResourceScope: "web:public-search", Parameter: "query", Description: "Search public text", InputMode: "text", MaxBytes: 64})
	schema, err := ActionSchema(tools)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct{ Properties map[string]map[string]any }
	if json.Unmarshal(schema, &shape) != nil {
		t.Fatal("bad schema")
	}
	if _, present := shape.Properties["input"]["enum"]; present {
		t.Fatal("search incorrectly restricted to fixed inputs")
	}
	if _, ok := FindTool(tools, "web.search", "web:public-search", "Zoom vs Teams"); !ok {
		t.Fatal("text refused")
	}
	for _, probe := range []struct{ tool, scope, input string }{{"web.search", "web:public-search", strings.Repeat("x", 65)}, {"web.search", "web:public-search", "x\ny"}, {"web.search", "web:public-search", " "}, {"web.search", "crm:customer-retention", "Zoom"}, {"repo.read", "repo:acme/payments", "Zoom"}} {
		if _, ok := FindTool(tools, probe.tool, probe.scope, probe.input); ok {
			t.Fatal("invalid input accepted")
		}
	}
	bad := tools[len(tools)-1]
	bad.MaxBytes = 0
	if ValidateTools([]Tool{bad}) == nil {
		t.Fatal("unbounded text allowed")
	}
	if !strings.Contains(LoopPrompt(Task{Tools: tools}, ""), "at most 64 UTF-8 bytes") {
		t.Fatal("worker missing text limit")
	}
}
