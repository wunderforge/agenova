// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/workerprotocol"
)

const (
	claimID    = "claim-a"
	modelToken = "synthetic-model-token"
	toolToken  = "synthetic-tool-token"
)

// testTask mirrors the host Task shape: one granted catalog entry and one
// token per Gateway audience.
func testTask(objective string) workerprotocol.Task {
	return workerprotocol.Task{ClaimID: claimID, ModelToken: modelToken, ToolToken: toolToken, Objective: objective, ModelProfile: "coding-standard", Mode: workerprotocol.ReAct, Tools: []workerprotocol.Tool{{Operation: "git.read", Description: "Read one repository file.", ResourceScope: "repo:acme/payments", Parameter: "file", AllowedValues: []string{"README.md"}}}}
}

// drive runs a scenario with a host that answers every operation, and
// returns the worker records in order.
func drive(t *testing.T, objective string, replies int) ([]string, []time.Duration, error) {
	t.Helper()
	task, _ := json.Marshal(testTask(objective))
	input := string(task) + "\n" + strings.Repeat(`{"allowed":true,"text":"{\"action\":\"finish\",\"answer\":\"ok\"}"}`+"\n", replies)
	var output strings.Builder
	var waits []time.Duration
	err := run(strings.NewReader(input), &output, func(d time.Duration) { waits = append(waits, d) })
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(output.String()))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, waits, err
}

func operation(t *testing.T, line string) workerprotocol.Operation {
	t.Helper()
	var m workerprotocol.Message
	if err := json.Unmarshal([]byte(line), &m); err != nil || m.Operation == nil {
		t.Fatalf("not an operation record: %q", line)
	}
	return *m.Operation
}

func TestAllowedOperationMatchesReferenceShape(t *testing.T) {
	lines, _, err := drive(t, "e15-scenario=control", 2)
	if err != nil || len(lines) != 3 {
		t.Fatalf("control: lines=%d err=%v", len(lines), err)
	}
	op := operation(t, lines[0])
	task := testTask("e15-scenario=control")
	if op.ClaimID != claimID || op.WorkloadToken != modelToken || op.Kind != "model" || op.Profile != "coding-standard" || !strings.HasPrefix(op.Prompt, workerprotocol.PromptPrefix(task)) {
		t.Fatalf("allowed operation does not match the reference shape: %+v", op)
	}
}

func TestNegativeScenariosChangeOnlyTheNamedField(t *testing.T) {
	cases := []struct {
		objective string
		check     func(workerprotocol.Operation) bool
	}{
		{"e15-scenario=cross-claim target=claim-b", func(op workerprotocol.Operation) bool {
			return op.ClaimID == "claim-b" && op.WorkloadToken == modelToken && op.Profile == "coding-standard"
		}},
		{"e15-scenario=profile", func(op workerprotocol.Operation) bool {
			return op.ClaimID == claimID && op.Profile == "e15-ungranted-profile"
		}},
		{"e15-scenario=tool", func(op workerprotocol.Operation) bool {
			return op.Kind == "tool" && op.WorkloadToken == toolToken && op.Tool == "git.write" && op.ResourceScope == "repo:acme/payments"
		}},
		{"e15-scenario=scope", func(op workerprotocol.Operation) bool {
			return op.Kind == "tool" && op.WorkloadToken == toolToken && op.Tool == "git.read" && op.ResourceScope == "repo:e15/ungranted"
		}},
		{"e15-scenario=foreign-scope scope=repo:acme/billing", func(op workerprotocol.Operation) bool {
			return op.Kind == "tool" && op.WorkloadToken == toolToken && op.Tool == "git.read" && op.ResourceScope == "repo:acme/billing"
		}},
		{"e15-scenario=unsupported", func(op workerprotocol.Operation) bool { return op.Kind == "memory" && op.ClaimID == claimID }},
	}
	for _, c := range cases {
		lines, _, err := drive(t, c.objective, 2)
		if err != nil || len(lines) != 2 {
			t.Fatalf("%s: lines=%d err=%v", c.objective, len(lines), err)
		}
		if first := operation(t, lines[0]); first.ClaimID != claimID || first.Kind != "model" {
			t.Fatalf("%s: first record must be the allowed control call: %+v", c.objective, first)
		}
		if op := operation(t, lines[1]); !c.check(op) {
			t.Fatalf("%s: unexpected second record %+v", c.objective, op)
		}
	}
}

func TestRefusedOperationEndsWhenSessionCloses(t *testing.T) {
	lines, _, err := drive(t, "e15-scenario=cross-claim target=claim-b", 1)
	if err == nil || len(lines) != 2 {
		t.Fatalf("worker must stop after the unanswered operation: lines=%d err=%v", len(lines), err)
	}
}

func TestRawAndTimedScenarios(t *testing.T) {
	lines, _, _ := drive(t, "e15-scenario=malformed", 1)
	if len(lines) != 2 || lines[1] != "{not-json" {
		t.Fatalf("malformed: %q", lines)
	}
	lines, _, _ = drive(t, "e15-scenario=after-failure", 1)
	if len(lines) != 3 || !strings.Contains(lines[1], `"result"`) {
		t.Fatalf("after-failure: %q", lines)
	}
	lines, waits, _ := drive(t, "e15-scenario=after-expiry wait=40s", 2)
	if len(lines) != 2 || len(waits) != 1 || waits[0] != 40*time.Second {
		t.Fatalf("after-expiry: lines=%q waits=%v", lines, waits)
	}
	lines, _, _ = drive(t, "e15-scenario=turn-limit", workerprotocol.MaxTurns+1)
	if len(lines) != workerprotocol.MaxTurns+1 {
		t.Fatalf("turn-limit: %d records", len(lines))
	}
}

func TestInvalidScenarioInputIsRejected(t *testing.T) {
	for _, objective := range []string{"e15-scenario=unknown", "no scenario", "e15-scenario=cross-claim", "e15-scenario=cross-claim target=claim-a", "e15-scenario=after-expiry wait=x", "e15-scenario=foreign-scope", "e15-scenario=foreign-scope scope=repo:acme/payments"} {
		if lines, _, err := drive(t, objective, 1); err == nil || len(lines) != 0 {
			t.Fatalf("%q: want error before any record, got lines=%d err=%v", objective, len(lines), err)
		}
	}
}
