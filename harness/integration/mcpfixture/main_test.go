// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) entries(t *testing.T) []entry {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []entry
	scanner := bufio.NewScanner(bytes.NewReader(b.buf.Bytes()))
	for scanner.Scan() {
		var e entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("log line is not JSON: %q", scanner.Text())
		}
		out = append(out, e)
	}
	return out
}

type headerTransport struct {
	value         string
	authorization string
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(correlationHeader, h.value)
	if h.authorization != "" {
		r.Header.Set("Authorization", h.authorization)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func fixtureData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, text string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "payment client notes\n")
	write("logs/timeout.log", "attempt 1 latency=4s\n")
	write("logs/slow.log", "slow\n")
	write("big.txt", strings.Repeat("x", 2048))
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	// ConfigMap volumes expose files through a relative ..data symlink.
	if err := os.MkdirAll(filepath.Join(dir, "..2026_09_30", "cfg"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("..2026_09_30/cfg/linked.md", "linked inside root\n")
	if err := os.Symlink("..2026_09_30", filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/cfg/linked.md", filepath.Join(dir, "linked.md")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func startFixture(t *testing.T, cfg config) (*httptest.Server, *syncBuffer) {
	t.Helper()
	if cfg.dataDir == "" {
		cfg.dataDir = fixtureData(t)
	}
	if cfg.maxFileBytes == 0 {
		cfg.maxFileBytes = 1024
	}
	root, err := os.OpenRoot(cfg.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	logs := &syncBuffer{}
	server := httptest.NewServer(newHandler(root, cfg, newLogger(logs, "fixture-pod")))
	t.Cleanup(server.Close)
	return server, logs
}

func connect(t *testing.T, server *httptest.Server, correlation string) *mcp.ClientSession {
	t.Helper()
	return connectAt(t, server.URL+openPath, correlation, "")
}

// dial opens an SDK client session on endpoint, sending authorization (when
// set) and correlation on every request of the session. It pins 2025-06-18,
// the version Agenova's client sends, so the SDK client skips its
// server/discover probe and each session is initialize, initialized, then
// its calls and DELETE, as on kind.
func dial(t *testing.T, endpoint, correlation, authorization string) (*mcp.ClientSession, error) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "fixture-test", Version: "0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: &http.Client{Transport: headerTransport{value: correlation, authorization: authorization}}, DisableStandaloneSSE: true, MaxRetries: -1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, transport, &mcp.ClientSessionOptions{ProtocolVersion: "2025-06-18"})
	if err == nil {
		t.Cleanup(func() { session.Close() })
	}
	return session, err
}

func connectAt(t *testing.T, endpoint, correlation, authorization string) *mcp.ClientSession {
	t.Helper()
	session, err := dial(t, endpoint, correlation, authorization)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return session
}

func callRead(t *testing.T, session *mcp.ClientSession, file string) (string, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{"file": file}})
	if err != nil {
		t.Fatalf("call %q: %v", file, err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("call %q returned %d content items", file, len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("call %q returned non-text content", file)
	}
	return text.Text, result.IsError
}

func TestReadFileReturnsRealContentAndLogsReceiptFirst(t *testing.T) {
	server, logs := startFixture(t, config{})
	session := connect(t, server, "inv-1")
	text, isError := callRead(t, session, "README.md")
	if isError || text != "payment client notes\n" {
		t.Fatalf("unexpected result %q isError=%v", text, isError)
	}
	receipt, tool := -1, -1
	for i, e := range logs.entries(t) {
		if e.Event == "receipt" && e.RPCMethod == "tools/call" {
			if e.Tool != toolName || e.File != "README.md" || e.Correlation != "inv-1" || e.Session == "" || e.Pod != "fixture-pod" {
				t.Fatalf("incomplete receipt: %+v", e)
			}
			receipt = i
		}
		if e.Event == "tool" {
			if e.Outcome != "ok" || e.Correlation != "inv-1" || e.Bytes != len(text) {
				t.Fatalf("incomplete tool record: %+v", e)
			}
			tool = i
		}
	}
	if receipt < 0 || tool < 0 || receipt > tool {
		t.Fatalf("receipt must precede handler record: receipt=%d tool=%d", receipt, tool)
	}
}

func TestReadFileRefusesEscapesAndReportsToolErrors(t *testing.T) {
	server, logs := startFixture(t, config{})
	session := connect(t, server, "inv-2")
	for file, code := range map[string]string{
		"../secret.txt": "refused",
		"escape.txt":    "refused",
		"missing.md":    "not-found",
		"logs":          "not-a-file",
		"big.txt":       "too-large",
	} {
		text, isError := callRead(t, session, file)
		if !isError || text != "read_file failed: "+code {
			t.Errorf("%s: got %q isError=%v, want code %s", file, text, isError, code)
		}
	}
	for _, e := range logs.entries(t) {
		if strings.Contains(e.File, "secret") && e.Event == "tool" && e.Outcome != "error" {
			t.Fatalf("escape must not succeed: %+v", e)
		}
	}
}

func TestReadFileFollowsSymlinksThatStayInsideRoot(t *testing.T) {
	server, _ := startFixture(t, config{})
	session := connect(t, server, "inv-3")
	if text, isError := callRead(t, session, "linked.md"); isError || text != "linked inside root\n" {
		t.Fatalf("ConfigMap-style link failed: %q isError=%v", text, isError)
	}
}

func TestSlowFileHonoursClientCancellation(t *testing.T) {
	server, _ := startFixture(t, config{slowFile: "logs/slow.log", slowDelay: 2 * time.Second})
	session := connect(t, server, "inv-4")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{"file": "logs/slow.log"}}); err == nil {
		t.Fatal("slow call must not complete before the client deadline")
	}
	if time.Since(start) > time.Second {
		t.Fatal("client deadline was not honoured")
	}
}

func TestOversizedRequestIsLoggedAndRejectedBeforeDispatch(t *testing.T) {
	server, logs := startFixture(t, config{})
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"file":"` + strings.Repeat("a", maxRequestBytes) + `"}}}`
	response, err := http.Post(server.URL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", response.StatusCode)
	}
	entries := logs.entries(t)
	if len(entries) != 1 || entries[0].Event != "receipt" || entries[0].Error != "request-too-large" {
		t.Fatalf("expected one rejected receipt, got %+v", entries)
	}
}

func TestLogsNeverContainFileContentOrRawSession(t *testing.T) {
	server, logs := startFixture(t, config{})
	session := connect(t, server, "inv-5")
	callRead(t, session, "README.md")
	logs.mu.Lock()
	raw := logs.buf.String()
	logs.mu.Unlock()
	if strings.Contains(raw, "payment client notes") {
		t.Fatal("log contains file content")
	}
	if id := session.ID(); id != "" && strings.Contains(raw, id) {
		t.Fatal("log contains a raw session ID")
	}
}

func TestLoadConfigValidatesFixtureSettings(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	if _, err := loadConfig(env(map[string]string{"FIXTURE_SLOW_FILE": "a"})); err == nil {
		t.Fatal("slow file without delay must fail")
	}
	if _, err := loadConfig(env(map[string]string{"FIXTURE_MAX_FILE_BYTES": "0"})); err == nil {
		t.Fatal("zero file limit must fail")
	}
	cfg, err := loadConfig(env(map[string]string{"FIXTURE_SLOW_FILE": "logs/slow.log", "FIXTURE_SLOW_DELAY": "10s", "POD_NAME": "p"}))
	if err != nil || cfg.slowDelay != 10*time.Second || cfg.pod != "p" || cfg.addr != ":8080" || cfg.tokenDigest != nil {
		t.Fatalf("unexpected config %+v err=%v", cfg, err)
	}
}

// Synthetic campaign-style tokens: 64 lowercase hex characters each.
var (
	testToken  = strings.Repeat("3c9f0a7e", 8)
	wrongToken = strings.Repeat("b41d6e25", 8)
)

const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"raw","version":"0"}}}`

func writeTokenFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// startTokenFixture loads its token through loadConfig from a real file, the
// way main does.
func startTokenFixture(t *testing.T) (*httptest.Server, *syncBuffer) {
	t.Helper()
	cfg, err := loadConfig(func(key string) string {
		return map[string]string{"FIXTURE_TOKEN_FILE": writeTokenFile(t, testToken), "FIXTURE_DATA_DIR": fixtureData(t)}[key]
	})
	if err != nil {
		t.Fatal(err)
	}
	return startFixture(t, cfg)
}

// send makes one raw request with the MCP media types, so only the headers
// under test decide how the fixture answers.
func send(t *testing.T, method, url string, header http.Header, body string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header = header.Clone()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, string(data)
}

func correlated(entries []entry, correlation string) []entry {
	var out []entry
	for _, e := range entries {
		if e.Correlation == correlation {
			out = append(out, e)
		}
	}
	return out
}

func TestTokenPathServesEveryRequestOfAValidSession(t *testing.T) {
	server, logs := startTokenFixture(t)
	session := connectAt(t, server.URL+tokenPath, "inv-t1", "Bearer "+testToken)
	text, isError := callRead(t, session, "README.md")
	if isError || text != "payment client notes\n" {
		t.Fatalf("unexpected result %q isError=%v", text, isError)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	var receipts []string
	tools := 0
	for _, e := range correlated(logs.entries(t), "inv-t1") {
		if e.Path != tokenPath {
			t.Fatalf("entry on the wrong path: %+v", e)
		}
		switch e.Event {
		case "receipt":
			if e.Auth != "ok" || e.Error != "" {
				t.Fatalf("receipt not accepted with the token: %+v", e)
			}
			receipts = append(receipts, e.HTTPMethod+" "+e.RPCMethod)
		case "response":
			if e.Auth != "" || e.Status < 200 || e.Status > 299 {
				t.Fatalf("unexpected response line: %+v", e)
			}
		case "tool":
			if e.Outcome != "ok" || e.File != "README.md" {
				t.Fatalf("unexpected tool line: %+v", e)
			}
			tools++
		}
	}
	want := []string{"POST initialize", "POST notifications/initialized", "POST tools/call", "DELETE "}
	if strings.Join(receipts, ",") != strings.Join(want, ",") || tools != 1 {
		t.Fatalf("receipts %q and %d tool lines, want %q and one", receipts, tools, want)
	}
}

func TestTokenPathRejectsWithoutTheTokenBeforeAnySession(t *testing.T) {
	server, logs := startTokenFixture(t)
	invalidRequest := bearerChallenge + `, error="invalid_request"`
	for name, tc := range map[string]struct {
		method    string
		values    []string
		body      string
		class     string
		challenge string
	}{
		"missing":           {method: http.MethodPost, class: "missing", challenge: bearerChallenge},
		"basic":             {method: http.MethodPost, values: []string{"Basic eDp5"}, class: "malformed", challenge: invalidRequest},
		"scheme only":       {method: http.MethodPost, values: []string{"Bearer"}, class: "malformed", challenge: invalidRequest},
		"two headers":       {method: http.MethodPost, values: []string{"Bearer " + testToken, "Bearer " + testToken}, class: "malformed", challenge: invalidRequest},
		"wrong token":       {method: http.MethodPost, values: []string{"Bearer " + wrongToken}, class: "invalid", challenge: bearerChallenge + `, error="invalid_token"`},
		"missing delete":    {method: http.MethodDelete, class: "missing", challenge: bearerChallenge},
		"missing get":       {method: http.MethodGet, class: "missing", challenge: bearerChallenge},
		"missing too large": {method: http.MethodPost, body: strings.Repeat("a", maxRequestBytes+1), class: "missing", challenge: bearerChallenge},
	} {
		t.Run(name, func(t *testing.T) {
			correlation := "inv-" + strings.ReplaceAll(name, " ", "-")
			header := http.Header{correlationHeader: {correlation}}
			for _, v := range tc.values {
				header.Add("Authorization", v)
			}
			body := tc.body
			if body == "" && tc.method == http.MethodPost {
				body = initializeBody
			}
			response, text := send(t, tc.method, server.URL+tokenPath, header, body)
			if response.StatusCode != http.StatusUnauthorized || text != "bearer token required\n" {
				t.Fatalf("status %d body %q, want 401 with the fixed body", response.StatusCode, text)
			}
			if got := response.Header.Values("WWW-Authenticate"); len(got) != 1 || got[0] != tc.challenge {
				t.Fatalf("WWW-Authenticate %q, want %q", got, tc.challenge)
			}
			if id := response.Header.Get("Mcp-Session-Id"); id != "" {
				t.Fatalf("a rejected request opened session %q", id)
			}
			entries := correlated(logs.entries(t), correlation)
			if len(entries) != 2 {
				t.Fatalf("want one receipt and one response line, got %+v", entries)
			}
			receipt, answer := entries[0], entries[1]
			if receipt.Event != "receipt" || receipt.Path != tokenPath || receipt.Auth != tc.class || receipt.HTTPMethod != tc.method {
				t.Fatalf("unexpected receipt %+v", receipt)
			}
			if tc.body == "" && tc.method == http.MethodPost && (receipt.RPCMethod != "initialize" || receipt.Error != "") {
				t.Fatalf("receipt does not record the rejected initialize: %+v", receipt)
			}
			if tc.body != "" && receipt.Error != "request-too-large" {
				t.Fatalf("receipt does not record the oversized body: %+v", receipt)
			}
			if answer.Event != "response" || answer.Path != tokenPath || answer.Status != http.StatusUnauthorized || answer.Auth != "" {
				t.Fatalf("unexpected response line %+v", answer)
			}
		})
	}
	for _, e := range logs.entries(t) {
		if e.Event == "tool" {
			t.Fatalf("a rejected request reached the tool: %+v", e)
		}
	}
}

// The middleware answers the token path itself; the SDK handler behind it
// must never run unless the token is ok. Headers set here skip the HTTP
// client's own validation, so control bytes are covered too.
func TestTokenCheckNeverReachesTheSDKHandler(t *testing.T) {
	sum := sha256.Sum256([]byte(testToken))
	for name, tc := range map[string]struct {
		values []string
		class  string
	}{
		"missing":          {class: "missing"},
		"empty value":      {values: []string{""}, class: "malformed"},
		"basic":            {values: []string{"Basic " + testToken}, class: "malformed"},
		"scheme only":      {values: []string{"Bearer"}, class: "malformed"},
		"empty token":      {values: []string{"Bearer "}, class: "malformed"},
		"lowercase scheme": {values: []string{"bearer " + testToken}, class: "malformed"},
		"double space":     {values: []string{"Bearer  " + testToken}, class: "malformed"},
		"inner space":      {values: []string{"Bearer " + testToken[:8] + " " + testToken[8:]}, class: "malformed"},
		"trailing newline": {values: []string{"Bearer " + testToken + "\n"}, class: "malformed"},
		"delete byte":      {values: []string{"Bearer " + testToken + "\x7f"}, class: "malformed"},
		"non-ascii":        {values: []string{"Bearer " + testToken + "\u00e9"}, class: "malformed"},
		"two headers":      {values: []string{"Bearer " + testToken, "Bearer " + testToken}, class: "malformed"},
		"wrong token":      {values: []string{"Bearer " + wrongToken}, class: "invalid"},
		"token prefix":     {values: []string{"Bearer " + testToken[:63]}, class: "invalid"},
		"token extended":   {values: []string{"Bearer " + testToken + "0"}, class: "invalid"},
		"valid token":      {values: []string{"Bearer " + testToken}, class: "ok"},
	} {
		t.Run(name, func(t *testing.T) {
			logs := &syncBuffer{}
			reached := false
			handler := receipts(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusAccepted)
			}), newLogger(logs, "p"), tokenPath, sum[:], true)
			request := httptest.NewRequest(http.MethodPost, tokenPath, strings.NewReader(initializeBody))
			for _, v := range tc.values {
				request.Header.Add("Authorization", v)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			entries := logs.entries(t)
			if len(entries) != 2 || entries[0].Auth != tc.class {
				t.Fatalf("want a receipt with auth %q and a response line, got %+v", tc.class, entries)
			}
			if tc.class == "ok" {
				if !reached || recorder.Code != http.StatusAccepted {
					t.Fatalf("a valid token must reach the handler: reached=%v status=%d", reached, recorder.Code)
				}
				return
			}
			if reached || recorder.Code != http.StatusUnauthorized || entries[1].Status != http.StatusUnauthorized {
				t.Fatalf("reached=%v status=%d response=%+v, want 401 before the handler", reached, recorder.Code, entries[1])
			}
		})
	}
}

