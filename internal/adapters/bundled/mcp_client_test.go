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
	authorization                string
}

// fakeMCP is a deterministic Streamable HTTP server. Each hook may override
// one step; the defaults implement a well-behaved JSON server.
type fakeMCP struct {
	t       *testing.T
	mu      sync.Mutex
	seen    []seenRequest
	version string
	// capabilities is the initialize result's capabilities object.
	capabilities string
	initNote     int
	onCall       func(w http.ResponseWriter, id json.RawMessage)
	// reject answers requests of this RPC (or HTTP) method with status.
	reject       string
	rejectStatus int
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
	f.seen = append(f.seen, seenRequest{httpMethod: r.Method, rpcMethod: message.Method, correlation: r.Header.Get(MCPCorrelationHeader), session: r.Header.Get("Mcp-Session-Id"), protov: r.Header.Get("MCP-Protocol-Version"), authorization: r.Header.Get("Authorization")})
	f.mu.Unlock()
	switch {
	case f.reject != "" && (message.Method == f.reject || r.Method == f.reject):
		w.WriteHeader(f.rejectStatus)
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case message.Method == "initialize":
		w.Header().Set("Mcp-Session-Id", "session-1")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":%q,"capabilities":%s,"serverInfo":{"name":"fake","version":"0"}}}`, message.ID, f.version, f.capabilities)
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
	fake := &fakeMCP{t: t, version: mcpProtocolVersion, capabilities: `{"tools":{}}`, initNote: http.StatusAccepted, onCall: onCall}
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
	for i, r := range fake.seen {
		if r.authorization != "" {
			t.Fatalf("credential-free backend sent Authorization on request %d", i)
		}
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
		"tools capability absent":        func(f *fakeMCP) { f.capabilities = `{"resources":{}}` },
		"tools capability null":          func(f *fakeMCP) { f.capabilities = `{"tools":null}` },
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

// N7: the cap applies to the whole wire body, whatever its framing: an
// event stream counts every event, including notifications before the
// result, and a chunked body without Content-Length is counted as it streams.
func TestMCPClientResponseLimitCoversEventStreamsAndChunkedBodies(t *testing.T) {
	text := strings.Repeat("y", 400)
	result := func(id json.RawMessage) string {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}})
		return string(body)
	}
	note := `{"jsonrpc":"2.0","method":"notifications/progress","params":{"progress":1,"message":"` + strings.Repeat("z", 300) + `"}}`
	for name, tc := range map[string]struct {
		contentType string
		body        func(json.RawMessage) string
		chunked     bool
	}{
		"event stream":                         {"text/event-stream", func(id json.RawMessage) string { return "data: " + result(id) + "\n\n" }, false},
		"event stream with large notification": {"text/event-stream", func(id json.RawMessage) string { return "data: " + note + "\n\ndata: " + result(id) + "\n\n" }, false},
		"chunked json":                         {"application/json", result, true},
	} {
		t.Run(name, func(t *testing.T) {
			var size int
			onCall := func(w http.ResponseWriter, id json.RawMessage) {
				body := tc.body(id)
				size = len(body)
				w.Header().Set("Content-Type", tc.contentType)
				if !tc.chunked {
					_, _ = io.WriteString(w, body)
					return
				}
				for start := 0; start < len(body); start += 64 {
					end := min(start+64, len(body))
					_, _ = io.WriteString(w, body[start:end])
					w.(http.Flusher).Flush()
				}
			}
			fake, server := startFake(t, onCall)
			// Learn the exact wire size with a generous limit, then probe both sides of it.
			if _, err := testClient(server.URL, 1<<20).Invoke(context.Background(), readCall("README.md")); err != nil {
				t.Fatal(err)
			}
			if _, err := testClient(server.URL, size-1).Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrResponseTooLarge) {
				t.Fatalf("one byte over the limit: error %v, want response too large", err)
			}
			if got, err := testClient(server.URL, size).Invoke(context.Background(), readCall("README.md")); err != nil || got.Text != text {
				t.Fatalf("a body exactly at the limit must pass: %v", err)
			}
			if calls := fake.calls("tools/call"); calls != 3 {
				t.Fatalf("tools/call sent %d times for three invocations; a rejected response must not be replayed", calls)
			}
		})
	}
}

