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
