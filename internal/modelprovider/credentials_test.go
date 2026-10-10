// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wunderforge/agenova/internal/credentials"
)

type credentialSource struct {
	mu      sync.Mutex
	value   []byte
	err     error
	calls   int
	buffers [][]byte
}

func (s *credentialSource) Resolve(context.Context, credentials.Reference) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	value := bytes.Clone(s.value)
	s.buffers = append(s.buffers, value)
	return value, s.err
}

func (s *credentialSource) replace(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = []byte(value)
}

func (s *credentialSource) assertCallsAndCleared(t *testing.T, calls int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls != calls {
		t.Fatalf("resolver calls = %d, want %d", s.calls, calls)
	}
	for _, value := range s.buffers {
		for _, b := range value {
			if b != 0 {
				t.Fatal("resolver-owned buffer was not cleared")
			}
		}
	}
}

func bindCredential(t *testing.T, resolver credentials.Resolver) *credentials.Binding {
	t.Helper()
	ref := credentials.Reference{Resolver: "agenova.io/credential/test", Name: "selected", Key: "token"}
	registry, err := credentials.New([]credentials.Registration{{ID: ref.Resolver, Version: "0.1.0", Resolver: resolver, Allowed: []credentials.Reference{ref}}})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := registry.Bind(ref)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

type credentialTransport func(*http.Request) (*http.Response, error)

func (f credentialTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func completionResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(completion))}
}

func TestCredentialIsResolvedPerCallAndRemovedFromRequest(t *testing.T) {
	source := &credentialSource{value: []byte("synthetic-first")}
	var requests []*http.Request
	var expected string
	transport := credentialTransport(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		deadline, bounded := req.Context().Deadline()
		if req.Context().Err() != nil || !bounded || time.Until(deadline) <= 2*credentials.CallTimeout {
			t.Error("resolver deadline shortened or canceled the provider invocation")
		}
		if req.Header.Get("Authorization") != "Bearer "+expected {
			t.Error("transport did not receive the current selected credential")
		}
		return completionResponse(), nil
	})
	binding := bindCredential(t, source)
	a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: binding, HTTPClient: &http.Client{Transport: transport}})
	for _, replacement := range []string{"synthetic-first", "synthetic-second"} {
		expected = replacement
		source.replace(replacement)
		result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"})
		if err != nil || result.Text != "A task-dependent synthetic answer." {
			t.Fatalf("completion failed: %v", err)
		}
		encoded, err := json.Marshal(result)
		if err != nil || bytes.Contains(encoded, []byte(replacement)) {
			t.Fatal("model result disclosed resolved material")
		}
		if a.apiKey != "" || a.credential != binding {
			t.Fatal("resolved material changed persistent adapter configuration")
		}
	}
	if len(requests) != 2 {
		t.Fatalf("HTTP calls = %d, want 2", len(requests))
	}
	for _, req := range requests {
		if req.Header.Get("Authorization") != "" {
			t.Fatal("authorization remained on an invocation request after return")
		}
	}
	source.assertCallsAndCleared(t, 2)
}

func TestCredentialResolutionFailuresMakeZeroHTTPCalls(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value []byte
		err   error
	}{
		{name: "unavailable", value: []byte("synthetic-private"), err: errors.New("synthetic-private backend existence details")},
		{name: "missing"},
		{name: "oversized", value: bytes.Repeat([]byte{'x'}, credentials.MaxValueBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &credentialSource{value: tc.value, err: tc.err}
			calls := 0
			a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(*http.Request) (*http.Response, error) {
				calls++
				return completionResponse(), nil
			})}})
			result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"})
			if result != (Result{}) || err != credentials.ErrUnavailable || calls != 0 {
				t.Fatalf("resolution failure: result=%+v error=%v HTTP calls=%d", result, err, calls)
			}
			source.assertCallsAndCleared(t, 1)
		})
	}
}

