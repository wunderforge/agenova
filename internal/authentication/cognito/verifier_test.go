// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cognito

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

const issuer = "https://cognito-idp.ap-southeast-2.amazonaws.com/ap-southeast-2_Test"

func fixture(t *testing.T) (*Verifier, *rsa.PrivateKey, *atomic.Int32) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(Config{Issuer: issuer, ClientIDs: []string{"native"}, RequiredScope: "agenova/work"})
	if err != nil {
		t.Fatal(err)
	}
	calls := &atomic.Int32{}
	v.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != issuer+"/.well-known/jwks.json" {
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
	return jwt.MapClaims{"iss": issuer, "sub": "person-a", "client_id": "native", "token_use": "access", "scope": "openid agenova/work", "auth_time": time.Now().Add(-2 * time.Minute).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Minute).Unix(), "jti": "token-a", "origin_jti": "session-a"}
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
func TestCognitoAccessToken(t *testing.T) {
	v, key, calls := fixture(t)
	c := validClaims()
	c["team"] = "forged-admin"
	c["cognito:groups"] = []string{"admin"}
	id, err := v.Verify(context.Background(), signed(t, key, c, nil))
	if err != nil {
		t.Fatal(err)
	}
	if id.Issuer != issuer || id.Subject != "person-a" || id.SessionID != "session-a" {
		t.Fatalf("wrong private identity: %+v", id)
	}
	_, err = v.Verify(context.Background(), signed(t, key, validClaims(), nil))
	if err != nil || calls.Load() != 1 {
		t.Fatal("known key was not cached", err, calls.Load())
	}
}
func TestRejectInvalidTokens(t *testing.T) {
	v, key, _ := fixture(t)
	cases := map[string]func(jwt.MapClaims){
		"id token":             func(c jwt.MapClaims) { c["token_use"] = "id"; c["aud"] = "native" },
		"wrong client":         func(c jwt.MapClaims) { c["client_id"] = "attacker" },
		"wrong issuer":         func(c jwt.MapClaims) { c["iss"] = "https://attacker.example" },
		"expired":              func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"missing expiry":       func(c jwt.MapClaims) { delete(c, "exp") },
		"missing issued at":    func(c jwt.MapClaims) { delete(c, "iat") },
		"future issued at":     func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Hour).Unix() },
		"future not before":    func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Hour).Unix() },
		"missing subject":      func(c jwt.MapClaims) { delete(c, "sub") },
		"wrong scope":          func(c jwt.MapClaims) { c["scope"] = "agenova/work-more" },
		"no token id":          func(c jwt.MapClaims) { delete(c, "jti") },
		"no revocable session": func(c jwt.MapClaims) { delete(c, "origin_jti") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			c := validClaims()
			change(c)
			raw := signed(t, key, c, nil)
			id, err := v.Verify(context.Background(), raw)
			if err == nil || id.Subject != "" {
				t.Fatal("invalid identity accepted")
			}
			if strings.Contains(err.Error(), raw) {
				t.Fatal("token leaked")
			}
		})
	}
	for _, h := range []map[string]any{{"jku": "https://attacker.example"}, {"jwk": map[string]any{}}, {"crit": []string{"unknown"}}, {"kid": "unknown"}, {"kid": 17}} {
		if _, err := v.Verify(context.Background(), signed(t, key, validClaims(), h)); err == nil {
			t.Fatal("unsupported header accepted")
		}
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := v.Verify(context.Background(), signed(t, other, validClaims(), nil)); err == nil {
		t.Fatal("invalid signature accepted")
	}
	for _, raw := range []string{"", "a.b.c", strings.Repeat("x", (16<<10)+1)} {
		if _, err := v.Verify(context.Background(), raw); err == nil {
			t.Fatal("malformed accepted")
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	token.Header["kid"] = "key1"
	raw, _ := token.SignedString([]byte("secret"))
	if _, err := v.Verify(context.Background(), raw); err == nil {
		t.Fatal("algorithm confusion")
	}
}
func TestAudienceAndKeyRefreshFailClosed(t *testing.T) {
	v, key, calls := fixture(t)
	v.config.Audience = "https://api.agenova.app"
	if _, err := v.Verify(context.Background(), signed(t, key, validClaims(), nil)); err == nil {
		t.Fatal("missing resource audience accepted")
	}
	c := validClaims()
	c["aud"] = "https://api.agenova.app"
	if _, err := v.Verify(context.Background(), signed(t, key, c, nil)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		_, _ = v.Verify(context.Background(), signed(t, key, c, map[string]any{"kid": "unknown"}))
	}
	if calls.Load() != 1 {
		t.Fatal("unknown key causes network flood")
	}
	v.expires = time.Now().Add(-time.Second)
	v.lastAttempt = time.Now().Add(-time.Minute)
	v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})
	if _, err := v.Verify(context.Background(), signed(t, key, c, nil)); err == nil {
		t.Fatal("stale keys accepted during outage")
	}
	v.lastAttempt = time.Now().Add(-time.Minute)
	v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(jwks(key, "key2")))}, nil
	})
	if _, err := v.Verify(context.Background(), signed(t, key, c, map[string]any{"kid": "key2"})); err != nil {
		t.Fatal("rotated key rejected", err)
	}
}
func TestBadJWKSAndCancellation(t *testing.T) {
	for _, body := range []string{`{"keys":[]}`, `{"keys":[{"kty":"RSA"}]}`, strings.Repeat("x", (64<<10)+1)} {
		v, key, _ := fixture(t)
		v.client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if _, err := v.Verify(context.Background(), signed(t, key, validClaims(), nil)); err == nil {
			t.Fatal("invalid JWKS accepted")
		}
	}
	v, key, calls := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := v.Verify(ctx, signed(t, key, validClaims(), nil)); err == nil || calls.Load() != 0 {
		t.Fatal("cancellation not respected")
	}
}
func TestExplicitConfiguration(t *testing.T) {
	for _, url := range []string{"http://localhost/pool", issuer + "?x=1", issuer + "/", "https://cognito-idp.us-east-1.amazonaws.com/ap-southeast-2_Test"} {
		if _, err := New(Config{Issuer: url, ClientIDs: []string{"native"}, RequiredScope: "agenova/work"}); err == nil {
			t.Fatal("bad issuer accepted")
		}
	}
}
