// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExistingMemberSessionAndInstallationAreTheOnlyAuthority(t *testing.T) {
	calls := 0
	issuer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cookies := r.Cookies()
		if len(cookies) != 1 || cookies[0].Name != "__Host-agenova-session" || r.Header.Get("Authorization") != "" || r.Header.Get("X-WEBAUTH-USER") != "" {
			t.Error("forwarded unrelated credentials")
		}
		if r.URL.Path == "/auth/me" {
			json.NewEncoder(w).Encode(v0.Principal{Subject: "trusted", Team: "team", AuthenticationContext: "oidc"})
			return
		}
		var input map[string]string
		json.NewDecoder(r.Body).Decode(&input)
		if r.URL.Path != "/api/platform/authorize" || input["action"] != "status" || input["installation"] != "installed" {
			t.Error("wrong authorization boundary")
		}
		w.WriteHeader(204)
	}))
	defer issuer.Close()
	b := memberBoundary{origin: issuer.URL, installation: "installed", client: issuer.Client()}
	r := httptest.NewRequest("GET", "https://app.example/grafana/", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-agenova-session", Value: "test-session"})
	r.AddCookie(&http.Cookie{Name: "other", Value: "private"})
	r.Header.Set("Authorization", "Bearer do-not-forward")
	r.Header.Set("X-WEBAUTH-USER", "spoofed")
	p, err := b.Authenticate(r)
	if err != nil || p.Subject != "trusted" || b.authorize(r, p) != nil || calls != 2 {
		t.Fatal("existing member boundary not used", err)
	}
	r.AddCookie(&http.Cookie{Name: "__Host-agenova-session", Value: "duplicate"})
	if _, err = b.Authenticate(r); err == nil || calls != 2 {
		t.Fatal("duplicate session accepted")
	}
}
