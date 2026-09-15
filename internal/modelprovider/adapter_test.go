// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const completion = `{"id":"synthetic-response-1","model":"llama3.1:latest","choices":[{"message":{"role":"assistant","content":"A task-dependent synthetic answer."}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`

func newTestAdapter(t *testing.T, cfg Config) *Adapter {
	t.Helper()
	if cfg.Models == nil {
		cfg.Models = map[string]string{"approved-local": "llama3.1:latest"}
	}
	a, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCompleteUsesPrivateProfileAndReturnsCompletion(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-test-key" {
			t.Error("missing synthetic authentication")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing JSON content type")
		}
		var payload struct {
			Model     string    `json:"model"`
			Messages  []message `json:"messages"`
			MaxTokens int       `json:"max_tokens"`
			Stream    bool      `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Model != "llama3.1:latest" || payload.MaxTokens != 64 || payload.Stream || len(payload.Messages) != 1 || payload.Messages[0].Role != "user" || payload.Messages[0].Content != "Summarize this synthetic task." {
			t.Errorf("unexpected request payload: %+v", payload)
		}
		_, _ = io.WriteString(w, completion)
	}))
	defer server.Close()
	models := map[string]string{"approved-local": "llama3.1:latest"}
	a := newTestAdapter(t, Config{Endpoint: server.URL + "/v1/", Models: models, APIKey: "synthetic-test-key", MaxTokens: 64})
	models["approved-local"] = "worker-nominated-replacement"
	models["unapproved"] = "llama3.1:latest"
	result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "Summarize this synthetic task."})
	if err != nil {
		t.Fatal(err)
	}
	if result != (Result{Text: "A task-dependent synthetic answer.", Model: "llama3.1:latest", ResponseID: "synthetic-response-1", InputTokens: 12, OutputTokens: 7}) {
		t.Errorf("unexpected result: %+v", result)
	}
	if _, err := a.Complete(context.Background(), Request{Profile: "unapproved", Prompt: "ignored"}); err == nil {
		t.Error("mutated allowlist accepted")
	}
	if calls.Load() != 1 {
		t.Errorf("got %d calls", calls.Load())
	}
}

func TestUnknownProfileAndInvalidPromptMakeZeroRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	a := newTestAdapter(t, Config{Endpoint: server.URL + "/v1"})
	for _, req := range []Request{
		{Profile: "llama3.1:latest", Prompt: "valid"},
		{Profile: "approved-local ", Prompt: "valid"},
		{Profile: "approved-local", Prompt: " "},
		{Profile: "approved-local", Prompt: strings.Repeat("x", maxPromptBytes+1)},
		{Profile: "approved-local", Prompt: strings.Repeat("\x00", maxPromptBytes)},
	} {
		if _, err := a.Complete(context.Background(), req); err == nil {
			t.Errorf("accepted invalid request with profile %q", req.Profile)
		}
	}
	if calls.Load() != 0 {
		t.Errorf("got %d unexpected calls", calls.Load())
	}
}

func TestStructuredOutputIsPrivateAndOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				format, exists := payload["response_format"]
				if exists != enabled {
					t.Errorf("structured output enabled=%v, present=%v", enabled, exists)
				}
				if enabled {
					var structured struct {
						Type   string `json:"type"`
						Schema struct {
							Strict bool            `json:"strict"`
							Schema json.RawMessage `json:"schema"`
						} `json:"json_schema"`
					}
					if json.Unmarshal(format, &structured) != nil || structured.Type != "json_schema" || !structured.Schema.Strict || string(structured.Schema.Schema) != `{"type":"object"}` {
						t.Errorf("unexpected output format: %s", format)
					}
				}
				_, _ = io.WriteString(w, completion)
			}))
			defer server.Close()
			cfg := Config{Endpoint: server.URL + "/v1"}
			if enabled {
				cfg.OutputSchema = []byte(`{"type":"object"}`)
			}
			a := newTestAdapter(t, cfg)
			if enabled {
				copy(cfg.OutputSchema, []byte(`{"type":"string"}`))
			}
			if _, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "Synthetic action."}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResponseFailuresAreSanitized(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"bad status", 401, "synthetic-test-key raw-private-prompt upstream-secret"},
		{"invalid JSON", 200, "synthetic-test-key raw-private-prompt upstream-secret"},
		{"trailing JSON", 200, completion + "{}"},
		{"missing choices", 200, `{}`},
		{"missing text", 200, `{"choices":[{"message":{"content":" "}}]}`},
		{"invalid text shape", 200, `{"choices":[{"message":{"content":{}}}]}`},
		{"negative usage", 200, `{"choices":[{"message":{"content":"answer"}}],"usage":{"prompt_tokens":-1}}`},
		{"oversized response", 200, strings.Repeat("x", maxResponseBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			a := newTestAdapter(t, Config{Endpoint: server.URL + "/v1", APIKey: "synthetic-test-key"})
			result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "raw-private-prompt"})
			if err == nil || result != (Result{}) {
				t.Fatalf("failure returned result=%+v error=%v", result, err)
			}
			for _, sensitive := range []string{"synthetic-test-key", "raw-private-prompt", "upstream-secret", server.URL} {
				if strings.Contains(err.Error(), sensitive) {
					t.Errorf("error disclosed %q", sensitive)
				}
			}
		})
	}
}

func TestLocalNeedsNoAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("unexpected local authentication")
		}
		_, _ = io.WriteString(w, completion)
	}))
	defer server.Close()
	if _, err := newTestAdapter(t, Config{Endpoint: server.URL + "/v1"}).Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"}); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledContextAndTimeout(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	a := newTestAdapter(t, Config{Endpoint: server.URL + "/v1", Timeout: 20 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Complete(ctx, Request{Profile: "approved-local", Prompt: "synthetic task"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("canceled request reached server")
	}
	if _, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout: %v", err)
	}
}

func TestRedirectNeverCallsTargetOrMutatesSuppliedClient(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1); _, _ = io.WriteString(w, completion) }))
	defer target.Close()
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/v1/chat/completions", status)
			}))
			defer server.Close()
			var redirects atomic.Int32
			client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { redirects.Add(1); return nil }}
			a := newTestAdapter(t, Config{Endpoint: server.URL + "/v1", APIKey: "synthetic-test-key", HTTPClient: client})
			if _, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"}); err == nil {
				t.Error("redirect accepted")
			}
			if redirects.Load() != 0 || client.Timeout != 3*time.Second {
				t.Error("supplied client was mutated or redirect callback used")
			}
		})
	}
	if targetCalls.Load() != 0 {
		t.Errorf("redirect target received %d calls", targetCalls.Load())
	}
}

func TestConfigurationValidation(t *testing.T) {
	for _, endpoint := range []string{"", "http://example.com/v1", "http://localhost:11434/v1", "ftp://127.0.0.1/v1", "https://synthetic:credential@example.com/v1", "https://example.com/v1?key=synthetic", "https://example.com/v1?", "https://example.com/v1#fragment", "http://[::1%25zone]:11434/v1"} {
		if _, err := New(Config{Endpoint: endpoint, Models: map[string]string{"approved": "local"}}); err == nil {
			t.Errorf("accepted invalid endpoint %q", endpoint)
		}
	}
	for _, endpoint := range []string{"https://example.com/v1", "http://127.0.0.1:11434/v1", "http://[::1]:11434/v1"} {
		if _, err := New(Config{Endpoint: endpoint, Models: map[string]string{"approved": "local"}}); err != nil {
			t.Errorf("rejected endpoint %q: %v", endpoint, err)
		}
	}
	for _, cfg := range []Config{
		{Models: map[string]string{}}, {Models: map[string]string{"": "local"}}, {Models: map[string]string{"approved": " "}},
		{MaxTokens: -1}, {MaxTokens: maxTokens + 1}, {Timeout: -time.Second}, {Timeout: maxTimeout + time.Second}, {APIKey: "synthetic\r\nkey"},
	} {
		cfg.Endpoint = "http://127.0.0.1:11434/v1"
		if cfg.Models == nil {
			cfg.Models = map[string]string{"approved": "local"}
		}
		if _, err := New(cfg); err == nil {
			t.Errorf("accepted invalid config (token limit %d, timeout %v)", cfg.MaxTokens, cfg.Timeout)
		}
	}
}

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("synthetic-key raw-private-prompt transport-secret")
}

func TestTransportErrorIsSanitized(t *testing.T) {
	a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", HTTPClient: &http.Client{Transport: errorTransport{}}})
	_, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "raw-private-prompt"})
	if err == nil || err.Error() != "model provider request failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}