// /mcp stays credential-free. It applies the same classification as
// /mcp-token but only logs it: no header is missing, a header that is not
// exactly "Bearer <visible ASCII>" is malformed, and a well-formed one is ok
// only when it matches a configured token and invalid otherwise, including
// when no token file is configured at all.
func TestOpenPathLogsTheAuthClassAndNeverRejects(t *testing.T) {
	withToken, withTokenLogs := startTokenFixture(t)
	withoutToken, withoutTokenLogs := startFixture(t, config{})
	for name, tc := range map[string]struct {
		server        *httptest.Server
		logs          *syncBuffer
		authorization string
		class         string
	}{
		"no header":            {server: withToken, logs: withTokenLogs, class: "missing"},
		"malformed":            {server: withToken, logs: withTokenLogs, authorization: "Basic eDp5", class: "malformed"},
		"wrong token":          {server: withToken, logs: withTokenLogs, authorization: "Bearer " + wrongToken, class: "invalid"},
		"configured token":     {server: withToken, logs: withTokenLogs, authorization: "Bearer " + testToken, class: "ok"},
		"no token, no header":  {server: withoutToken, logs: withoutTokenLogs, class: "missing"},
		"no token, any bearer": {server: withoutToken, logs: withoutTokenLogs, authorization: "Bearer " + testToken, class: "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			correlation := "inv-open-" + strings.NewReplacer(" ", "-", ",", "").Replace(name)
			session := connectAt(t, tc.server.URL+openPath, correlation, tc.authorization)
			if text, isError := callRead(t, session, "README.md"); isError || text != "payment client notes\n" {
				t.Fatalf("unexpected result %q isError=%v", text, isError)
			}
			if err := session.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			receipts := 0
			for _, e := range correlated(tc.logs.entries(t), correlation) {
				if e.Path != openPath {
					t.Fatalf("entry on the wrong path: %+v", e)
				}
				if e.Event == "receipt" {
					receipts++
					if e.Auth != tc.class || e.Error != "" {
						t.Fatalf("receipt auth %q error %q, want %q and no error: %+v", e.Auth, e.Error, tc.class, e)
					}
				}
				if e.Event == "response" && (e.Status < 200 || e.Status > 299) {
					t.Fatalf("/mcp rejected a request: %+v", e)
				}
			}
			if receipts != 4 {
				t.Fatalf("%d receipts, want initialize, initialized, tools/call and DELETE", receipts)
			}
		})
	}
}

