// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func workFile(t *testing.T, ref string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "work.yaml")
	data := []byte(fmt.Sprintf(`apiVersion: agenova.io/v1alpha1
kind: ClaimRequest
metadata: {name: %q}
spec:
  templateRef: engineer
  projectRef: payments
  task: {type: investigation, input: {objective: Investigate a synthetic issue}}
  requestedAccess: {modelProfile: coding-standard}
  runtime: {profileRef: standard-isolated, timeout: 1m}
`, ref))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func installedEvidenceJSON(ref, outcome string) string {
	suffix := ""
	if outcome != "" {
		decision := "Allow"
		claim := fmt.Sprintf(`,"claim":{"id":%q,"requestRef":%q,"templateRef":"engineer","authorityRef":"authority:demo","phase":%q},"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}}`, "claim:"+ref, ref, outcome)
		if outcome == "Deny" || outcome == "ApprovalRequired" {
			decision, claim = outcome, ""
		}
		state := fmt.Sprintf(`{"requestRef":%q,"principal":{"subject":"user:demo","team":"team-a"},"action":{"name":"claim.create","project":"payments","templateRef":"engineer"},"policyRef":{"id":"policy:demo","version":"1"},"decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":%q,"policyRef":{"id":"policy:demo","version":"1"},"reason":"test"},"evidence":{"requestRef":%q,"decisionIds":["decision:demo"]}%s}`, ref, decision, ref, claim)
		suffix = fmt.Sprintf(`,"state":%s,"outcome":{"status":%q}`, state, outcome)
	}
	return fmt.Sprintf(`{"version":"agenova.evidence/v0","requestRef":%q,"request":{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":%q},"spec":{"templateRef":"engineer","projectRef":"payments","task":{"type":"investigation","input":{"objective":"Synthetic test"}},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts":[]%s}`, ref, ref, suffix)
}

func TestRunFileUsesInstalledHTTPAPIAndCanonicalDocument(t *testing.T) {
	path := workFile(t, "demo")
	calls := 0
	closed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/requests" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("not installed Work POST: %s %s %#v", r.Method, r.URL.Path, r.Header)
		}
		var document map[string]any
		if err := json.NewDecoder(r.Body).Decode(&document); err != nil {
			t.Errorf("not JSON: %v", err)
			return
		}
		runtime := document["spec"].(map[string]any)["runtime"].(map[string]any)
		if runtime["timeout"] != "1m" {
			t.Errorf("duration changed: %#v", runtime)
		}
		_, _ = w.Write([]byte(installedEvidenceJSON("demo", "Deny")))
	}))
	defer server.Close()
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() { closed = true }, nil
	}}
	view, err := client.RunFile(path)
	if err != nil || view.Outcome == nil || view.Outcome.Status != "Deny" || calls != 1 || !closed {
		t.Fatalf("RunFile = %#v, %v, calls %d, closed %t", view, err, calls, closed)
	}
}

