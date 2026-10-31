// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"io"
	"os"
	"testing"
	"time"
)

// Explicit existing local-model probe. No external endpoint, credential or download.
func TestLocalModelRejectsUnrelatedAvailableTool(t *testing.T) {
	name := os.Getenv("AGENOVA_TEST_RESEARCH_MODEL")
	if name == "" {
		t.Skip("opt-in existing loopback model")
	}
	previous := allowNativeHTTPS
	researcherRole(t)
	allowNativeHTTPS = false
	t.Cleanup(func() { allowNativeHTTPS = previous })
	tools := []workerprotocol.Tool{{Operation: "crm.read", Description: "Read internal synthetic customer retention records, not public information", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}}
	task := workerprotocol.Task{ClaimID: "local-probe", ModelToken: "local-model", ToolToken: "local-tool", ModelProfile: "local", Mode: workerprotocol.ReAct, Objective: "Explain what Python is used for, based on its official website.", Tools: tools}
	schema, err := workerprotocol.ResearchSchema(tools, false)
	if err != nil {
		t.Fatal(err)
	}
	schema, err = workerprotocol.GatewayResearchSchema(schema, tools)
	if err != nil {
		t.Fatal(err)
	}
	client, err := modelprovider.New(modelprovider.Config{Endpoint: "http://127.0.0.1:11434/v1", Models: map[string]string{"local": name}, MaxTokens: 384, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	defer inWriter.Close()
	defer outReader.Close()
	done := make(chan error, 1)
	go func() { e := run(inReader, outWriter); outWriter.CloseWithError(e); inReader.Close(); done <- e }()
	var last workerprotocol.Action
	toolCalls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := workerprotocol.Exchange(ctx, outReader, inWriter, task, func(ctx context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		if op.Kind == "tool" {
			toolCalls++
			return workerprotocol.Reply{Allowed: false, Error: "irrelevant tool must not be called"}, nil
		}
		reply, e := client.Complete(ctx, modelprovider.Request{Profile: "local", Prompt: op.Prompt, OutputSchema: json.RawMessage(schema)})
		if e != nil {
			return workerprotocol.Reply{}, e
		}
		last, _ = workerprotocol.ParseAgentAction(reply.Text, tools, workerprotocol.ResearchProtocol)
		return workerprotocol.Reply{Allowed: true, Text: reply.Text}, nil
	})
	inWriter.Close()
	outReader.Close()
	workerErr := <-done
	if err != nil || workerErr != nil || result == "" || toolCalls != 0 || last.Completion != "incomplete" {
		t.Fatalf("model=%s toolCalls=%d completion=%s exchange=%v worker=%v", name, toolCalls, last.Completion, err, workerErr)
	}
	t.Logf("model=%s: incomplete without an unrelated tool call", name)
}
