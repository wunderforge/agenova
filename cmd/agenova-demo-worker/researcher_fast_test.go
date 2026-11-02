// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"strings"
	"testing"
)

func TestDiagnosticAgentRecoversCloudFinishSequenceWithoutRepeatingRead(t *testing.T) {
	researcherRole(t)
	for _, second := range []string{
		`{"answer":"Synthetic customer C001 needs help.","completion":"completed","input":"customer-retention"}`,
		`{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention"}`,
	} {
		var in, out bytes.Buffer
		e := json.NewEncoder(&in)
		e.Encode(workerprotocol.Task{ClaimID: "c", ModelToken: "model-identity", ToolToken: "tool-identity", ModelProfile: "m", Objective: "Find at-risk internal customers", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, MaxModelCalls: 6, Tools: []workerprotocol.Tool{{Operation: "crm.read", ResourceScope: "crm:customer-retention", Description: "customers", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}})
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention"}`})
		e.Encode(workerprotocol.Reply{Allowed: true})
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"synthetic":true,"customers":[{"id":"C001","risk":"high"}]}`})
		for _, candidate := range []string{`{"action":"finish","resource":"crm:customer-retention","input":"customer-retention","answer":"Synthetic C001","completion":"completed"}`, second} {
			e.Encode(workerprotocol.Reply{Allowed: true, Text: candidate})
			e.Encode(workerprotocol.Reply{Allowed: true})
		}
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Synthetic customer C001 is at risk.","completion":"completed"}`})
		e.Encode(workerprotocol.Reply{Allowed: true})
		if err := run(&in, &out); err != nil {
			t.Fatal(err)
		}
		var calls, reads int
		var lastPrompt string
		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			var m workerprotocol.Message
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatal(err)
			}
			if m.Operation == nil {
				continue
			}
			op := m.Operation
			if op.Kind == "model" {
				calls++
				lastPrompt = op.Prompt
			}
			if op.Kind == "tool" {
				reads++
			}
			wantToken := "model-identity"
			if op.Kind == "tool" {
				wantToken = "tool-identity"
			}
			if op.ClaimID != "c" || op.WorkloadToken != wantToken {
				t.Fatal("repair changed Work identity")
			}
		}
		if calls != 4 || reads != 1 || !strings.Contains(lastPrompt, `"id":"C001"`) || !strings.Contains(out.String(), `"result":"Synthetic customer C001 is at risk."`) {
			t.Fatal("mixed repairs must retain evidence and recover without a second backend read")
		}
	}
}

func TestDiagnosticAgentRepairsDeniedFinishWithoutInventingAuthority(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Read the official website", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, MaxModelCalls: 5, Tools: []workerprotocol.Tool{{Operation: "web.read", ResourceScope: "web:public-pages", Parameter: "url", Description: "public page", InputMode: "text", MaxBytes: 1024}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.org"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: false, Error: "tool access denied"})
	for _, candidate := range []string{`{"answer":"A guessed answer","completion":"completed"}`, `{"action":"finish","answer":"A guessed answer","completion":"completed"}`} {
		e.Encode(workerprotocol.Reply{Allowed: true, Text: candidate})
		e.Encode(workerprotocol.Reply{Allowed: true})
	}
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"web.read was denied for this Work.","completion":"incomplete","reason":"tool-denied","target":"web.read"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"kind":"model"`) != 4 || strings.Count(out.String(), `"kind":"tool"`) != 1 || !strings.Contains(out.String(), `"result":"web.read was denied for this Work."`) || strings.Contains(out.String(), `"result":"A guessed answer"`) {
		t.Fatal("repair lost the actual Gateway denial")
	}
}

func TestDiagnosticAgentRepairExhaustionHonorsWorkAndImageLimits(t *testing.T) {
	researcherRole(t)
	for _, budget := range []int{1, 3, 100} {
		var in, out bytes.Buffer
		e := json.NewEncoder(&in)
		e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "What is 2+2?", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, MaxModelCalls: budget})
		limit := min(budget, workerprotocol.AgentSafetyTurns)
		for i := 0; i < limit; i++ {
			e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"answer":"4"}`})
			e.Encode(workerprotocol.Reply{Allowed: true})
		}
		e.Encode(workerprotocol.Reply{Allowed: true}) // explicit exhaustion feedback
		if err := run(&in, &out); !errors.Is(err, workerprotocol.ErrTurnLimit) {
			t.Fatalf("budget %d: %v", budget, err)
		}
		if strings.Count(out.String(), `"kind":"model"`) != limit || !strings.Contains(out.String(), `"diagnosticCode":"format-retry-limit"`) || strings.Contains(out.String(), `"kind":"tool"`) || strings.Contains(out.String(), `"result"`) {
			t.Fatal("invalid output exceeded budget, executed a tool or fabricated success")
		}
	}
}