// N7: a valid result does not end the byte budget; an oversized tail after it
// still fails the call, and no observation is returned.
func TestMCPClientRejectsOversizedEventStreamTailAfterResult(t *testing.T) {
	tail := ": " + strings.Repeat("c", 2000) + "\n\n"
	fake, server := startFake(t, func(w http.ResponseWriter, id json.RawMessage) {
		w.Header().Set("Content-Type", "text/event-stream")
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}})
		_, _ = io.WriteString(w, "data: "+string(body)+"\n\n"+tail)
	})
	result, err := testClient(server.URL, 256).Invoke(context.Background(), readCall("README.md"))
	if !errors.Is(err, toolbackend.ErrResponseTooLarge) || result.Text != "" {
		t.Fatalf("result %+v error %v, want response too large and no observation", result, err)
	}
	if fake.calls("tools/call") != 1 {
		t.Fatal("tools/call must not be replayed")
	}
	if _, err := testClient(server.URL, 4096).Invoke(context.Background(), readCall("README.md")); err != nil {
		t.Fatalf("the same stream within the limit must pass: %v", err)
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

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// secretDouble records each read; it fails with err (whose text must never
// surface) or returns value.
type secretDouble struct {
	mu    sync.Mutex
	reads []string
	value []byte
	err   error
	block bool
}

func (s *secretDouble) ReadSecretKey(ctx context.Context, name, key string) ([]byte, error) {
	s.mu.Lock()
	s.reads = append(s.reads, name+"/"+key)
	s.mu.Unlock()
	if s.block {
		<-ctx.Done()
		return nil, errors.New("kubectl output: " + testToken)
	}
	if s.err != nil {
		return nil, s.err
	}
	return append([]byte(nil), s.value...), nil
}

func tokenClient(endpoint string, secrets ProvisionalSecretReader) *mcpClient {
	client := testClient(endpoint, 65536)
	client.token = &mcpTokenRef{name: "e16-mcp-token", key: "token"}
	client.secrets = secrets
	return client
}

func TestMCPClientSendsResolvedTokenOnEveryRequestAndRereadsPerCall(t *testing.T) {
	fake, server := startFake(t, jsonResult("payment notes"))
	secrets := &secretDouble{value: []byte(testToken)}
	client := tokenClient(server.URL, secrets)
	for i := 0; i < 2; i++ {
		if _, err := client.Invoke(context.Background(), readCall("README.md")); err != nil {
			t.Fatal(err)
		}
	}
	if len(fake.seen) != 8 {
		t.Fatalf("requests %+v", fake.seen)
	}
	for i, r := range fake.seen {
		if r.authorization != "Bearer "+testToken {
			t.Fatalf("request %d (%s %s) did not carry the bearer token", i, r.httpMethod, r.rpcMethod)
		}
	}
	if len(secrets.reads) != 2 || secrets.reads[0] != "e16-mcp-token/token" {
		t.Fatalf("token must be read once per call from the backend's own reference: %v", secrets.reads)
	}
	session := &mcpSession{client: client, correlation: "inv-42", authorization: func() string { return "Bearer " + testToken }}
	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
		if strings.Contains(fmt.Sprintf(format, session), testToken) || strings.Contains(fmt.Sprintf(format, *session), testToken) || strings.Contains(fmt.Sprintf(format, session.authorization), testToken) {
			t.Fatalf("session formatting with %s exposed the token", format)
		}
	}
}

// An unresolvable token fails before any request: no header-less fallback,
// no retry, and nothing from the reader reaches the error.
func TestMCPClientUnresolvableTokenSendsNothing(t *testing.T) {
	previous := mcpTokenTimeout
	mcpTokenTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mcpTokenTimeout = previous })
	for name, secrets := range map[string]ProvisionalSecretReader{
		"no reader":        nil,
		"read failure":     &secretDouble{err: errors.New("secrets \"e16-mcp-token\" not found: " + testToken)},
		"slow reader":      &secretDouble{block: true},
		"empty value":      &secretDouble{value: []byte{}},
		"trailing newline": &secretDouble{value: []byte(testToken + "\n")},
		"inner space":      &secretDouble{value: []byte("abc def")},
		"non-ascii":        &secretDouble{value: []byte("tok\xc3\xa9n")},
		"oversized":        &secretDouble{value: []byte(strings.Repeat("a", 4097))},
		"header injection": &secretDouble{value: []byte("abc\r\nX-Evil: 1")},
	} {
		t.Run(name, func(t *testing.T) {
			fake, server := startFake(t, jsonResult("x"))
			_, err := tokenClient(server.URL, secrets).Invoke(context.Background(), readCall("README.md"))
			if !errors.Is(err, toolbackend.ErrCredentialUnavailable) {
				t.Fatalf("error %v, want credential unavailable", err)
			}
			if strings.Contains(err.Error(), testToken) || strings.Contains(err.Error(), "not found") {
				t.Fatal("reader detail leaked into the error")
			}
			if len(fake.seen) != 0 {
				t.Fatalf("an unresolved token still sent %d requests", len(fake.seen))
			}
			if double, ok := secrets.(*secretDouble); ok && len(double.reads) != 1 {
				t.Fatalf("token read %d times, want once", len(double.reads))
			}
		})
	}
	// Cancelling the Work while the token is read is a cancellation, not a
	// credential failure.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, server := startFake(t, jsonResult("x"))
	if _, err := tokenClient(server.URL, &secretDouble{block: true}).Invoke(ctx, readCall("README.md")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read returned %v", err)
	}
}

