// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
)

func example() (attempt, evidence.View, []receipt) {
	a := attempt{Status: "Unknown", Failures: []string{}, HumanReviewNeeded: true}
	v := evidence.View{
		RequestRef: "request-example",
		Request:    &v0.ClaimRequest{Spec: v0.ClaimRequestSpec{TemplateRef: "demo"}},
		State: &v0.IssuedState{
			Principal: v0.Principal{Subject: "test-engineer"}, PolicyRef: v0.PolicyReference{ID: "policy", Version: "v1"},
			Claim:              &v0.SandboxClaim{ID: "claim-example", TemplateRef: "demo"},
			Decision:           v0.Decision{Result: v0.DecisionResultAllow},
			EffectiveAuthority: &v0.EffectiveAuthority{Tools: []string{"repo.read"}, ResourceScopes: []string{"repo:demo/payments"}, ModelProfile: "coding-standard"},
		},
		Outcome: &evidence.Outcome{Status: "Succeeded", Text: "logs.txt and retry.txt show the deadline exceeded by retry backoff; reuse an idempotency key and respect the total budget.", Model: &evidence.ModelResult{Model: "real-example", ResponseID: "provider-response", OutputTokens: 90}},
		Facts: []facts.Fact{
			{Kind: "ProviderAttempt", RequestRef: "request-example", ClaimID: "claim-example", InvocationID: "m1", Operation: "model.invoke"},
			{Kind: "ProviderAttempt", RequestRef: "request-example", ClaimID: "claim-example", InvocationID: "t1", Operation: "tool.invoke"},
			{Kind: "ProviderAttempt", RequestRef: "request-example", ClaimID: "claim-example", InvocationID: "t2", Operation: "tool.invoke"},
			{Kind: "ProviderOutcome", RequestRef: "request-example", ClaimID: "claim-example", InvocationID: "t1", Operation: "tool.invoke", ProviderStatus: "Succeeded"},
			{Kind: "ProviderOutcome", RequestRef: "request-example", ClaimID: "claim-example", InvocationID: "t2", Operation: "tool.invoke", ProviderStatus: "Succeeded"},
		},
	}
	var receipts []receipt
	for i, id := range []string{"t1", "t2"} {
		receipts = append(receipts, receipt{Event: "receipt", RPCMethod: "tools/call", Correlation: id}, receipt{Event: "tool", Tool: "read_file", Correlation: id, Outcome: "ok", File: scenarios[0].files[i]})
	}
	return a, v, receipts
}

func TestScreenRequiresActualCorrelatedReadsAndRealModelMetadata(t *testing.T) {
	a, v, receipts := example()
	result := evaluate(a, scenarios[0], v, receipts, "real-example", "demo")
	if len(result.Failures) != 0 || len(result.FilesRead) != 2 || !result.HumanReviewNeeded {
		t.Fatalf("valid mechanical screen rejected or human review lost: %+v", result)
	}
	for _, name := range []string{"no-receipts", "duplicate-receipt", "wrong-scope-file", "no-model-metadata", "wrong-model", "cross-work-fact", "public-network", "vague-answer"} {
		t.Run(name, func(t *testing.T) {
			a, v, receipts := example()
			switch name {
			case "no-receipts":
				receipts = nil
			case "duplicate-receipt":
				receipts = append(receipts, receipts[0])
			case "wrong-scope-file":
				receipts[1].File = "billing/INC-B2081/logs.txt"
			case "no-model-metadata":
				v.Outcome.Model.ResponseID = ""
			case "wrong-model":
				v.Outcome.Model.Model = "scripted-example"
			case "cross-work-fact":
				v.Facts[1].ClaimID = "other-claim"
			case "public-network":
				v.State.EffectiveAuthority.NetworkProfiles = []string{"public-web"}
			case "vague-answer":
				v.Outcome.Text = "This is fixed."
			}
			if len(evaluate(a, scenarios[0], v, receipts, "real-example", "demo").Failures) == 0 {
				t.Fatal("invalid evidence passed the screen")
			}
		})
	}
}

func TestDefaultInvocationCannotSpendOrCreateOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-created")
	if run(config{out: path}) == nil {
		t.Fatal("missing explicit execution authorization accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("non-executing command wrote campaign output")
	}
}

func TestBoundedSubprocessOutput(t *testing.T) {
	var b limitedBuffer
	if _, err := b.Write(make([]byte, 16<<20)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte{1}); err == nil || !b.exceeded {
		t.Fatal("output bound not enforced")
	}
}
