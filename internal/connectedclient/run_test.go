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
	facts := "[]"
	if outcome != "" {
		decision := "Allow"
		claim := fmt.Sprintf(`,"claim":{"id":%q,"requestRef":%q,"templateRef":"engineer","authorityRef":"authority:demo","phase":%q,"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}},"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}}`, "claim:"+ref, ref, outcome)
		if outcome == "Deny" || outcome == "ApprovalRequired" {
			decision, claim = outcome, ""
		}
		state := fmt.Sprintf(`{"requestRef":%q,"principal":{"subject":"user:demo","team":"team-a","authenticationContext":"upstream:test"},"action":{"name":"claim.create","project":"payments","templateRef":"engineer"},"policyRef":{"id":"policy:demo","version":"1"},"decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":%q,"policyRef":{"id":"policy:demo","version":"1"},"reason":"test"},"evidence":{"requestRef":%q,"decisionIds":["decision:demo"]}%s}`, ref, decision, ref, claim)
		suffix = fmt.Sprintf(`,"state":%s,"outcome":{"status":%q}`, state, outcome)
		facts = "[" + receivedFact(ref) + "," + resolutionFact(ref, decision) + "]"
		if decision == "Allow" {
			facts = facts[:len(facts)-1] + "," + authorityFact(ref) + "," + boundFact(ref) + "," + backendReadyFact(ref) + "," + runningFact(ref) + "," + fmt.Sprintf(`{"id":"fact:7","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":%q,"claimId":%q,"operation":%q}`, ref, "claim:"+ref, outcome) + "," + fmt.Sprintf(`{"id":"fact:8","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":%q,"claimId":%q,"operation":%q}]`, ref, "claim:"+ref, outcome)
		}
	}
	return fmt.Sprintf(`{"version":"agenova.evidence/v0","requestRef":%q,"request":{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":%q},"spec":{"templateRef":"engineer","projectRef":"payments","task":{"type":"investigation","input":{"objective":"Synthetic test"}},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts":%s%s}`, ref, ref, facts, suffix)
}

func resolutionFact(ref, decision string) string {
	return fmt.Sprintf(`{"id":"fact:2","sequence":2,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":%q,"result":%q,"policyRef":{"id":"policy:demo","version":"1"},"decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":%q,"policyRef":{"id":"policy:demo","version":"1"},"reason":"test"}}`, ref, decision, decision)
}

func receivedFact(ref string) string {
	return fmt.Sprintf(`{"id":"fact:1","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":%q}`, ref)
}

func boundFact(ref string) string {
	return fmt.Sprintf(`{"id":"fact:4","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":%q,"claimId":%q,"operation":"Bound","backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}}`, ref, "claim:"+ref)
}

func authorityFact(ref string) string {
	return fmt.Sprintf(`{"id":"fact:3","sequence":3,"timestamp":"2026-09-18T00:00:00Z","kind":"AuthorityResolved","requestRef":%q,"claimId":%q,"policyRef":{"id":"policy:demo","version":"1"},"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}`, ref, "claim:"+ref)
}

func modelAuthorityFact(ref string) string {
	return strings.Replace(authorityFact(ref), `"id":"authority:demo"`, `"id":"authority:demo","modelProfile":"coding-standard"`, 1)
}

func runningFact(ref string) string {
	return fmt.Sprintf(`{"id":"fact:6","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":%q,"claimId":%q,"operation":"Running","backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}}`, ref, "claim:"+ref)
}

func backendReadyFact(ref string) string {
	return fmt.Sprintf(`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":%q,"claimId":%q,"operation":"BackendReady","backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}}`, ref, "claim:"+ref)
}

