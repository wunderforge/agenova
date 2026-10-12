// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/wunderforge/agenova/internal/login"
)

const nativeCookie = "__Host-agenova-native"

var nativeStatePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var nativeVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

type nativeRequest struct{ State, Challenge string }
type nativeCode struct{ Kind, Issuer, Subject, Challenge string }

func (h *Handler) nativeConfig() login.Config {
	return login.Config{Issuer: h.c.Origin, ClientID: "agenova-cli", AuthorizationEndpoint: h.c.Origin + "/auth/authorize", TokenEndpoint: h.c.Origin + "/auth/token", RedirectURI: login.Callback, Scopes: []string{"agenova/api"}, SessionMode: "application"}
}
func (h *Handler) nativeAuthorize(w http.ResponseWriter, r *http.Request) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if r.Method != "GET" || err != nil || len(r.URL.RawQuery) > 2048 || len(q) != 7 {
		failure(w, 400)
		return
	}
	for _, v := range q {
		if len(v) != 1 {
			failure(w, 400)
			return
		}
	}
	if q.Get("client_id") != "agenova-cli" || q.Get("redirect_uri") != login.Callback || q.Get("response_type") != "code" || q.Get("scope") != "agenova/api" || q.Get("code_challenge_method") != "S256" || !nativeStatePattern.MatchString(q.Get("code_challenge")) || !nativeStatePattern.MatchString(q.Get("state")) {
		failure(w, 400)
		return
	}
	handle, err := randomHandle()
	if err != nil {
		failure(w, 503)
		return
	}
	if h.c.Store.PutAttempt(r.Context(), "native:"+handle, nativeRequest{State: q.Get("state"), Challenge: q.Get("code_challenge")}) != nil {
		failure(w, 429)
		return
	}
	cookie(w, nativeCookie, handle, 300)
	// Explicit login POST follows; never silently transfer a browser session to CLI.
	http.Redirect(w, r, "/login", 303)
}
func (h *Handler) nativeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
		failure(w, 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	if r.ParseForm() != nil || len(r.PostForm) != 5 {
		failure(w, 400)
		return
	}
	q := r.PostForm
	for _, v := range q {
		if len(v) != 1 {
			failure(w, 400)
			return
		}
	}
	if q.Get("grant_type") != "authorization_code" || q.Get("client_id") != "agenova-cli" || q.Get("redirect_uri") != login.Callback || !nativeStatePattern.MatchString(q.Get("code")) || !nativeVerifierPattern.MatchString(q.Get("code_verifier")) {
		failure(w, 400)
		return
	}
	var grant nativeCode
	if h.c.Store.TakeAttempt(r.Context(), "code:"+q.Get("code"), &grant) != nil || grant.Kind != "cli-code" {
		failure(w, 401)
		return
	}
	sum := sha256.Sum256([]byte(q.Get("code_verifier")))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(challenge), []byte(grant.Challenge)) != 1 {
		failure(w, 401)
		return
	}
	raw, id, err := h.c.ApplicationSessions.IssueApplicationSession(r.Context(), grant.Issuer, grant.Subject, 12*time.Hour)
	if err != nil {
		failure(w, 403)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": raw, "token_type": "Bearer", "expires_in": int(time.Until(id.ExpiresAt).Seconds()), "scope": "agenova/api"})
}
