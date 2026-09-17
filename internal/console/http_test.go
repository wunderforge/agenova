// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/runtime"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type httpBackend struct {
	calls    atomic.Int32
	released atomic.Bool
	claim    string
}

func (b *httpBackend) Allocate(r runtime.AllocateRequest) (runtime.Allocation, error) {
	b.calls.Add(1)
	b.claim = r.ClaimID
	return runtime.Allocation{ClaimID: r.ClaimID, Identity: v0.SandboxClaimBackendIdentity{Backend: "synthetic", WorkerID: "worker:1"}}, nil
}
func (b *httpBackend) Observe(id v0.SandboxClaimBackendIdentity) (runtime.Observation, error) {
	b.calls.Add(1)
	return runtime.Observation{ClaimID: b.claim, Identity: id, Ready: true, Released: b.released.Load()}, nil
}
func (b *httpBackend) Start(v0.SandboxClaimBackendIdentity) error     { b.calls.Add(1); return nil }
func (b *httpBackend) Terminate(v0.SandboxClaimBackendIdentity) error { b.calls.Add(1); return nil }
func (b *httpBackend) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	b.calls.Add(1)
	b.released.Store(true)
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}

type httpExecutor struct{}

func (httpExecutor) Execute(ctx context.Context, _ v0.SandboxClaimBackendIdentity, task workerprotocol.Task, handler workerprotocol.Handler) (string, error) {
	reply, err := handler(ctx, workerprotocol.Operation{ClaimID: task.ClaimID, Kind: "model", Profile: task.ModelProfile, Prompt: task.Objective})
	if err != nil {
		return "", err
	}
	if !reply.Allowed {
		return "", errors.New("not allowed")
	}
	return reply.Text, nil
}

type httpProvider struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	fail    bool
}

