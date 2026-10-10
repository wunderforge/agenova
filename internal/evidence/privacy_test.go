// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func privateMemoryView() View {
	return View{Version: "agenova.evidence/v0", RequestRef: "private-test", Request: &v0.ClaimRequest{
		Metadata: v0.ObjectMeta{Name: "private-test"}, Spec: v0.ClaimRequestSpec{
			TemplateRef: "engineer", ProjectRef: "payments",
			Task: &v0.ClaimRequestTask{Type: "investigation", Input: map[string]any{
				"objective": "private-input-sentinel", "nested": map[string]any{"body": "stored-body-sentinel"},
			}}, RequestedAccess: v0.ClaimRequestedAccess{MemoryScopes: []string{"team-docs"}},
		}}, Outcome: &Outcome{Status: "Succeeded", Text: "private-result-sentinel"}}
}

func TestPublicMemoryProjectionAndSerialization(t *testing.T) {
	for _, intent := range []string{"scopes", "operations", "both"} {
		t.Run(intent, func(t *testing.T) {
			private := privateMemoryView()
			if intent != "scopes" {
				private.Request.Spec.RequestedAccess.MemoryOperations = []string{"read"}
			}
			if intent == "operations" {
				private.Request.Spec.RequestedAccess.MemoryScopes = nil
			}
			before := Clone(private)
			public := ProjectPublic(private)
			if !ValidContentProjection(public) || !reflect.DeepEqual(public, ProjectPublic(public)) {
				t.Fatal("projection is invalid or not idempotent")
			}
			if !reflect.DeepEqual(before, Clone(private)) || private.Outcome.Text == "" || len(private.Request.Spec.Task.Input) != 2 {
				t.Fatal("private execution data was changed")
			}
			for _, value := range []any{private, public, []View{private}} {
				data, err := MarshalPublic(value)
				if err != nil {
					t.Fatal(err)
				}
				for _, sentinel := range []string{"private-input-sentinel", "stored-body-sentinel", "private-result-sentinel"} {
					if strings.Contains(string(data), sentinel) {
						t.Fatal("public serialization leaked content")
					}
				}
				if !strings.Contains(string(data), `"input":{}`) || strings.Contains(string(data), `"text":`) {
					t.Fatal("wire projection omitted empty input or retained text")
				}
			}
			encoded, _ := MarshalPublic(public)
			var decoded View
			if json.Unmarshal(encoded, &decoded) != nil || !ValidContentProjection(decoded) || decoded.Request.Spec.Task.Type != "investigation" || decoded.Request.Spec.ProjectRef != "payments" {
				t.Fatal("projection lost typed request metadata")
			}
		})
	}
}

func TestProjectionRejectsCorruptionAndPreservesOrdinaryWork(t *testing.T) {
	mutations := []func(*View){
		func(v *View) { v.ContentRedactions = nil },
		func(v *View) { v.ContentRedactions = []string{TaskInputRedaction} },
		func(v *View) { v.ContentRedactions[1] = "unknown.path" },
		func(v *View) { v.ContentRedactions[1] = TaskInputRedaction },
		func(v *View) { v.Request.Spec.Task.Input["objective"] = "hidden" },
		func(v *View) { v.Request.Spec.Task.Input = nil },
		func(v *View) { v.Request.Spec.Task = nil },
		func(v *View) { v.Outcome.Text = "hidden" },
	}
	for index, mutate := range mutations {
		view := ProjectPublic(privateMemoryView())
		mutate(&view)
		if ValidContentProjection(view) {
			t.Fatalf("accepted corrupt projection %d", index)
		}
	}
	ordinary := privateMemoryView()
	ordinary.Request.Spec.RequestedAccess = v0.ClaimRequestedAccess{}
	if !ValidContentProjection(ordinary) || !reflect.DeepEqual(ProjectPublic(ordinary), Clone(ordinary)) {
		t.Fatal("ordinary Work was redacted")
	}
	data, _ := json.Marshal(ordinary)
	if !strings.Contains(string(data), "private-result-sentinel") || !strings.Contains(string(data), "private-input-sentinel") {
		t.Fatal("ordinary Work content disappeared")
	}
	ordinary.ContentRedactions = []string{TaskInputRedaction, OutcomeTextRedaction}
	if ValidContentProjection(ordinary) {
		t.Fatal("accepted misleading ordinary Work redaction")
	}
}
