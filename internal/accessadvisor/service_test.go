// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package accessadvisor

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/toolbackend"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type selectorFunc func(context.Context, SelectionContext) (Choice, error)

func (f selectorFunc) Select(c context.Context, s SelectionContext) (Choice, error) { return f(c, s) }

type providerFunc func(context.Context, modelprovider.Request) (modelprovider.Result, error)

func (f providerFunc) Complete(c context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	return f(c, r)
}
func fixture() (*v0.ClaimRequest, *v0.AgentTemplate, []toolbackend.Descriptor, v0.Principal) {
	t := app.ReferenceTemplate()
	t.Spec.AccessGuide = &v0.AgentAccessGuide{Version: "1", Markdown: "Read evidence only; no write operation is required.", Profiles: []v0.TaskAccessProfile{{ID: "read-evidence", TaskType: "investigation", Description: "Read repository evidence", RequestedAccess: v0.ClaimRequestedAccess{Tools: []string{"git.read"}, ResourceScopes: []string{"repo:acme/payments"}}}}}
	timeout := v0.Duration(time.Minute)
	r := &v0.ClaimRequest{APIVersion: v0.ClaimRequestAPIVersion, Kind: v0.ClaimRequestKind, Metadata: v0.ObjectMeta{Name: "advice-test"}, Spec: v0.ClaimRequestSpec{TemplateRef: t.Metadata.Name, ProjectRef: "payments", Task: &v0.ClaimRequestTask{Type: "investigation", Input: map[string]any{"objective": "Read repository evidence."}}, Runtime: &v0.ClaimRuntimeRequirements{ProfileRef: t.Spec.CapabilityCeiling.RuntimeProfiles[0], Timeout: &timeout}}}
	return r, t, []toolbackend.Descriptor{{Operation: "git.read", ResourceScope: "repo:acme/payments"}}, v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}
}
func enabled() *v0.AccessAdvisorConfig {
	return &v0.AccessAdvisorConfig{Enabled: true, Engine: "rules", MaxCalls: 8}
}

func TestPlanningIsIntentOnlyAndCacheIsOwnerBound(t *testing.T) {
	r, template, catalog, p := fixture()
	s, _ := New(enabled(), Rules{})
	plan, e := s.Plan(context.Background(), p, r, template, catalog)
	if e != nil || plan.Status != "ready" {
		t.Fatal(plan, e)
	}
	if HasAccess(r.Spec.RequestedAccess) || plan.Request.Spec.RequestedAccess.Tools[0] != "git.read" || plan.Model != nil {
		t.Fatal("planning mutated intent or invented model")
	}
	again, e := s.Plan(context.Background(), p, r, template, catalog)
	if e != nil || again.ID != plan.ID || s.Info().CallsUsed != 1 {
		t.Fatal("replanned", e)
	}
	again.Request.Spec.RequestedAccess.Tools[0] = "tampered"
	own, _ := s.Get(p, plan.ID)
	if own.Request.Spec.RequestedAccess.Tools[0] != "git.read" {
		t.Fatal("mutable cache escaped")
	}
	other := p
	other.Subject = "bob"
	if _, e = s.Get(other, plan.ID); e == nil {
		t.Fatal("cross-user receipt exposed")
	}
	r.Spec.Task.Input["objective"] = "different"
	if _, e = s.Plan(context.Background(), p, r, template, catalog); e == nil {
		t.Fatal("name reused for changed task")
	}
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	if _, e = s.Get(p, plan.ID); e == nil {
		t.Fatal("expired receipt accepted")
	}
}

func TestConcurrentSameIntentCallsSelectorOnce(t *testing.T) {
	r, template, catalog, p := fixture()
	var calls atomic.Int32
	gate := make(chan struct{})
	s, _ := New(enabled(), selectorFunc(func(ctx context.Context, c SelectionContext) (Choice, error) {
		calls.Add(1)
		<-gate
		return Rules{}.Select(ctx, c)
	}))
	var wg sync.WaitGroup
	var errorsFound atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Plan(context.Background(), p, r, template, catalog); e != nil {
				errorsFound.Add(1)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(gate)
	wg.Wait()
	if calls.Load() != 1 || errorsFound.Load() != 0 {
		t.Fatal(calls.Load(), errorsFound.Load())
	}
}

func TestInvalidAmbiguousUnavailableAndBudgetNeverInventAccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		choice Choice
		fail   bool
	}{{"unknown", Choice{ProfileID: "admin-all"}, true}, {"clarification", Choice{Questions: []string{"Which task?"}}, false}, {"profile-and-question", Choice{ProfileID: "read-evidence", Questions: []string{"Which repository?"}}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			r, tmp, cat, p := fixture()
			s, _ := New(enabled(), selectorFunc(func(context.Context, SelectionContext) (Choice, error) { return tc.choice, nil }))
			plan, e := s.Plan(context.Background(), p, r, tmp, cat)
			if (e != nil) != tc.fail || plan.Request != nil {
				t.Fatal(plan, e)
			}
		})
	}
	r, tmp, cat, p := fixture()
	cfg := enabled()
	cfg.MaxCalls = 1
	s, _ := New(cfg, Rules{})
	if _, e := s.Plan(context.Background(), p, r, tmp, cat); e != nil {
		t.Fatal(e)
	}
	r.Metadata.Name = "next"
	if _, e := s.Plan(context.Background(), p, r, tmp, cat); e == nil {
		t.Fatal("unbounded budget")
	}
	s, _ = New(enabled(), Rules{})
	r.Metadata.Name = "unavailable"
	if _, e := s.Plan(context.Background(), p, r, tmp, nil); e == nil {
		t.Fatal("unconfigured capability suggested")
	}
	s, _ = New(enabled(), Rules{})
	r.Spec.RequestedAccess.Tools = []string{"git.read"}
	if _, e := s.Plan(context.Background(), p, r, tmp, cat); e == nil {
		t.Fatal("explicit access overwritten")
	}
}

