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
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/login"
	"golang.org/x/oauth2"
)

//go:embed login.html
var loginPage string

//go:embed login.css
var loginCSS string

//go:embed google-login.js
var googleLoginScript string

const sessionCookie = "__Host-agenova-session"
const loginCookie = "__Host-agenova-login"

type Revoker interface {
	Revoke(context.Context, authentication.Identity) error
}
type GoogleLogin interface {
	AuthorizationURL(string, string, string) (string, error)
	Exchange(context.Context, string, string, string) (googleoidc.LoginIdentity, error)
}
type InvitationAcceptor interface {
	AcceptGoogleInvitation(context.Context, googleoidc.LoginIdentity, string) (v0.Principal, error)
}
type ApplicationSessions interface {
	IssueApplicationSession(context.Context, string, string, time.Duration) (string, authentication.Identity, error)
}
type AccessRegistrar interface {
	RegisterGoogleAccess(context.Context, googleoidc.LoginIdentity, string) (string, error)
}
type Config struct {
	Registration        AccessRegistrar
	RegistrationTeam    string
	Invitations         InvitationAcceptor
	Google              GoogleLogin
	ApplicationSessions ApplicationSessions
	Origin              string
	Native              login.Config
	Web                 oauth2.Config
	Store               *Store
	Boundary            *authentication.Boundary
	Revoker             Revoker
	Next                http.Handler
	// Landing is explicitly public product material, not a second Portal/API.
	// Only anonymous root and the /landing/ asset namespace may reach it.
	Landing    http.Handler
	HTTPClient *http.Client
}
type session struct {
	Token    oauth2.Token
	Identity authentication.Identity
}
type attempt struct {
	State, Verifier, Nonce, Invitation string
	Native                             *nativeRequest
}
type Handler struct {
	c      Config
	client *http.Client
}

