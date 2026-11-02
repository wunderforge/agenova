// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"io"
	"strings"
	"testing"
)

func TestRawDatasetRepairThroughActualExchange(t *testing.T) {
	researcherRole(t)
	for _, mode := range []string{"repair", "denied", "repeated-invalid"} {
		t.Run(mode, func(t *testing.T) {
			input, hostWriter := io.Pipe()
			hostReader, output := io.Pipe()
			defer input.Close()
			defer output.Close()
			defer hostReader.Close()
			defer hostWriter.Close()
			ended := make(chan error, 1)
			go func() { err := run(input, output); output.Close(); ended <- err }()
			task := workerprotocol.Task{ClaimID: "claim", ModelToken: "model", ToolToken: "tool", ModelProfile: "m", Objective: "Review synthetic internal customer risks.", Mode: workerprotocol.ReAct, Diagnostics: true, Discovery: true, MaxModelCalls: 6, Tools: []workerprotocol.Tool{
				{Operation: "crm.read", ResourceScope: "crm:customer-retention", Description: "synthetic customers", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}},
				{Operation: "web.search", ResourceScope: "web:public-pages", Description: "search", Parameter: "query", InputMode: "text", MaxBytes: 512},
			}}
			bad := `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"dataset_id=customer-retention","answer":"","completion":"","reason":"","target":"","decisionSummary":"Read customer risks"}`
			good := strings.Replace(bad, "dataset_id=customer-retention", "customer-retention", 1)
			models, calls, backend := 0, 0, 0
			exhausted := false
			got, err := workerprotocol.Exchange(context.Background(), hostReader, hostWriter, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
				switch op.Kind {
				case "feedback":
					exhausted = exhausted || op.DiagnosticCode == "format-retry-limit"
					return workerprotocol.Reply{Allowed: true}, nil
				case "model":
					models++
					if models == 1 {
						return workerprotocol.Reply{Allowed: true, Text: bad}, nil
					}
					if mode == "repeated-invalid" {
						return workerprotocol.Reply{Allowed: true, Text: bad}, nil
					}
					if models == 2 {
						if !strings.Contains(op.Prompt, `"input":"customer-retention"`) || !strings.Contains(op.Prompt, "RAW INPUT REPAIR") || !strings.Contains(op.Prompt, "Invalid input for crm.read") {
							return workerprotocol.Reply{}, errors.New("retry lacks exact catalog correction")
						}
						return workerprotocol.Reply{Allowed: true, Text: good}, nil
					}
					if mode == "denied" {
						return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"CRM access was denied.","completion":"incomplete","reason":"tool-denied","target":"crm.read"}`}, nil
					}
					return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Synthetic customer data: C001, C002, C003 need follow-up.","completion":"completed"}`}, nil
				case "tool":
					calls++
					if op.Input != "customer-retention" {
						return workerprotocol.Reply{}, errors.New("invalid argument reached Gateway")
					}
					if mode == "denied" {
						return workerprotocol.Reply{Allowed: false, Error: "tool-not-granted"}, nil
					}
					backend++
					return workerprotocol.Reply{Allowed: true, Text: `{"synthetic":true,"customers":[{"id":"C001"},{"id":"C002"},{"id":"C003"}]}`}, nil
				}
				return workerprotocol.Reply{}, errors.New("unexpected operation")
			})
			hostWriter.Close()
			hostReader.Close()
			workerErr := <-ended
			if mode == "repeated-invalid" {
				if !errors.Is(workerErr, workerprotocol.ErrTurnLimit) || !errors.Is(err, workerprotocol.ErrNoFinalResult) || calls != 0 || models != task.MaxModelCalls || !exhausted {
					t.Fatalf("repeated malformed output escaped bounds: %v %v %d %d", err, workerErr, calls, models)
				}
				return
			}
			if err != nil || workerErr != nil || models != 3 || calls != 1 {
				t.Fatalf("repair did not complete the bound exchange: %v %v %d %d", err, workerErr, models, calls)
			}
			if mode == "denied" {
				if backend != 0 || !strings.Contains(got, "denied") {
					t.Fatal("repair bypassed denial")
				}
			} else if backend != 1 || !strings.Contains(got, "Synthetic customer data") {
				t.Fatal("valid repair not invoked exactly once")
			}
		})
	}
}
