// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package login

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type memoryStore struct {
	saved   Credentials
	deleted bool
}

func (s *memoryStore) Get(string) (Credentials, error)   { return s.saved, nil }
func (s *memoryStore) Set(_ string, c Credentials) error { s.saved = c; return nil }
func (s *memoryStore) Delete(string) error               { s.deleted = true; return nil }
func TestNativeLoginPKCERefreshAndLogout(t *testing.T) {
	var server *httptest.Server
	challenge := ""
	exchanges := 0
	revocations := 0
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/cli-config":
			_ = json.NewEncoder(w).Encode(Config{Issuer: server.URL + "/issuer", ClientID: "native", AuthorizationEndpoint: server.URL + "/authorize", TokenEndpoint: server.URL + "/token", Scopes: []string{"openid", "agenova/api"}, RedirectURI: Callback})
		case "/token":
			if r.Method != "POST" {
				t.Error("token not POST")
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "authorization_code" {
				if r.Form.Get("code") != "one-time-code" || r.Form.Get("redirect_uri") != Callback || oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) != challenge {
					t.Error("PKCE/exact callback binding failed")
				}
			} else if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "private-refresh" {
				t.Error("bad refresh")
			}
			if r.Form.Get("client_id") != "native" {
				t.Error("client binding missing")
			}
			exchanges++
			_, _ = fmt.Fprintf(w, `{"access_token":"private-access-%d","refresh_token":"private-refresh","token_type":"Bearer","expires_in":3600}`, exchanges)
		case "/auth/me":
			if r.Header.Get("Authorization") != "Bearer private-access-1" {
				t.Error("member check missing")
			}
			_, _ = w.Write([]byte(`{}`))
		case "/auth/logout":
			if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer private-access-2" {
				t.Error("logout binding missing")
			}
			revocations++
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	store := &memoryStore{}
	client := &Client{Server: server.URL, Store: store, HTTPClient: server.Client(), OpenBrowser: func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		q := u.Query()
		if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "native" || len(q.Get("state")) < 40 {
			t.Error("unsafe authorization request")
		}
		if strings.Contains(raw, "private-") {
			t.Error("token in browser URL")
		}
		challenge = q.Get("code_challenge")
		bad, err := http.Get(Callback + "?state=wrong&code=attacker")
		if err != nil {
			return err
		}
		bad.Body.Close()
		if bad.StatusCode != 400 {
			t.Error("state forgery accepted")
		}
		good, err := http.Get(Callback + "?state=" + url.QueryEscape(q.Get("state")) + "&code=one-time-code")
		if err != nil {
			return err
		}
		good.Body.Close()
		if good.StatusCode != 200 {
			t.Error("callback rejected")
		}
		return nil
	}}
	if err := client.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.saved.Token.RefreshToken != "private-refresh" {
		t.Fatal("refresh not saved")
	}
	store.saved.Token.Expiry = time.Now().Add(-time.Minute)
	if token, err := client.AccessToken(context.Background()); err != nil || token != "private-access-2" {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := client.Logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.deleted || revocations != 1 || exchanges != 2 {
		t.Fatal("logout/refresh did not complete")
	}
}
func TestHTTPSAndRedirectIsolation(t *testing.T) {
	for _, server := range []string{"http://example.com", "https://user:password@example.com", "https://example.com/path", "https://example.com?x=y", "https://example.com#"} {
		if Origin(server) {
			t.Fatalf("accepted %s", server)
		}
	}
	leaked := 0
	dest := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++ }))
	defer dest.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 307) }))
	defer source.Close()
	client := &Client{Server: source.URL, HTTPClient: source.Client()}
	if err := client.check(context.Background(), "private-access", "/auth/me", "GET"); err == nil {
		t.Fatal("redirect accepted")
	}
	if leaked != 0 {
		t.Fatal("bearer leaked")
	}
}
