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

func TestSourceCitationCheckIsAnAgentQualityRule(t *testing.T) {
	if !missingSourceCitation("Facts without a citation.", []string{"https://example.com/about/"}) {
		t.Fatal("uncited public answer accepted")
	}
	if missingSourceCitation("Source: https://example.com/about/", []string{"https://example.com/about/"}) {
		t.Fatal("valid source rejected")
	}
	if missingSourceCitation("Synthetic customer C001", []string{"customer-retention"}) {
		t.Fatal("URL required for an internal task")
	}
}

func TestReferenceAgentDoesNotUseCustomerRecordsForAnExplicitWebsiteGoal(t *testing.T) {
	researcherRole(t)
	previous := allowNativeHTTPS
	allowNativeHTTPS = false
	t.Cleanup(func() { allowNativeHTTPS = previous })
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Explain Python based on its official website.", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "crm.read", Description: "Internal customers", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"I could not verify the website because web.read is unavailable.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"kind":"tool"`) || !strings.Contains(output.String(), `"result":"I could not verify`) {
		t.Fatal("reference Agent used unrelated records or lost incomplete explanation")
	}
	if publicSourceRequired("Prioritize our internal customers and draft follow-ups.") {
		t.Fatal("ordinary internal task was classified as public")
	}
}

func TestResearcherRecoversIncorrectMissingCapabilityWithModelSelectedCall(t *testing.T) {
	researcherRole(t)
	previous := allowNativeHTTPS
	allowNativeHTTPS = false
	t.Cleanup(func() { allowNativeHTTPS = previous })
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Summarize the official product information", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "web.read", Description: "Read a public page", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}}})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"web.read is unavailable.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.com/about/"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"url":"https://example.com/about/","content":"Observed product information."}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Observed product information. Source: https://example.com/about/","completion":"completed"}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "IS available") || !strings.Contains(output.String(), `"kind":"tool"`) || !strings.Contains(output.String(), `"result":"Observed product information.`) {
		t.Fatal("incorrect capability refusal was not recovered through a real model-selected call")
	}
}

func TestGatewayOnlyResearcherRefusesNativeOperationAndExplainsLimit(t *testing.T) {
	researcherRole(t)
	previous := allowNativeHTTPS
	allowNativeHTTPS = false
	t.Cleanup(func() { allowNativeHTTPS = previous })
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read a public page", ModelProfile: "m", Mode: workerprotocol.ReAct})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"native_https","input":"https://jitsi.org/security/"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"I could not verify the public information without an available reader.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"kind":"native-`) {
		t.Fatal("local Agent attempted a native network operation")
	}
	if !strings.Contains(output.String(), "I could not verify") {
		t.Fatal("lost incomplete response")
	}
	prompt := researcherV1Prompt(workerprotocol.Task{}, "")
	if !strings.Contains(prompt, "user need not supply a URL") || !strings.Contains(prompt, "Never invent current facts") || strings.Contains(prompt, "ONLY allowed input is") {
		t.Fatal("local prompt still depends on a fixed URL")
	}
}
