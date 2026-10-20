// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

type grafanaAuth struct{ denied atomic.Bool }

func (a *grafanaAuth) Authenticate(*http.Request) (v0.Principal, error) {
	if a.denied.Load() {
		return v0.Principal{}, errors.New("revoked")
	}
	return v0.Principal{Subject: "verified-user", Team: "team", AuthenticationContext: "oidc"}, nil
}
func TestGrafanaProxyReauthorizesAndIsolatesCredentials(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/grafana/api/ds/query" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Baggage") != "" || r.Header.Get("X-WEBAUTH-ROLE") != "Viewer" || r.Header.Get("X-WEBAUTH-USER") == "attacker" || r.Header.Get("X-Forwarded-Host") != "app.example" {
			t.Error("identity or credentials crossed proxy boundary", r.Header)
		}
		w.Header().Add("Set-Cookie", "grafana_session=must-not-persist")
		w.WriteHeader(200)
	}))
	defer upstream.Close()
	auth := &grafanaAuth{}
	var revoked atomic.Bool
	proxy, err := GrafanaProxy(upstream.URL, "https://app.example", auth, func(*http.Request, v0.Principal) error {
		if revoked.Load() {
			return errors.New("revoked role")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(origin, method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://app.example/grafana/api/ds/query", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("Authorization", "Bearer private")
		r.Header.Set("Cookie", "__Host-agenova-session=private")
		r.Header.Set("X-WEBAUTH-USER", "attacker")
		r.Header.Set("X-WEBAUTH-ROLE", "Admin")
		r.Header.Set("Baggage", "private")
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		return w
	}
	if w := request("https://app.example", "POST"); w.Code != 200 || len(w.Header().Values("Set-Cookie")) != 0 {
		t.Fatal("positive or cookie isolation failed", w.Code)
	}
	for _, origin := range []string{"https://evil.example", ""} {
		if w := request(origin, "POST"); w.Code != 403 {
			t.Fatal("cross-site write accepted")
		}
	}
	revoked.Store(true)
	if request("https://app.example", "POST").Code != 403 {
		t.Fatal("revoked role retained access")
	}
	revoked.Store(false)
	auth.denied.Store(true)
	if request("https://app.example", "POST").Code != 401 {
		t.Fatal("revoked session retained access")
	}
	if calls.Load() != 1 {
		t.Fatal("denied request reached Grafana")
	}
}
func TestGrafanaProxyRejectsUnconfiguredBoundary(t *testing.T) {
	for _, target := range []string{"https://external.example", "http://user:password@host", "http://host/path", "http://host?q=x"} {
		if _, err := GrafanaProxy(target, "https://app.example", nil, nil); err == nil {
			t.Fatal("unsafe target/boundary accepted")
		}
	}
}

func TestGrafanaProxyAgainstPinnedRealGrafana(t *testing.T) {
	target := os.Getenv("AGENOVA_TEST_GRAFANA_URL")
	if target == "" {
		t.Skip("requires disposable Grafana 11.4 auth-proxy backend")
	}
	auth := &grafanaAuth{}
	var revoked atomic.Bool
	proxy, err := GrafanaProxy(target, "https://app.example", auth, func(*http.Request, v0.Principal) error {
		if revoked.Load() {
			return errors.New("revoked")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	call := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "https://app.example/grafana"+path, nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		return w
	}
	user := call("/api/user")
	var identity struct {
		Login string `json:"login"`
		Admin bool   `json:"isGrafanaAdmin"`
	}
	if user.Code != 200 || json.Unmarshal(user.Body.Bytes(), &identity) != nil || !strings.HasPrefix(identity.Login, "agenova-") || identity.Admin {
		t.Fatalf("real Grafana identity failed: %d %s", user.Code, user.Body.String())
	}
	orgs := call("/api/user/orgs")
	var roles []struct {
		Role string `json:"role"`
	}
	if orgs.Code != 200 || json.Unmarshal(orgs.Body.Bytes(), &roles) != nil || len(roles) != 1 || roles[0].Role != "Viewer" {
		t.Fatal("real Grafana did not enforce Viewer", orgs.Code, orgs.Body.String())
	}
	revoked.Store(true)
	if call("/api/user").Code != 403 {
		t.Fatal("revoked role kept real Grafana access")
	}
}