func TestDiagnosticAgentDoesNotRetryProviderOrFeedbackFailure(t *testing.T) {
	researcherRole(t)
	for _, failFeedback := range []bool{false, true} {
		var in, out bytes.Buffer
		e := json.NewEncoder(&in)
		e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "What is 2+2?", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, MaxModelCalls: 6})
		if failFeedback {
			e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"answer":"4"}`})
			e.Encode(workerprotocol.Reply{Allowed: false})
		} else {
			e.Encode(workerprotocol.Reply{Allowed: false, Error: "model budget exhausted"})
		}
		if err := run(&in, &out); err == nil {
			t.Fatal("provider or evidence failure accepted")
		}
		if strings.Count(out.String(), `"kind":"model"`) != 1 || strings.Contains(out.String(), `"result"`) {
			t.Fatal("a non-format failure launched a repair call")
		}
	}
}

func TestDiagnosticAgentFinishesAfterSearchAndReportsDenialHonestly(t *testing.T) {
	researcherRole(t)
	for _, denied := range []bool{false, true} {
		var in, out bytes.Buffer
		e := json.NewEncoder(&in)
		e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "When is the festival?", Discovery: true, Diagnostics: true, Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "web.search", ResourceScope: "web:public-pages", Description: "search", Parameter: "query", InputMode: "text", MaxBytes: 512}}})
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.search","resource":"web:public-pages","input":"festival date"}`})
		e.Encode(workerprotocol.Reply{Allowed: true}) // feedback recorded
		answer := `{"action":"finish","answer":"Observed date (https://example.org).","completion":"completed"}`
		if denied {
			e.Encode(workerprotocol.Reply{Allowed: false, Error: "tool access denied"})
			answer = `{"action":"finish","answer":"The Gateway denied web.search.","completion":"incomplete","reason":"tool-denied","target":"web.search"}`
		} else {
			e.Encode(workerprotocol.Reply{Allowed: true, Text: `[UNTRUSTED TOOL DATA]
{"results":[{"url":"https://example.org","snippet":"observed date"}]}`})
		}
		e.Encode(workerprotocol.Reply{Allowed: true, Text: answer})
		e.Encode(workerprotocol.Reply{Allowed: true})
		if err := run(&in, &out); err != nil {
			t.Fatal(err)
		}
		if strings.Count(out.String(), `"kind":"model"`) != 2 || !strings.Contains(out.String(), `"diagnosticCode":"finish-accepted"`) || !strings.Contains(out.String(), `"result"`) {
			t.Fatal("unnecessary model round or lost feedback")
		}
		if denied && strings.Contains(out.String(), `"kind":"tool","tool":"crm.read"`) {
			t.Fatal("irrelevant substitution")
		}
	}
}

func TestDiagnosticAgentDoesNotRequireToolsForOrdinaryKnowledge(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "What is 2+2?", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"4","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"kind":"model"`) != 1 || !strings.Contains(out.String(), `"result":"4"`) {
		t.Fatal("unnecessary tool or review loop")
	}
}

func TestDiagnosticAgentRepairsCloudFinishFieldMixing(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "What is 2+2?", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct})
	// A real cloud model kept the previous tool's resource on a finish action.
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","resource":"web:public-pages","answer":"4","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","tool":"","resource":"","input":"","answer":"4","completion":"completed","reason":"","target":"","decisionSummary":"Answer the question"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"kind":"model"`) != 2 || !strings.Contains(out.String(), `"result":"4"`) || !strings.Contains(out.String(), "A finish action must leave tool, resource and input empty") {
		t.Fatal("format repair must explain the actual field error and recover without tools")
	}
}

func TestDiagnosticAgentRepairsSeparateMistakesAcrossActualToolProgress(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "When is the festival?", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, MaxModelCalls: 6, Tools: []workerprotocol.Tool{{Operation: "web.search", ResourceScope: "web:public-pages", Description: "search", Parameter: "query", InputMode: "text", MaxBytes: 512}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.search","resource":"web:public-pages","input":"festival date","completion":"incomplete"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.search","resource":"web:public-pages","input":"festival date"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: false, Error: "tool access denied"})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"A remembered date","completion":"completed","reason":"tool-denied","target":"web.search"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Unable to verify; web.search was denied.","completion":"incomplete","reason":"tool-denied","target":"web.search"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"kind":"model"`) != 4 || strings.Count(out.String(), `"kind":"tool"`) != 1 || !strings.Contains(out.String(), `"result":"Unable to verify; web.search was denied."`) {
		t.Fatal("separate bounded repairs must preserve one actual denied call and honest final result")
	}
}

func TestDiagnosticExplicitPageMustActuallyBeAttempted(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Read https://example.org/missing and report whether it was accessible.", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "web.read", ResourceScope: "web:public-pages", Description: "public page", Parameter: "url", InputMode: "text", MaxBytes: 1024}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"404 without checking","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.org/missing"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Error: "source-unavailable: HTTP 404"})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"The actual read returned HTTP 404.","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"diagnosticCode":"evidence-missing"`) || !strings.Contains(out.String(), `"result":"The actual read returned HTTP 404."`) {
		t.Fatal("unobserved page status accepted or actual failure discarded")
	}
}

func TestDiagnosticWeatherNeedsMeasurementsButNotVerbatimURL(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Sydney weather now", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "web.read", ResourceScope: "web:public-pages", Description: "public page", Parameter: "url", InputMode: "text", MaxBytes: 1024}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"20 C without observing weather.","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.org/weather"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"url":"https://example.org/weather","content":"Observed 20 C"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Observed 20 C","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"diagnosticCode":"evidence-missing"`) || !strings.Contains(out.String(), `"result":"Observed 20 C"`) {
		t.Fatal("fabricated weather or unnecessary citation retry")
	}
}

func TestDiagnosticAgentKeepsSyntheticProvenanceAndExplainsQualityRetry(t *testing.T) {
	researcherRole(t)
	var in, out bytes.Buffer
	e := json.NewEncoder(&in)
	e.Encode(workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Analyze internal customers", Diagnostics: true, Discovery: true, Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "crm.read", ResourceScope: "crm:customer-retention", Description: "customer data", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: "[UNTRUSTED TOOL DATA]\n" + `{"synthetic":true,"customers":[{"id":"C001"}]}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Customer C001 needs help.","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Synthetic customer C001 needs help.","completion":"completed"}`})
	e.Encode(workerprotocol.Reply{Allowed: true})
	if err := run(&in, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"diagnosticCode":"synthetic-label-missing"`) || !strings.Contains(out.String(), `"result":"Synthetic customer C001 needs help."`) {
		t.Fatal("source provenance was silently lost")
	}
}
