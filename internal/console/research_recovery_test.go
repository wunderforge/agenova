// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/workstore"
)

func TestResearchCompletionSurvivesBothRecoveryWindows(t *testing.T) {
	for _, completion := range []string{"incomplete", "partial"} {
		for _, completed := range []bool{false, true} {
			t.Run(completion+map[bool]string{false: "-interrupted-cleanup", true: "-lost-final-ack"}[completed], func(t *testing.T) {
				s := compatibilitySnapshot(t)
				s.View.State.Claim.Phase = v0.ClaimPhaseSucceeded
				claim := s.View.State.Claim
				s.StagedResult = &evidence.Outcome{Status: "Succeeded", Text: "The page could not be retrieved.", Model: &evidence.ModelResult{Model: "fixture", InvocationID: "i"}, Completion: &evidence.Completion{Status: completion, Reason: "network-unavailable", Target: "direct-https"}}
				s.View.Facts = append(s.View.Facts, facts.Fact{Kind: "Runtime", RequestRef: s.View.RequestRef, ClaimID: claim.ID, Operation: "Succeeded"})
				if completed {
					s.View.Facts = append(s.View.Facts, facts.Fact{Kind: "RunOutcome", RequestRef: s.View.RequestRef, ClaimID: claim.ID, Operation: "Succeeded"})
				}
				// Persisted JSON must round-trip before either reconstruction path.
				b, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				var persisted workstore.Snapshot
				if err = json.Unmarshal(b, &persisted); err != nil {
					t.Fatal(err)
				}
				store := &recoveryActivationCrashStore{snapshot: persisted}
				backend := &recoveryTestBackend{}
				count, err := RecoverDurableWork(context.Background(), store, backend)
				if err != nil || count != 1 || store.finished == nil || store.finished.Outcome == nil {
					t.Fatal("recovery failed", count, err)
				}
				got := store.finished.Outcome
				if got.Completion == nil || *got.Completion != *s.StagedResult.Completion || got.Text != s.StagedResult.Text || got.Model.Model != "fixture" {
					t.Fatalf("lost staged result metadata: %+v", got)
				}
				if completed && (backend.stopCalls != 0 || backend.cleanupCalls != 0) {
					t.Fatal("replayed completed cleanup")
				}
			})
		}
	}
}
