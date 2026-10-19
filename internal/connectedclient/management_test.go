// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package connectedclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagementHTTPSAuthorizationAndRedirectIsolation(t *testing.T) {
	calls := 0
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; t.Error("credential followed redirect") }))
	defer remote.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-test" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing authenticated JSON")
		}
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("bad body")
		}
		if r.URL.Path == "/api/platform/authorize" {
			if body["installation"] != "sydney" || body["action"] != "apply" {
				t.Error(body)
			}
			http.Redirect(w, r, remote.URL, 307)
			return
		}
		if body["project"] != "payments" || body["kind"] != "policy" || body["document"] != "document" {
			t.Error(body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"kind":"PolicyBundle","name":"test","changed":true,"active":true}`))
	}))
	defer server.Close()
	c := Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "private-test", nil }}
	result, err := c.ApplyGovernance("payments", "policy", "document")
	if err != nil || !result.Active {
		t.Fatal(result, err)
	}
	err = c.AuthorizePlatform("sydney", "apply")
	if err == nil || strings.Contains(err.Error(), "private-test") || calls != 0 {
		t.Fatal("redirect failure", err, calls)
	}
}