func withEvidenceFacts(source string, facts ...string) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	view["facts"] = json.RawMessage("[" + strings.Join(facts, ",") + "]")
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func appendEvidenceFact(source, fact string) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	var facts []json.RawMessage
	if err := json.Unmarshal(view["facts"], &facts); err != nil {
		panic(err)
	}
	facts = append(facts, json.RawMessage(fact))
	encodedFacts, err := json.Marshal(facts)
	if err != nil {
		panic(err)
	}
	view["facts"] = encodedFacts
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func insertBeforeRunOutcome(source, fact string) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	var recorded []json.RawMessage
	if err := json.Unmarshal(view["facts"], &recorded); err != nil || len(recorded) == 0 {
		panic("missing recorded facts")
	}
	var terminal, inserted map[string]json.RawMessage
	if err := json.Unmarshal(recorded[len(recorded)-1], &terminal); err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(fact), &inserted); err != nil {
		panic(err)
	}
	if string(terminal["kind"]) != `"RunOutcome"` {
		panic("last fact is not RunOutcome")
	}
	var sequence uint64
	if err := json.Unmarshal(terminal["sequence"], &sequence); err != nil {
		panic(err)
	}
	inserted["sequence"] = json.RawMessage(fmt.Sprintf("%d", sequence))
	terminal["sequence"] = json.RawMessage(fmt.Sprintf("%d", sequence+1))
	insertedJSON, err := json.Marshal(inserted)
	if err != nil {
		panic(err)
	}
	terminalJSON, err := json.Marshal(terminal)
	if err != nil {
		panic(err)
	}
	recorded = append(recorded[:len(recorded)-1], insertedJSON, terminalJSON)
	view["facts"], err = json.Marshal(recorded)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func insertBeforeRuntimeTerminal(source, fact string) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	var recorded []map[string]json.RawMessage
	if err := json.Unmarshal(view["facts"], &recorded); err != nil {
		panic(err)
	}
	var inserted map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fact), &inserted); err != nil {
		panic(err)
	}
	index := -1
	for i, entry := range recorded {
		if string(entry["kind"]) == `"Runtime"` &&
			(string(entry["operation"]) == `"Succeeded"` || string(entry["operation"]) == `"Failed"`) {
			index = i
			break
		}
	}
	if index < 0 {
		panic("missing terminal Runtime")
	}
	recorded = append(recorded[:index], append([]map[string]json.RawMessage{inserted}, recorded[index:]...)...)
	for i := index; i < len(recorded); i++ {
		recorded[i]["sequence"] = json.RawMessage(fmt.Sprintf("%d", i+1))
	}
	var err error
	view["facts"], err = json.Marshal(recorded)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func changeInvocationPolicy(source, kind string, policy json.RawMessage) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	var recorded []map[string]json.RawMessage
	if err := json.Unmarshal(view["facts"], &recorded); err != nil {
		panic(err)
	}
	found := false
	for _, fact := range recorded {
		if string(fact["kind"]) != fmt.Sprintf("%q", kind) {
			continue
		}
		found = true
		if policy == nil {
			delete(fact, "policyRef")
		} else {
			fact["policyRef"] = policy
		}
	}
	if !found {
		panic("missing invocation decision")
	}
	var err error
	view["facts"], err = json.Marshal(recorded)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func offsetFactSequences(source string, offset uint64) string {
	var view map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &view); err != nil {
		panic(err)
	}
	var recorded []map[string]json.RawMessage
	if err := json.Unmarshal(view["facts"], &recorded); err != nil {
		panic(err)
	}
	for _, fact := range recorded {
		var sequence uint64
		if err := json.Unmarshal(fact["sequence"], &sequence); err != nil {
			panic(err)
		}
		fact["sequence"] = json.RawMessage(fmt.Sprintf("%d", sequence+offset))
	}
	var err error
	view["facts"], err = json.Marshal(recorded)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		panic(err)
	}
	return string(encoded)
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