func New(c Config) (*Handler, error) {
	commonInvalid := !login.Origin(c.Origin) || c.Store == nil || c.Boundary == nil || c.Revoker == nil || c.Next == nil
	legacyInvalid := c.Native.Validate() != nil || c.Web.ClientID == "" || c.Web.ClientID == c.Native.ClientID || c.Web.RedirectURL != c.Origin+"/auth/callback" || c.Web.Endpoint.AuthURL != c.Native.AuthorizationEndpoint || c.Web.Endpoint.TokenURL != c.Native.TokenEndpoint
	if (c.Registration != nil && (c.Google == nil || c.RegistrationTeam == "")) || commonInvalid || (c.Google == nil && legacyInvalid) || (c.Google != nil && (c.ApplicationSessions == nil || c.Web.ClientID != "" || c.Native.ClientID != "")) {
		return nil, errors.New("complete and unambiguous authentication configuration required")
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
	// Keep Origin on same-origin form POSTs; suppress referrers to other sites.
	w.Header().Set("Referrer-Policy", "same-origin")
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
	if h.c.Landing != nil && strings.HasPrefix(r.URL.Path, "/landing/") {
		h.c.Landing.ServeHTTP(w, r)
		return
	}
	switch r.URL.Path {
	case "/auth/cli-config":

		if r.Method != "GET" {
			failure(w, 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		native := h.c.Native
		if h.c.Google != nil {
			native = h.nativeConfig()
		}
		_ = json.NewEncoder(w).Encode(native)
		return
	case "/auth/authorize":
		if h.c.Google == nil {
			failure(w, 404)
			return
		}
		h.nativeAuthorize(w, r)
		return
	case "/auth/token":
		if h.c.Google == nil {
			failure(w, 404)
			return
		}
		h.nativeToken(w, r)
		return
	case "/access-pending", "/access-declined":
		if r.Method != "GET" || h.c.Registration == nil {
			failure(w, 404)
			return
		}
		message := "Your account is registered. Contact your administrator to assign project access and a role. After approval, sign in again. If you started from the CLI, run agenova login again."
		title := "Access pending"
		if r.URL.Path == "/access-declined" {
			title = "Access declined"
			message = "Your access request was declined. Contact your administrator."
		}
		styleHash := sha256.Sum256([]byte(loginCSS))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'; frame-ancestors 'none'")
		_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>"+title+" | Agenova</title><style>"+loginCSS+"</style><main><p>Agenova</p><h1>"+title+"</h1><p>"+message+"</p><a href=\"/login\">Back to sign in</a></main></html>")
		return
	case "/login":
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			styleHash := sha256.Sum256([]byte(loginCSS))
			w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'sha256-"+base64.StdEncoding.EncodeToString(styleHash[:])+"'; form-action 'self'; frame-ancestors 'none'")
			page := strings.Replace(loginPage, "{{styles}}", loginCSS, 1)
			if h.c.Google != nil {
				// Chromium applies form-action to the redirect destination too.
				// Permit only our pinned IdP, while the POST origin check stays local.
				w.Header().Set("Content-Security-Policy", strings.Replace(w.Header().Get("Content-Security-Policy"), "form-action 'self'", "form-action 'self' https://accounts.google.com", 1))
				page = strings.Replace(page, ">Sign in</button>", ">Continue with Google</button>", 1)
				if readCookie(r, nativeCookie) != "" {
					page = strings.Replace(page, "Sign in with your invited account to run agents and review your Work.", "Authorize Agenova CLI on your computer. Continue only if you just ran agenova login.", 1)
				}
				scriptHash := sha256.Sum256([]byte(googleLoginScript))
				w.Header().Set("Content-Security-Policy", w.Header().Get("Content-Security-Policy")+"; script-src 'sha256-"+base64.StdEncoding.EncodeToString(scriptHash[:])+"'")
				page = strings.Replace(page, "</body>", "<script>"+googleLoginScript+"</script></body>", 1)
			}
			if h.c.Registration != nil {
				page = strings.ReplaceAll(page, "Sign in with your invited account to run agents and review your Work.", "Sign in with Google. New accounts require administrator approval before accessing projects.")
				page = strings.ReplaceAll(page, "Need access? Ask your project administrator for an invitation.", "New here? Sign in to register, then contact your administrator for access.")
			}
			_, _ = io.WriteString(w, page)
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
		a := attempt{State: state, Verifier: oauth2.GenerateVerifier()}
		if h.c.Google != nil {
			a.Nonce, err = randomHandle()
			if err != nil {
				failure(w, 503)
				return
			}
		}
		if h.c.Google != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 2048)
			if r.ParseForm() != nil || len(r.PostForm["invitation"]) > 1 {
				failure(w, 400)
				return
			}
			a.Invitation = r.PostForm.Get("invitation")
			if a.Invitation != "" {
				b, e := base64.RawURLEncoding.Strict().DecodeString(a.Invitation)
				if e != nil || len(b) != 32 || len(a.Invitation) != 43 || h.c.Invitations == nil {
					failure(w, 400)
					return
				}
			}
		}
		if h.c.Google != nil {
			if nativeHandle := readCookie(r, nativeCookie); nativeHandle != "" {
				var n nativeRequest
				if h.c.Store.TakeAttempt(ctx, "native:"+nativeHandle, &n) != nil {
					failure(w, 400)
					return
				}
				a.Native = &n
				cookie(w, nativeCookie, "", -1)
			}
		}
		if h.c.Store.PutAttempt(ctx, handle, a) != nil {
			failure(w, 429)
			return
		}
		cookie(w, loginCookie, handle, 300)
		destination := h.c.Web.AuthCodeURL(state, oauth2.S256ChallengeOption(a.Verifier))
		if h.c.Google != nil {
			destination, err = h.c.Google.AuthorizationURL(a.State, a.Nonce, a.Verifier)
			if err != nil {
				failure(w, 503)
				return
			}
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
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
		token := &current.Token
		if h.c.Google == nil {
			token, err = h.c.Web.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, h.client), &current.Token).Token()
		}
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
	if h.c.Landing != nil && (r.URL.Path == "/" || r.URL.Path == "/index.html") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		h.c.Landing.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
		failure(w, 401)
		return
	}
	http.Redirect(w, r, "/login", 303)
}

