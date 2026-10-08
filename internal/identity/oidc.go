// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxJWKSBytes = 1 << 20
const defaultJWKSCacheTTL = 5 * time.Minute

type OIDCConfig struct {
	Issuer                     string
	Audience                   string
	JWKSURL                    string
	TeamClaim                  string
	TeamMappings               map[string]string
	AuthenticationContextClaim string
	HTTPClient                 *http.Client
	Clock                      func() time.Time
	JWKSCacheTTL               time.Duration
}

// OIDCVerifier implements the deliberately bounded M2 profile: explicit
// issuer/audience/JWKS configuration and RS256 tokens. Discovery, token
// issuance, refresh, and user-directory behavior remain outside Agenova.
type OIDCVerifier struct {
	config    OIDCConfig
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewOIDCVerifier(config OIDCConfig) (*OIDCVerifier, error) {
	config.Issuer = strings.TrimSpace(config.Issuer)
	config.Audience = strings.TrimSpace(config.Audience)
	config.JWKSURL = strings.TrimSpace(config.JWKSURL)
	config.TeamClaim = strings.TrimSpace(config.TeamClaim)
	if config.AuthenticationContextClaim == "" {
		config.AuthenticationContextClaim = "acr"
	}
	parsed, err := url.Parse(config.JWKSURL)
	if config.Issuer == "" || config.Audience == "" || config.TeamClaim == "" || err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || len(config.TeamMappings) == 0 {
		return nil, errors.New("OIDC verifier configuration is incomplete")
	}
	for source, team := range config.TeamMappings {
		if strings.TrimSpace(source) == "" || strings.TrimSpace(team) == "" {
			return nil, errors.New("OIDC team mapping is invalid")
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.JWKSCacheTTL == 0 {
		config.JWKSCacheTTL = defaultJWKSCacheTTL
	}
	if config.JWKSCacheTTL < 0 || config.JWKSCacheTTL > time.Hour {
		return nil, errors.New("OIDC JWKS cache TTL is invalid")
	}
	return &OIDCVerifier{config: config, keys: map[string]*rsa.PublicKey{}}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, bearer string) (VerifiedPrincipal, error) {
	if strings.TrimSpace(bearer) == "" {
		return VerifiedPrincipal{}, reject(CategoryMissingCredential)
	}
	parts := strings.Split(bearer, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return VerifiedPrincipal{}, reject(CategoryMalformedToken)
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if err := decodeSegment(parts[0], &header); err != nil || header.Algorithm != "RS256" || strings.TrimSpace(header.KeyID) == "" {
		return VerifiedPrincipal{}, reject(CategoryMalformedToken)
	}
	key, ok := v.key(header.KeyID)
	if !ok || !v.keysFresh() {
		if err := v.refreshKeys(ctx); err != nil {
			return VerifiedPrincipal{}, reject(CategoryKeyUnavailable)
		}
		key, ok = v.key(header.KeyID)
		if !ok {
			return VerifiedPrincipal{}, reject(CategoryInvalidSignature)
		}
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return VerifiedPrincipal{}, reject(CategoryMalformedToken)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		// Refresh once for a rotated key carrying the same kid.
		if err := v.refreshKeys(ctx); err != nil {
			return VerifiedPrincipal{}, reject(CategoryInvalidSignature)
		}
		key, ok = v.key(header.KeyID)
		if !ok || rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
			return VerifiedPrincipal{}, reject(CategoryInvalidSignature)
		}
	}
	claims := map[string]any{}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return VerifiedPrincipal{}, reject(CategoryMalformedToken)
	}
	issuer, _ := claims["iss"].(string)
	if issuer != v.config.Issuer {
		return VerifiedPrincipal{}, reject(CategoryInvalidIssuer)
	}
	if !containsAudience(claims["aud"], v.config.Audience) {
		return VerifiedPrincipal{}, reject(CategoryInvalidAudience)
	}
	now := v.config.Clock().Unix()
	expires, ok := integerClaim(claims["exp"])
	if !ok || expires <= now {
		return VerifiedPrincipal{}, reject(CategoryExpiredToken)
	}
	if notBefore, present := claims["nbf"]; present {
		value, valid := integerClaim(notBefore)
		if !valid || value > now {
			return VerifiedPrincipal{}, reject(CategoryInactiveToken)
		}
	}
	subject, _ := claims["sub"].(string)
	authenticationContext, _ := claims[v.config.AuthenticationContextClaim].(string)
	team, teamValues, ok := v.mapTeam(claims[v.config.TeamClaim])
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(authenticationContext) == "" || !ok {
		return VerifiedPrincipal{}, reject(CategoryInvalidIdentity)
	}
	principal := VerifiedPrincipal{Issuer: issuer, Subject: subject, Team: team, AuthenticationContext: authenticationContext, Attributes: []Attribute{{Name: "team-source", Values: teamValues}}}
	if err := principal.Validate(); err != nil {
		return VerifiedPrincipal{}, reject(CategoryInvalidIdentity)
	}
	return principal, nil
}

func (v *OIDCVerifier) mapTeam(raw any) (string, []string, bool) {
	values, ok := stringValues(raw)
	if !ok || len(values) == 0 {
		return "", nil, false
	}
	slices.Sort(values)
	values = slices.Compact(values)
	mapped := ""
	for _, value := range values {
		team, exists := v.config.TeamMappings[value]
		if !exists {
			continue
		}
		if mapped != "" && mapped != team {
			return "", nil, false
		}
		mapped = team
	}
	return mapped, values, mapped != ""
}

func (v *OIDCVerifier) key(id string) (*rsa.PublicKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[id]
	return key, ok
}

func (v *OIDCVerifier) keysFresh() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return !v.fetchedAt.IsZero() && v.config.Clock().Sub(v.fetchedAt) >= 0 && v.config.Clock().Sub(v.fetchedAt) < v.config.JWKSCacheTTL
}

func (v *OIDCVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.config.JWKSURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("JWKS unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil || len(data) > maxJWKSBytes {
		return errors.New("JWKS response invalid")
	}
	var document struct {
		Keys []struct {
			KeyType string `json:"kty"`
			Use     string `json:"use"`
			Alg     string `json:"alg"`
			KeyID   string `json:"kid"`
			N       string `json:"n"`
			E       string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(data, &document) != nil {
		return errors.New("JWKS response invalid")
	}
	keys := map[string]*rsa.PublicKey{}
	for _, item := range document.Keys {
		if item.KeyType != "RSA" || item.Alg != "RS256" || (item.Use != "" && item.Use != "sig") || strings.TrimSpace(item.KeyID) == "" {
			continue
		}
		n, nErr := base64.RawURLEncoding.DecodeString(item.N)
		e, eErr := base64.RawURLEncoding.DecodeString(item.E)
		if nErr != nil || eErr != nil || len(n) == 0 || len(e) == 0 || len(e) > 4 {
			continue
		}
		exponent := 0
		for _, value := range e {
			exponent = exponent<<8 | int(value)
		}
		if exponent < 3 {
			continue
		}
		modulus := new(big.Int).SetBytes(n)
		if modulus.BitLen() < 2048 {
			continue
		}
		keys[item.KeyID] = &rsa.PublicKey{N: modulus, E: exponent}
	}
	if len(keys) == 0 {
		return errors.New("JWKS contains no supported signing key")
	}
	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = v.config.Clock()
	v.mu.Unlock()
	return nil
}

func decodeSegment(segment string, target any) error {
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func integerClaim(raw any) (int64, bool) {
	number, ok := raw.(json.Number)
	if !ok {
		return 0, false
	}
	value, err := number.Int64()
	return value, err == nil
}

func containsAudience(raw any, expected string) bool {
	values, ok := stringValues(raw)
	return ok && slices.Contains(values, expected)
}

func stringValues(raw any) ([]string, bool) {
	switch value := raw.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, false
		}
		return []string{value}, true
	case []any:
		values := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return nil, false
			}
			values = append(values, text)
		}
		return values, true
	default:
		return nil, false
	}
}