func TestInstalledAPIErrorUsesOnlyStableDiagnostics(t *testing.T) {
	response := `{"code":"agent_template_unavailable","message":"raw internal detail must not leak"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	client := Client{}
	_, err := client.call(context.Background(), server.URL, []byte(`{}`), "")
	if err == nil || !strings.Contains(err.Error(), "register the requested template") || strings.Contains(err.Error(), "raw internal detail") {
		t.Fatalf("unsafe or missing diagnostic: %v", err)
	}
	response = `{"code":"unexpected","message":"provider token=secret"}`
	_, err = client.call(context.Background(), server.URL, []byte(`{}`), "")
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "HTTP 422") {
		t.Fatalf("unknown diagnostic leaked body: %v", err)
	}
}

func TestShowAndListUseInstalledLoopbackAPI(t *testing.T) {
	const ref = "work?one#two"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("query method = %s", r.Method)
		}
		view := installedEvidenceJSON(ref, "Succeeded")
		switch r.URL.EscapedPath() {
		case "/api/requests":
			_, _ = w.Write([]byte("[" + view + "]"))
		case "/api/requests/work%3Fone%23two/evidence":
			_, _ = w.Write([]byte(view))
		default:
			t.Errorf("query path = %s", r.URL.EscapedPath())
		}
	}))
	defer server.Close()
	closed := 0
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() { closed++ }, nil
	}}
	shown, err := client.Show(ref)
	if err != nil || shown.RequestRef != ref {
		t.Fatalf("Show = %#v, %v", shown, err)
	}
	listed, err := client.List()
	if err != nil || len(listed) != 1 || listed[0].RequestRef != ref || requests != 2 || closed != 2 {
		t.Fatalf("List = %#v, %v, requests %d, closed %d", listed, err, requests, closed)
	}
}

func TestListRejectsDuplicateRequestReferences(t *testing.T) {
	view := installedEvidenceJSON("demo", "Deny")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[" + view + "," + view + "]"))
	}))
	defer server.Close()
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() {}, nil
	}}
	if _, err := client.List(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate Work list accepted: %v", err)
	}
}

func TestRunFilePollsEscapedReference(t *testing.T) {
	const ref = "demo?ref#one"
	path := workFile(t, ref)
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(installedEvidenceJSON(ref, "")))
			return
		}
		reads++
		if r.URL.EscapedPath() != "/api/requests/demo%3Fref%23one/evidence" {
			t.Errorf("evidence path was not escaped: %s", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(installedEvidenceJSON(ref, "Succeeded")))
	}))
	defer server.Close()
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", PollInterval: time.Millisecond, OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() {}, nil
	}}
	view, err := client.RunFile(path)
	if err != nil || view.Outcome == nil || view.Outcome.Status != "Succeeded" || reads != 1 {
		t.Fatalf("RunFile = %#v, %v, reads %d", view, err, reads)
	}
}

func TestShowAndListRejectIncompleteInstalledEvidence(t *testing.T) {
	response := `{"version":"agenova.evidence/v0","requestRef":"demo","facts":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/requests" {
			_, _ = w.Write([]byte("[" + response + "]"))
			return
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() {}, nil
	}}
	for _, incomplete := range []string{
		`{"version":"agenova.evidence/v0","requestRef":"demo","facts":[]}`,
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"projectRef":"payments"`, `"projectRef":"billing"`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"templateRef":"engineer"`, `"templateRef":"analyst"`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","tools":["shell.exec"]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}`, `"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"2m"}`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":2,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo"},{"id":"fact:1","sequence":3,"timestamp":"2026-09-18T00:00:01Z","kind":"RequestReceived","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":2,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo"},{"id":"fact:2","sequence":1,"timestamp":"2026-09-18T00:00:01Z","kind":"RequestReceived","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"other"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:other"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:other"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"AuthorityResolved","requestRef":"demo","claimId":"claim:demo","effectiveAuthority":{"id":"authority:other","runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"AuthorityResolved","requestRef":"demo","claimId":"claim:demo","effectiveAuthority":{"id":"authority:demo","tools":["shell.exec"],"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","result":"Deny","decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":"Allow","policyRef":{"id":"policy:demo","version":"1"}}}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","result":"Deny","policyRef":{"id":"policy:other","version":"1"},"decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":"Deny","policyRef":{"id":"policy:demo","version":"1"}}}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Deny"), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","decision":{"id":"decision:other","principalRef":"user:demo","action":"claim.create","result":"Deny","policyRef":{"id":"policy:demo","version":"1"}}}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","invocationId":"inv:demo","operation":"model.invoke","result":"Allow"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"outcome":{"status":"Succeeded"}`, `"outcome":{"status":"Succeeded","model":{"invocationId":"inv:other","model":"llama3.1:latest","inputTokens":1,"outputTokens":1}}`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[],"unknownField":true`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"status":"Succeeded"`, `"status":"Succeeded","unknownField":true`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo","unknownField":true}]`, 1),
		installedEvidenceJSON("demo", "") + ` {}`,
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"phase":"Succeeded"`, `"phase":"Running"`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[],"outcome":{}`, 1),
		`{"version":"agenova.evidence/v0","requestRef":"demo","request":{"metadata":{"name":"demo"}}}`,
		`{"version":"agenova.evidence/v0","requestRef":"demo","request":{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":"demo"}},"facts":[]}`,
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"state":{"requestRef":"demo"},"facts":[]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"state":{"requestRef":"other"},"facts":[]`, 1),
		`{"version":"agenova.evidence/v0","requestRef":"demo","request":{"metadata":{"name":"other"}},"facts":[]}`,
	} {
		response = incomplete
		if _, err := client.Show("demo"); err == nil {
			t.Fatalf("Show accepted %s", incomplete)
		}
		if _, err := client.List(); err == nil {
			t.Fatalf("List accepted %s", incomplete)
		}
	}
}

func TestModelOutcomeRequiresOneOrderedInvocationSequence(t *testing.T) {
	base := strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"outcome":{"status":"Succeeded"}`, `"outcome":{"status":"Succeeded","model":{"invocationId":"inv:demo","model":"llama3.1:latest","inputTokens":1,"outputTokens":1}}`, 1)
	fact := func(id string, sequence int, kind, result, providerStatus string) string {
		return fmt.Sprintf(`{"id":%q,"sequence":%d,"timestamp":"2026-09-18T00:00:00Z","kind":%q,"requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke","result":%q,"providerStatus":%q}`, id, sequence, kind, result, providerStatus)
	}
	decision := fact("fact:1", 1, "ModelDecision", "Allow", "")
	attempt := fact("fact:2", 2, "ProviderAttempt", "", "Attempted")
	outcome := fact("fact:3", 3, "ProviderOutcome", "", "Succeeded")
	withFacts := func(items ...string) string {
		return strings.Replace(base, `"facts":[]`, `"facts":[`+strings.Join(items, ",")+`]`, 1)
	}
	if _, err := decodeView([]byte(withFacts(decision, attempt, outcome)), "demo"); err != nil {
		t.Fatalf("valid model invocation rejected: %v", err)
	}
	for _, malformed := range []string{
		withFacts(fact("fact:1", 1, "ProviderOutcome", "", "Succeeded"), fact("fact:2", 2, "ModelDecision", "Allow", ""), fact("fact:3", 3, "ProviderAttempt", "", "Attempted")),
		withFacts(decision, attempt, outcome, fact("fact:4", 4, "ProviderOutcome", "", "Succeeded")),
	} {
		if _, err := decodeView([]byte(malformed), "demo"); err == nil {
			t.Fatal("unordered or repeated model invocation accepted")
		}
	}
}

