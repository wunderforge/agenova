// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/accessadvisor"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/workstore"
	"net/http"
	"strings"
	"testing"
	"time"
)

type advisorTemplate struct{ t *v0.AgentTemplate }

func (a advisorTemplate) Lookup(string) (*v0.AgentTemplate, error) {
	b, _ := json.Marshal(a.t)
	var copy v0.AgentTemplate
	_ = json.Unmarshal(b, &copy)
	return &copy, nil
}
func advisorFixture(t *testing.T) (*Service, http.Handler, *verticalBackend, *policy.Loader, *testAuth) {
	t.Helper()
	template := app.ReferenceTemplate()
	template.Spec.AccessGuide = &v0.AgentAccessGuide{Version: "1", Profiles: []v0.TaskAccessProfile{{ID: "model-answer", TaskType: "repository-change", Description: "Answer with the configured model", RequestedAccess: v0.ClaimRequestedAccess{ModelProfile: template.Spec.Defaults.ModelProfile}}}}
	loader := &policy.Loader{}
	if e := loader.Load(app.ReferencePolicy()); e != nil {
		t.Fatal(e)
	}
	planner, e := accessadvisor.New(&v0.AccessAdvisorConfig{Enabled: true, Engine: "rules", DefaultMode: "automatic"}, accessadvisor.Rules{})
	if e != nil {
		t.Fatal(e)
	}
	backend := &verticalBackend{}
	s, e := NewServiceWithOptions(backend, httpExecutor{}, &httpProvider{}, app.ReferencePrincipalTeamA, Options{Advisor: planner, Setup: memberTestSetup, SetupFor: func(_ context.Context, p v0.Principal, project, name string) (Setup, error) {
		return Setup{Project: project, Template: template, Policy: app.ReferencePolicy()}, nil
	}, Configure: memberTestConfigure, Access: memberAccess{}, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, advisorTemplate{template})
	}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	auth := &testAuth{}
	h, e := AuthenticatedHandler(s, auth)
	if e != nil {
		t.Fatal(e)
	}
	return s, h, backend, loader, auth
}
func adviceBody(name, mode string) ([]byte, *v0.ClaimRequest) {
	var r v0.ClaimRequest
	_ = json.Unmarshal(httpInput(name), &r)
	r.Spec.RequestedAccess = v0.ClaimRequestedAccess{}
	b, _ := json.Marshal(adviceInput{Mode: mode, Request: &r})
	return b, &r
}
func memberHeaders() map[string]string { return map[string]string{"Authorization": "Bearer alice"} }

