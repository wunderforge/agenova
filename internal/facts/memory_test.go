// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package facts

import (
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestJournalMemoryCorrelationAndMetadata(t *testing.T) {
	newAttempt := func(t *testing.T) (*Journal, Fact) {
		t.Helper()
		j := newRegisteredJournal(t)
		base := Fact{Kind: "MemoryDecision", RequestRef: "a", ClaimID: "claim:a", InvocationID: "inv:memory", Operation: "memory.read", Target: "docs", Result: v0.DecisionResultAllow, ReasonCode: "memory-allowed", PolicyRef: &v0.PolicyReference{ID: "policy", Version: "1"}}
		if _, err := j.Append(base); err != nil {
			t.Fatal(err)
		}
		base.Kind, base.Result, base.ProviderStatus = "ProviderAttempt", "", "Attempted"
		return j, base
	}
	for _, change := range []func(*Fact){
		func(f *Fact) { f.Target = "foreign" },
		func(f *Fact) { f.Operation = "memory.write" },
		func(f *Fact) { f.PolicyRef = &v0.PolicyReference{ID: "foreign", Version: "1"} },
		func(f *Fact) { f.Memory = &MemoryMetadata{Status: "Empty"} },
	} {
		j, attempt := newAttempt(t)
		change(&attempt)
		if _, err := j.Append(attempt); err == nil {
			t.Fatal("mis-correlated memory attempt accepted")
		}
	}
	for _, metadata := range []*MemoryMetadata{
		nil, {Status: "private-status-sentinel"}, {Status: "Empty", Count: 1}, {Status: "Found"},
		{Status: "Written", Count: 1, References: []string{"memory:00000000000000000000000000000001"}},
		{Status: "Found", Count: 1, References: []string{"private-body-sentinel"}},
		{Status: "Empty", DurationMilliseconds: -1},
	} {
		j, attempt := newAttempt(t)
		if _, err := j.Append(attempt); err != nil {
			t.Fatal(err)
		}
		attempt.Kind, attempt.ProviderStatus, attempt.Memory = "ProviderOutcome", "Succeeded", metadata
		if _, err := j.Append(attempt); err == nil {
			t.Fatalf("invalid metadata accepted: %+v", metadata)
		}
	}
	j, attempt := newAttempt(t)
	if _, err := j.Append(attempt); err != nil {
		t.Fatal(err)
	}
	attempt.Kind, attempt.ProviderStatus = "ProviderOutcome", "Succeeded"
	attempt.Memory = &MemoryMetadata{Status: "Found", Count: 1, References: []string{"memory:00000000000000000000000000000001"}}
	if _, err := j.Append(attempt); err != nil {
		t.Fatal(err)
	}
	attempt.Memory.References[0] = "mutated"
	got := j.ForClaim("claim:a")
	got[2].Memory.References[0] = "mutated"
	if j.ForClaim("claim:a")[2].Memory.References[0] != "memory:00000000000000000000000000000001" {
		t.Fatal("metadata mutation rewrote journal")
	}
}

func TestMemoryMetadataCannotDecorateOtherProvider(t *testing.T) {
	j := newRegisteredJournal(t)
	f := Fact{Kind: "ToolDecision", RequestRef: "a", ClaimID: "claim:a", InvocationID: "inv:tool", Result: v0.DecisionResultAllow, ReasonCode: "allowed", PolicyRef: &v0.PolicyReference{ID: "policy", Version: "1"}, Memory: &MemoryMetadata{Status: "Empty"}}
	if _, err := j.Append(f); err == nil {
		t.Fatal("memory metadata accepted outside memory outcome")
	}
}