func TestListRejectsClaimAndWorkerIdentityReuseAcrossWorks(t *testing.T) {
	first := installedEvidenceJSON("first", "Succeeded")
	second := strings.ReplaceAll(installedEvidenceJSON("second", "Succeeded"), `"id":"fact:`, `"id":"fact:second-`)
	independent := offsetFactSequences(strings.ReplaceAll(second, "worker:demo", "worker:second"), 100)
	response := "[" + first + "," + independent + "]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	client := Client{Context: "kind-agenova", Namespace: "agenova-system", OpenTunnel: func(context.Context) (string, func(), error) {
		return server.URL, func() {}, nil
	}}
	if views, err := client.List(); err != nil || len(views) != 2 {
		t.Fatalf("independent Work list rejected: %v", err)
	}
	for _, invalid := range []string{
		"[" + first + "," + offsetFactSequences(second, 100) + "]",
		"[" + first + "," + strings.ReplaceAll(independent, "claim:second", "claim:first") + "]",
		"[" + first + "," + offsetFactSequences(strings.ReplaceAll(installedEvidenceJSON("second", "Succeeded"), "worker:demo", "worker:second"), 100) + "]",
	} {
		response = invalid
		if _, err := client.List(); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("reused Claim or worker identity accepted: %v", err)
		}
	}
	response = "[" + first + "," + strings.ReplaceAll(second, "worker:demo", "worker:second") + "]"
	if _, err := client.List(); err == nil || !strings.Contains(err.Error(), "duplicate Fact sequence") {
		t.Fatalf("reused global Fact sequence accepted: %v", err)
	}
	withModelDecision := func(source, ref, invocationID, factID string) string {
		source = strings.Replace(source, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
		source = strings.ReplaceAll(source, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
		fact := fmt.Sprintf(`{"id":%q,"sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":%q,"claimId":%q,"invocationId":%q,"policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`, factID, ref, "claim:"+ref, invocationID)
		return insertBeforeRuntimeTerminal(source, fact)
	}
	firstCall := withModelDecision(first, "first", "inv:shared", "fact:first-call")
	secondWithWorker := strings.ReplaceAll(second, "worker:demo", "worker:second")
	secondCall := offsetFactSequences(withModelDecision(secondWithWorker, "second", "inv:second", "fact:second-call"), 100)
	response = "[" + firstCall + "," + secondCall + "]"
	if views, err := client.List(); err != nil || len(views) != 2 {
		t.Fatalf("independent invocation list rejected: %v", err)
	}
	response = "[" + firstCall + "," + offsetFactSequences(withModelDecision(secondWithWorker, "second", "inv:shared", "fact:second-call"), 100) + "]"
	if _, err := client.List(); err == nil || !strings.Contains(err.Error(), "duplicate invocation") {
		t.Fatalf("cross-Work invocation identity reuse accepted: %v", err)
	}
}

func TestRunFilePollsEscapedReference(t *testing.T) {
	const ref = "demo?ref#one"
	path := workFile(t, ref)
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(withEvidenceFacts(installedEvidenceJSON(ref, ""), receivedFact(ref))))
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
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `,"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}`, ``, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","tools":["shell.exec"]`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`, 1),
		strings.Replace(installedEvidenceJSON("demo", "Succeeded"), `"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}`, `"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"2m"}`, 1),
		withEvidenceFacts(installedEvidenceJSON("demo", "Succeeded")),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":2,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo"},{"id":"fact:1","sequence":3,"timestamp":"2026-09-18T00:00:01Z","kind":"RequestReceived","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:1","sequence":2,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo"},{"id":"fact:2","sequence":1,"timestamp":"2026-09-18T00:00:01Z","kind":"RequestReceived","requestRef":"demo"}]`, 1),
		strings.Replace(installedEvidenceJSON("demo", ""), `"facts":[]`, `"facts":[{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"other"}]`, 1),
		appendEvidenceFact(installedEvidenceJSON("demo", "Succeeded"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:other"}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:other"}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo"}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo"}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Succeeded"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"AuthorityResolved","requestRef":"demo","claimId":"claim:demo","effectiveAuthority":{"id":"authority:other","runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Succeeded"), `{"id":"fact:other","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"AuthorityResolved","requestRef":"demo","claimId":"claim:demo","effectiveAuthority":{"id":"authority:demo","tools":["shell.exec"],"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}}`),
		withEvidenceFacts(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","result":"Deny","decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":"Allow","policyRef":{"id":"policy:demo","version":"1"}}}`),
		withEvidenceFacts(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","result":"Deny","policyRef":{"id":"policy:other","version":"1"},"decision":{"id":"decision:demo","principalRef":"user:demo","action":"claim.create","result":"Deny","policyRef":{"id":"policy:demo","version":"1"}}}`),
		withEvidenceFacts(installedEvidenceJSON("demo", "Deny"), `{"id":"fact:other","sequence":1,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","decision":{"id":"decision:other","principalRef":"user:demo","action":"claim.create","result":"Deny","policyRef":{"id":"policy:demo","version":"1"}}}`),
		appendEvidenceFact(installedEvidenceJSON("demo", "Succeeded"), `{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","invocationId":"inv:demo","operation":"model.invoke","result":"Allow"}`),
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
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	fact := func(id string, sequence int, kind, result, providerStatus, target string) string {
		return fmt.Sprintf(`{"id":%q,"sequence":%d,"timestamp":"2026-09-18T00:00:00Z","kind":%q,"requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":%q,"result":%q,"providerStatus":%q}`, id, sequence, kind, target, result, providerStatus)
	}
	decision := fact("fact:decision", 7, "ModelDecision", "Allow", "", "coding-standard")
	attempt := fact("fact:attempt", 8, "ProviderAttempt", "", "Attempted", "coding-standard")
	outcome := fact("fact:provider-outcome", 9, "ProviderOutcome", "", "Succeeded", "coding-standard")
	withFacts := func(items ...string) string {
		items = append([]string{receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo")}, items...)
		items = append(items, `{"id":"fact:terminal","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`, `{"id":"fact:outcome","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
		return withEvidenceFacts(base, items...)
	}
	if _, err := decodeView([]byte(withFacts(decision, attempt, outcome)), "demo"); err != nil {
		t.Fatalf("valid model invocation rejected: %v", err)
	}
	for _, malformed := range []string{
		withFacts(fact("fact:decision", 7, "ProviderOutcome", "", "Succeeded", "coding-standard"), fact("fact:attempt", 8, "ModelDecision", "Allow", "", "coding-standard"), fact("fact:provider-outcome", 9, "ProviderAttempt", "", "Attempted", "coding-standard")),
		withFacts(decision, attempt, outcome, fact("fact:10", 10, "ProviderOutcome", "", "Succeeded", "coding-standard")),
		withFacts(fact("fact:decision", 7, "ModelDecision", "Allow", "", "unapproved-profile"), attempt, outcome),
		withFacts(decision, fact("fact:attempt", 8, "ProviderAttempt", "", "Attempted", "unapproved-profile"), outcome),
		withFacts(decision, attempt, fact("fact:provider-outcome", 9, "ProviderOutcome", "", "Succeeded", "unapproved-profile")),
		withFacts(decision, fact("fact:attempt", 8, "ProviderAttempt", "", "Failed", "coding-standard"), outcome),
	} {
		if _, err := decodeView([]byte(malformed), "demo"); err == nil {
			t.Fatal("unordered or repeated model invocation accepted")
		}
	}
}

func TestSuccessfulWorkWithoutModelRemainsValidForToolOnlyAgents(t *testing.T) {
	if _, err := decodeView([]byte(installedEvidenceJSON("tool-only", "Succeeded")), "tool-only"); err != nil {
		t.Fatalf("shared evidence rejected tool-only success: %v", err)
	}
}

func TestAllowedTerminalOutcomeRequiresCorrelatedRunOutcome(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	for _, invalid := range []string{
		withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"), boundFact("demo"), runningFact("demo")),
		withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"), boundFact("demo"), runningFact("demo"), `{"id":"fact:6","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Failed"}`),
		appendEvidenceFact(base, `{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted terminal outcome without one matching RunOutcome")
		}
	}
	failed := strings.Replace(installedEvidenceJSON("demo", "Failed"), `"outcome":{"status":"Failed"}`, `"outcome":{"status":"Failed","failure":"worker start failed"}`, 1)
	if _, err := decodeView([]byte(failed), "demo"); err == nil {
		t.Fatal("accepted terminal failure whose RunOutcome fact omitted the failure reason")
	}
	matching := withEvidenceFacts(failed, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), `{"id":"fact:7","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Failed"}`, `{"id":"fact:8","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Failed","reason":"worker start failed"}`)
	if _, err := decodeView([]byte(matching), "demo"); err != nil {
		t.Fatalf("matching terminal failure reason rejected: %v", err)
	}
}

func TestIssuedDecisionRequiresOneCorrelatedResolutionFact(t *testing.T) {
	for _, status := range []string{"Deny", "Succeeded"} {
		valid := installedEvidenceJSON("demo", status)
		if _, err := decodeView([]byte(valid), "demo"); err != nil {
			t.Fatalf("valid %s decision rejected: %v", status, err)
		}
		facts := []string{receivedFact("demo")}
		if status == "Succeeded" {
			facts = append(facts, `{"id":"fact:4","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
		}
		for _, invalid := range []string{
			withEvidenceFacts(valid, facts...),
			appendEvidenceFact(valid, `{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestResolution","requestRef":"demo","result":"Deny","policyRef":{"id":"policy:demo","version":"1"},"decision":{"id":"decision:other","principalRef":"user:demo","action":"claim.create","result":"Deny","policyRef":{"id":"policy:demo","version":"1"}}}`),
		} {
			if _, err := decodeView([]byte(invalid), "demo"); err == nil {
				t.Fatalf("accepted %s issued decision without exactly one matching resolution", status)
			}
		}
	}
}

func TestEvidenceRequiresReceivedFactBeforeResolution(t *testing.T) {
	base := installedEvidenceJSON("demo", "Deny")
	for _, invalid := range []string{
		withEvidenceFacts(base, resolutionFact("demo", "Deny")),
		appendEvidenceFact(base, `{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"RequestReceived","requestRef":"demo"}`),
		withEvidenceFacts(base, resolutionFact("demo", "Deny"), receivedFact("demo")),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted missing, repeated, or out-of-order request reception")
		}
	}
	if _, err := decodeView([]byte(withEvidenceFacts(installedEvidenceJSON("pending", ""), receivedFact("pending"))), "pending"); err != nil {
		t.Fatalf("received-only pending Work rejected: %v", err)
	}
}

func TestIssuedDecisionRequiresCompleteTrustedPrincipal(t *testing.T) {
	base := installedEvidenceJSON("demo", "Deny")
	for _, invalid := range []string{
		strings.Replace(base, `"team":"team-a"`, `"team":""`, 1),
		strings.Replace(base, `"authenticationContext":"upstream:test"`, `"authenticationContext":""`, 1),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted issued decision without complete trusted identity")
		}
	}
}

func TestWorkDecisionMustAuthorizeClaimCreation(t *testing.T) {
	base := installedEvidenceJSON("demo", "Deny")
	invalid := strings.ReplaceAll(base, `"action":"claim.create"`, `"action":"claim.delete"`)
	invalid = strings.Replace(invalid, `"name":"claim.create"`, `"name":"claim.delete"`, 1)
	if _, err := decodeView([]byte(invalid), "demo"); err == nil {
		t.Fatal("accepted a decision for an action other than claim.create")
	}
}

func TestEveryAllowedModelDecisionMustUseGrantedProfile(t *testing.T) {
	for _, status := range []string{"Succeeded", "Failed"} {
		base := installedEvidenceJSON("demo", status)
		base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
		base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
		modelDecision := func(target string) string {
			return insertBeforeRuntimeTerminal(base, fmt.Sprintf(`{"id":"fact:extra","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:earlier","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":%q,"result":"Allow"}`, target))
		}
		if _, err := decodeView([]byte(modelDecision("coding-standard")), "demo"); err != nil {
			t.Fatalf("%s: granted model decision rejected: %v", status, err)
		}
		for _, target := range []string{"", "unapproved-profile"} {
			if _, err := decodeView([]byte(modelDecision(target)), "demo"); err == nil {
				t.Fatalf("%s: accepted allowed model decision for %q", status, target)
			}
		}
		for _, invalid := range []string{
			changeInvocationPolicy(modelDecision("coding-standard"), "ModelDecision", nil),
			changeInvocationPolicy(modelDecision("coding-standard"), "ModelDecision", json.RawMessage(`{"id":"policy:other","version":"1"}`)),
		} {
			if _, err := decodeView([]byte(invalid), "demo"); err == nil {
				t.Fatalf("%s: accepted model decision without issued Policy attribution", status)
			}
		}
	}
}

func TestNonSuccessOutcomeCannotCarryResultText(t *testing.T) {
	for _, status := range []string{"Deny", "Failed", "Expired"} {
		base := installedEvidenceJSON("demo", status)
		invalid := strings.Replace(base, `"outcome":{"status":"`+status+`"}`, `"outcome":{"status":"`+status+`","text":"fabricated result"}`, 1)
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatalf("accepted result text on %s Work", status)
		}
	}
}

func TestAllowedToolDecisionMustBeWithinEffectiveAuthority(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"tools":["git.read"]},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","tools":["git.read"]`)
	toolDecision := func(target string) string {
		return insertBeforeRuntimeTerminal(base, fmt.Sprintf(`{"id":"fact:extra","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"ToolDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:tool","policyRef":{"id":"policy:demo","version":"1"},"operation":"tool.invoke","target":%q,"result":"Allow"}`, target))
	}
	if _, err := decodeView([]byte(toolDecision("git.read")), "demo"); err != nil {
		t.Fatalf("granted tool decision rejected: %v", err)
	}
	for _, target := range []string{"", "shell.exec"} {
		if _, err := decodeView([]byte(toolDecision(target)), "demo"); err == nil {
			t.Fatalf("accepted ungranted allowed tool target %q", target)
		}
	}
	for _, invalid := range []string{
		changeInvocationPolicy(toolDecision("git.read"), "ToolDecision", nil),
		changeInvocationPolicy(toolDecision("git.read"), "ToolDecision", json.RawMessage(`{"id":"policy:other","version":"1"}`)),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted tool decision without issued Policy attribution")
		}
	}
}

func TestEveryProviderFactRequiresAnOrderedAllowedInvocation(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	for _, orphan := range []string{
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:orphan","operation":"model.invoke","target":"coding-standard","providerStatus":"Attempted"}`,
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:orphan","operation":"tool.invoke","target":"Mock git.read","providerStatus":"Succeeded"}`,
	} {
		if _, err := decodeView([]byte(appendEvidenceFact(base, orphan)), "demo"); err == nil {
			t.Fatal("accepted an orphan provider fact")
		}
	}
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	decision := `{"id":"fact:decision","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:earlier","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	attempt := `{"id":"fact:attempt","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:earlier","operation":"model.invoke","target":"coding-standard","providerStatus":"Attempted"}`
	provided := `{"id":"fact:provided","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:earlier","operation":"model.invoke","target":"coding-standard","providerStatus":"Failed"}`
	terminalRuntime := `{"id":"fact:terminal","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	terminal := `{"id":"fact:outcome","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	valid := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, provided, terminalRuntime, terminal)
	if _, err := decodeView([]byte(valid), "demo"); err != nil {
		t.Fatalf("intermediate failed provider call rejected: %v", err)
	}
	for _, invalid := range []string{
		withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, provided, terminalRuntime, terminal),
		withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, strings.Replace(attempt, `"operation":"model.invoke"`, `"operation":"tool.invoke"`, 1), provided, terminalRuntime, terminal),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted an unordered or mismatched provider invocation")
		}
	}
}

