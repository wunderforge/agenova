// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package googleoidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const issuer = Issuer
const clientID = "123-test.apps.googleusercontent.com"
const nonce = "01234567890123456789012345678901ab"

func fixture(t *testing.T) (*Verifier, *rsa.PrivateKey, *atomic.Int32) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(clientID)
	if err != nil {
		t.Fatal(err)
	}
	calls := &atomic.Int32{}
	v.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != keysURL {
			t.Errorf("unexpected key URL: %s", r.URL)
		}
		body := jwks(key, "key1")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	return v, key, calls
}
func jwks(k *rsa.PrivateKey, kid string) string {
	b, _ := json.Marshal(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes())}}})
	return string(b)
}
func validClaims() jwt.MapClaims {
	return jwt.MapClaims{"iss": issuer, "sub": "google-person-a", "aud": clientID, "azp": clientID, "nonce": nonce, "email": "member@gmail.com", "email_verified": true, "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix()}
}
func signed(t *testing.T, key *rsa.PrivateKey, c jwt.MapClaims, header map[string]any) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	token.Header["kid"] = "key1"
	for k, v := range header {
		token.Header[k] = v
	}
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestGoogleLogin(t *testing.T) {
	v, k, calls := fixture(t)
	for _, iss := range []string{Issuer, "accounts.google.com"} {
		c := validClaims()
		c["iss"] = iss
		c["role"] = "Owner"
		c["team"] = "forged"
		id, err := v.VerifyLogin(context.Background(), signed(t, k, c, nil), nonce)
		if err != nil || id.Issuer != Issuer || id.Subject != "google-person-a" || !id.EmailAuthoritative {
			t.Fatalf("login: %+v %v", id, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("key cache missed")
	}
	c := validClaims()
	c["email"] = "member@external.example"
	id, err := v.VerifyLogin(context.Background(), signed(t, k, c, nil), nonce)
	if err != nil || id.EmailAuthoritative {
		t.Fatal("third-party Google email treated as authoritative")
	}
	c["hd"] = "external.example"
	id, err = v.VerifyLogin(context.Background(), signed(t, k, c, nil), nonce)
	if err != nil || !id.EmailAuthoritative {
		t.Fatal("verified Workspace domain rejected")
	}
}
func TestRejectedLoginAssertions(t *testing.T) {
	v, k, _ := fixture(t)
	cases := map[string]func(jwt.MapClaims){
		"issuer":                      func(c jwt.MapClaims) { c["iss"] = "https://attacker.example" },
		"audience":                    func(c jwt.MapClaims) { c["aud"] = "other.apps.googleusercontent.com" },
		"authorized party":            func(c jwt.MapClaims) { c["azp"] = "other.apps.googleusercontent.com" },
		"multiple audiences no party": func(c jwt.MapClaims) { c["aud"] = []string{clientID, "other"}; delete(c, "azp") },
		"nonce":                       func(c jwt.MapClaims) { c["nonce"] = "different" },
		"missing nonce":               func(c jwt.MapClaims) { delete(c, "nonce") },
		"expired":                     func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Second).Unix() },
		"no expiry":                   func(c jwt.MapClaims) { delete(c, "exp") },
		"no issued at":                func(c jwt.MapClaims) { delete(c, "iat") },
		"future":                      func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Hour).Unix() },
		"stale":                       func(c jwt.MapClaims) { c["iat"] = time.Now().Add(-11 * time.Minute).Unix() },
		"subject":                     func(c jwt.MapClaims) { delete(c, "sub") },
		"email unverified":            func(c jwt.MapClaims) { c["email_verified"] = false },
		"email type":                  func(c jwt.MapClaims) { c["email_verified"] = "true" },
		"email missing":               func(c jwt.MapClaims) { delete(c, "email") },
		"email display":               func(c jwt.MapClaims) { c["email"] = "Member <member@gmail.com>" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			c := validClaims()
			change(c)
			raw := signed(t, k, c, nil)
			id, err := v.VerifyLogin(context.Background(), raw, nonce)
			if err == nil || id.Subject != "" || strings.Contains(err.Error(), raw) {
				t.Fatal("bad assertion accepted or leaked")
			}
		})
	}
	for _, header := range []map[string]any{{"jku": "https://evil.example/keys"}, {"crit": []string{"x"}}, {"kid": "unknown"}} {
		if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), header), nonce); err == nil {
			t.Fatal("header accepted")
		}
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := v.VerifyLogin(context.Background(), signed(t, other, validClaims(), nil), nonce); err == nil {
		t.Fatal("invalid signature accepted")
	}
	raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims()).SignedString([]byte("secret"))
	if _, err := v.VerifyLogin(context.Background(), raw, nonce); err == nil {
		t.Fatal("algorithm confusion")
	}
	for _, n := range []string{"", "short", strings.Repeat("x", 257)} {
		if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), nil), n); err == nil {
			t.Fatal("unbound nonce")
		}
	}
}
func TestKeysBoundedAndFailClosed(t *testing.T) {
	v, k, calls := fixture(t)
	if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), nil), nonce); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		_, _ = v.VerifyLogin(context.Background(), signed(t, k, validClaims(), map[string]any{"kid": "unknown"}), nonce)
	}
	if calls.Load() != 1 {
		t.Fatal("unknown key network flood")
	}
	v.expires = time.Now().Add(-time.Second)
	v.lastAttempt = time.Now().Add(-time.Minute)
	v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("down"))}, nil
	})
	if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), nil), nonce); err == nil {
		t.Fatal("expired keys used")
	}
	v.lastAttempt = time.Now().Add(-time.Minute)
	v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(jwks(k, "key2")))}, nil
	})
	if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), map[string]any{"kid": "key2"}), nonce); err != nil {
		t.Fatal("rotation rejected", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := v.VerifyLogin(ctx, signed(t, k, validClaims(), nil), nonce); err == nil {
		t.Fatal("cancel ignored")
	}
}

func TestMalformedKeysAndConfiguration(t *testing.T) {
	for _, body := range []string{`{"keys":[]}`, `{"keys":[{"kty":"RSA"}]}`, strings.Repeat("x", (64<<10)+1)} {
		v, k, _ := fixture(t)
		v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if _, err := v.VerifyLogin(context.Background(), signed(t, k, validClaims(), nil), nonce); err == nil {
			t.Fatal("malformed key set accepted")
		}
	}
	for _, id := range []string{"", ".apps.googleusercontent.com", "attacker.example", "x y.apps.googleusercontent.com"} {
		if _, err := New(id); err == nil {
			t.Fatal("invalid client ID accepted")
		}
	}
}
