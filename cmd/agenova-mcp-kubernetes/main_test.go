// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	args []string
	data []byte
	err  error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.args = append([]string(nil), args...)
	return f.data, f.err
}

func testServer(t *testing.T, runner commandRunner) (*server, string) {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("test-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &server{config: config{namespace: "monitoring", tokenFile: tokenFile, allowed: map[string]bool{"pods": true, "deployments": true}}, runner: runner}, tokenFile
}

func rpc(t *testing.T, s http.Handler, method string, id any, params any, token string, initialized bool) *httptest.ResponseRecorder {
	t.Helper()
	message := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		message["id"] = id
	}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(correlationHeader, "inv-1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if initialized {
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
	}
	res := httptest.NewRecorder()
	s.ServeHTTP(res, req)
	return res
}

func TestMCPHandshakeAndAllowlistedList(t *testing.T) {
	runner := &fakeRunner{data: []byte(`{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"b","namespace":"monitoring"},"status":{"phase":"Pending","containerStatuses":[]}},{"metadata":{"name":"a","namespace":"monitoring"},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":2},{"ready":false,"restartCount":1}]}}]}`)}
	s, _ := testServer(t, runner)
	init := rpc(t, s, "initialize", 1, map[string]any{"protocolVersion": protocolVersion}, "test-token", false)
	if init.Code != http.StatusOK || !strings.Contains(init.Body.String(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize = %d %s", init.Code, init.Body.String())
	}
	note := rpc(t, s, "notifications/initialized", nil, nil, "test-token", true)
	if note.Code != http.StatusAccepted {
		t.Fatalf("initialized = %d", note.Code)
	}
	call := rpc(t, s, "tools/call", 2, map[string]any{"name": "list_resources", "arguments": map[string]string{"kind": "pods"}}, "test-token", true)
	if call.Code != http.StatusOK || !strings.Contains(call.Body.String(), `monitoring/a`) || !strings.Contains(call.Body.String(), `monitoring/b`) {
		t.Fatalf("call = %d %s", call.Code, call.Body.String())
	}
	for _, expected := range []string{`\"phase\":\"Running\"`, `\"readyContainers\":1`, `\"totalContainers\":2`, `\"restartCount\":3`} {
		if !strings.Contains(call.Body.String(), expected) {
			t.Fatalf("call omitted bounded pod diagnostic %s: %s", expected, call.Body.String())
		}
	}
	want := []string{"get", "pods", "--namespace", "monitoring", "--output=json", "--request-timeout=5s"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("kubectl args = %q, want %q", runner.args, want)
	}
}

func TestMCPRejectsCredentialScopeAndMutationBeforeKubectl(t *testing.T) {
	for _, tc := range []struct{ name, token, method, kind string }{
		{"wrong token", "wrong", "list_resources", "pods"},
		{"wrong tool", "test-token", "delete_resources", "pods"},
		{"outside kind", "test-token", "list_resources", "secrets"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{data: []byte(`{"apiVersion":"v1","kind":"PodList","items":[]}`)}
			s, _ := testServer(t, runner)
			res := rpc(t, s, "tools/call", 2, map[string]any{"name": tc.method, "arguments": map[string]string{"kind": tc.kind}}, tc.token, true)
			if res.Code == http.StatusOK && !strings.Contains(res.Body.String(), `"error"`) {
				t.Fatalf("request unexpectedly allowed: %s", res.Body.String())
			}
			if len(runner.args) != 0 {
				t.Fatalf("rejected request invoked kubectl: %q", runner.args)
			}
		})
	}
}

func TestMCPProviderFailureIsSanitized(t *testing.T) {
	runner := &fakeRunner{err: errors.New("secret kubeconfig detail")}
	s, _ := testServer(t, runner)
	res := rpc(t, s, "tools/call", 2, map[string]any{"name": "list_resources", "arguments": map[string]string{"kind": "pods"}}, "test-token", true)
	if !strings.Contains(res.Body.String(), `"isError":true`) || strings.Contains(res.Body.String(), "secret kubeconfig") {
		t.Fatalf("provider failure leaked: %s", res.Body.String())
	}
}

func TestBoundedResourceSummaryRejectsOversizeAndSorts(t *testing.T) {
	text, err := boundedResourceSummary([]byte(`{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"z","namespace":"n"},"status":{"replicas":3,"readyReplicas":2,"updatedReplicas":3,"availableReplicas":2}},{"metadata":{"name":"a","namespace":"n"},"status":{"replicas":1,"readyReplicas":1,"updatedReplicas":1,"availableReplicas":1}}]}`), "n", "deployments")
	if err != nil || strings.Index(text, "n/a") > strings.Index(text, "n/z") {
		t.Fatalf("result = %q err=%v", text, err)
	}
	for _, expected := range []string{`"replicas":3`, `"readyReplicas":2`, `"updatedReplicas":3`, `"availableReplicas":2`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("deployment summary omitted %s: %s", expected, text)
		}
	}
	if _, err := boundedResourceSummary(bytes.Repeat([]byte("x"), (2<<20)+1), "n", "pods"); err == nil {
		t.Fatal("oversized Kubernetes response accepted")
	}
	foreign := []byte(`{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"p","namespace":"other"}}]}`)
	if _, err := boundedResourceSummary(foreign, "n", "pods"); err == nil {
		t.Fatal("cross-namespace response accepted")
	}
	mismatched := []byte(`{"apiVersion":"v1","kind":"SecretList","items":[]}`)
	if _, err := boundedResourceSummary(mismatched, "n", "pods"); err == nil {
		t.Fatal("mismatched Kubernetes list kind accepted")
	}
	invalidPhase := []byte(`{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"p","namespace":"n"},"status":{"phase":"Injected"}}]}`)
	if _, err := boundedResourceSummary(invalidPhase, "n", "pods"); err == nil {
		t.Fatal("invalid pod phase accepted")
	}
	inconsistentDeployment := []byte(`{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"d","namespace":"n"},"status":{"replicas":1,"readyReplicas":2}}]}`)
	if _, err := boundedResourceSummary(inconsistentDeployment, "n", "deployments"); err == nil {
		t.Fatal("inconsistent deployment status accepted")
	}
}

func TestLoadConfigRequiresExplicitBoundary(t *testing.T) {
	t.Setenv("AGENOVA_MCP_NAMESPACE", "")
	t.Setenv("AGENOVA_MCP_BEARER_TOKEN_FILE", "")
	t.Setenv("AGENOVA_MCP_ALLOWED_KINDS", "")
	t.Setenv("AGENOVA_MCP_TLS_CERT_FILE", "")
	t.Setenv("AGENOVA_MCP_TLS_KEY_FILE", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("empty security boundary accepted")
	}
}