func TestNoActivityAfterFinalRunOutcome(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	finishing := strings.Replace(base, `,"outcome":{"status":"Succeeded"}`, ``, 1)
	if finishing == base || strings.Contains(finishing, `"outcome":`) {
		t.Fatal("finishing fixture still contains an Outcome")
	}
	for _, valid := range []string{base, finishing} {
		if _, err := decodeView([]byte(valid), "demo"); err != nil {
			t.Fatalf("valid terminal/finishing baseline rejected: %v", err)
		}
		for _, postTerminal := range []string{
			`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:late","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`,
			`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Bound","backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}}`,
		} {
			if _, err := decodeView([]byte(appendEvidenceFact(valid, postTerminal)), "demo"); err == nil {
				t.Fatal("accepted activity after the final RunOutcome")
			}
		}
	}
}

func TestFinishingRunOutcomeStillMatchesTerminalClaim(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	finishing := strings.Replace(base, `,"outcome":{"status":"Succeeded"}`, ``, 1)
	if finishing == base {
		t.Fatal("fixture did not remove Outcome")
	}
	if _, err := decodeView([]byte(finishing), "demo"); err != nil {
		t.Fatalf("valid finishing projection rejected: %v", err)
	}
	for _, invalid := range []string{
		strings.Replace(finishing, `"operation":"Succeeded"`, `"operation":"Failed"`, 1),
		strings.Replace(finishing, `"phase":"Succeeded"`, `"phase":"Running"`, 1),
	} {
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted contradictory RunOutcome in Finishing window")
		}
	}
}

