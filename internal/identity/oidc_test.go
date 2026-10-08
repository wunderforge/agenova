// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type oidcFixture struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	other  *rsa.PrivateKey
	now    time.Time
}

func newOIDCFixture(t *testing.T) *oidcFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &oidcFixture{key: key, other: other, now: time.Unix(1_800_000_000, 0).UTC()}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk("primary", &key.PublicKey)}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *oidcFixture) verifier(t *testing.T) *OIDCVerifier {
	t.Helper()
	verifier, err := NewOIDCVerifier(OIDCConfig{
		Issuer: "https://identity.example.test", Audience: "agenova", JWKSURL: f.server.URL + "/keys",
		TeamClaim: "groups", TeamMappings: map[string]string{"engineering": "team-a", "support": "team-b", "eng-readonly": "team-a"},
		Clock: func() time.Time { return f.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func (f *oidcFixture) claims(subject string, groups any) map[string]any {
	return map[string]any{
		"iss": "https://identity.example.test", "aud": []string{"other", "agenova"}, "sub": subject,
		"exp": f.now.Add(time.Hour).Unix(), "nbf": f.now.Add(-time.Minute).Unix(), "acr": "company-mfa", "groups": groups,
	}
}

func TestOIDCVerifierAcceptsTwoMappedUsers(t *testing.T) {
	f := newOIDCFixture(t)
	verifier := f.verifier(t)
	for _, test := range []struct {
		subject string
		group   string
		team    string
	}{{"user-a", "engineering", "team-a"}, {"user-b", "support", "team-b"}} {
		t.Run(test.subject, func(t *testing.T) {
			principal, err := verifier.Verify(context.Background(), sign(t, f.key, "primary", f.claims(test.subject, test.group)))
			if err != nil {
				t.Fatal(err)
			}
			if principal.Subject != test.subject || principal.Team != test.team || principal.AuthenticationContext != "company-mfa" || principal.Issuer != "https://identity.example.test" {
				t.Fatalf("unexpected principal: %+v", principal)
			}
			public := principal.Principal()
			if public.Subject != test.subject || public.Team != test.team || public.AuthenticationContext != "company-mfa" {
				t.Fatalf("unexpected public projection: %+v", public)
			}
			if principal.OwnerKey() == "" || strings.Contains(principal.OwnerKey(), test.subject) {
				t.Fatalf("owner key is not opaque: %q", principal.OwnerKey())
			}
		})
	}
}

func TestOIDCVerifierRejectsInvalidTokensWithoutLeakingInput(t *testing.T) {
	f := newOIDCFixture(t)
	base := f.claims("user-a", "engineering")
	tests := []struct {
		name     string
		category ErrorCategory
		token    func() string
	}{
		{"missing", CategoryMissingCredential, func() string { return "" }},
		{"malformed", CategoryMalformedToken, func() string { return "secret-token" }},
		{"wrong-signature", CategoryInvalidSignature, func() string { return sign(t, f.other, "primary", base) }},
		{"unknown-key", CategoryInvalidSignature, func() string { return sign(t, f.key, "unknown", base) }},
		{"wrong-issuer", CategoryInvalidIssuer, func() string { return sign(t, f.key, "primary", with(base, "iss", "https://evil.example")) }},
		{"wrong-audience", CategoryInvalidAudience, func() string { return sign(t, f.key, "primary", with(base, "aud", "elsewhere")) }},
		{"expired", CategoryExpiredToken, func() string { return sign(t, f.key, "primary", with(base, "exp", f.now.Add(-time.Second).Unix())) }},
		{"future", CategoryInactiveToken, func() string { return sign(t, f.key, "primary", with(base, "nbf", f.now.Add(time.Second).Unix())) }},
		{"missing-subject", CategoryInvalidIdentity, func() string { return sign(t, f.key, "primary", without(base, "sub")) }},
		{"missing-auth-context", CategoryInvalidIdentity, func() string { return sign(t, f.key, "primary", without(base, "acr")) }},
		{"unmapped-team", CategoryInvalidIdentity, func() string { return sign(t, f.key, "primary", with(base, "groups", "unknown")) }},
		{"conflicting-teams", CategoryInvalidIdentity, func() string {
			return sign(t, f.key, "primary", with(base, "groups", []string{"engineering", "support"}))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := test.token()
			_, err := f.verifier(t).Verify(context.Background(), token)
			if !IsCategory(err, test.category) {
				t.Fatalf("got %v, want %s", err, test.category)
			}
			if err != nil && ((token != "" && strings.Contains(err.Error(), token)) || strings.Contains(err.Error(), "engineering") || strings.Contains(err.Error(), "evil.example")) {
				t.Fatalf("error leaked credential or claims: %v", err)
			}
		})
	}
}

func TestOIDCVerifierFailsClosedWhenKeysUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider secret", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	verifier, err := NewOIDCVerifier(OIDCConfig{Issuer: "https://identity.example", Audience: "agenova", JWKSURL: server.URL, TeamClaim: "groups", TeamMappings: map[string]string{"engineering": "team-a"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifier.Verify(context.Background(), encode(t, map[string]any{"alg": "RS256", "kid": "key"})+"."+encode(t, map[string]any{"iss": "https://identity.example"})+".signature")
	if !IsCategory(err, CategoryKeyUnavailable) || strings.Contains(err.Error(), "provider secret") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("unexpected key failure: %v", err)
	}
}

func TestOIDCVerifierRefreshesExpiredJWKSAndRejectsRevokedKey(t *testing.T) {
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	active := &first.PublicKey
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk("shared", active)}})
	}))
	defer server.Close()
	now := time.Unix(1_800_000_000, 0).UTC()
	verifier, err := NewOIDCVerifier(OIDCConfig{
		Issuer: "https://identity.example", Audience: "agenova", JWKSURL: server.URL,
		TeamClaim: "groups", TeamMappings: map[string]string{"engineering": "team-a"},
		Clock: func() time.Time { return now }, JWKSCacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]any{"iss": "https://identity.example", "aud": "agenova", "sub": "user-a", "exp": now.Add(time.Hour).Unix(), "acr": "mfa", "groups": "engineering"}
	oldToken := sign(t, first, "shared", claims)
	if _, err := verifier.Verify(context.Background(), oldToken); err != nil {
		t.Fatal(err)
	}
	active = &second.PublicKey
	now = now.Add(2 * time.Minute)
	if _, err := verifier.Verify(context.Background(), oldToken); !IsCategory(err, CategoryInvalidSignature) {
		t.Fatalf("revoked cached key remained valid: %v", err)
	}
	newToken := sign(t, second, "shared", with(claims, "exp", now.Add(time.Hour).Unix()))
	if _, err := verifier.Verify(context.Background(), newToken); err != nil {
		t.Fatalf("rotated key was not accepted: %v", err)
	}
}

