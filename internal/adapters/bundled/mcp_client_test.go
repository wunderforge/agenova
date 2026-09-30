// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/toolbackend"
)

type seenRequest struct {
	httpMethod, rpcMethod        string
	correlation, session, protov string
}

// fakeMCP is a deterministic Streamable HTTP server. Each hook may override
// one step; the defaults implement a well-behaved JSON server.
type fakeMCP struct {
	t        *testing.T
	mu       sync.Mutex
	seen     []seenRequest
	version  string
	initNote int
	onCall   func(w http.ResponseWriter, id json.RawMessage)
}

func (f *fakeMCP) calls(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.seen {
		if r.rpcMethod == method || r.httpMethod == method {
			n++
		}
	}
	return n
}

func (f *fakeMCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var message struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &message); err != nil {
			f.t.Errorf("client sent invalid JSON: %s", body)
		}
		if r.Header.Get("Accept") != "application/json, text/event-stream" || r.Header.Get("Content-Type") != "application/json" {
			f.t.Errorf("missing Streamable HTTP headers on %s", message.Method)
		}
	}
	f.mu.Lock()
	f.seen = append(f.seen, seenRequest{httpMethod: r.Method, rpcMethod: message.Method, correlation: r.Header.Get(MCPCorrelationHeader), session: r.Header.Get("Mcp-Session-Id"), protov: r.Header.Get("MCP-Protocol-Version")})
	f.mu.Unlock()
	switch {
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case message.Method == "initialize":
		w.Header().Set("Mcp-Session-Id", "session-1")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":%q,"capabilities":{"tools":{}},"serverInfo":{"name":"fake","version":"0"}}}`, message.ID, f.version)
	case message.Method == "notifications/initialized":
		w.WriteHeader(f.initNote)
	case message.Method == "tools/call":
		f.onCall(w, message.ID)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func jsonResult(text string) func(http.ResponseWriter, json.RawMessage) {
	return func(w http.ResponseWriter, id json.RawMessage) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}})
		_, _ = w.Write(body)
	}
}

func rawResult(contentType, body string) func(http.ResponseWriter, json.RawMessage) {
	return func(w http.ResponseWriter, id json.RawMessage) {
		w.Header().Set("Content-Type", contentType)
		_, _ = io.WriteString(w, strings.ReplaceAll(body, "$ID", string(id)))
	}
}

func startFake(t *testing.T, onCall func(http.ResponseWriter, json.RawMessage)) (*fakeMCP, *httptest.Server) {
	t.Helper()
	fake := &fakeMCP{t: t, version: mcpProtocolVersion, initNote: http.StatusAccepted, onCall: onCall}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return fake, server
}

func testClient(endpoint string, maxResponse int) *mcpClient {
	return newMCPClient(endpoint, 2*time.Second, 8192, maxResponse, map[string]mcpRoute{
		mcpRouteKey("repo.read", "repo:agenova/e16-fixture"): {tool: "read_file", parameter: "file"},
	})
}

func readCall(file string) toolbackend.Invocation {
	return toolbackend.Invocation{ID: "inv-42", ClaimID: "claim-1", Operation: "repo.read", ResourceScope: "repo:agenova/e16-fixture", Parameters: map[string]string{"file": file}}
}