func TestRequestResolutionCannotClaimAllocatedResources(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("valid resolution rejected: %v", err)
	}
	for _, injected := range []string{
		`"claimId":"claim:demo",`,
		`"invocationId":"inv:demo",`,
		`"effectiveAuthority":{"id":"authority:demo","runtime":{"profileRef":"standard-isolated","timeout":"1m"}},`,
		`"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"},`,
	} {
		invalid := strings.Replace(base, `"kind":"RequestResolution","requestRef":"demo",`, `"kind":"RequestResolution","requestRef":"demo",`+injected, 1)
		if invalid == base {
			t.Fatal("fixture did not modify RequestResolution")
		}
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatal("accepted pre-admission claim attribution")
		}
	}
}

func TestInvocationRequiresRunningAndResolvedAuthority(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	decision := `{"id":"fact:call","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	valid := insertBeforeRuntimeTerminal(base, decision)
	if _, err := decodeView([]byte(valid), "demo"); err != nil {
		t.Fatalf("valid governed call rejected: %v", err)
	}
	withoutRunning := withEvidenceFacts(valid, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), decision, `{"id":"fact:7","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutRunning), "demo"); err == nil {
		t.Fatal("accepted governed call before Runtime/Running")
	}
	withoutWorkerStart := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), `{"id":"fact:6","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutWorkerStart), "demo"); err == nil {
		t.Fatal("accepted successful Work without Runtime/Running evidence")
	}
	withoutAuthority := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), boundFact("demo"), runningFact("demo"), `{"id":"fact:6","sequence":6,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutAuthority), "demo"); err == nil {
		t.Fatal("accepted runtime activity without authority resolution")
	}
}