func TestInvalidBearerMaterialMakesZeroHTTPCalls(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"CRLF", "synthetic\r\nInjected:value"}, {"NUL", "synthetic\x00value"}, {"tab", "synthetic\tvalue"},
		{"space", "synthetic value"}, {"DEL", "synthetic\x7fvalue"}, {"non-ASCII", "syntheticé"},
		{"padding only", "="}, {"embedded padding", "a=b"}, {"invalid symbol", "a:b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &credentialSource{value: []byte(tc.value)}
			calls := 0
			a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(*http.Request) (*http.Response, error) {
				calls++
				return completionResponse(), nil
			})}})
			result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"})
			if result != (Result{}) || err == nil || err.Error() != "model provider credentials are invalid" || calls != 0 {
				t.Fatalf("invalid material: error=%v HTTP calls=%d", err, calls)
			}
			source.assertCallsAndCleared(t, 1)
		})
	}
	for _, valid := range []string{"a", "a-z.A_Z~+/==", "123"} {
		if !validBearerMaterial([]byte(valid)) {
			t.Fatal("valid Bearer grammar rejected")
		}
	}
}

func TestCredentialRejectedRequestsDoNotResolve(t *testing.T) {
	source := &credentialSource{value: []byte("synthetic-private")}
	calls := 0
	a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return completionResponse(), nil
	})}})
	for _, req := range []Request{
		{Profile: "unknown", Prompt: "valid"},
		{Profile: "approved-local", Prompt: " "},
		{Profile: "approved-local", Prompt: strings.Repeat("x", maxPromptBytes+1)},
		{Profile: "approved-local", Prompt: "valid", OutputSchema: []byte("{")},
		{Profile: "approved-local", Prompt: "valid", OutputSchema: bytes.Repeat([]byte{'x'}, 8193)},
	} {
		if _, err := a.Complete(context.Background(), req); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Complete(ctx, Request{Profile: "approved-local", Prompt: "valid"}); err != context.Canceled {
		t.Fatalf("caller cancellation lost: %v", err)
	}
	if _, err := a.Complete(nil, Request{Profile: "approved-local", Prompt: "valid"}); err == nil {
		t.Fatal("missing context accepted")
	}
	if calls != 0 {
		t.Fatalf("HTTP calls = %d, want zero", calls)
	}
	source.assertCallsAndCleared(t, 0)
}

func TestCredentialProviderErrorsRemainSanitized(t *testing.T) {
	for _, tc := range []struct {
		name      string
		response  *http.Response
		err       error
		wantError string
	}{
		{name: "transport", err: errors.New("synthetic-private raw-private-prompt transport-secret"), wantError: "model provider request failed"},
		{name: "status", response: &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("synthetic-private upstream-secret"))}, wantError: "model provider returned an unsuccessful status"},
		{name: "invalid JSON", response: &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("synthetic-private upstream-secret"))}, wantError: "model provider response is invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &credentialSource{value: []byte("synthetic-private")}
			var request *http.Request
			a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(req *http.Request) (*http.Response, error) {
				request = req
				return tc.response, tc.err
			})}})
			result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "raw-private-prompt"})
			if result != (Result{}) || err == nil || err.Error() != tc.wantError {
				t.Fatalf("provider error was changed or disclosed: %v", err)
			}
			if request == nil || request.Header.Get("Authorization") != "" {
				t.Fatal("authorization remained on a failed invocation request")
			}
			source.assertCallsAndCleared(t, 1)
		})
	}
}

func TestCredentialPreservesCallerCancellationAndProviderTimeout(t *testing.T) {
	for _, callerCancels := range []bool{true, false} {
		t.Run(map[bool]string{true: "caller cancellation", false: "provider child deadline"}[callerCancels], func(t *testing.T) {
			source := &credentialSource{value: []byte("synthetic-private")}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var request *http.Request
			a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), Timeout: 10 * time.Millisecond, HTTPClient: &http.Client{Transport: credentialTransport(func(req *http.Request) (*http.Response, error) {
				request = req
				if callerCancels {
					cancel()
				}
				<-req.Context().Done()
				return nil, errors.New("synthetic-private transport cancellation")
			})}})
			want := context.DeadlineExceeded
			if callerCancels {
				want = context.Canceled
			}
			result, err := a.Complete(ctx, Request{Profile: "approved-local", Prompt: "synthetic task"})
			if result != (Result{}) || err != want {
				t.Fatalf("typed terminal error lost: %v", err)
			}
			if !callerCancels && ctx.Err() != nil {
				t.Fatal("provider timeout terminated caller context")
			}
			if request == nil || request.Header.Get("Authorization") != "" {
				t.Fatal("authorization remained after terminal invocation")
			}
			source.assertCallsAndCleared(t, 1)
		})
	}
}

