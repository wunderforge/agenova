// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package websession

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/login"
	"golang.org/x/oauth2"
)

//go:embed login.html
var loginPage string

//go:embed login.css
var loginCSS string

const sessionCookie = "__Host-agenova-session"
const loginCookie = "__Host-agenova-login"

type Revoker interface {
	Revoke(context.Context, authentication.Identity) error
}
type Config struct {
	Origin     string
	Native     login.Config
	Web        oauth2.Config
	Store      *Store
	Boundary   *authentication.Boundary
	Revoker    Revoker
	Next       http.Handler
	HTTPClient *http.Client
}
type session struct {
	Token    oauth2.Token
	Identity authentication.Identity
}
type attempt struct{ State, Verifier string }
type Handler struct {
	c      Config
	client *http.Client
}

func New(c Config) (*Handler, error) {
	if !login.Origin(c.Origin) || c.Native.Validate() != nil || c.Web.ClientID == "" || c.Web.ClientID == c.Native.ClientID || c.Web.RedirectURL != c.Origin+"/auth/callback" || c.Web.Endpoint.AuthURL != c.Native.AuthorizationEndpoint || c.Web.Endpoint.TokenURL != c.Native.TokenEndpoint || c.Store == nil || c.Boundary == nil || c.Revoker == nil || c.Next == nil {
		return nil, errors.New("complete web/native authentication configuration required")
	}
	client := http.Client{Timeout: 15 * time.Second}
	if c.HTTPClient != nil {
		client = *c.HTTPClient
	}
	client.Timeout = 15 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Handler{c, &client}, nil
}
func randomHandle() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errSession
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}
func readCookie(r *http.Request, name string) string {
	values := r.Cookies()
	value := ""
	for _, v := range values {
		if v.Name == name {
			if value != "" || len(v.Value) != 43 {
				return ""
			}
			value = v.Value
		}
	}
	return value
}
func (h *Handler) origin(r *http.Request) bool {
	values := r.Header.Values("Origin")
	return len(values) == 1 && values[0] == h.c.Origin && r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func failure(w http.ResponseWriter, status int) {
	http.Error(w, "Authentication or permission check failed.", status)
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.TLS == nil {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			failure(w, 400)
			return
		}
	}
	// Public origin is operator-pinned. Never derive redirects/trust from forwarded headers.
	if "https://"+r.Host != h.c.Origin {
		failure(w, 400)
		return
	}
	requestContext := r.Context()
	ctx, cancel := context.WithTimeout(requestContext, 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	switch r.URL.Path {
	case "/auth/cli-config":
		if r.Method != "GET" {
			failure(w, 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(h.c.Native)
		return
	case "/login":
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			styleHash := sha256.Sum256([]byte(loginCSS))
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'; form-action 'self'; frame-ancestors 'none'")
			_, _ = io.WriteString(w, strings.Replace(loginPage, "{{styles}}", loginCSS, 1))
			return
		}
		if r.Method != "POST" || !h.origin(r) {
			failure(w, 403)
			return
		}
		handle, err := randomHandle()
		if err != nil {
			failure(w, 503)
			return
		}
		state, err := randomHandle()
		if err != nil {
			failure(w, 503)
			return
		}
		a := attempt{state, oauth2.GenerateVerifier()}
		if h.c.Store.PutAttempt(ctx, handle, a) != nil {
			failure(w, 429)
			return
		}
		cookie(w, loginCookie, handle, 300)
		http.Redirect(w, r, h.c.Web.AuthCodeURL(state, oauth2.S256ChallengeOption(a.Verifier)), http.StatusSeeOther)
		return
	case "/auth/callback":
		h.callback(w, r)
		return
	}
	// Browser credentials cannot be combined with a bearer token. HTTP Basic and
	// proxy identity headers are never accepted as application identity.
	handle := readCookie(r, sessionCookie)
	if handle != "" && len(r.Header.Values("Authorization")) != 0 {
		failure(w, 400)
		return
	}
	authenticated := r.Clone(ctx)
	var current session
	if handle != "" {
		version, err := h.c.Store.Load(ctx, handle, &current)
		if err != nil {
			cookie(w, sessionCookie, "", -1)
			h.unauthorized(w, r)
			return
		}
		token, err := h.c.Web.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, h.client), &current.Token).Token()
		if err != nil || token == nil || !token.Valid() {
			h.unauthorized(w, r)
			return
		}
		authenticated.Header.Set("Authorization", "Bearer "+token.AccessToken)
		identity, _, err := h.c.Boundary.AuthenticateIdentity(authenticated)
		if err != nil || identity.Issuer != current.Identity.Issuer || identity.Subject != current.Identity.Subject || identity.SessionID != current.Identity.SessionID {
			failure(w, 403)
			return
		}
		if token.AccessToken != current.Token.AccessToken || token.RefreshToken != current.Token.RefreshToken {
			current.Token = *token
			if h.c.Store.Save(ctx, handle, version, current) != nil {
				failure(w, 401)
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && !h.origin(r) {
			failure(w, 403)
			return
		}
	}
	identity, p, err := h.c.Boundary.AuthenticateIdentity(authenticated)
	if err != nil {
		h.unauthorized(w, r)
		return
	}
	switch r.URL.Path {
	case "/auth/me":
		if r.Method != "GET" {
			failure(w, 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
		return
	case "/auth/logout", "/logout":
		if r.Method != "POST" {
			failure(w, 405)
			return
		}
		// Persist revocation before declaring logout successful. Even a concurrent
		// request holding a loaded cookie must pass the revoked-family check again.
		if h.c.Revoker.Revoke(ctx, identity) != nil {
			failure(w, 503)
			return
		}
		if handle != "" {
			if h.c.Store.Delete(ctx, handle) != nil {
				failure(w, 503)
				return
			}
			cookie(w, sessionCookie, "", -1)
		}
		if r.URL.Path == "/logout" {
			http.Redirect(w, r, "/login", 303)
		} else {
			w.WriteHeader(204)
		}
		return
	}
	// TLS may terminate at the explicitly configured local reverse proxy. Origin
	// matching above pins the public host; the inner console sees that same scheme.
	authenticated.TLS = &tls.ConnectionState{}
	authenticated.Header.Del("X-Forwarded-User")
	authenticated.Header.Del("X-Forwarded-Email")
	h.c.Next.ServeHTTP(w, authenticated.WithContext(requestContext))
}
func (h *Handler) unauthorized(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
		failure(w, 401)
		return
	}
	http.Redirect(w, r, "/login", 303)
}
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" || len(r.URL.RawQuery) > 8192 {
		failure(w, 400)
		return
	}
	handle := readCookie(r, loginCookie)
	if handle == "" {
		failure(w, 400)
		return
	}
	var a attempt
	if h.c.Store.TakeAttempt(r.Context(), handle, &a) != nil {
		failure(w, 400)
		return
	}
	cookie(w, loginCookie, "", -1)
	q := r.URL.Query()
	if len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(a.State)) != 1 || len(q["code"]) != 1 || q.Get("code") == "" || q.Get("error") != "" {
		failure(w, 400)
		return
	}
	token, err := h.c.Web.Exchange(context.WithValue(r.Context(), oauth2.HTTPClient, h.client), q.Get("code"), oauth2.VerifierOption(a.Verifier))
	if err != nil || token == nil || !token.Valid() || token.RefreshToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
		failure(w, 401)
		return
	}
	req := r.Clone(r.Context())
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	identity, _, err := h.c.Boundary.AuthenticateIdentity(req)
	if err != nil {
		failure(w, 403)
		return
	}
	fresh, err := randomHandle()
	if err != nil {
		failure(w, 503)
		return
	}
	if h.c.Store.Create(r.Context(), fresh, session{*token, identity}, time.Now().Add(12*time.Hour)) != nil {
		failure(w, 503)
		return
	}
	if old := readCookie(r, sessionCookie); old != "" {
		if h.c.Store.Delete(r.Context(), old) != nil {
			failure(w, 503)
			return
		}
	}
	cookie(w, sessionCookie, fresh, 12*60*60)
	http.Redirect(w, r, "/?mode=connected", 303)
}
