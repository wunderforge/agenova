// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/accessadvisor"
	"github.com/wunderforge/agenova/internal/evidence"
	"testing"
)

type objectiveSelector struct{ t *testing.T }

func (s objectiveSelector) Select(_ context.Context, c accessadvisor.SelectionContext) (accessadvisor.Choice, error) {
	if c.Task.Type != "" || c.Task.Input["objective"] != "Answer the ordinary business question" {
		s.t.Fatal("submission invented a category", c.Task)
	}
	return accessadvisor.Choice{ProfileID: "model-answer", Explanation: "Test model selector, no task category"}, nil
}

func TestOrdinaryHTTPManualAutomaticAndCanonicalRetry(t *testing.T) {
	s, h, _, _, _ := advisorFixture(t)
	var err error
	s.advisor, err = accessadvisor.New(&v0.AccessAdvisorConfig{Enabled: true, Engine: "model", ModelProfile: "planner", DefaultMode: "automatic"}, objectiveSelector{t})
	if err != nil {
		t.Fatal(err)
	}
	_, r := adviceBody("untyped-auto", "automatic")
	r.Spec.Task.Type = ""
	r.Spec.Task.Input["objective"] = "Answer the ordinary business question"
	body, _ := json.Marshal(adviceInput{Mode: "automatic", Request: r})
	first := httpCall(h, "POST", "/api/access-submit", body, memberHeaders())
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	var view evidence.View
	if err = json.Unmarshal(first.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Request.Spec.Task.Type != "" || view.AccessAdvice == nil {
		t.Fatal("ordinary request was classified or lost provenance")
	}
	// Empty and omitted type must identify the same logical request, not infer twice.
	var generic map[string]any
	json.Unmarshal(body, &generic)
	generic["request"].(map[string]any)["spec"].(map[string]any)["task"].(map[string]any)["type"] = ""
	retry, _ := json.Marshal(generic)
	again := httpCall(h, "POST", "/api/access-submit", retry, memberHeaders())
	if again.Code != 200 || s.advisor.Info().CallsUsed != 1 {
		t.Fatal("retry inferred again", again.Code, again.Body.String())
	}
	r.Metadata.Name = "untyped-manual"
	r.Spec.RequestedAccess = v0.ClaimRequestedAccess{ModelProfile: "approved-coding-model"}
	explicit, _ := json.Marshal(r)
	manual := httpCall(h, "POST", "/api/requests", explicit, memberHeaders())
	if manual.Code != 202 || s.advisor.Info().CallsUsed != 1 {
		t.Fatal("Manual needs category or Advisor", manual.Code, manual.Body.String())
	}
}
