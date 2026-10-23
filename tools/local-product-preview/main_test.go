// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/workerprotocol"
	"testing"
)

// A probe must use valid arguments for the target route, so a Gateway decision
// (not a malformed-argument error) is what the presenter can inspect.
func TestPreviewRetainsLegacyAndResearchCrossScopeProbes(t *testing.T) {
	for _, tc := range []struct{ tool, scope, input, target, scope2, input2 string }{
		{"git.read", "repo:acme/payments", "README.md", "git.read", "repo:acme/billing", "billing/INC-B2081/logs.txt"},
		{"git.read", "repo:acme/billing", "billing/INC-B2081/logs.txt", "git.read", "repo:acme/payments", "README.md"},
		{"web.fetch", "web:product-comparison", "product-a", "crm.read", "crm:customer-retention", "customer-retention"},
		{"crm.read", "crm:customer-retention", "customer-retention", "web.fetch", "web:product-comparison", "product-a"},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			var calls []workerprotocol.Operation
			task := workerprotocol.Task{ClaimID: "local-test", ToolToken: "local-synthetic", Objective: "cross-scope", Tools: []workerprotocol.Tool{{Operation: tc.tool, ResourceScope: tc.scope, AllowedValues: []string{tc.input}}}}
			_, err := (previewExecutor{}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
				calls = append(calls, op)
				if op.Kind == "model" {
					return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"synthetic test answer"}`}, nil
				}
				return workerprotocol.Reply{Allowed: len(calls) == 1, Text: "synthetic test evidence"}, nil
			})
			if err != nil || len(calls) != 3 {
				t.Fatal(err, calls)
			}
			probe := calls[1]
			if probe.Tool != tc.target || probe.ResourceScope != tc.scope2 || probe.Input != tc.input2 {
				t.Fatal("probe no longer reaches intended target", probe)
			}
		})
	}
}

func TestPreviewMissingWebPermissionDoesNotFabricateSuccess(t *testing.T) {
	for _, tools := range [][]workerprotocol.Tool{nil, {{Operation: "crm.read", ResourceScope: "crm:customer-retention", AllowedValues: []string{"customer-retention"}}}} {
		var calls []workerprotocol.Operation
		task := workerprotocol.Task{ClaimID: "local-test", ToolToken: "local-synthetic", Objective: "Read https://jitsi.org/jitsi-meet/ and summarize meeting features.", Tools: tools}
		answer, err := (previewExecutor{}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, task, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			calls = append(calls, op)
			return workerprotocol.Reply{Allowed: false, Error: "tool access denied"}, nil
		})
		if err == nil || answer != "" || len(calls) != 1 || calls[0].Kind != "tool" || calls[0].Tool != "web.read" || calls[0].ResourceScope != "web:public-pages" {
			t.Fatalf("task skipped the needed read or reported success: answer=%q err=%v calls=%v", answer, err, calls)
		}
	}
}

func TestPreviewOrdinaryToolFreeTaskStillWorks(t *testing.T) {
	var calls []workerprotocol.Operation
	answer, err := (previewExecutor{}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, workerprotocol.Task{ClaimID: "local-test", Objective: "Explain bounded retries."}, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		calls = append(calls, op)
		return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"No external data needed."}`}, nil
	})
	if err != nil || answer != "No external data needed." || len(calls) != 1 || calls[0].Kind != "model" {
		t.Fatal(answer, err, calls)
	}
}

func TestPreviewNeedsActualObservationBeforeModel(t *testing.T) {
	for _, reply := range []workerprotocol.Reply{{Allowed: true}, {Allowed: true, Text: "ignored", Error: "read failed"}} {
		calls := 0
		answer, err := (previewExecutor{}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, workerprotocol.Task{ClaimID: "local-test", Objective: "Read https://example.org/"}, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
			calls++
			if op.Kind != "tool" {
				t.Fatal("model was called without a successful observation")
			}
			return reply, nil
		})
		if err == nil || answer != "" || calls != 1 {
			t.Fatal(answer, err, calls)
		}
	}
}

func TestPreviewCodingTaskWithURLKeepsLegacyRead(t *testing.T) {
	var first workerprotocol.Operation
	_, err := (previewExecutor{templateFor: func(v0.SandboxClaimBackendIdentity) string { return "engineer" }}).Execute(context.Background(), v0.SandboxClaimBackendIdentity{}, workerprotocol.Task{ClaimID: "local-test", Objective: "Investigate customer retry code; reference https://example.org/", Tools: []workerprotocol.Tool{{Operation: "git.read", ResourceScope: "repo:acme/payments", AllowedValues: []string{"README.md"}}}}, func(_ context.Context, op workerprotocol.Operation) (workerprotocol.Reply, error) {
		if op.Kind == "tool" {
			first = op
			return workerprotocol.Reply{Allowed: true, Text: "synthetic repository evidence"}, nil
		}
		return workerprotocol.Reply{Allowed: true, Text: `{"action":"finish","answer":"Repository analysis."}`}, nil
	})
	if err != nil || first.Tool != "git.read" {
		t.Fatal("Researcher semantics leaked into coding fixture", err, first)
	}
}