func TestMCPClientJSONSessionCarriesCorrelationAndClosesSession(t *testing.T) {
	fake, server := startFake(t, jsonResult("payment notes"))
	result, err := testClient(server.URL, 65536).Invoke(context.Background(), readCall("README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "payment notes" || result.ResultRef != "repo:agenova/e16-fixture/README.md" {
		t.Fatalf("unexpected result %+v", result)
	}
	want := []string{"initialize", "notifications/initialized", "tools/call", http.MethodDelete}
	if len(fake.seen) != len(want) {
		t.Fatalf("requests %+v, want %v", fake.seen, want)
	}
	for i, r := range fake.seen {
		if r.rpcMethod != want[i] && r.httpMethod != want[i] {
			t.Fatalf("request %d was %+v, want %s", i, r, want[i])
		}
		if r.correlation != "inv-42" {
			t.Fatalf("request %d lacks the correlation header", i)
		}
		if i > 0 && (r.session != "session-1" || r.protov != mcpProtocolVersion) {
			t.Fatalf("request %d lacks session or protocol headers: %+v", i, r)
		}
	}
	if fake.seen[0].session != "" || fake.seen[0].protov != "" {
		t.Fatal("initialize must not carry session or negotiated version headers")
	}
}

func TestMCPClientReadsEventStreamPastNotifications(t *testing.T) {
	stream := ": keep-alive\n\nevent: message\nid: 7\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{}}\n\n" +
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":$ID,\ndata: \"result\":{\"content\":[{\"type\":\"text\",\"text\":\"a\"},{\"type\":\"text\",\"text\":\"b\"}]}}\n\n"
	_, server := startFake(t, rawResult("text/event-stream", stream))
	result, err := testClient(server.URL, 65536).Invoke(context.Background(), readCall("README.md"))
	if err != nil || result.Text != "a\nb" {
		t.Fatalf("result %+v err %v", result, err)
	}
}

func TestMCPClientFailuresAreClassifiedWithoutReplay(t *testing.T) {
	tests := []struct {
		name   string
		onCall func(http.ResponseWriter, json.RawMessage)
		want   error
	}{
		{"session expired", func(w http.ResponseWriter, _ json.RawMessage) { w.WriteHeader(http.StatusNotFound) }, toolbackend.ErrProtocol},
		{"server error", func(w http.ResponseWriter, _ json.RawMessage) { w.WriteHeader(http.StatusBadGateway) }, toolbackend.ErrUnavailable},
		{"redirect not followed", func(w http.ResponseWriter, _ json.RawMessage) {
			w.Header().Set("Location", "http://127.0.0.1:1/elsewhere")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}, toolbackend.ErrProtocol},
		{"json-rpc error", rawResult("application/json", `{"jsonrpc":"2.0","id":$ID,"error":{"code":-32602,"message":"secret detail"}}`), toolbackend.ErrProvider},
		{"tool error", rawResult("application/json", `{"jsonrpc":"2.0","id":$ID,"result":{"isError":true,"content":[{"type":"text","text":"not-found"}]}}`), toolbackend.ErrProvider},
		{"non-text content", rawResult("application/json", `{"jsonrpc":"2.0","id":$ID,"result":{"content":[{"type":"image","data":"AA==","mimeType":"image/png"}]}}`), toolbackend.ErrProtocol},
		{"empty content", rawResult("application/json", `{"jsonrpc":"2.0","id":$ID,"result":{"content":[]}}`), toolbackend.ErrProtocol},
		{"mismatched id", rawResult("application/json", `{"jsonrpc":"2.0","id":99,"result":{"content":[{"type":"text","text":"x"}]}}`), toolbackend.ErrProtocol},
		{"malformed json", rawResult("application/json", `{"jsonrpc":`), toolbackend.ErrProtocol},
		{"trailing json", rawResult("application/json", `{"jsonrpc":"2.0","id":$ID,"result":{"content":[{"type":"text","text":"x"}]}} {}`), toolbackend.ErrProtocol},
		{"unsupported media type", rawResult("text/plain", `hello`), toolbackend.ErrProtocol},
		{"server request in stream", rawResult("text/event-stream", "data: {\"jsonrpc\":\"2.0\",\"id\":\"s1\",\"method\":\"sampling/createMessage\",\"params\":{}}\n\n"), toolbackend.ErrProtocol},
		{"stream ends early", rawResult("text/event-stream", "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n"), toolbackend.ErrProtocol},
		{"slow server", func(w http.ResponseWriter, _ json.RawMessage) { time.Sleep(3 * time.Second) }, toolbackend.ErrTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake, server := startFake(t, tc.onCall)
			_, err := testClient(server.URL, 65536).Invoke(context.Background(), readCall("README.md"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want %v", err, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret detail") {
				t.Fatal("raw server text leaked")
			}
			if got := fake.calls("tools/call"); got != 1 {
				t.Fatalf("tools/call sent %d times; it must never be replayed", got)
			}
		})
	}
}

func TestMCPClientHandshakeFailuresStopBeforeToolCall(t *testing.T) {
	for name, change := range map[string]func(*fakeMCP){
		"unsupported negotiated version": func(f *fakeMCP) { f.version = "2024-11-05" },
		"initialized not accepted":       func(f *fakeMCP) { f.initNote = http.StatusOK },
	} {
		t.Run(name, func(t *testing.T) {
			fake, server := startFake(t, jsonResult("x"))
			change(fake)
			if _, err := testClient(server.URL, 65536).Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrProtocol) {
				t.Fatalf("error %v, want protocol error", err)
			}
			if fake.calls("tools/call") != 0 {
				t.Fatal("tools/call sent after a failed handshake")
			}
		})
	}
}

func TestMCPClientEnforcesResponseByteLimit(t *testing.T) {
	text := strings.Repeat("x", 500)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}})
	_, server := startFake(t, rawResult("application/json", string(body)))
	if _, err := testClient(server.URL, len(body)-1).Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrResponseTooLarge) {
		t.Fatalf("error %v, want response too large", err)
	}
	if result, err := testClient(server.URL, len(body)).Invoke(context.Background(), readCall("README.md")); err != nil || result.Text != text {
		t.Fatalf("a response exactly at the limit must pass: %v", err)
	}
}

func TestMCPClientRejectsBeforeNetworkWhenRequestIsTooLarge(t *testing.T) {
	fake, server := startFake(t, jsonResult("x"))
	client := testClient(server.URL, 65536)
	client.maxRequest = 64
	if _, err := client.Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrArguments) {
		t.Fatalf("error %v, want argument error", err)
	}
	if len(fake.seen) != 0 {
		t.Fatal("an oversized request reached the server")
	}
}

func TestMCPClientUnreachableServerIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	endpoint := server.URL
	server.Close()
	if _, err := testClient(endpoint, 65536).Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrUnavailable) {
		t.Fatalf("error %v, want unavailable", err)
	}
}

// Interoperability with the independent official-SDK fixture. Opt-in: build
// and run harness/integration/mcpfixture, then set the endpoint.
func TestMCPClientInteroperatesWithFixtureServer(t *testing.T) {
	endpoint := os.Getenv("AGENOVA_MCP_INTEROP_ENDPOINT")
	if endpoint == "" {
		t.Skip("set AGENOVA_MCP_INTEROP_ENDPOINT to a running mcpfixture /mcp endpoint")
	}
	client := testClient(endpoint, 65536)
	result, err := client.Invoke(context.Background(), readCall("README.md"))
	if err != nil || !strings.Contains(result.Text, "payment-client") {
		t.Fatalf("README read failed: %+v %v", result, err)
	}
	if _, err := client.Invoke(context.Background(), readCall("logs/full-trace.log")); !errors.Is(err, toolbackend.ErrResponseTooLarge) {
		t.Fatalf("oversized file: %v", err)
	}
	if _, err := client.Invoke(context.Background(), readCall("missing.md")); !errors.Is(err, toolbackend.ErrProvider) {
		t.Fatalf("missing file: %v", err)
	}
}