func TestAdvisorHTTPAutoExplicitAndRetry(t *testing.T) {
	s, h, b, _, _ := advisorFixture(t)
	data, _ := adviceBody("automatic", "automatic")
	res := httpCall(h, "POST", "/api/access-submit", data, memberHeaders())
	if res.Code != 202 {
		t.Fatal(res.Code, res.Body.String())
	}
	var first evidence.View
	_ = json.Unmarshal(res.Body.Bytes(), &first)
	if first.AccessAdvice == nil || first.AccessAdvice.Mode != "automatic" {
		t.Fatal("provenance absent")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, _ := s.QueryRequest(first.RequestRef)
		if v.Outcome != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	calls := b.calls.Load()
	again := httpCall(h, "POST", "/api/access-submit", data, memberHeaders())
	if again.Code != 200 || b.calls.Load() != calls || s.advisor.Info().CallsUsed != 1 {
		t.Fatal("duplicate plan or allocation", again.Code, again.Body.String())
	}
	manual := httpCall(h, "POST", "/api/requests", httpInput("explicit"), memberHeaders())
	if manual.Code != 202 || s.advisor.Info().CallsUsed != 1 {
		t.Fatal("explicit request implicitly planned", manual.Code, manual.Body.String())
	}
}

func TestAdvisorSuggestionPrivateAndEditedAccessStillGoverned(t *testing.T) {
	s, h, b, _, _ := advisorFixture(t)
	data, _ := adviceBody("suggestion", "suggest")
	res := httpCall(h, "POST", "/api/access-advice", data, memberHeaders())
	if res.Code != 200 || b.calls.Load() != 0 {
		t.Fatal(res.Code, res.Body.String())
	}
	var plan accessadvisor.Plan
	_ = json.Unmarshal(res.Body.Bytes(), &plan)
	if plan.Status != "ready" {
		t.Fatal(plan)
	}
	foreign := httpCall(h, "GET", "/api/access-advice/"+plan.ID, nil, map[string]string{"Authorization": "Bearer bob"})
	if foreign.Code != 404 {
		t.Fatal("foreign advice exposed", foreign.Code)
	}
	headers := memberHeaders()
	headers["X-Agenova-Advice-ID"] = plan.ID
	// An edited request is still resolved by the normal authority path.
	plan.Request.Spec.RequestedAccess.ModelProfile = ""
	raw, _ := json.Marshal(plan.Request)
	accepted := httpCall(h, "POST", "/api/requests", raw, headers)
	if accepted.Code != 202 {
		t.Fatal(accepted.Code, accepted.Body.String())
	}
	var v evidence.View
	_ = json.Unmarshal(accepted.Body.Bytes(), &v)
	if v.AccessAdvice == nil || v.AccessAdvice.Mode != "suggest" || v.AccessAdvice.AcceptedRequestDigest != accessadvisor.Digest(plan.Request) {
		t.Fatal("missing accepted provenance")
	}
	// A different task cannot take the original receipt, even for the same caller.
	plan.Request.Spec.Task.Input["objective"] = "changed"
	raw, _ = json.Marshal(plan.Request)
	bad := httpCall(h, "POST", "/api/requests", raw, headers)
	if bad.Code != 422 || !strings.Contains(bad.Body.String(), "advice_stale") {
		t.Fatal("changed task reused receipt", bad.Code, bad.Body.String())
	}
	_ = s
}

func TestAdvisorCurrentPolicyNarrowingStopsAllocation(t *testing.T) {
	_, h, b, loader, _ := advisorFixture(t)
	bundle := app.ReferencePolicy()
	bundle.Version = "2"
	bundle.Rules[0].Constraints = &policy.AuthorityConstraints{ModelProfiles: &policy.StringSetConstraint{Values: []string{}}}
	if e := loader.Load(bundle); e != nil {
		t.Fatal(e)
	}
	data, _ := adviceBody("narrowed", "automatic")
	res := httpCall(h, "POST", "/api/access-submit", data, memberHeaders())
	if res.Code != 422 || b.calls.Load() != 0 || !(strings.Contains(res.Body.String(), "required_access_missing") || strings.Contains(res.Body.String(), "authority_request_rejected")) {
		t.Fatal(res.Code, res.Body.String(), b.calls.Load())
	}
}

func TestAdvisorToolPermissionRemovedByPolicyStopsBeforeAnyRuntimeCall(t *testing.T) {
	s, h, b, loader, _ := advisorFixture(t)
	setup, err := s.setupFor(context.Background(), v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}, "payments", app.ReferenceTemplate().Metadata.Name)
	if err != nil {
		t.Fatal(err)
	}
	profile := &setup.Template.Spec.AccessGuide.Profiles[0]
	profile.RequestedAccess.Tools = []string{"git.read"}
	profile.RequestedAccess.ResourceScopes = []string{"repo:acme/payments"}
	tool := &toolDouble{}
	s.tools = boundTools(t, tool, "git.read", "repo:acme/payments")
	bundle := app.ReferencePolicy()
	bundle.Version = "2"
	bundle.Rules[0].Constraints = &policy.AuthorityConstraints{Tools: &policy.StringSetConstraint{Values: []string{}}}
	if e := loader.Load(bundle); e != nil {
		t.Fatal(e)
	}
	data, _ := adviceBody("tool-narrowed", "automatic")
	res := httpCall(h, "POST", "/api/access-submit", data, memberHeaders())
	if res.Code != 422 || !(strings.Contains(res.Body.String(), "required_access_missing") || strings.Contains(res.Body.String(), "authority_request_rejected")) || b.calls.Load() != 0 || tool.calls.Load() != 0 {
		t.Fatal(res.Code, res.Body.String(), b.calls.Load(), tool.calls.Load())
	}
}

