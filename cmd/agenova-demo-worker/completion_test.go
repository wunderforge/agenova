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

func TestResearcherReturnsIncompleteExplanationWithoutObservation(t *testing.T) {
	researcherRole(t)
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read a public page", ModelProfile: "m", Mode: workerprotocol.ReAct})
	answer := "I could not retrieve the page because web.read is unavailable."
	action := workerprotocol.Action{Action: "finish", Answer: answer, Completion: "incomplete", Reason: "missing-capability", Target: "web.read"}
	data, _ := json.Marshal(action)
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: string(data)})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&output)
	var first, last workerprotocol.Message
	dec.Decode(&first)
	dec.Decode(&last)
	if first.Operation.AgentProtocol != workerprotocol.ResearchProtocol || last.Result != answer {
		t.Fatal("incomplete output lost")
	}
}

func TestResearcherRefusesUnsupportedCompletedGoal(t *testing.T) {
	researcherRole(t)
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read a page", ModelProfile: "m", Mode: workerprotocol.ReAct})
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Invented facts","completion":"completed"}`})
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"No evidence available","completion":"incomplete","reason":"insufficient-evidence"}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), `"result":"Invented facts"`) || !strings.Contains(output.String(), `"result":"No evidence available"`) {
		t.Fatal("false success or missing incomplete answer")
	}
}

func TestNativeClientRejectsOtherTargetsBeforeNetworking(t *testing.T) {
	for _, target := range []string{"https://example.com/", "https://169.254.169.254/", "http://jitsi.org/security/"} {
		report, _ := nativeHTTPS(target)
		if report.Result != "error" || report.IP != "" {
			t.Fatal("unapproved target reached network")
		}
	}
}

func TestNativeBodyMarksOnlyTruncatedResponses(t *testing.T) {
	for _, size := range []int{32, 16384, 16385, 20000} {
		data, truncated, err := nativeBody(strings.NewReader(strings.Repeat("x", size)))
		if err != nil || truncated != (size > 16384) || len(data) != min(size, 16384) {
			t.Fatal(size, len(data), truncated, err)
		}
	}
}
