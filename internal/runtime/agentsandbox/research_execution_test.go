// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package agentsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
)

func TestOptInIncompleteFinalBindsModelAnswer(t *testing.T) {
	task := workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Read a page", Mode: workerprotocol.ReAct}
	action := `{"action":"finish","answer":"No web tool available.","completion":"incomplete","reason":"missing-capability","target":"web.read"}`
	for _, answer := range []string{"No web tool available.", "Forged final"} {
		var input, output bytes.Buffer
		enc := json.NewEncoder(&input)
		enc.Encode(workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "model", ClaimID: "c", Profile: "m", AgentProtocol: workerprotocol.ResearchProtocol, Prompt: workerprotocol.PromptPrefix(task) + "Next action:"}})
		enc.Encode(workerprotocol.Message{Result: answer})
		got, err := exchangeWorker(context.Background(), &input, &output, task, func(context.Context, workerprotocol.Operation) (workerprotocol.Reply, error) {
			return workerprotocol.Reply{Allowed: true, Text: action}, nil
		})
		if answer == "Forged final" {
			if err == nil {
				t.Fatal("forged answer accepted")
			}
		} else if err != nil || got != answer {
			t.Fatal(got, err)
		}
	}
}

func TestNativeReportsRequireMatchingModelActionAndOrder(t *testing.T) {
	task := workerprotocol.Task{ClaimID: "c", ModelProfile: "m", Objective: "Try direct HTTPS", Mode: workerprotocol.ReAct}
	model := func() workerprotocol.Message {
		return workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "model", ClaimID: "c", Profile: "m", AgentProtocol: workerprotocol.ResearchProtocol, Prompt: workerprotocol.PromptPrefix(task) + "Next action:"}}
	}
	start := workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "native-start", ClaimID: "c", Input: workerprotocol.NativeTarget}}
	end := workerprotocol.Message{Operation: &workerprotocol.Operation{Kind: "native-end", ClaimID: "c", Input: workerprotocol.NativeTarget, Native: &workerprotocol.NativeObservation{Phase: "tcp", Result: "timeout", Milliseconds: 5000}}}
	for _, tc := range []struct {
		name     string
		messages []workerprotocol.Message
		valid    bool
	}{
		{"valid", []workerprotocol.Message{model(), start, end, model(), {Result: "Direct HTTPS timed out."}}, true},
		{"no-start", []workerprotocol.Message{model(), end}, false},
		{"duplicate", []workerprotocol.Message{model(), start, start}, false},
		{"no-model", []workerprotocol.Message{start}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var input, output bytes.Buffer
			enc := json.NewEncoder(&input)
			for _, m := range tc.messages {
				enc.Encode(m)
			}
			turns := 0
			reports := 0
			_, err := exchangeWorker(context.Background(), &input, &output, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
				if op.Kind == "model" {
					turns++
					text := `{"action":"native_https","input":"https://jitsi.org/security/"}`
					if turns > 1 {
						text = `{"action":"finish","answer":"Direct HTTPS timed out.","completion":"incomplete","reason":"network-unavailable","target":"direct-https"}`
					}
					return workerprotocol.Reply{Allowed: true, Text: text}, nil
				}
				reports++
				return workerprotocol.Reply{Allowed: true}, nil
			})
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if tc.valid && reports != 2 {
				t.Fatal("attempt/outcome missing")
			}
		})
	}
}
