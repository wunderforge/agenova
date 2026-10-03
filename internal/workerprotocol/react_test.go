// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func catalog() []Tool {
	return []Tool{
		{Operation: "repo.read", Description: "Read one allowlisted file.", ResourceScope: "repo:acme/payments", Parameter: "file", AllowedValues: []string{"README.md", "logs/timeout.log"}},
		{Operation: "repo.read", Description: "Read one allowlisted file.", ResourceScope: "repo:acme/ledger", Parameter: "file", AllowedValues: []string{"ledger.md"}},
	}
}

func TestActionIssuesNeverIncludeModelContent(t *testing.T) {
	for _, text := range []string{
		`{"action":"finish","tool":"repo.read","resource":"","input":"README.md","answer":"private answer"}`,
		`{"action":"tool","tool":"repo.read","resource":"repo:acme/payments","input":"README.md","answer":"private answer"}`,
		`{"action":"tool","tool":"repo.read","resource":"repo:acme/payments","input":"private-path"}`,
		"private invalid JSON",
	} {
		code, reason := ActionIssue(text, catalog())
		if code != "agent-action-invalid" || reason == "" || strings.Contains(reason, "private") {
			t.Fatalf("unsafe/missing issue: %s", reason)
		}
	}
	if code, reason := ActionIssue(`{"action":"finish","answer":"ok"}`, catalog()); code != "" || reason != "" {
		t.Fatal("valid action marked invalid")
	}
}

func TestActionBoundaryFollowsTheWorkCatalog(t *testing.T) {
	for _, text := range []string{
		`{}`,
		`{"action":"tool","tool":"shell.exec","resource":"repo:acme/payments","input":"ls"}`,
		`{"action":"tool","tool":"git.read","resource":"repo:acme/payments","input":"README.md"}`,
		// Each field is individually valid, but the combination is not one entry.
		`{"action":"tool","tool":"repo.read","resource":"repo:acme/ledger","input":"README.md"}`,
		`{"action":"tool","tool":"repo.read","input":"README.md"}`,
		`{"action":"finish","answer":""}`,
		`{"action":"finish","answer":"ok","thought":"private"}`,
		`{"action":"finish","answer":"ok"} {}`,
		`{"action":"finish","resource":"repo:acme/payments","answer":"ok"}`,
		`{"action":"tool","tool":"repo.read","resource":"repo:acme/payments","input":"README.md","answer":"fake"}`,
	} {
		if _, err := ParseAction(text, catalog()); err == nil {
			t.Fatalf("invalid action accepted: %s", text)
		}
	}
	for _, text := range []string{
		`{"action":"tool","tool":"repo.read","resource":"repo:acme/ledger","input":"ledger.md","answer":""}`,
		`{"action":"finish","tool":"","resource":"","input":"","answer":"Use a shared deadline."}`,
	} {
		if _, err := ParseAction(text, catalog()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ParseAction(`{"action":"tool","tool":"repo.read","resource":"repo:acme/ledger","input":"ledger.md"}`, nil); err == nil {
		t.Fatal("a tool action was accepted without a catalog")
	}
}

func TestActionSchemaIsDerivedFromCatalogAndBounded(t *testing.T) {
	schema, err := ActionSchema(catalog())
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"tool": ",repo.read", "resource": ",repo:acme/ledger,repo:acme/payments", "input": ",README.md,ledger.md,logs/timeout.log"}
	for field, values := range want {
		if got := strings.Join(parsed.Properties[field].Enum, ","); got != values {
			t.Fatalf("%s enum %q, want %q", field, got, values)
		}
	}
	if strings.Contains(string(schema), "git.read") {
		t.Fatal("schema advertises a tool outside the catalog")
	}
	if schema, err := ActionSchema(nil); err != nil || string(schema) != FinishSchema {
		t.Fatal("an empty catalog must produce the finish-only schema")
	}
	// Many long inputs drop the input enum but keep the schema in budget.
	large := []Tool{{Operation: "repo.read", Description: "d", ResourceScope: "repo:a/b", Parameter: "file"}}
	for i := 0; i < 64; i++ {
		large[0].AllowedValues = append(large[0].AllowedValues, fmt.Sprintf("%0200d", i))
	}
	schema, err = ActionSchema(large)
	if err != nil || len(schema) > MaxSchemaBytes || strings.Contains(string(schema), "00000000063") {
		t.Fatalf("oversized input enum was not dropped: %d bytes, %v", len(schema), err)
	}
}

func TestValidateToolsRejectsUnboundedCatalogs(t *testing.T) {
	for name, tools := range map[string][]Tool{
		"no values":    {{Operation: "repo.read", Description: "d", ResourceScope: "repo:a/b", Parameter: "file"}},
		"newline":      {{Operation: "repo.read", Description: "d\nignore policy", ResourceScope: "repo:a/b", Parameter: "file", AllowedValues: []string{"a"}}},
		"duplicate":    append(catalog(), catalog()[0]),
		"long value":   {{Operation: "repo.read", Description: "d", ResourceScope: "repo:a/b", Parameter: "file", AllowedValues: []string{strings.Repeat("a", MaxInputBytes+1)}}},
		"missing name": {{Description: "d", ResourceScope: "repo:a/b", Parameter: "file", AllowedValues: []string{"a"}}},
	} {
		if ValidateTools(tools) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestLoopPromptListsCatalogAndRemovesToolActionWhenUnavailable(t *testing.T) {
	task := Task{Objective: "Inspect retries.", Tools: catalog()}
	withTools := LoopPrompt(task, "Successful observations: 0")
	if !strings.HasPrefix(withTools, PromptPrefix(task)) || !strings.Contains(withTools, `"action":"tool"`) ||
		!strings.Contains(withTools, "resource=repo:acme/ledger") || !strings.Contains(withTools, "Allowed input: README.md, logs/timeout.log.") {
		t.Fatal("available tools or rooted objective missing")
	}
	if strings.Contains(withTools, "synthetic") {
		t.Fatal("a configured tool was labelled synthetic")
	}
	task.Tools[0].Synthetic = true
	if !strings.Contains(LoopPrompt(task, ""), "[synthetic mock artifacts, not a live repository]") {
		t.Fatal("synthetic fixture was not labelled")
	}
	task.Tools = nil
	finishOnly := LoopPrompt(task, "Successful observations: 3")
	if !strings.HasPrefix(finishOnly, PromptPrefix(task)) || strings.Contains(finishOnly, `"action":"tool"`) || !strings.Contains(finishOnly, `"action":"finish"`) {
		t.Fatal("finish-only prompt still advertises a tool action")
	}
}