func TestAdvisorRejectsChangedGuideAndDisallowedAutomaticMode(t *testing.T) {
	s, h, b, _, _ := advisorFixture(t)
	data, _ := adviceBody("stale-guide", "suggest")
	res := httpCall(h, "POST", "/api/access-advice", data, memberHeaders())
	var plan accessadvisor.Plan
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &plan) != nil {
		t.Fatal(res.Code, res.Body.String())
	}
	setup, _ := s.setupFor(context.Background(), v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}, "payments", plan.Template)
	setup.Template.Spec.AccessGuide.Markdown = "Updated author guidance"
	headers := memberHeaders()
	headers["X-Agenova-Advice-ID"] = plan.ID
	raw, _ := json.Marshal(plan.Request)
	if r := httpCall(h, "POST", "/api/requests", raw, headers); r.Code != 422 || !strings.Contains(r.Body.String(), "advice_stale") {
		t.Fatal(r.Code, r.Body.String())
	}
	s.advisor, _ = accessadvisor.New(&v0.AccessAdvisorConfig{Enabled: true, AllowedModes: []string{"suggest"}, DefaultMode: "suggest"}, accessadvisor.Rules{})
	data, _ = adviceBody("mode-disabled", "automatic")
	if r := httpCall(h, "POST", "/api/access-submit", data, memberHeaders()); r.Code != 403 {
		t.Fatal(r.Code, r.Body.String())
	}
	if b.calls.Load() != 0 || s.advisor.Info().CallsUsed != 0 {
		t.Fatal("rejected request planned or allocated")
	}
}

// Simulate a fresh process receiving previously persisted accepted evidence.
// This tests restart orchestration, not a real PostgreSQL restart.
type storedAdviceWork struct {
	DurableWork
	owner v0.Principal
	view  evidence.View
}

func (s storedAdviceWork) GetFor(_ context.Context, p v0.Principal, ref string) (evidence.View, error) {
	if p != s.owner || ref != s.view.RequestRef {
		return evidence.View{}, workstore.ErrNotFound
	}
	return evidence.Clone(s.view), nil
}
func TestAdvisorAutomaticRecoversStoredWorkWithoutDraftOrNewInference(t *testing.T) {
	s, h, b, _, _ := advisorFixture(t)
	data, intent := adviceBody("accepted-before-restart", "automatic")
	p := v0.Principal{Subject: "Bearer alice", Team: "team-a", AuthenticationContext: "oidc"}
	v := evidence.View{Version: "agenova.evidence/v0", RequestRef: workstore.Reference(p, intent.Metadata.Name), Request: intent, AccessAdvice: &evidence.AccessAdvice{ID: "prior-advice", Mode: "automatic", IntentDigest: accessadvisor.IntentDigest(intent)}}
	s.durable = storedAdviceWork{owner: p, view: v}
	res := httpCall(h, "POST", "/api/access-submit", data, memberHeaders())
	if res.Code != 200 || s.advisor.Info().CallsUsed != 0 || b.calls.Load() != 0 {
		t.Fatal(res.Code, res.Body.String())
	}
	var recovered evidence.View
	if json.Unmarshal(res.Body.Bytes(), &recovered) != nil || recovered.AccessAdvice.ID != "prior-advice" {
		t.Fatal("accepted provenance was lost")
	}
}

func TestAdvisorRejectsUnauthorizedExplicitAndUntrustedContext(t *testing.T) {
	s, h, b, _, auth := advisorFixture(t)
	data, r := adviceBody("invalid", "automatic")
	unauth := httpCall(h, "POST", "/api/access-submit", data, nil)
	if unauth.Code != 401 {
		t.Fatal(unauth.Code)
	}
	r.Spec.ProjectRef = "other"
	raw, _ := json.Marshal(adviceInput{Mode: "automatic", Request: r})
	denied := httpCall(h, "POST", "/api/access-submit", raw, memberHeaders())
	if denied.Code != 403 {
		t.Fatal(denied.Code)
	}
	r.Spec.ProjectRef = "payments"
	r.Spec.RequestedAccess.Tools = []string{"git.read"}
	raw, _ = json.Marshal(adviceInput{Mode: "automatic", Request: r})
	if res := httpCall(h, "POST", "/api/access-submit", raw, memberHeaders()); res.Code != 422 {
		t.Fatal(res.Code)
	}
	injected := append([]byte(`{"principal":{"team":"admin"},`), data[1:]...)
	if res := httpCall(h, "POST", "/api/access-submit", injected, memberHeaders()); res.Code != 400 {
		t.Fatal(res.Code)
	}
	auth.mu.Lock()
	auth.disabled = true
	auth.mu.Unlock()
	if res := httpCall(h, "POST", "/api/access-submit", data, memberHeaders()); res.Code != 403 {
		t.Fatal(res.Code)
	}
	if b.calls.Load() != 0 || s.advisor.Info().CallsUsed != 0 {
		t.Fatal("invalid request caused execution or planning")
	}
}
