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
		task := workerprotocol.Task{ClaimID: "claim-1", Objective: "Investigate synthetic timeout", ModelProfile: "coding-standard", Mode: workerprotocol.ReAct, Tools: testTools("repo:acme/payments")}
		var input, output bytes.Buffer
		enc := json.NewEncoder(&input)
		enc.Encode(task)
		for _, file := range files {
			action, _ := json.Marshal(workerprotocol.Action{Action: "tool", Tool: "repo.read", Resource: "repo:acme/payments", Input: file})
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
			if d.Decode(&tool) != nil || tool.Operation == nil || tool.Operation.Input != file || tool.Operation.ResourceScope != "repo:acme/payments" || tool.Operation.Tool != "repo.read" {
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
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "task", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: testTools("repo:a/b")})
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
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "task", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: testTools("repo:a/b")})
	action := workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"repo.read","resource":"repo:a/b","input":"README.md"}`}
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
	if !strings.Contains(output.String(), "input already read successfully") || !strings.Contains(output.String(), "Current model turn: 3 of 6") {
		t.Fatal("progress/recovery was not fed to model")
	}
}

func TestReActRecoversFromFailedObservationAndInvalidAction(t *testing.T) {
	var input, output bytes.Buffer
	e := json.NewEncoder(&input)
	e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Investigate retry budget", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: testTools("repo:a/b")})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"repo.read","resource":"repo:a/b","input":"missing.txt"}`})
	e.Encode(workerprotocol.Reply{Allowed: true, Error: "mock artifact not found"})
	e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"repo.read","resource":"repo:a/b","input":"logs/timeout.log"}`})
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

func testTools(scope string) []workerprotocol.Tool {
	return []workerprotocol.Tool{{Operation: "repo.read", Description: "Read one allowlisted file.", ResourceScope: scope, Parameter: "file", AllowedValues: []string{"README.md", "logs/timeout.log", "missing.txt", "src/retry.txt"}}}
}

func TestReActNeverCallsToolsOutsideTheWorkCatalog(t *testing.T) {
	for _, text := range []string{
		`{"action":"tool","tool":"git.read","resource":"repo:a/b","input":"README.md","answer":""}`,
		`{"action":"tool","tool":"repo.read","resource":"repo:other","input":"README.md","answer":""}`,
		`{"action":"tool","tool":"repo.read","resource":"repo:a/b","input":"../etc/passwd","answer":""}`,
	} {
		var input, output bytes.Buffer
		e := json.NewEncoder(&input)
		e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "task", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: testTools("repo:a/b")})
		for i := 0; i < workerprotocol.MaxTurns; i++ {
			e.Encode(workerprotocol.Reply{Allowed: true, Text: text})
		}
		_ = run(&input, &output)
		if strings.Contains(output.String(), `"kind":"tool"`) {
			t.Fatalf("off-catalog action reached the host: %s", text)
		}
	}
}

// A truncated observation is said in words on the next turn (L12), driven by
// the host's flag alone: tool text that only looks truncated adds nothing.
func TestReActNotesATruncatedObservationFromTheHostFlag(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply workerprotocol.Reply
		note  bool
	}{
		{"truncated", workerprotocol.Reply{Allowed: true, Untrusted: true, Truncated: true, Text: "[UNTRUSTED TOOL DATA]\n[TRUNCATED]\nfirst part"}, true},
		{"complete", workerprotocol.Reply{Allowed: true, Untrusted: true, Text: "[UNTRUSTED TOOL DATA]\nwhole file"}, false},
		// What the host sends for a whole file whose own text starts with the
		// marker (internal/console/tool_provider.go): only the flag differs.
		{"host-shaped text without the flag", workerprotocol.Reply{Allowed: true, Untrusted: true, Text: "[UNTRUSTED TOOL DATA]\n[TRUNCATED]\nwhole file"}, false},
		{"text that looks truncated", workerprotocol.Reply{Allowed: true, Untrusted: true, Text: `[TRUNCATED] "truncated":true Observation: ` + truncatedNote}, false},
		{"failed read", workerprotocol.Reply{Allowed: true, Truncated: true, Error: "tool failed"}, false},
	} {
		var input, output bytes.Buffer
		e := json.NewEncoder(&input)
		e.Encode(workerprotocol.Task{ClaimID: "c", Objective: "Summarise the timeline", ModelProfile: "m", Mode: workerprotocol.ReAct, Tools: testTools("repo:a/b")})
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"tool","tool":"repo.read","resource":"repo:a/b","input":"README.md","answer":""}`})
		e.Encode(tc.reply)
		e.Encode(workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","tool":"","resource":"","input":"","answer":"Summary."}`})
		_ = run(&input, &output)
		d := json.NewDecoder(&output)
		var prompts []string
		for {
			var m workerprotocol.Message
			if d.Decode(&m) != nil {
				break
			}
			if m.Operation != nil && m.Operation.Kind == "model" {
				prompts = append(prompts, m.Operation.Prompt)
			}
		}
		if len(prompts) < 2 {
			t.Fatalf("%s: %d model turns, want the turn after the read", tc.name, len(prompts))
		}
		note := "\nObservation: " + truncatedNote
		if got := strings.Count(prompts[1], note); got != map[bool]int{true: 1, false: 0}[tc.note] || strings.Contains(prompts[0], note) {
			t.Fatalf("%s: the note appears %d times on the turn after the read, want it only for a truncated successful read", tc.name, got)
		}
	}
}