func TestCredentialConfigurationRejectsLegacyKeyConflict(t *testing.T) {
	source := &credentialSource{value: []byte("synthetic-private")}
	_, err := New(Config{Endpoint: "https://example.com/v1", Models: map[string]string{"approved-local": "llama3.1:latest"}, Credential: bindCredential(t, source), APIKey: "legacy-synthetic-private"})
	if err == nil || err.Error() != "model provider credentials are invalid" {
		t.Fatalf("conflicting credential sources accepted: %v", err)
	}
	source.assertCallsAndCleared(t, 0)
}

func TestUninitializedCredentialMakesZeroHTTPCalls(t *testing.T) {
	calls := 0
	a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: &credentials.Binding{}, HTTPClient: &http.Client{Transport: credentialTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return completionResponse(), nil
	})}})
	result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "valid"})
	if result != (Result{}) || err != credentials.ErrRejected || calls != 0 {
		t.Fatalf("uninitialized binding: error=%v HTTP calls=%d", err, calls)
	}
}

func TestCredentialConcurrentInvocationsHaveIndependentRequestHeaders(t *testing.T) {
	source := &credentialSource{value: []byte("synthetic-private")}
	var mu sync.Mutex
	var requests []*http.Request
	a := newTestAdapter(t, Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer synthetic-private" {
			t.Error("concurrent request omitted selected credential")
		}
		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()
		return completionResponse(), nil
	})}})
	const count = 12
	var finished sync.WaitGroup
	finished.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer finished.Done()
			if _, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "valid"}); err != nil {
				t.Errorf("concurrent invocation failed: %v", err)
			}
		}()
	}
	finished.Wait()
	if len(requests) != count {
		t.Fatalf("HTTP calls = %d, want %d", len(requests), count)
	}
	seen := map[*http.Request]bool{}
	for _, req := range requests {
		if seen[req] || req.Header.Get("Authorization") != "" {
			t.Fatal("concurrent invocation retained or shared an authorization request")
		}
		seen[req] = true
	}
	source.assertCallsAndCleared(t, count)
}

func TestProviderCannotEchoResolvedCredentialIntoResults(t *testing.T) {
	for _, field := range []string{"text", "response-id", "configured-model"} {
		t.Run(field, func(t *testing.T) {
			source := &credentialSource{value: []byte("synthetic-private-token")}
			resultJSON := `{"id":"response","choices":[{"message":{"role":"assistant","content":"safe completion"}}]}`
			if field == "text" {
				resultJSON = strings.Replace(resultJSON, "safe completion", "synthetic-private-token", 1)
			}
			if field == "response-id" {
				resultJSON = strings.Replace(resultJSON, "response", "synthetic-private-token", 1)
			}
			cfg := Config{Endpoint: "https://example.com/v1", Credential: bindCredential(t, source), HTTPClient: &http.Client{Transport: credentialTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(resultJSON))}, nil
			})}}
			if field == "configured-model" {
				cfg.Models = map[string]string{"approved-local": "synthetic-private-token"}
			}
			a := newTestAdapter(t, cfg)
			result, err := a.Complete(context.Background(), Request{Profile: "approved-local", Prompt: "synthetic task"})
			if result != (Result{}) || err == nil || strings.Contains(err.Error(), "synthetic-private-token") {
				t.Fatalf("provider echo reached public result: error=%v", err)
			}
			source.assertCallsAndCleared(t, 1)
		})
	}
}
