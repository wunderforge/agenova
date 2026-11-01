// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"strings"
	"testing"
)

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