// 401 and 403 are credential rejections at any request of the session. At
// initialize no session exists, so nothing else is sent; later, tools/call is
// never replayed and the session close still carries the token.
func TestMCPClientCredentialRejectionIsExplicitAndNotRetried(t *testing.T) {
	for _, tc := range []struct {
		method   string
		status   int
		requests []string
	}{
		{"initialize", http.StatusUnauthorized, []string{"initialize"}},
		{"initialize", http.StatusForbidden, []string{"initialize"}},
		{"notifications/initialized", http.StatusUnauthorized, []string{"initialize", "notifications/initialized", http.MethodDelete}},
		{"tools/call", http.StatusUnauthorized, []string{"initialize", "notifications/initialized", "tools/call", http.MethodDelete}},
	} {
		t.Run(fmt.Sprintf("%s %d", tc.method, tc.status), func(t *testing.T) {
			fake, server := startFake(t, jsonResult("x"))
			fake.reject, fake.rejectStatus = tc.method, tc.status
			_, err := tokenClient(server.URL, &secretDouble{value: []byte(testToken)}).Invoke(context.Background(), readCall("README.md"))
			if !errors.Is(err, toolbackend.ErrCredentialRejected) {
				t.Fatalf("error %v, want credential rejected", err)
			}
			if len(fake.seen) != len(tc.requests) {
				t.Fatalf("requests %+v, want %v", fake.seen, tc.requests)
			}
			for i, r := range fake.seen {
				if (r.rpcMethod != tc.requests[i] && r.httpMethod != tc.requests[i]) || r.authorization != "Bearer "+testToken {
					t.Fatalf("request %d was %+v, want %s with the token", i, r, tc.requests[i])
				}
			}
		})
	}
	// A credential-free backend rejected by a token-required server is a
	// rejection too, never a protocol or transport fault.
	fake, server := startFake(t, jsonResult("x"))
	fake.reject, fake.rejectStatus = "initialize", http.StatusUnauthorized
	if _, err := testClient(server.URL, 65536).Invoke(context.Background(), readCall("README.md")); !errors.Is(err, toolbackend.ErrCredentialRejected) || len(fake.seen) != 1 {
		t.Fatalf("error %v after %d requests", err, len(fake.seen))
	}
}

// NewToolProvider binds the backend's own reference; without a reader every
// call fails closed, and the reference reaches only that backend's reader.
func TestMCPProviderBindsTokenReferencePerBackend(t *testing.T) {
	input, _ := toolPlatform(t)
	backend := input.Spec.Services.ToolBackends[0].Config
	backend[mcpTokenSecretKey] = "e16-mcp-token-wrong/token"
	// A token reference goes with the token route, never the credential-free /mcp.
	backend["endpoint"] = "http://" + fixtureMCPHost + ":8080/mcp-token"
	profiles := []map[string]any{input.Spec.Services.ToolProfiles[0].Config}
	secrets := &secretDouble{value: []byte(testToken)}
	provider, err := (&MCPHTTPTool{}).NewToolProviderWithSecrets(backend, profiles, secrets)
	if err != nil {
		t.Fatal(err)
	}
	client := provider.(*mcpClient)
	if client.token == nil || client.token.name != "e16-mcp-token-wrong" || client.token.key != "token" || client.secrets != secrets {
		t.Fatalf("reference not bound: %+v", client.token)
	}
	if len(secrets.reads) != 0 {
		t.Fatal("construction read the Secret")
	}
	plain, err := (&MCPHTTPTool{}).NewToolProvider(backend, profiles)
	if err != nil || plain.(*mcpClient).secrets != nil || plain.(*mcpClient).token == nil {
		t.Fatal("a provider without a reader must keep the reference and fail closed")
	}
	delete(backend, mcpTokenSecretKey)
	free, err := (&MCPHTTPTool{}).NewToolProviderWithSecrets(backend, profiles, secrets)
	if err != nil || free.(*mcpClient).token != nil {
		t.Fatal("a credential-free backend gained a token reference")
	}
}

// fileSecret reads a local token file, standing in for the installed reader.
type fileSecret string

func (f fileSecret) ReadSecretKey(context.Context, string, string) ([]byte, error) {
	return os.ReadFile(string(f))
}

