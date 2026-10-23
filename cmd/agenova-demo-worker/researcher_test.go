// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

func researcherRole(t *testing.T) {
	t.Helper()
	previous := agentRole
	agentRole = "researcher"
	t.Cleanup(func() { agentRole = previous })
}

func TestResearcherDoesNotCompleteWithoutToolEvidence(t *testing.T) {
	researcherRole(t)
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read public meeting information", ModelProfile: "m", Mode: workerprotocol.ReAct})
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"An unsupported recommendation."}`})
	if err := run(&input, &output); !errors.Is(err, workerprotocol.ErrNoFinalResult) {
		t.Fatalf("expected missing evidence failure, got %v", err)
	}
	if strings.Contains(output.String(), `"result"`) {
		t.Fatal("Researcher returned a completed answer without an observation")
	}
}

func TestResearcherUsesModelSelectedTextInputAndObservedAnswer(t *testing.T) {
	researcherRole(t)
	tools := []workerprotocol.Tool{{Operation: "web.read", Description: "Read a public page", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}}
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	objective := "Compare public meeting information."
	enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: objective, ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: tools})
	// The URL is chosen by the model reply, not parsed out of the objective.
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.com/meeting","answer":""}`})
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: "Source: https://example.com/meeting. Meeting feature evidence."})
	enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"The source describes meeting features (https://example.com/meeting)."}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(&output)
	var first, tool, second, result workerprotocol.Message
	for _, message := range []*workerprotocol.Message{&first, &tool, &second, &result} {
		if err := dec.Decode(message); err != nil {
			t.Fatal(err)
		}
	}
	if first.Operation == nil || !strings.HasPrefix(first.Operation.Prompt, workerprotocol.PromptPrefix(workerprotocol.Task{Objective: objective})) || !strings.Contains(first.Operation.Prompt, researchInstructions) {
		t.Fatal("Researcher objective/prompt missing")
	}
	if tool.Operation == nil || tool.Operation.Tool != "web.read" || tool.Operation.Input != "https://example.com/meeting" {
		t.Fatal("model-selected operation was not preserved")
	}
	if second.Operation == nil || !strings.Contains(second.Operation.Prompt, "Meeting feature evidence.") || result.Result != "The source describes meeting features (https://example.com/meeting)." {
		t.Fatal("final answer was not based on the observed model exchange")
	}
	if !strings.Contains(second.Operation.Prompt, `"operation":"web.read"`) || !strings.Contains(second.Operation.Prompt, "does NOT invalidate earlier observations") {
		t.Fatal("Researcher hid its callable capability or lost the evidence-first completion rule")
	}
}

func TestResearcherPreservesFiniteReadFinishingPhase(t *testing.T) {
	for _, role := range []string{"", "researcher"} {
		t.Run(map[string]string{"": "legacy", "researcher": "researcher"}[role], func(t *testing.T) {
			previous := agentRole
			agentRole = role
			t.Cleanup(func() { agentRole = previous })
			var input, output bytes.Buffer
			enc := json.NewEncoder(&input)
			enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read the synthetic CRM dataset and cite an observed customer", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "crm.read", Description: "Read synthetic CRM", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}})
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention","answer":""}`})
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"synthetic":true,"customers":[{"id":"C001"}]}`})
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Synthetic customer C001 was observed."}`})
			if err := run(&input, &output); err != nil {
				t.Fatal(err)
			}
			dec := json.NewDecoder(&output)
			var messages []workerprotocol.Message
			for dec.More() {
				var m workerprotocol.Message
				if err := dec.Decode(&m); err != nil {
					t.Fatal(err)
				}
				messages = append(messages, m)
			}
			if len(messages) != 4 {
				t.Fatalf("unexpected messages: %d", len(messages))
			}
			secondPrompt := messages[2].Operation.Prompt
			catalogPresent := strings.Contains(secondPrompt, `"operation":"crm.read"`)
			if catalogPresent {
				t.Fatal("finite-input finishing phase changed")
			}
			if messages[3].Result != "Synthetic customer C001 was observed." {
				t.Fatal("observed answer changed")
			}
		})
	}
}

func TestResearcherFinalTurnCanUsePriorEvidenceWithoutCallableTools(t *testing.T) {
	prompt := researcherPrompt(workerprotocol.Task{Objective: "Cite an observed customer"}, `Successful observations: 1. {"observation":{"allowed":true,"text":"Synthetic customer C001"}}`)
	if !strings.Contains(prompt, "does NOT invalidate earlier observations") || !strings.Contains(prompt, "finish with the supported findings") || !strings.Contains(prompt, "Callable tool catalog") {
		t.Fatal("final-turn evidence instruction missing")
	}
}

func TestResearcherCannotTurnAnOutOfCatalogModelChoiceIntoAProviderCall(t *testing.T) {
	researcherRole(t)
	var input, output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Attempt a web read", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "crm.read", Description: "Read synthetic CRM", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}})
	for range workerprotocol.MaxTurns {
		enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"web.read","resource":"web:public-pages","input":"https://example.com/","answer":""}`})
	}
	if !errors.Is(run(&input, &output), workerprotocol.ErrTurnLimit) {
		t.Fatal("out-of-catalog model actions must not complete")
	}
	dec := json.NewDecoder(&output)
	for dec.More() {
		var message workerprotocol.Message
		if err := dec.Decode(&message); err != nil {
			t.Fatal(err)
		}
		if message.Result != "" || message.Operation == nil || message.Operation.Kind != "model" {
			t.Fatal("invalid tool choice escaped the agent action boundary")
		}
	}
}

func TestResearcherBlockedFinishFailsEvenAfterUnrelatedSuccessfulRead(t *testing.T) {
	for _, observed := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-observation", true: "unrelated-CRM-observation"}[observed], func(t *testing.T) {
			researcherRole(t)
			var input, output bytes.Buffer
			enc := json.NewEncoder(&input)
			tools := []workerprotocol.Tool{{Operation: "crm.read", Description: "Read synthetic CRM", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}
			enc.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Read a public meeting security page", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: tools})
			if observed {
				enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"customer-retention","answer":""}`})
				enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"synthetic":true,"customers":[{"id":"C001"}]}`})
			}
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"RESEARCH_TASK_BLOCKED"}`})
			if err := run(&input, &output); !errors.Is(err, workerprotocol.ErrNoFinalResult) {
				t.Fatalf("blocked research completed: %v", err)
			}
			if strings.Contains(output.String(), `"result"`) {
				t.Fatal("blocked status emitted a success result")
			}
			var first workerprotocol.Message
			if err := json.NewDecoder(&output).Decode(&first); err != nil {
				t.Fatal(err)
			}
			prompt := first.Operation.Prompt
			if strings.Contains(prompt, "Read at least one input before finishing") || strings.Contains(prompt, "At least two model turns are required") || !strings.Contains(prompt, researchBlocked) {
				t.Fatal("Researcher inherited mandatory-read/success-only instructions")
			}
		})
	}
}
