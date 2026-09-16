// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

func TestWorkerUsesActualObjectiveAndResponse(t *testing.T) {
	task := workerprotocol.Task{ClaimID: "claim-1", Objective: "Explain a payment timeout", ModelProfile: "coding-standard"}
	var input, output bytes.Buffer
	json.NewEncoder(&input).Encode(task)
	json.NewEncoder(&input).Encode(workerprotocol.Reply{Allowed: true, Text: "Check the upstream deadline."})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var op, result workerprotocol.Message
	if decoder.Decode(&op) != nil || op.Operation == nil || op.Operation.Prompt != task.Objective || op.Operation.Profile != task.ModelProfile || op.Operation.ClaimID != task.ClaimID {
		t.Fatalf("wrong model operation: %+v", op)
	}
	if decoder.Decode(&result) != nil || result.Result != "Check the upstream deadline." {
		t.Fatalf("wrong task result: %+v", result)
	}
}

func TestWorkerRejectsMissingDeniedOrMalformedInput(t *testing.T) {
	for _, input := range []string{
		"", "{}\n", "{bad}\n", strings.Repeat("x", lineLimit) + "\n",
		"{\"claimId\":\"c\",\"objective\":\"task\",\"modelProfile\":\"m\"}\n{\"allowed\":false}\n",
		"{\"claimId\":\"c\",\"objective\":\"task\",\"modelProfile\":\"m\"}\n{\"allowed\":true}\n",
		"{\"claimId\":\"c\",\"objective\":\"task\",\"modelProfile\":\"m\",\"key\":\"secret\"}\n",
	} {
		var output bytes.Buffer
		if err := run(strings.NewReader(input), &output); err == nil {
			t.Fatal("invalid or denied input succeeded")
		}
		if strings.Contains(output.String(), "\"result\"") {
			t.Fatal("failed operation emitted result")
		}
	}
}

func TestReActFollowsModelSelectedPathsAndObservations(t *testing.T) {
	for _, files := range [][]string{{"README.md"}, {"logs/timeout.log", "src/retry.txt", "README.md"}} {
		task := workerprotocol.Task{ClaimID: "claim-1", Objective: "Investigate synthetic timeout", ModelProfile: "coding-standard", Mode: workerprotocol.ReAct, ResourceScope: "repo:acme/payments"}
		var input, output bytes.Buffer
		enc := json.NewEncoder(&input)
		enc.Encode(task)
		for _, file := range files {
			action, _ := json.Marshal(workerprotocol.Action{Action: "tool", Tool: "git.read", Input: file})
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: string(action)})
			enc.Encode(workerprotocol.Reply{Allowed: true, Text: "observation from " + file})
		}
		enc.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Fix the shared deadline."}`})
		if err := run(&input, &output); err != nil {
			t.Fatal(err)
		}
		d := json.NewDecoder(&output)
		for i, file := range files {
			var model, tool workerprotocol.Message
			if d.Decode(&model) != nil || model.Operation == nil || model.Operation.Kind != "model" {
				t.Fatal("missing model turn")
			}
			if i > 0 && !strings.Contains(model.Operation.Prompt, "observation from "+files[i-1]) {
				t.Fatal("tool observation not fed back")
			}
			if d.Decode(&tool) != nil || tool.Operation == nil || tool.Operation.Input != file || tool.Operation.ResourceScope != task.ResourceScope {
				t.Fatal("did not follow model-selected file")
			}
		}
		var last, result workerprotocol.Message
		d.Decode(&last)
		d.Decode(&result)
		if !strings.Contains(last.Operation.Prompt, "observation from "+files[len(files)-1]) || result.Result != "Fix the shared deadline." {
			t.Fatal("missing reflection/result")
		}
	}
}

func TestReActExhaustionNeverInventsSuccess(t *testing.T) {
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "task", ModelProfile: "m", Mode: workerprotocol.ReAct, ResourceScope: "repo:a/b"})
	for i := 0; i < workerprotocol.MaxTurns; i++ {
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"premature"}`})
	}
	if err := run(&input, &output); err == nil {
		t.Fatal("premature completion accepted")
	}
	if strings.Contains(output.String(), `"result"`) {
		t.Fatal("exhaustion emitted success")
	}
}

func TestReActDoesNotRereadSuccessfulObservation(t *testing.T) {
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "task", ModelProfile: "m", Mode: workerprotocol.ReAct, ResourceScope: "repo:a/b"})
	action := workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"git.read","input":"README.md"}`}
	e.Encode(action)
	e.Encode(workerprotocol.Reply{Allowed: true, Text: "deadline evidence"})
	e.Encode(action)
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Use the observed deadline."}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), `"kind":"tool"`) != 1 {
		t.Fatal("redundant successful read executed twice")
	}
	if !strings.Contains(output.String(), "file already read successfully") || !strings.Contains(output.String(), "Current model turn: 3 of 6") {
		t.Fatal("progress/recovery was not fed to model")
	}
}

func TestReActRecoversFromFailedObservationAndInvalidAction(t *testing.T) {
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Investigate retry budget", ModelProfile: "m", Mode: workerprotocol.ReAct, ResourceScope: "repo:a/b"})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"git.read","input":"missing.txt"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Error: "mock artifact not found"})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"git.read","input":"logs/timeout.log"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: "Total budget was exceeded after backoff."})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"unsupported","answer":"not a finish"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Use remaining budget for backoff and attempts."}`})
	if err := run(&input, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), `"kind":"model"`) != 4 || strings.Count(output.String(), `"kind":"tool"`) != 2 || !strings.Contains(output.String(), "mock artifact not found") || !strings.Contains(output.String(), "required tool/finish JSON format") || !strings.Contains(output.String(), `"result":"Use remaining budget`) {
		t.Fatal("recovery path did not feed observations and format feedback into later turns")
	}
}