func TestPortForwardBoundaryAndEvidenceValidation(t *testing.T) {
	client := Client{Context: "kind-test", Namespace: "agenova-system"}
	args := strings.Join(client.portForwardArgs(), " ")
	if strings.Contains(args, " exec ") || args != "--context kind-test --namespace agenova-system port-forward deployment/agenova-control-plane :8081 --address 127.0.0.1" {
		t.Fatalf("unsafe port-forward arguments: %s", args)
	}
	if forwardedPort.MatchString("Forwarding from 0.0.0.0:8088 -> 8081") ||
		forwardedPort.MatchString("Forwarding from 127.0.0.1:8088 -> 9999") ||
		!forwardedPort.MatchString("Forwarding from 127.0.0.1:54321 -> 8081") {
		t.Fatal("port-forward readiness matched an unsafe address or port")
	}
	for _, ref := range []string{"", "  ", "../other", "a\\b", "line\nfeed", strings.Repeat("a", 257)} {
		if validRequestRef(ref) {
			t.Errorf("accepted invalid reference %q", ref)
		}
	}
	if _, err := decodeView([]byte(`{"version":"agenova.evidence/v0","requestRef":"other"}`), "expected"); err == nil {
		t.Fatal("accepted mismatched evidence")
	}
	closed := false
	client.OpenTunnel = func(context.Context) (string, func(), error) {
		return "http://example.com:8088", func() { closed = true }, nil
	}
	if _, _, err := client.openTunnel(context.Background()); err == nil || !closed {
		t.Fatalf("non-loopback endpoint was accepted: err=%v closed=%t", err, closed)
	}
}

func TestHTTPErrorDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("synthetic-secret"))
	}))
	defer server.Close()
	client := Client{}
	_, err := client.call(context.Background(), server.URL, nil, "demo")
	if err == nil || strings.Contains(err.Error(), "synthetic-secret") || !strings.Contains(err.Error(), "403") {
		t.Fatalf("unsafe HTTP error: %v", err)
	}
}

func TestInstalledAPIDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected = true
		w.WriteHeader(http.StatusOK)
	}))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.Header().Set("Location", remote.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer local.Close()
	client := Client{}
	_, err := client.call(context.Background(), local.URL, []byte(`{"kind":"ClaimRequest"}`), "")
	if err == nil || !strings.Contains(err.Error(), "307") || redirected {
		t.Fatalf("unsafe redirect result: err=%v redirected=%t", err, redirected)
	}
}