func (p *httpProvider) Complete(ctx context.Context, r modelprovider.Request) (modelprovider.Result, error) {
	p.calls.Add(1)
	if p.started != nil {
		close(p.started)
		select {
		case <-p.release:
		case <-ctx.Done():
			return modelprovider.Result{}, ctx.Err()
		}
	}
	if p.fail {
		return modelprovider.Result{}, errors.New("synthetic-secret-provider-detail")
	}
	return modelprovider.Result{Text: "Answer: " + r.Prompt, Model: "synthetic-model", ResponseID: "synthetic-response", InputTokens: 3, OutputTokens: 4}, nil
}
func httpService(t *testing.T, preset app.ReferencePrincipalPreset, provider *httpProvider) (*Service, *httpBackend, http.Handler) {
	t.Helper()
	b := &httpBackend{}
	s, err := NewService(b, httpExecutor{}, provider, preset)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, b, Handler(s)
}
func httpInput(name string) []byte {
	return []byte(`{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":"` + name + `"},"spec":{"templateRef":"engineer","projectRef":"payments","task":{"type":"repository-change","input":{"objective":"Explain retry backoff","repository":"acme/payments"}},"requestedAccess":{"tools":["git.read","shell.exec"],"resourceScopes":["repo:acme/payments"],"modelProfile":"approved-coding-model","memoryScopes":["team-docs"]},"runtime":{"profileRef":"standard-isolated","timeout":"45m"}}}`)
}
func httpCall(h http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://console.local"+path, bytes.NewReader(body))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPSubmissionReturnsStableOperatorDiagnostic(t *testing.T) {
	service, err := NewServiceWithOptions(&httpBackend{}, httpExecutor{}, &httpProvider{}, app.ReferencePrincipalTeamA, Options{
		Prepare: func([]byte) (app.PreparedAssignment, error) {
			return app.PreparedAssignment{}, &SubmissionError{Code: "agent_template_unavailable", Message: "AgentTemplate is unavailable; register the requested template.", Cause: errors.New("private kube detail")}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	response := httpCall(Handler(service), "POST", "/api/requests", httpInput("unknown-template"), nil)
	if response.Code != 422 || !strings.Contains(response.Body.String(), `"code":"agent_template_unavailable"`) || strings.Contains(response.Body.String(), "private kube detail") {
		t.Fatalf("unsafe diagnostic: %d %s", response.Code, response.Body.String())
	}
}
func httpView(t *testing.T, w *httptest.ResponseRecorder) evidence.View {
	t.Helper()
	var v evidence.View
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func waitHTTPOutcome(t *testing.T, h http.Handler, ref string) evidence.View {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w := httpCall(h, "GET", "/api/requests/"+url.PathEscape(ref)+"/evidence", nil, nil)
		v := httpView(t, w)
		if v.Outcome != nil {
			return v
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("terminal outcome did not become visible")
	return evidence.View{}
}

func TestHTTPSubmissionRunningThenTerminalEvidence(t *testing.T) {
	p := &httpProvider{started: make(chan struct{}), release: make(chan struct{})}
	_, b, h := httpService(t, app.ReferencePrincipalTeamA, p)
	w := httpCall(h, "POST", "/api/requests", httpInput("http-run"), map[string]string{"Origin": "http://console.local"})
	if w.Code != 202 {
		t.Fatalf("submit=%d %s", w.Code, w.Body.String())
	}
	select {
	case <-p.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	v := httpView(t, httpCall(h, "GET", "/api/requests/http-run/evidence", nil, nil))
	if v.State.Claim.Phase != v0.ClaimPhaseRunning || v.Outcome != nil {
		t.Fatalf("running view=%+v", v)
	}
	if v.State.EffectiveAuthority.Runtime.Timeout != v0.Duration(30*time.Minute) {
		t.Fatal("timeout was not narrowed")
	}
	for _, tool := range v.State.EffectiveAuthority.Tools {
		if tool == "shell.exec" {
			t.Fatal("over-ceiling tool was granted")
		}
	}
	close(p.release)
	v = waitHTTPOutcome(t, h, "http-run")
	if v.State.Claim.Phase != v0.ClaimPhaseSucceeded || v.Outcome.Text != "Answer: Explain retry backoff" || !b.released.Load() {
		t.Fatalf("terminal=%+v", v)
	}
	claim := httpView(t, httpCall(h, "GET", "/api/claims/"+url.PathEscape(v.State.Claim.ID)+"/evidence", nil, nil))
	if claim.RequestRef != v.RequestRef || claim.Outcome.Text != v.Outcome.Text {
		t.Fatal("claim and request evidence diverged")
	}
	var list []evidence.View
	w = httpCall(h, "GET", "/api/requests", nil, nil)
	if json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list) != 1 {
		t.Fatal("list is not current evidence")
	}
	if httpCall(h, "POST", "/api/requests", httpInput("http-run"), nil).Code != 409 {
		t.Fatal("duplicate did not conflict")
	}
}

func TestHTTPTeamBDeniedBeforeAllExternalWork(t *testing.T) {
	p := &httpProvider{}
	_, b, h := httpService(t, app.ReferencePrincipalTeamB, p)
	w := httpCall(h, "POST", "/api/requests", httpInput("denied"), nil)
	if w.Code != 200 {
		t.Fatalf("denial=%d %s", w.Code, w.Body.String())
	}
	v := httpView(t, w)
	if v.State.Decision.Result != v0.DecisionResultDeny || v.State.Claim != nil || v.State.EffectiveAuthority != nil || v.State.Principal.Team != "team-b" || len(v.Facts) != 2 || b.calls.Load() != 0 || p.calls.Load() != 0 {
		t.Fatalf("denial fabricated execution: %+v", v)
	}
	w = httpCall(h, "GET", "/api/setup", nil, nil)
	var setup Setup
	if json.Unmarshal(w.Body.Bytes(), &setup) != nil || setup.Principal.Team != "team-b" || setup.Template.Metadata.Name != "engineer" || setup.Capabilities["model"] != "configured" || setup.Capabilities["memory"] != "notConnected" || setup.Installation.Kind != "local-demo" {
		t.Fatalf("setup=%s", w.Body.String())
	}
}

func TestHTTPSetupProviderFailsClosed(t *testing.T) {
	backend := &httpBackend{}
	service, err := NewServiceWithOptions(backend, httpExecutor{}, &httpProvider{}, app.ReferencePrincipalTeamA, Options{
		Setup: func() (Setup, error) { return Setup{}, errors.New("synthetic-secret-registry-error") },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	w := httpCall(Handler(service), "GET", "/api/setup", nil, nil)
	if w.Code != http.StatusServiceUnavailable || strings.Contains(w.Body.String(), "synthetic-secret") || strings.Contains(w.Body.String(), "engineer") {
		t.Fatalf("invalid setup leaked or fell back: %d %s", w.Code, w.Body.String())
	}
}

func TestHTTPBoundaryFailuresDoNotLeakOrExecute(t *testing.T) {
	p := &httpProvider{}
	_, b, h := httpService(t, app.ReferencePrincipalTeamA, p)
	for _, tc := range []struct {
		name, method, path string
		body               []byte
		headers            map[string]string
		status             int
	}{
		{"malformed", "POST", "/api/requests", []byte(`{"synthetic-secret":"bad"`), nil, 400},
		{"principal", "POST", "/api/requests", bytes.Replace(httpInput("identity"), []byte(`"templateRef":"engineer"`), []byte(`"templateRef":"engineer","principal":{"subject":"synthetic-secret"}`), 1), nil, 400},
		{"oversized", "POST", "/api/requests", bytes.Repeat([]byte("x"), maxSubmissionBytes+1), nil, 413},
		{"media", "POST", "/api/requests", httpInput("media"), map[string]string{"Content-Type": "text/plain"}, 415},
		{"origin", "POST", "/api/requests", httpInput("origin"), map[string]string{"Origin": "http://evil.local"}, 403},
		{"wrong-scheme", "POST", "/api/requests", httpInput("scheme"), map[string]string{"Origin": "https://console.local"}, 403},
		{"null-origin", "POST", "/api/requests", httpInput("null"), map[string]string{"Origin": "null"}, 403},
		{"fetch-site", "GET", "/api/setup", nil, map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"method", "DELETE", "/api/requests", nil, nil, 405},
		{"setup-method", "POST", "/api/setup", nil, nil, 405},
		{"unknown", "GET", "/api/requests/missing/evidence", nil, nil, 404},
		{"invalid-ref", "GET", "/api/claims/a%2Fb/evidence", nil, nil, 400},
		{"empty-ref", "GET", "/api/requests//evidence", nil, nil, 400},
		{"unknown-route", "GET", "/api/other", nil, nil, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httpCall(h, tc.method, tc.path, tc.body, tc.headers)
			if w.Code != tc.status {
				t.Fatalf("got%d want%d: %s", w.Code, tc.status, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "synthetic-secret") || w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("leaked details or unsafe response headers")
			}
		})
	}
	if b.calls.Load() != 0 || p.calls.Load() != 0 {
		t.Fatal("invalid submissions performed work")
	}
}

func TestHTTPProviderFailureIsSanitizedEvidenceNotPermissionDenial(t *testing.T) {
	p := &httpProvider{fail: true}
	_, _, h := httpService(t, app.ReferencePrincipalTeamA, p)
	w := httpCall(h, "POST", "/api/requests", httpInput("provider-failure"), nil)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	v := waitHTTPOutcome(t, h, "provider-failure")
	if v.Outcome.Status != "Failed" || v.State.Decision.Result != v0.DecisionResultAllow || p.calls.Load() != 1 {
		t.Fatalf("provider failure=%+v", v)
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "synthetic-secret") {
		t.Fatal("raw provider detail leaked")
	}
	allowed, failed := false, false
	for _, f := range v.Facts {
		if f.Kind == "ModelDecision" && f.Result == v0.DecisionResultAllow {
			allowed = true
		}
		if f.Kind == "ProviderOutcome" && f.ProviderStatus == "Failed" {
			failed = true
		}
	}
	if !allowed || !failed {
		t.Fatal("permission and provider outcome are not distinguished")
	}
}

func TestHTTPBoundedProcessRecordsAndReferences(t *testing.T) {
	p := &httpProvider{}
	_, b, h := httpService(t, app.ReferencePrincipalTeamB, p)
	for i := 0; i < 32; i++ {
		name := strings.Repeat("a", i+1)
		if w := httpCall(h, "POST", "/api/requests", httpInput(name), nil); w.Code != 200 {
			t.Fatalf("bounded denial %d=%d", i, w.Code)
		}
	}
	if w := httpCall(h, "POST", "/api/requests", httpInput("capacity-next"), nil); w.Code != 429 {
		t.Fatalf("capacity=%d %s", w.Code, w.Body.String())
	}
	if !validReference("claim:"+strings.Repeat("a", 256)+":issuance:0123456789") || validReference("x\x00") || validReference(strings.Repeat("a", 1025)) {
		t.Fatal("reference bounds do not preserve system IDs or reject invalid input")
	}
	if b.calls.Load() != 0 || p.calls.Load() != 0 {
		t.Fatal("denial record retention performed external work")
	}
}

func TestHTTPSameOriginDoesNotTrustForwardedOrMultipleOrigins(t *testing.T) {
	r := httptest.NewRequest("POST", "http://console.local/api/requests", nil)
	r.Header.Set("Origin", "https://console.local")
	r.Header.Set("X-Forwarded-Proto", "https")
	if sameOrigin(r) {
		t.Fatal("forwarded header widened trust")
	}
	r.Header.Set("Origin", "http://console.local")
	r.Header.Add("Origin", "http://console.local")
	if sameOrigin(r) {
		t.Fatal("multiple origins accepted")
	}
	r = httptest.NewRequest("POST", "https://console.local/api/requests", nil)
	r.Header.Set("Origin", "https://console.local")
	if !sameOrigin(r) {
		t.Fatal("exact TLS origin rejected")
	}
	for _, origin := range []string{"https://console.local/", "https://user@console.local", "https://console.local?x=1", "https://console.local:443", "https://console.local#"} {
		r.Header.Set("Origin", origin)
		if sameOrigin(r) {
			t.Fatalf("nonexact origin accepted: %s", origin)
		}
	}
}
