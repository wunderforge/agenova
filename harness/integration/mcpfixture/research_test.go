// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

type responseTransport func(*http.Request) (*http.Response, error)

func (f responseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestResearchSourceBoundary(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.100.100.200", "::1", "::ffff:127.0.0.1", "192.0.2.2", "fc00::1"} {
		if publicIP(netip.MustParseAddr(ip)) {
			t.Fatal("private/reserved accepted", ip)
		}
	}
	if !publicIP(netip.MustParseAddr("1.1.1.1")) {
		t.Fatal("public refused")
	}
	calls := 0
	client := &http.Client{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			t.Fatal("unsafe request")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<nav>noise</nav><main>Useful product evidence</main>"))}, nil
	})}
	if _, code := fetchSource(context.Background(), "http://169.254.169.254/", client); code != "unknown-source" || calls != 0 {
		t.Fatal("arbitrary target dispatched")
	}
	text, code := fetchSource(context.Background(), "product-a", client)
	if code != "" || !strings.Contains(text, "Useful product evidence") || strings.Contains(text, "noise") {
		t.Fatal(text, code)
	}
	client.Transport = responseTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (512<<10)+1)))}, nil
	})
	if _, code = fetchSource(context.Background(), "product-a", client); code != "invalid-or-large-response" {
		t.Fatal(code)
	}
	if publicClient().CheckRedirect(nil, nil) == nil {
		t.Fatal("redirect accepted")
	}
}
func TestResearchMCPRetainsLegacyAndQueriesSyntheticCRM(t *testing.T) {
	root, err := os.OpenRoot("../../demo/incident-mcp/data")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cfg := config{research: true, maxFileBytes: 1 << 20}
	buffer := &syncBuffer{}
	server := httptest.NewServer(newHandler(root, cfg, newLogger(buffer, "research-test")))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 4 {
		t.Fatal(list, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "crm_read", Arguments: map[string]any{"dataset_id": "customer-retention"}})
	if err != nil || result.IsError {
		t.Fatal(result, err)
	}
	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, `"synthetic":true`) || !strings.Contains(text, "Northstar") {
		t.Fatal(text)
	}
	denied, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "crm_read", Arguments: map[string]any{"dataset_id": "../../etc/passwd"}})
	if err != nil || !denied.IsError {
		t.Fatal("dataset escape", err)
	}
	old, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "read_file", Arguments: map[string]any{"file": "README.md"}})
	if err != nil || old.IsError {
		t.Fatal("legacy read regressed", err)
	}
	found := false
	for _, e := range buffer.entries(t) {
		if e.Event == "tool" && e.Tool == "crm_read" && e.Outcome == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatal("independent receipt missing")
	}
}
func TestResearchActualPublicSources(t *testing.T) {
	if os.Getenv("AGENOVA_TEST_PUBLIC_RESEARCH") != "1" {
		t.Skip("explicit public source verification only")
	}
	for _, id := range []string{"product-a", "product-b"} {
		text, code := fetchSource(context.Background(), id, publicClient())
		if code != "" || !strings.Contains(text, "fetched_at") {
			t.Fatalf("%s: %s", id, code)
		}
	}
}
