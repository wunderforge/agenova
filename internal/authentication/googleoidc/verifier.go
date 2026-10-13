// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package googleoidc verifies Google login assertions. It deliberately does not
// implement authentication.Verifier: Google ID tokens cannot authorize our API.
package googleoidc

import (
	"context"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const Issuer = "https://accounts.google.com"
const keysURL = "https://www.googleapis.com/oauth2/v3/certs"

var errInvalid = errors.New("Google login assertion rejected")

// LoginIdentity is callback-only metadata, not a principal or application grant.
// Email can identify an explicit invitation, but issuer+subject is permanent.
type LoginIdentity struct {
	Issuer, Subject, Email string
	// EmailAuthoritative is true only for Gmail or verified Workspace accounts.
	// Other domains require independent email ownership proof for invitation use.
	EmailAuthoritative bool
}

type Verifier struct {
	clientID             string
	client               *http.Client
	mu                   sync.Mutex
	keys                 map[string]*rsa.PublicKey
	expires, lastAttempt time.Time
}

type claims struct {
	jwt.RegisteredClaims
	Nonce           string `json:"nonce"`
	AuthorizedParty string `json:"azp"`
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	HostedDomain    string `json:"hd"`
}

func New(clientID string) (*Verifier, error) {
	if !strings.HasSuffix(clientID, ".apps.googleusercontent.com") || len(clientID) > 256 || len(clientID) <= len(".apps.googleusercontent.com") || strings.ContainsAny(clientID, " /\\\t\r\n") {
		return nil, errors.New("explicit Google OAuth client ID required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Verifier{clientID: clientID, client: &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, keys: map[string]*rsa.PublicKey{}}, nil
}

// VerifyLogin must be called only after a stored one-use state/PKCE login attempt
// has been consumed. The nonce must come from that server-owned attempt, never
// from query parameters or the assertion itself. It does not consume the nonce.
func (v *Verifier) VerifyLogin(ctx context.Context, raw, expectedNonce string) (LoginIdentity, error) {
	if v == nil || ctx.Err() != nil || len(raw) == 0 || len(raw) > 16<<10 || len(expectedNonce) < 32 || len(expectedNonce) > 256 {
		return LoginIdentity{}, errInvalid
	}
	c := &claims{}
	token, err := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 || t.Method != jwt.SigningMethodRS256 {
			return nil, errInvalid
		}
		for _, name := range []string{"jku", "jwk", "x5u", "crit"} {
			if _, present := t.Header[name]; present {
				return nil, errInvalid
			}
		}
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithAudience(v.clientID), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithStrictDecoding())
	if err != nil || token == nil || !token.Valid || ctx.Err() != nil {
		return LoginIdentity{}, errInvalid
	}
	if (c.Issuer != Issuer && c.Issuer != "accounts.google.com") || c.Subject == "" || len(c.Subject) > 255 || strings.ContainsAny(c.Subject, " \t\r\n") || c.IssuedAt == nil || time.Since(c.IssuedAt.Time) > 10*time.Minute || c.ExpiresAt == nil || !c.ExpiresAt.After(c.IssuedAt.Time) || subtle.ConstantTimeCompare([]byte(c.Nonce), []byte(expectedNonce)) != 1 {
		return LoginIdentity{}, errInvalid
	}
	if (c.AuthorizedParty != "" && c.AuthorizedParty != v.clientID) || (len(c.Audience) > 1 && c.AuthorizedParty != v.clientID) {
		return LoginIdentity{}, errInvalid
	}
	email, err := mail.ParseAddress(c.Email)
	if err != nil || email.Address != c.Email || len(c.Email) > 254 || !c.EmailVerified {
		return LoginIdentity{}, errInvalid
	}
	_, domain, _ := strings.Cut(c.Email, "@")
	return LoginIdentity{Issuer: Issuer, Subject: c.Subject, Email: strings.ToLower(c.Email), EmailAuthoritative: strings.EqualFold(domain, "gmail.com") || c.HostedDomain != ""}, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keysURL, nil)
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
