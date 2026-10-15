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
	runner := &fakeRunner{data: []byte(`{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"b","namespace":"monitoring"}},{"metadata":{"name":"a","namespace":"monitoring"}}]}`)}
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

func TestBoundedResourceNamesRejectsOversizeAndSorts(t *testing.T) {
	text, err := boundedResourceNames([]byte(`{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"z","namespace":"n"}},{"metadata":{"name":"a","namespace":"n"}}]}`))
	if err != nil || strings.Index(text, "n/a") > strings.Index(text, "n/z") {
		t.Fatalf("result = %q err=%v", text, err)
	}
	if _, err := boundedResourceNames(bytes.Repeat([]byte("x"), (2<<20)+1)); err == nil {
		t.Fatal("oversized Kubernetes response accepted")
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