func TestSessionsAreNotSharedBetweenPaths(t *testing.T) {
	server, logs := startTokenFixture(t)
	open := connectAt(t, server.URL+openPath, "inv-open", "")
	token := connectAt(t, server.URL+tokenPath, "inv-token", "Bearer "+testToken)
	call := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"read_file","arguments":{"file":"README.md"}}}`
	for _, tc := range []struct {
		path, session, correlation string
	}{
		{path: tokenPath, session: open.ID(), correlation: "inv-cross-to-token"},
		{path: openPath, session: token.ID(), correlation: "inv-cross-to-open"},
	} {
		header := http.Header{correlationHeader: {tc.correlation}, "Authorization": {"Bearer " + testToken}, "Mcp-Session-Id": {tc.session}}
		response, _ := send(t, http.MethodPost, server.URL+tc.path, header, call)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s accepted a session from the other path: status %d", tc.path, response.StatusCode)
		}
		entries := correlated(logs.entries(t), tc.correlation)
		// The token was ok, so the SDK handler's own session table refused it.
		if len(entries) != 2 || entries[0].Auth != "ok" || entries[0].Session != sessionHash(tc.session) || entries[1].Status != http.StatusNotFound {
			t.Fatalf("unexpected entries %+v", entries)
		}
	}
	for _, e := range logs.entries(t) {
		if e.Event == "tool" {
			t.Fatalf("a cross-path call reached the tool: %+v", e)
		}
	}
}

func TestTokenPathIsNotServedWithoutATokenFile(t *testing.T) {
	server, logs := startFixture(t, config{})
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		header := http.Header{correlationHeader: {"inv-unserved"}, "Authorization": {"Bearer " + testToken}}
		if response, _ := send(t, method, server.URL+tokenPath, header, initializeBody); response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s: status %d, want 404", method, tokenPath, response.StatusCode)
		}
	}
	if entries := logs.entries(t); len(entries) != 0 {
		t.Fatalf("an unserved path was logged: %+v", entries)
	}
}

func TestLoadConfigReadsTheTokenFileAndKeepsOnlyItsDigest(t *testing.T) {
	load := func(path string) (config, error) {
		return loadConfig(func(key string) string {
			if key == "FIXTURE_TOKEN_FILE" {
				return path
			}
			return ""
		})
	}
	for _, token := range []string{testToken, "!", strings.Repeat("~", maxTokenBytes)} {
		cfg, err := load(writeTokenFile(t, token))
		want := sha256.Sum256([]byte(token))
		if err != nil || !bytes.Equal(cfg.tokenDigest, want[:]) {
			t.Fatalf("token of %d bytes: digest %x err=%v", len(token), cfg.tokenDigest, err)
		}
		if strings.Contains(fmt.Sprintf("%+v %#v", cfg, cfg), token) {
			t.Fatal("config holds the token itself")
		}
	}
	// Each rejected file holds a marker the error must not echo.
	const marker = "s3ntinelvalue"
	for name, path := range map[string]string{
		"missing file":     filepath.Join(t.TempDir(), "absent"),
		"directory":        t.TempDir(),
		"empty":            writeTokenFile(t, ""),
		"oversized":        writeTokenFile(t, marker+strings.Repeat("a", maxTokenBytes+1-len(marker))),
		"space":            writeTokenFile(t, marker+" "+marker),
		"tab":              writeTokenFile(t, marker+"\t"),
		"delete byte":      writeTokenFile(t, marker+"\x7f"),
		"non-ascii":        writeTokenFile(t, marker+"\u00e9"),
		"trailing newline": writeTokenFile(t, marker+"\n"),
		"trailing crlf":    writeTokenFile(t, marker+"\r\n"),
	} {
		cfg, err := load(path)
		if err == nil {
			t.Fatalf("%s: token file accepted", name)
		}
		if strings.Contains(err.Error(), marker) || cfg.tokenDigest != nil {
			t.Fatalf("%s: error %q or config %+v reveals the file", name, err, cfg)
		}
	}
}

func TestLogsNeverContainTheTokenOrItsDigest(t *testing.T) {
	server, logs := startTokenFixture(t)
	valid := connectAt(t, server.URL+tokenPath, "inv-scan-valid", "Bearer "+testToken)
	callRead(t, valid, "README.md")
	if err := valid.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	for correlation, authorization := range map[string]string{
		"inv-scan-missing": "",
		"inv-scan-wrong":   "Bearer " + wrongToken,
	} {
		if _, err := dial(t, server.URL+tokenPath, correlation, authorization); err == nil {
			t.Fatalf("%s: connect succeeded without the token", correlation)
		}
		// The real SDK client stops at its rejected initialize.
		entries := correlated(logs.entries(t), correlation)
		if len(entries) != 2 || entries[0].RPCMethod != "initialize" || entries[1].Status != http.StatusUnauthorized {
			t.Fatalf("%s: want one initialize answered 401, got %+v", correlation, entries)
		}
	}
	// /mcp logs a sent header's class but never the header.
	callRead(t, connectAt(t, server.URL+openPath, "inv-scan-open", "Bearer "+wrongToken), "README.md")
	logs.mu.Lock()
	raw := logs.buf.String()
	logs.mu.Unlock()
	for _, token := range []string{testToken, wrongToken} {
		sum := sha256.Sum256([]byte(token))
		digest := hex.EncodeToString(sum[:])
		for _, leak := range []string{token, digest, digest[:12], strings.ToUpper(digest)} {
			if strings.Contains(raw, leak) {
				t.Fatalf("log contains the token or its digest (%d characters)", len(leak))
			}
		}
	}
	if strings.Contains(raw, "Bearer") {
		t.Fatal("log contains an Authorization value")
	}
}
