// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/modelprovider"
)

// The provider credential is configured on the host adapter. The worker only
// receives a claim-scoped Agenova token and cannot nominate this bearer value.
func TestGovernedWorkUsesHostProviderTokenOnlyAfterAllow(t *testing.T) {
	const hostToken = "synthetic-host-provider-token"
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer "+hostToken {
			t.Errorf("provider request lacked host authentication or used wrong route")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"provider-1","model":"controlled-model","choices":[{"message":{"role":"assistant","content":"Controlled answer"}}]}`)
	}))
	defer provider.Close()
	client, err := modelprovider.New(modelprovider.Config{
		Endpoint: provider.URL + "/v1",
		Models:   map[string]string{"approved-coding-model": "controlled-model"},
		APIKey:   hostToken,
	})
	if err != nil {
		t.Fatal(err)
	}

	denied, err := NewService(&verticalBackend{}, &verticalExecutor{}, client, app.ReferencePrincipalTeamB)
	if err != nil {
		t.Fatal(err)
	}
	defer denied.Close()
	if _, err := denied.Submit(verticalRequest(t, "provider-denied")); err != nil {
		t.Fatal(err)
	}
	deniedView := awaitVertical(t, denied, "provider-denied")
	if deniedView.State == nil || deniedView.State.Claim != nil || calls.Load() != 0 {
		t.Fatalf("denied request allocated claim or reached provider: calls=%d", calls.Load())
	}

	allowed, err := NewService(&verticalBackend{}, &verticalExecutor{}, client, app.ReferencePrincipalTeamA)
	if err != nil {
		t.Fatal(err)
	}
	defer allowed.Close()
	if _, err := allowed.Submit(verticalRequest(t, "provider-allowed")); err != nil {
		t.Fatal(err)
	}
	allowedView := awaitVertical(t, allowed, "provider-allowed")
	if allowedView.Outcome == nil || allowedView.Outcome.Status != "Succeeded" || calls.Load() != 1 {
		t.Fatalf("allowed Work did not use authenticated provider exactly once: outcome=%+v calls=%d", allowedView.Outcome, calls.Load())
	}
	for _, view := range []any{deniedView, allowedView} {
		data, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), hostToken) {
			t.Fatal("host provider token leaked into public evidence")
		}
	}
}