func TestOIDCConfigurationRejectsUnsafeOrIncompleteInputs(t *testing.T) {
	valid := OIDCConfig{Issuer: "https://identity.example", Audience: "agenova", JWKSURL: "https://identity.example/keys", TeamClaim: "groups", TeamMappings: map[string]string{"engineering": "team-a"}}
	for _, mutate := range []func(*OIDCConfig){
		func(c *OIDCConfig) { c.Issuer = "" }, func(c *OIDCConfig) { c.Audience = "" }, func(c *OIDCConfig) { c.JWKSURL = "/keys" },
		func(c *OIDCConfig) { c.TeamClaim = "" }, func(c *OIDCConfig) { c.TeamMappings = nil }, func(c *OIDCConfig) { c.TeamMappings = map[string]string{"engineering": ""} },
	} {
		config := valid
		mutate(&config)
		if _, err := NewOIDCVerifier(config); err == nil {
			t.Fatalf("invalid config accepted: %+v", config)
		}
	}
}

func sign(t *testing.T, key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()
	header := encode(t, map[string]any{"alg": "RS256", "kid": keyID, "typ": "JWT"})
	payload := encode(t, claims)
	digest := sha256.Sum256([]byte(header + "." + payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encode(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func jwk(keyID string, key *rsa.PublicKey) map[string]any {
	exponent := big.NewInt(int64(key.E)).Bytes()
	return map[string]any{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID, "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(exponent)}
}

func with(source map[string]any, key string, value any) map[string]any {
	result := clone(source)
	result[key] = value
	return result
}

func without(source map[string]any, key string) map[string]any {
	result := clone(source)
	delete(result, key)
	return result
}

func clone(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func ExampleError() {
	fmt.Println((&Error{Category: CategoryInvalidSignature}).Error())
	// Output: identity verification failed: invalid_signature
}
