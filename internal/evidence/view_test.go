// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/facts"
	"strings"
	"testing"
	"time"
)

// Golden projections fix the public envelope and canonical typed/correlation
// fields while the actual full view retains canonical state and ordered facts.
func TestViewGoldenJSONCasesAndStableSerialization(t *testing.T) {
	for _, tc := range []struct{ name, result, phase, outcome, expected string }{
		{"success", "Allow", "Succeeded", "Succeeded", `{"version":"agenova.evidence/v0","requestRef":"request:1","principal":"user:1","decision":"decision:1","result":"Allow","claim":"claim:1","phase":"Succeeded","invocation":"inv:1","outcome":"Succeeded"}`},
		{"preclaim-denial", "Deny", "", "Deny", `{"version":"agenova.evidence/v0","requestRef":"request:1","principal":"user:1","decision":"decision:1","result":"Deny","claim":"","phase":"","invocation":"","outcome":"Deny"}`},
		{"terminal-failure", "Allow", "Failed", "Failed", `{"version":"agenova.evidence/v0","requestRef":"request:1","principal":"user:1","decision":"decision:1","result":"Allow","claim":"claim:1","phase":"Failed","invocation":"inv:1","outcome":"Failed"}`},
		{"approval-not-granted", "ApprovalRequired", "", "ApprovalRequired", `{"version":"agenova.evidence/v0","requestRef":"request:1","principal":"user:1","decision":"decision:1","result":"ApprovalRequired","claim":"","phase":"","invocation":"","outcome":"ApprovalRequired"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := v0.PolicyReference{ID: "policy:1", Version: "1"}
			state := &v0.IssuedState{
				RequestRef: "request:1",
				Principal:  v0.Principal{Subject: "user:1", Team: "team-a", AuthenticationContext: "trusted-test"},
				Action:     v0.Action{Name: "claim.create", Project: "project:1", TemplateRef: "template:1"},
				PolicyRef:  policy,
				Decision:   v0.Decision{ID: "decision:1", PrincipalRef: "user:1", Action: "claim.create", Result: v0.DecisionResult(tc.result), Reason: "stable reason", PolicyRef: policy},
				Evidence:   v0.Evidence{RequestRef: "request:1", DecisionIDs: []string{"decision:1"}, RuntimeEvents: []v0.EvidenceRuntimeEvent{}, ToolInvocations: []v0.EvidenceToolInvocation{}, ModelInvocations: []v0.EvidenceModelInvocation{}},
			}
			view := View{Version: "agenova.evidence/v0", RequestRef: state.RequestRef, State: state, Facts: []facts.Fact{}, Outcome: &Outcome{Status: tc.outcome}}
			if tc.phase != "" {
				state.EffectiveAuthority = &v0.EffectiveAuthority{ID: "authority:1", ModelProfile: "model:1", Runtime: v0.EffectiveAuthorityRuntime{ProfileRef: "runtime:1", Timeout: v0.Duration(time.Minute)}}
				state.Claim = &v0.SandboxClaim{ID: "claim:1", RequestRef: state.RequestRef, TemplateRef: state.Action.TemplateRef, AuthorityRef: state.EffectiveAuthority.ID, Phase: v0.ClaimPhase(tc.phase), BackendIdentity: &v0.SandboxClaimBackendIdentity{Backend: "opaque-runtime", WorkerID: "worker:1"}}
				state.Evidence.ClaimID = state.Claim.ID
				view.Facts = []facts.Fact{{ID: "fact:1", Sequence: 1, Timestamp: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Kind: "ModelDecision", RequestRef: state.RequestRef, ClaimID: state.Claim.ID, InvocationID: "inv:1", Result: v0.DecisionResultAllow}}
			}
			if err := v0.ValidateIssuedState(state); err != nil {
				t.Fatalf("golden case must carry a valid canonical state: %v", err)
			}
			copy := Clone(view)
			data, err := json.Marshal(view)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := json.Marshal(copy)
			if err != nil || string(data) != string(roundTrip) {
				t.Fatal("view serialization drifted")
			}
			projection := struct {
				Version    string `json:"version"`
				Ref        string `json:"requestRef"`
				Principal  string `json:"principal"`
				Decision   string `json:"decision"`
				Result     string `json:"result"`
				Claim      string `json:"claim"`
				Phase      string `json:"phase"`
				Invocation string `json:"invocation"`
				Outcome    string `json:"outcome"`
			}{Version: copy.Version, Ref: copy.RequestRef, Principal: copy.State.Principal.Subject, Decision: copy.State.Decision.ID, Result: string(copy.State.Decision.Result), Outcome: copy.Outcome.Status}
			if copy.State.Claim != nil {
				projection.Claim = copy.State.Claim.ID
				projection.Phase = string(copy.State.Claim.Phase)
				projection.Invocation = copy.Facts[0].InvocationID
			}
			got, _ := json.Marshal(projection)
			if string(got) != tc.expected {
				t.Fatalf("golden mismatch: %s", got)
			}
			if tc.phase == "" && strings.Contains(string(data), `"claimId"`) {
				t.Fatal("fabricated preclaim evidence")
			}
			copy.Outcome.Status = "mutated"
			if view.Outcome.Status == "mutated" {
				t.Fatal("outcome not defensive")
			}
		})
	}
}
