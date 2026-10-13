// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package cognito validates explicitly configured Cognito access tokens. It never
// interprets token groups, email or team fields as application authority.
package cognito

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wunderforge/agenova/internal/authentication"
)

var issuerPattern = regexp.MustCompile(`^https://cognito-idp\.([a-z0-9-]+)\.amazonaws\.com/([a-z0-9-]+)_[A-Za-z0-9]+$`)
var errInvalid = authentication.ErrUnauthenticated

type Config struct {
	Issuer        string
	ClientIDs     []string
	RequiredScope string
	// Audience is optional only when resource-bound access tokens are not used.
	// Client binding is always checked independently through client_id.
	Audience string
}

type Verifier struct {
	config               Config
	clients              map[string]bool
	client               *http.Client
	mu                   sync.Mutex
	keys                 map[string]*rsa.PublicKey
	expires, lastAttempt time.Time
}

type claims struct {
	jwt.RegisteredClaims
	ClientID  string `json:"client_id"`
	Use       string `json:"token_use"`
	Scope     string `json:"scope"`
	SessionID string `json:"origin_jti"`
	AuthTime  int64  `json:"auth_time"`
}

func New(c Config) (*Verifier, error) {
	match := issuerPattern.FindStringSubmatch(c.Issuer)
	if len(match) != 3 || match[1] != match[2] || len(c.ClientIDs) == 0 || c.RequiredScope == "" || strings.ContainsAny(c.RequiredScope, " \t\n\r") {
		return nil, errors.New("explicit Cognito issuer, clients and API scope are required")
	}
	clients := map[string]bool{}
	for _, id := range c.ClientIDs {
		if id == "" || len(id) > 128 || strings.ContainsAny(id, " \t\r\n") {
			return nil, errors.New("invalid Cognito client configuration")
		}
		clients[id] = true
	}
	c.ClientIDs = append([]string(nil), c.ClientIDs...)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Verifier{config: c, clients: clients, client: &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, keys: map[string]*rsa.PublicKey{}}, nil
}

func (v *Verifier) Verify(ctx context.Context, raw string) (authentication.Identity, error) {
	if v == nil || len(raw) == 0 || len(raw) > 16<<10 || ctx.Err() != nil {
		return authentication.Identity{}, errInvalid
	}
	c := &claims{}
	options := []jwt.ParserOption{jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.config.Issuer), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithStrictDecoding()}
	if v.config.Audience != "" {
		options = append(options, jwt.WithAudience(v.config.Audience))
	}
	token, err := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 || t.Method != jwt.SigningMethodRS256 {
			return nil, errInvalid
		}
		// Remote key URLs and critical extensions are unsupported. Only the pinned
		// operator issuer can ever select the key endpoint.
		for _, name := range []string{"jku", "jwk", "x5u", "crit"} {
			if _, present := t.Header[name]; present {
				return nil, errInvalid
			}
		}
		return v.key(ctx, kid)
	}, options...)
	if err != nil || !token.Valid || c.Use != "access" || !v.clients[c.ClientID] || c.Subject == "" || len(c.Subject) > 256 || c.ID == "" || len(c.ID) > 256 || c.SessionID == "" || len(c.SessionID) > 256 || c.IssuedAt == nil || c.AuthTime <= 0 || (c.IssuedAt != nil && c.AuthTime > c.IssuedAt.Unix()) || c.ExpiresAt == nil || !c.ExpiresAt.After(c.IssuedAt.Time) || ctx.Err() != nil {
		return authentication.Identity{}, errInvalid
	}
	hasScope := false
	for _, scope := range strings.Fields(c.Scope) {
		if scope == v.config.RequiredScope {
			hasScope = true
		}
	}
	if !hasScope {
		return authentication.Identity{}, errInvalid
	}
	return authentication.Identity{Issuer: c.Issuer, Subject: c.Subject, TokenID: c.ID, SessionID: c.SessionID, ExpiresAt: c.ExpiresAt.Time, AuthenticatedAt: time.Unix(c.AuthTime, 0)}, nil
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	if ctx.Err() != nil {
		return nil, errInvalid
	}
	if k := v.keys[kid]; k != nil && now.Before(v.expires) {
		return k, nil
	}
	// One bounded refresh per issuer/30s, including unknown-kid and outage traffic.
	if now.Sub(v.lastAttempt) < 30*time.Second {
		return nil, errInvalid
	}
	v.lastAttempt = now
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.config.Issuer+"/.well-known/jwks.json", nil)
	if err != nil {
		return nil, errInvalid
	}
	response, err := v.client.Do(req)
	if err != nil {
		return nil, errInvalid
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errInvalid
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return nil, errInvalid
	}
	var set struct {
		Keys []struct{ Kty, Kid, Use, Alg, N, E string }
	}
	if json.Unmarshal(data, &set) != nil || len(set.Keys) == 0 || len(set.Keys) > 16 {
		return nil, errInvalid
	}
	keys := map[string]*rsa.PublicKey{}
	for _, j := range set.Keys {
		if j.Kty != "RSA" || j.Use != "sig" || j.Alg != "RS256" || j.Kid == "" || len(j.Kid) > 256 || keys[j.Kid] != nil {
			return nil, errInvalid
		}
		n, errN := base64.RawURLEncoding.Strict().DecodeString(j.N)
		e, errE := base64.RawURLEncoding.Strict().DecodeString(j.E)
		if errN != nil || errE != nil || len(n) < 256 || len(n) > 512 || len(e) == 0 || len(e) > 4 {
			return nil, errInvalid
		}
		modulus := new(big.Int).SetBytes(n)
		exp := new(big.Int).SetBytes(e).Int64()
		if modulus.BitLen() < 2048 || modulus.BitLen() > 4096 || modulus.Bit(0) != 1 || exp < 3 || exp > 2147483647 || exp%2 != 1 {
			return nil, errInvalid
		}
		keys[j.Kid] = &rsa.PublicKey{N: modulus, E: int(exp)}
	}
	v.keys = keys
	v.expires = now.Add(15 * time.Minute)
	if k := keys[kid]; k != nil {
		return k, nil
	}
	return nil, errInvalid
}