// callbackFailure records only a fixed stage, never OAuth or personal data.
func callbackFailure(w http.ResponseWriter, status int, stage string) {
	log.Printf("authentication callback failed: stage=%s status=%d", stage, status)
	failure(w, status)
}
func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" || len(r.URL.RawQuery) > 8192 {
		callbackFailure(w, 400, "invalid_request")
		return
	}
	handle := readCookie(r, loginCookie)
	if handle == "" {
		callbackFailure(w, 400, "missing_attempt_cookie")
		return
	}
	var a attempt
	if h.c.Store.TakeAttempt(r.Context(), handle, &a) != nil {
		callbackFailure(w, 400, "attempt_unavailable")
		return
	}
	cookie(w, loginCookie, "", -1)
	q := r.URL.Query()
	if len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(a.State)) != 1 || len(q["code"]) != 1 || q.Get("code") == "" || q.Get("error") != "" {
		callbackFailure(w, 400, "invalid_callback_parameters")
		return
	}
	var token *oauth2.Token
	var err error
	if h.c.Google != nil {
		// Google tokens terminate here; only an application-issued credential enters
		// the encrypted browser session and the authenticated application boundary.
		id, verifyErr := h.c.Google.Exchange(r.Context(), q.Get("code"), a.Verifier, a.Nonce)
		if verifyErr != nil {
			callbackFailure(w, 401, "google_verification")
			return
		}
		if a.Invitation != "" {
			if h.c.Invitations == nil {
				callbackFailure(w, 403, "invitation_service_unavailable")
				return
			}
			if _, err := h.c.Invitations.AcceptGoogleInvitation(r.Context(), id, a.Invitation); err != nil {
				callbackFailure(w, 403, "invitation_acceptance")
				return
			}
		}
		if h.c.Registration != nil {
			status, err := h.c.Registration.RegisterGoogleAccess(r.Context(), id, h.c.RegistrationTeam)
			if err != nil {
				callbackFailure(w, 403, "access_registration")
				return
			}
			if status != "active" {
				cookie(w, sessionCookie, "", -1)
				destination := "/access-pending"
				if status == "rejected" {
					destination = "/access-declined"
				}
				http.Redirect(w, r, destination, http.StatusSeeOther)
				return
			}
		}
		if a.Native != nil {
			// Only a one-use application authorization code reaches the loopback URL.
			code, err := randomHandle()
			if err != nil {
				callbackFailure(w, 503, "native_code_generation")
				return
			}
			grant := nativeCode{Kind: "cli-code", Issuer: id.Issuer, Subject: id.Subject, Challenge: a.Native.Challenge}
			if h.c.Store.PutAttempt(r.Context(), "code:"+code, grant) != nil {
				callbackFailure(w, 503, "native_code_storage")
				return
			}
			q := url.Values{"code": {code}, "state": {a.Native.State}}
			http.Redirect(w, r, login.Callback+"?"+q.Encode(), 303)
			return
		}
		raw, grant, issueErr := h.c.ApplicationSessions.IssueApplicationSession(r.Context(), id.Issuer, id.Subject, 12*time.Hour)
		if issueErr != nil {
			callbackFailure(w, 403, "membership_session")
			return
		}
		token = &oauth2.Token{AccessToken: raw, TokenType: "Bearer", Expiry: grant.ExpiresAt}
	} else {
		token, err = h.c.Web.Exchange(context.WithValue(r.Context(), oauth2.HTTPClient, h.client), q.Get("code"), oauth2.VerifierOption(a.Verifier))
		if err != nil || token == nil || !token.Valid() || token.RefreshToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
			callbackFailure(w, 401, "legacy_exchange")
			return
		}
	}
	req := r.Clone(r.Context())
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	identity, _, err := h.c.Boundary.AuthenticateIdentity(req)
	if err != nil {
		callbackFailure(w, 403, "session_authentication")
		return
	}
	fresh, err := randomHandle()
	if err != nil {
		callbackFailure(w, 503, "session_handle")
		return
	}
	if h.c.Store.Create(r.Context(), fresh, session{*token, identity}, time.Now().Add(12*time.Hour)) != nil {
		callbackFailure(w, 503, "session_storage")
		return
	}
	if old := readCookie(r, sessionCookie); old != "" {
		if h.c.Store.Delete(r.Context(), old) != nil {
			callbackFailure(w, 503, "old_session_removal")
			return
		}
	}
	cookie(w, sessionCookie, fresh, 12*60*60)
	http.Redirect(w, r, "/?mode=connected", 303)
}