func TestInFlightProviderOutcomeCanCloseAfterRuntimeCancellation(t *testing.T) {
	base := installedEvidenceJSON("demo", "Failed")
	base = strings.Replace(base, `"outcome":{"status":"Failed"}`, `"outcome":{"status":"Cancelled"}`, 1)
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	decision := `{"id":"fact:decision","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:active","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	attempt := `{"id":"fact:attempt","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:active","operation":"model.invoke","target":"coding-standard","providerStatus":"Attempted"}`
	terminalRuntime := `{"id":"fact:terminal","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Cancelled"}`
	providerOutcome := `{"id":"fact:provider-outcome","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:active","operation":"model.invoke","target":"coding-standard","providerStatus":"Cancelled"}`
	terminalWork := `{"id":"fact:outcome","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Cancelled"}`
	valid := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, terminalRuntime, providerOutcome, terminalWork)
	if _, err := decodeView([]byte(valid), "demo"); err != nil {
		t.Fatalf("in-flight cancelled provider outcome rejected: %v", err)
	}
	lateDecision := `{"id":"fact:late","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:new","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	if _, err := decodeView([]byte(insertBeforeRunOutcome(valid, lateDecision)), "demo"); err == nil {
		t.Fatal("accepted a new invocation decision after runtime cancellation")
	}
	withGap := strings.Replace(valid, `"id":"fact:outcome","sequence":11`, `"id":"fact:outcome","sequence":13`, 1)
	if _, err := decodeView([]byte(withGap), "demo"); err != nil {
		t.Fatalf("canonical cancellation with sequence gap rejected: %v", err)
	}
	for _, operation := range []string{"Bound", "BackendReady", "Running", "UnexpectedRuntimeStatus"} {
		lateRuntime := fmt.Sprintf(`{"id":"fact:late-runtime","sequence":12,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":%q}`, operation)
		if _, err := decodeView([]byte(insertBeforeRunOutcome(withGap, lateRuntime)), "demo"); err == nil {
			t.Fatalf("accepted runtime %s after cancellation", operation)
		}
	}
}

func TestRequestReceiptHasNoLaterStageAttribution(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("canonical request receipt rejected: %v", err)
	}
	for _, field := range []string{
		`"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}`,
		`"policyRef":{"id":"policy:demo","version":"1"}`,
		`"invocationId":"inv:early"`,
		`"operation":"Bound"`,
		`"target":"coding-standard"`,
		`"providerStatus":"Attempted"`,
	} {
		invalid := strings.Replace(base, `"kind":"RequestReceived","requestRef":"demo"`, `"kind":"RequestReceived","requestRef":"demo",`+field, 1)
		if invalid == base {
			t.Fatal("request receipt fixture did not change")
		}
		if _, err := decodeView([]byte(invalid), "demo"); err == nil {
			t.Fatalf("accepted request receipt with %s", field)
		}
	}
}

func TestRuntimeOperationsUseCanonicalVocabulary(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("canonical Work with sequence gap rejected: %v", err)
	}
	pending := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"),
		`{"id":"fact:pending","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Pending"}`,
		strings.Replace(boundFact("demo"), `"id":"fact:4","sequence":4`, `"id":"fact:bound","sequence":5`, 1),
		strings.Replace(backendReadyFact("demo"), `"id":"fact:5","sequence":5`, `"id":"fact:ready","sequence":6`, 1),
		strings.Replace(runningFact("demo"), `"id":"fact:6","sequence":6`, `"id":"fact:running","sequence":7`, 1),
		`{"id":"fact:terminal","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`,
		`{"id":"fact:outcome","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(pending), "demo"); err != nil {
		t.Fatalf("RunService Pending event rejected: %v", err)
	}
	for _, operation := range []string{"KubernetesReady", "Allocated", "Unknown"} {
		fact := fmt.Sprintf(`{"id":"fact:unexpected","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":%q}`, operation)
		if _, err := decodeView([]byte(insertBeforeRuntimeTerminal(base, fact)), "demo"); err == nil {
			t.Fatalf("accepted unknown runtime operation %s", operation)
		}
	}
}

func TestWorkerActionRequiresCompletedSuccessfulModelInvocation(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	decision := `{"id":"fact:decision","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	attempt := `{"id":"fact:attempt","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke","target":"coding-standard","providerStatus":"Attempted"}`
	provider := `{"id":"fact:provider","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke","target":"coding-standard","providerStatus":"Succeeded"}`
	action := `{"id":"fact:action","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"WorkerActivity","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"ActionValidated"}`
	terminal := `{"id":"fact:terminal","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	outcome := `{"id":"fact:outcome","sequence":12,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	prefix := []string{receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, provider}
	valid := withEvidenceFacts(base, append(append([]string{}, prefix...), action, terminal, outcome)...)
	if _, err := decodeView([]byte(valid), "demo"); err != nil {
		t.Fatalf("correlated model action rejected: %v", err)
	}
	for _, invalidAction := range []string{
		strings.Replace(action, `"invocationId":"inv:demo"`, `"invocationId":"inv:other"`, 1),
		strings.Replace(action, `"operation":"ActionValidated"`, `"operation":"FinalAnswer"`, 1),
	} {
		if _, err := decodeView([]byte(withEvidenceFacts(base, append(append([]string{}, prefix...), invalidAction, terminal, outcome)...)), "demo"); err == nil {
			t.Fatal("accepted uncorrelated worker action")
		}
	}
	failedProvider := strings.Replace(provider, `"providerStatus":"Succeeded"`, `"providerStatus":"Failed"`, 1)
	if _, err := decodeView([]byte(withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, failedProvider, action, terminal, outcome)), "demo"); err == nil {
		t.Fatal("accepted worker action attributed to a failed provider call")
	}
}

