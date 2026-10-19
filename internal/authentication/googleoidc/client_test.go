// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package googleoidc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestServerLoginExchange(t *testing.T) {
	c, err := NewClient(clientID, "private-test-secret", "https://app.example/auth/callback")
	if err != nil {
		t.Fatal(err)
	}
	v, k, _ := fixture(t)
	c.verifier = v
	state := strings.Repeat("s", 43)
	n := strings.Repeat("n", 43)
	pkce := oauth2.GenerateVerifier()
	auth, err := c.AuthorizationURL(state, n, pkce)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(auth)
	q := u.Query()
	if u.Scheme+"://"+u.Host+u.Path != authorizationURL || q.Get("scope") != "openid email" || q.Get("access_type") != "" || q.Get("client_secret") != "" || q.Get("nonce") != n || q.Get("state") != state || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != oauth2.S256ChallengeFromVerifier(pkce) {
		t.Fatal("incorrect Google authorization request")
	}
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != tokenURL || r.Method != "POST" {
			t.Fatal("unpinned token endpoint")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("code_verifier") != pkce || r.Form.Get("client_secret") != "private-test-secret" || r.Form.Get("redirect_uri") != "https://app.example/auth/callback" {
			t.Fatal("missing exchange binding")
		}
		claims := validClaims()
		claims["nonce"] = n
		b, _ := json.Marshal(map[string]any{"access_token": "discard-google-token", "token_type": "Bearer", "expires_in": 3600, "id_token": signed(t, k, claims, nil)})
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})
	id, err := c.Exchange(context.Background(), "one-time-code", pkce, n)
	if err != nil || id.Subject != "google-person-a" {
		t.Fatal("exchange failed", err)
	}
	if _, err := c.Exchange(context.Background(), "one-time-code", pkce, strings.Repeat("x", 43)); err == nil {
		t.Fatal("nonce mismatch accepted")
	}
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("private-test-secret discard-google-token"))}, nil
	})
	if _, err := c.Exchange(context.Background(), "code", pkce, n); err == nil || strings.Contains(err.Error(), "private-test-secret") {
		t.Fatal("exchange error leaked")
	}
}

func TestLoginConfigurationAndBinding(t *testing.T) {
	for _, uri := range []string{"http://app.example/auth/callback", "https://evil@app.example/auth/callback", "https://app.example/auth/callback?next=evil", "https://app.example/other"} {
		if _, err := NewClient(clientID, "secret", uri); err == nil {
			t.Fatal("bad callback accepted")
		}
	}
	c, _ := NewClient(clientID, "secret", "https://app.example/auth/callback")
	h := strings.Repeat("x", 43)
	if _, err := c.AuthorizationURL(h, h, oauth2.GenerateVerifier()); err == nil {
		t.Fatal("state and nonce reused")
	}
	if _, err := c.AuthorizationURL("short", h, oauth2.GenerateVerifier()); err == nil {
		t.Fatal("weak state")
	}
	if _, err := c.AuthorizationURL(strings.Repeat("s", 43), h, "invalid"); err == nil {
		t.Fatal("invalid PKCE")
	}
}
