// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package googleoidc

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"golang.org/x/oauth2"
)

const authorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
const tokenURL = "https://oauth2.googleapis.com/token"

var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// Client performs the server-side Google leg of login. The caller owns one-use
// attempt persistence and state validation. No Google credential is returned.
type Client struct {
	oauth    oauth2.Config
	verifier *Verifier
	http     *http.Client
}

func NewClient(clientID, clientSecret, redirectURL string) (*Client, error) {
	v, err := New(clientID)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(redirectURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/auth/callback" || clientSecret == "" {
		return nil, errors.New("Google web client requires secret and exact HTTPS callback")
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: http.DefaultTransport.(*http.Transport).Clone(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{oauth: oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, Scopes: []string{"openid", "email"}, Endpoint: oauth2.Endpoint{AuthURL: authorizationURL, TokenURL: tokenURL, AuthStyle: oauth2.AuthStyleInParams}}, verifier: v, http: client}, nil
}

// AuthorizationURL requests identity only, no Google API permissions or offline
// access. State/nonce must be independent cryptographically random handles.
func (c *Client) AuthorizationURL(state, nonce, verifier string) (string, error) {
	if c == nil || !handlePattern.MatchString(state) || !handlePattern.MatchString(nonce) || state == nonce || !verifierPattern.MatchString(verifier) {
		return "", errInvalid
	}
	return c.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

// Exchange is callback-only and must follow atomic attempt consumption and
// constant-time state validation. It never refreshes or persists Google tokens.
func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (LoginIdentity, error) {
	if c == nil || len(code) == 0 || len(code) > 4096 || !verifierPattern.MatchString(verifier) || !handlePattern.MatchString(nonce) {
		return LoginIdentity{}, errInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	token, err := c.oauth.Exchange(context.WithValue(ctx, oauth2.HTTPClient, c.http), code, oauth2.VerifierOption(verifier))
	if err != nil || token == nil {
		return LoginIdentity{}, errInvalid
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return LoginIdentity{}, errInvalid
	}
	return c.verifier.VerifyLogin(ctx, raw, nonce)
}