func TestRuntimeEvidenceFollowsCanonicalLifecycleStages(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("canonical lifecycle rejected: %v", err)
	}
	withoutReady := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"), boundFact("demo"), runningFact("demo"),
		`{"id":"fact:terminal","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`,
		`{"id":"fact:outcome","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutReady), "demo"); err == nil {
		t.Fatal("accepted Running without BackendReady")
	}
	withoutTerminal := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), authorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"),
		`{"id":"fact:outcome","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutTerminal), "demo"); err == nil {
		t.Fatal("accepted successful Work without terminal Runtime evidence")
	}
	for _, operation := range []string{"TerminateSucceeded", "TerminateFailed", "CleanupSucceeded", "CleanupFailed"} {
		teardown := fmt.Sprintf(`{"id":"fact:early-teardown","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":%q}`, operation)
		if _, err := decodeView([]byte(insertBeforeRuntimeTerminal(base, teardown)), "demo"); err == nil {
			t.Fatalf("accepted %s before terminal Runtime", operation)
		}
	}
}

func TestAuthorityResolutionEvidenceMatchesRequestAndPolicy(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	prefix := []string{receivedFact("demo"), resolutionFact("demo", "Allow")}
	suffix := []string{boundFact("demo"), backendReadyFact("demo"), runningFact("demo"),
		`{"id":"fact:terminal","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`,
		`{"id":"fact:outcome","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`}
	withoutPolicy := strings.Replace(authorityFact("demo"), `,"policyRef":{"id":"policy:demo","version":"1"}`, "", 1)
	fabricatedChange := strings.Replace(authorityFact("demo"), `"effectiveAuthority":`, `"authorityChanges":[{"field":"tools","requested":"shell.exec","reasonCode":"outside-template-ceiling"}],"effectiveAuthority":`, 1)
	for _, invalidAuthority := range []string{withoutPolicy, fabricatedChange} {
		facts := append(append(append([]string{}, prefix...), invalidAuthority), suffix...)
		if _, err := decodeView([]byte(withEvidenceFacts(base, facts...)), "demo"); err == nil {
			t.Fatal("accepted uncorrelated authority provenance")
		}
	}
	allRemoved := strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"tools":["shell.exec"]},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	if allRemoved == base {
		t.Fatal("requested access fixture did not change")
	}
	if _, err := decodeView([]byte(allRemoved), "demo"); err == nil {
		t.Fatal("accepted an allowed claim with its requested tool dimension entirely removed")
	}
	requested := strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"tools":["git.read","shell.exec"]},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	granted := strings.ReplaceAll(requested, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","tools":["git.read"]`)
	narrowedFact := strings.Replace(authorityFact("demo"), `"effectiveAuthority":{"id":"authority:demo"`, `"authorityChanges":[{"field":"tools","requested":"shell.exec","effective":"","reasonCode":"outside-template-ceiling"}],"effectiveAuthority":{"id":"authority:demo","tools":["git.read"]`, 1)
	facts := append(append(append([]string{}, prefix...), narrowedFact), suffix...)
	if _, err := decodeView([]byte(withEvidenceFacts(granted, facts...)), "demo"); err != nil {
		t.Fatalf("real template-ceiling narrowing rejected: %v", err)
	}
}

func TestToolProviderOutcomeRetainsAttemptTarget(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"tools":["git.read"]},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","tools":["git.read"]`)
	decision := `{"id":"fact:decision","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"ToolDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:tool","policyRef":{"id":"policy:demo","version":"1"},"operation":"tool.invoke","target":"git.read","result":"Allow"}`
	attempt := `{"id":"fact:attempt","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:tool","operation":"tool.invoke","target":"Mock git.read · a","providerStatus":"Attempted"}`
	providerOutcome := `{"id":"fact:provided","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:tool","operation":"tool.invoke","target":"Mock git.read · a","providerStatus":"Succeeded"}`
	terminalRuntime := `{"id":"fact:terminal","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	terminal := `{"id":"fact:outcome","sequence":11,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	valid := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), strings.Replace(authorityFact("demo"), `"id":"authority:demo"`, `"id":"authority:demo","tools":["git.read"]`, 1), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, providerOutcome, terminalRuntime, terminal)
	if _, err := decodeView([]byte(valid), "demo"); err != nil {
		t.Fatalf("consistent tool provider target rejected: %v", err)
	}
	invalid := strings.Replace(valid, `"target":"Mock git.read · a","providerStatus":"Succeeded"`, `"target":"Mock git.read · b","providerStatus":"Succeeded"`, 1)
	if invalid == valid {
		t.Fatal("fixture did not change tool provider target")
	}
	if _, err := decodeView([]byte(invalid), "demo"); err == nil {
		t.Fatal("accepted a tool outcome for a different provider target")
	}
	emptyTarget := strings.Replace(valid, `"target":"Mock git.read · a","providerStatus":"Attempted"`, `"target":"","providerStatus":"Attempted"`, 1)
	if emptyTarget == valid {
		t.Fatal("fixture did not empty the tool attempt target")
	}
	if _, err := decodeView([]byte(emptyTarget), "demo"); err == nil {
		t.Fatal("accepted a tool attempt without a target")
	}
}