func TestTimeoutFailureAndModeBounds(t *testing.T) {
	r, tmp, cat, p := fixture()
	cfg := enabled()
	cfg.Timeout = "5ms"
	var n atomic.Int32
	s, _ := New(cfg, selectorFunc(func(ctx context.Context, _ SelectionContext) (Choice, error) {
		n.Add(1)
		<-ctx.Done()
		return Choice{}, ctx.Err()
	}))
	if _, e := s.Plan(context.Background(), p, r, tmp, cat); e == nil {
		t.Fatal("timeout ignored")
	}
	if _, e := s.Plan(context.Background(), p, r, tmp, cat); e == nil || n.Load() != 1 {
		t.Fatal("failure retried implicitly")
	}
	off, _ := New(nil, nil)
	if off.Allows("automatic") || off.Info().DefaultMode != "manual" {
		t.Fatal("disabled defaults")
	}
	cfg = enabled()
	cfg.AllowedModes = []string{"suggest"}
	cfg.DefaultMode = "suggest"
	limited, _ := New(cfg, Rules{})
	if limited.Allows("automatic") {
		t.Fatal("mode escalated")
	}
}

func TestModelOutputCannotSupplyArbitraryAuthority(t *testing.T) {
	_, tmp, _, _ := fixture()
	for _, text := range []string{`{"profileID":"read-evidence","explanation":"read","questions":[],"effectiveAuthority":{"tools":["*"]}}`, `{"profileID":"read-evidence","explanation":"read","questions":[]} {}`, "```json\n{}\n```", `{}`, `{"profileID":"read-evidence","questions":[]}`, `{"profileID":"read-evidence","explanation":"read","questions":null}`, `{"profileID":null,"explanation":"read","questions":[]}`, `{"profileID":"read-evidence","explanation":null,"questions":[]}`} {
		m := ModelSelector{Profile: "planner", Client: providerFunc(func(context.Context, modelprovider.Request) (modelprovider.Result, error) {
			return modelprovider.Result{Text: text, Model: "test-model"}, nil
		})}
		if _, e := m.Select(context.Background(), SelectionContext{Guide: tmp.Spec.AccessGuide}); e == nil {
			t.Fatal("untrusted output accepted", text)
		}
	}
	m := ModelSelector{Profile: "planner", Client: providerFunc(func(context.Context, modelprovider.Request) (modelprovider.Result, error) {
		return modelprovider.Result{}, errors.New("private upstream error")
	})}
	if _, e := m.Select(context.Background(), SelectionContext{}); e == nil || e.Error() == "private upstream error" {
		t.Fatal("provider error leaked")
	}
}

func TestConcurrentCapacityDoesNotSpendOrWaitForAnotherIntent(t *testing.T) {
	r, tmp, cat, p := fixture()
	entered, release := make(chan struct{}), make(chan struct{})
	s, _ := New(enabled(), selectorFunc(func(ctx context.Context, c SelectionContext) (Choice, error) {
		close(entered)
		<-release
		return Rules{}.Select(ctx, c)
	}))
	done := make(chan error, 1)
	go func() { _, e := s.Plan(context.Background(), p, r, tmp, cat); done <- e }()
	<-entered
	b, _ := json.Marshal(r)
	var other v0.ClaimRequest
	_ = json.Unmarshal(b, &other)
	other.Metadata.Name = "another-intent"
	_, err := s.Plan(context.Background(), p, &other, tmp, cat)
	if e, ok := err.(*Error); !ok || e.Code != "advisor_busy" {
		t.Errorf("expected busy, got %v", err)
	}
	if s.Info().CallsUsed != 1 {
		t.Error("busy attempt consumed another call")
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}

func TestRequiredGrantChecksAllDimensions(t *testing.T) {
	a := v0.ClaimRequestedAccess{Tools: []string{"x"}, ResourceScopes: []string{"s"}, ModelProfile: "m", NetworkProfiles: []string{v0.NetworkProfilePublicWeb}}
	g := &v0.EffectiveAuthority{Tools: []string{"x"}, ResourceScopes: []string{"s"}, ModelProfile: "m"}
	if RequiredGranted(a, g) {
		t.Fatal("missing network ignored")
	}
	g.NetworkProfiles = a.NetworkProfiles
	if !RequiredGranted(a, g) {
		t.Fatal("matching grant rejected")
	}
}

// Opt-in uses an existing loopback model; never downloads or calls a paid host.
func TestRealLocalOllamaAdvisor(t *testing.T) {
	if os.Getenv("AGENOVA_TEST_LOCAL_ADVISOR") != "1" {
		t.Skip("opt-in real local inference")
	}
	client, e := modelprovider.New(modelprovider.Config{Endpoint: "http://127.0.0.1:11434/v1", Models: map[string]string{"planner": "qwen2.5-coder:7b"}, MaxTokens: 256, Timeout: time.Minute})
	if e != nil {
		t.Fatal(e)
	}
	r, tmp, cat, p := fixture()
	cfg := enabled()
	cfg.Engine = "model"
	cfg.ModelProfile = "planner"
	cfg.Timeout = "60s"
	s, e := Configured(cfg, client)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := s.Plan(context.Background(), p, r, tmp, cat)
	if e != nil || plan.Status != "ready" || plan.Model == nil || plan.Model.Name == "" {
		t.Fatal(plan, e)
	}
	b, _ := json.Marshal(map[string]any{"status": plan.Status, "profile": plan.ProfileID, "model": plan.Model, "guideDigest": plan.GuideDigest})
	t.Log(string(b))
}
