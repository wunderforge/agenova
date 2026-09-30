// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	value string
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(correlationHeader, h.value)
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
	client := mcp.NewClient(&mcp.Implementation{Name: "fixture-test", Version: "0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: headerTransport{correlation}}, DisableStandaloneSSE: true, MaxRetries: -1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
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
	if err != nil || cfg.slowDelay != 10*time.Second || cfg.pod != "p" || cfg.addr != ":8080" {
		t.Fatalf("unexpected config %+v err=%v", cfg, err)
	}
}