func TestTerminalRuntimeFactMustMatchClaimOutcome(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	good := base
	if _, err := decodeView([]byte(good), "demo"); err != nil {
		t.Fatalf("matching runtime success rejected: %v", err)
	}
	for _, operation := range []string{"Failed", "Expired", "Cancelled"} {
		bad := strings.Replace(good, `"operation":"Succeeded"`, fmt.Sprintf(`"operation":%q`, operation), 1)
		if _, err := decodeView([]byte(bad), "demo"); err == nil {
			t.Fatalf("accepted contradictory runtime operation %s", operation)
		}
	}
}

func TestClaimActivityCannotPrecedeRequestResolution(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("valid admitted Work rejected: %v", err)
	}
	earlyBound := strings.Replace(boundFact("demo"), `"id":"fact:3","sequence":3`, `"id":"fact:2","sequence":2`, 1)
	lateResolution := strings.Replace(resolutionFact("demo", "Allow"), `"id":"fact:2","sequence":2`, `"id":"fact:3","sequence":3`, 1)
	terminal := `{"id":"fact:4","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	invalid := withEvidenceFacts(base, receivedFact("demo"), earlyBound, lateResolution, terminal)
	if _, err := decodeView([]byte(invalid), "demo"); err == nil {
		t.Fatal("accepted worker allocation before admission resolution")
	}
}

func TestTerminalBackendIdentityRequiresBoundFact(t *testing.T) {
	base := installedEvidenceJSON("demo", "Succeeded")
	withoutBound := withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), `{"id":"fact:4","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`)
	if _, err := decodeView([]byte(withoutBound), "demo"); err == nil {
		t.Fatal("accepted terminal worker identity without a Bound allocation fact")
	}
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("correlated Bound worker identity rejected: %v", err)
	}
}

func TestFailedClaimNeedsBackendIdentityOnlyAfterAllocation(t *testing.T) {
	failedBeforeAllocation := strings.ReplaceAll(installedEvidenceJSON("demo", "Failed"), `,"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}`, ``)
	failedBeforeAllocation = withEvidenceFacts(failedBeforeAllocation, receivedFact("demo"), resolutionFact("demo", "Allow"), `{"id":"fact:4","sequence":4,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Failed"}`)
	if _, err := decodeView([]byte(failedBeforeAllocation), "demo"); err != nil {
		t.Fatalf("pre-allocation failure must remain representable: %v", err)
	}
	for _, fact := range []string{
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Bound"}`,
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"StartFailed"}`,
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"WorkerActivity","requestRef":"demo","claimId":"claim:demo","operation":"TurnStarted"}`,
		`{"id":"fact:5","sequence":5,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke"}`,
	} {
		if _, err := decodeView([]byte(appendEvidenceFact(failedBeforeAllocation, fact)), "demo"); err == nil {
			t.Fatal("accepted post-allocation evidence without backend identity")
		}
	}
}

func TestCancelledOutcomeMatchesFailedClaimAndAuditFact(t *testing.T) {
	cancelled := installedEvidenceJSON("cancelled", "Failed")
	cancelled = strings.Replace(cancelled, `"outcome":{"status":"Failed"}`, `"outcome":{"status":"Cancelled"}`, 1)
	cancelled = strings.ReplaceAll(cancelled, `"operation":"Failed"`, `"operation":"Cancelled"`)
	if _, err := decodeView([]byte(cancelled), "cancelled"); err != nil {
		t.Fatalf("service's Failed claim/Cancelled outcome rejected: %v", err)
	}
	lastOperation := strings.LastIndex(cancelled, `"operation":"Cancelled"`)
	if lastOperation < 0 {
		t.Fatal("terminal audit mutation did not find the fixture")
	}
	invalid := cancelled[:lastOperation] + strings.Replace(cancelled[lastOperation:], `"operation":"Cancelled"`, `"operation":"Failed"`, 1)
	if _, err := decodeView([]byte(invalid), "cancelled"); err == nil {
		t.Fatal("accepted Cancelled outcome without matching terminal audit fact")
	}
	expired := strings.Replace(cancelled, `"phase":"Failed"`, `"phase":"Expired"`, 1)
	if _, err := decodeView([]byte(expired), "cancelled"); err == nil {
		t.Fatal("accepted Expired claim with Cancelled Work outcome")
	}
}

func TestListTransportSupportsBoundedAggregateEvidence(t *testing.T) {
	const aggregate = 9 << 20
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", aggregate)))
	}))
	defer server.Close()
	client := Client{}
	if _, err := client.call(context.Background(), server.URL, nil, ""); err != nil {
		t.Fatalf("valid aggregate list capacity rejected: %v", err)
	}
	if _, err := client.call(context.Background(), server.URL, nil, "demo"); err == nil {
		t.Fatal("single evidence response exceeded the per-view limit")
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