// Token-path interoperability with the fixture's /mcp-token. Opt-in: run
// harness/integration/mcpfixture with FIXTURE_TOKEN_FILE, then set the
// endpoint and the same token file. Never used by the kind campaign.
func TestMCPClientInteroperatesWithTokenFixture(t *testing.T) {
	endpoint, tokenFile := os.Getenv("AGENOVA_MCP_INTEROP_TOKEN_ENDPOINT"), os.Getenv("AGENOVA_MCP_INTEROP_TOKEN_FILE")
	if endpoint == "" || tokenFile == "" {
		t.Skip("set AGENOVA_MCP_INTEROP_TOKEN_ENDPOINT and AGENOVA_MCP_INTEROP_TOKEN_FILE for a running mcpfixture /mcp-token")
	}
	call := func(id string) toolbackend.Invocation {
		c := readCall("README.md")
		c.ID = id
		return c
	}
	valid := tokenClient(endpoint, fileSecret(tokenFile))
	if result, err := valid.Invoke(context.Background(), call("inv-token-valid")); err != nil || !strings.Contains(result.Text, "payment-client") {
		t.Fatalf("valid token read failed: %+v %v", result, err)
	}
	wrong := tokenClient(endpoint, &secretDouble{value: []byte(strings.Repeat("0", 64))})
	if _, err := wrong.Invoke(context.Background(), call("inv-token-wrong")); !errors.Is(err, toolbackend.ErrCredentialRejected) {
		t.Fatalf("wrong token: %v", err)
	}
	if _, err := tokenClient(endpoint, nil).Invoke(context.Background(), call("inv-token-missing")); !errors.Is(err, toolbackend.ErrCredentialUnavailable) {
		t.Fatalf("unresolved token: %v", err)
	}
	if _, err := testClient(endpoint, 65536).Invoke(context.Background(), call("inv-token-none")); !errors.Is(err, toolbackend.ErrCredentialRejected) {
		t.Fatalf("credential-free client on the token path: %v", err)
	}
}

// A configured file whose reference would break the shared result-reference
// contract still reads successfully through the Set; only the optional
// reference is omitted.
func TestMCPClientOmitsAResultRefTheContractRejects(t *testing.T) {
	_, server := startFake(t, jsonResult("payment notes"))
	scope := "repo:agenova/e16-fixture"
	files := map[string]string{"README.md": scope + "/README.md", "notes/incident timeline.md": "", "a?b.md": "", "a#b.md": "", "a@b.md": ""}
	allowed := make([]string, 0, len(files))
	for file := range files {
		allowed = append(allowed, file)
	}
	tools, err := toolbackend.NewSet([]toolbackend.Binding{{
		Descriptor: toolbackend.Descriptor{Description: "Read one allowlisted file.", Operation: "repo.read", ResourceScope: scope, Parameter: "file", MaxBytes: 128, AllowedValues: allowed},
		Provider:   testClient(server.URL, 65536), Backend: "e16-mcp", MaxObservationBytes: 4096, MaxConcurrentCalls: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	for file, want := range files {
		result, err := tools.Invoke(context.Background(), toolbackend.Invocation{ID: "inv-42", ClaimID: "claim-1", Operation: "repo.read", ResourceScope: scope, Parameters: map[string]string{"file": file}})
		if err != nil || result.Text != "payment notes" || result.ResultRef != want {
			t.Fatalf("%q: result %+v, err %v; want reference %q", file, result, err, want)
		}
	}
	// A configured scope "file:" and file "etc/passwd" would join into the
	// fetchable "file:/etc/passwd".
	fileClient := newMCPClient(server.URL, 2*time.Second, 8192, 65536, map[string]mcpRoute{mcpRouteKey("repo.read", "file:"): {tool: "read_file", parameter: "file"}})
	fileTools, err := toolbackend.NewSet([]toolbackend.Binding{{
		Descriptor: toolbackend.Descriptor{Description: "Read one allowlisted file.", Operation: "repo.read", ResourceScope: "file:", Parameter: "file", MaxBytes: 128, AllowedValues: []string{"etc/passwd"}},
		Provider:   fileClient, Backend: "e16-mcp", MaxObservationBytes: 4096, MaxConcurrentCalls: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fileTools.Invoke(context.Background(), toolbackend.Invocation{ID: "inv-43", ClaimID: "claim-1", Operation: "repo.read", ResourceScope: "file:", Parameters: map[string]string{"file": "etc/passwd"}})
	if err != nil || result.Text != "payment notes" || result.ResultRef != "" {
		t.Fatalf("fetchable reference was not omitted: result %+v, err %v", result, err)
	}
}
