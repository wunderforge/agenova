// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Demo-only shared perimeter authentication; not an application user identity.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const origin = "https://demo.agenova.app"
const cookieName = "__Host-agenova_demo"
const lifetime = 8 * time.Hour

type credentials struct {
	Username     string
	PasswordHash string
}
type auth struct {
	credentials credentials
	csrfKey     []byte
	mu          sync.Mutex
	sessions    map[string]time.Time
	nextLogin   time.Time
	now         func() time.Time
}

func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (a *auth) valid(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, ok := a.sessions[c.Value]
	return ok && a.now().Before(expires)
}
func (a *auth) cookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: value, Path: "/", MaxAge: age, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

var page = template.Must(template.New("login").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Agenova — Sign in</title><style>body{margin:0;background:#0b1220;color:#e5edf9;font:16px system-ui;display:grid;place-items:center;min-height:100vh}main{width:min(360px,85vw);padding:32px;background:#152039;border-radius:16px}h1{margin-top:0}label{display:block;margin:18px 0 6px}input,button{box-sizing:border-box;width:100%;padding:12px;border-radius:8px;border:1px solid #66758f;font:inherit}button{margin-top:24px;background:#2563eb;color:white;cursor:pointer}p{color:#bac8de;line-height:1.5}a{color:#93c5fd}.error{color:#ffb4b4}</style><main><h1>Agenova</h1>{{if .SignedIn}}<p>You are signed in to the team demo.</p><a href="/?mode=connected#/work">Open Portal</a><form action="/logout" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><button>Sign out</button></form>{{else}}<p>Sign in to the team demo.</p>{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}<form action="/login" method="post"><input type="hidden" name="csrf" value="{{.CSRF}}"><label for="username">Username</label><input id="username" name="username" autocomplete="username" required autofocus><label for="password">Password</label><input id="password" name="password" type="password" autocomplete="current-password" required><button>Sign in</button></form>{{end}}<p>Shared demo access · not an individual user account.</p></main></html>`))

func (a *auth) render(w http.ResponseWriter, status int, err string, signed bool) {
	nonce := token() + "." + strconv.FormatInt(a.now().Add(20*time.Minute).Unix(), 10)
	mac := hmac.New(sha256.New, a.csrfKey)
	mac.Write([]byte(nonce))
	nonce += "." + hex.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{Name: "__Host-agenova_csrf", Value: nonce, Path: "/", MaxAge: 1200, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = page.Execute(w, struct {
		Error    string
		CSRF     string
		SignedIn bool
	}{err, nonce, signed})
}
func (a *auth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if r.URL.Path == "/check" {
		if a.valid(r) {
			w.WriteHeader(204)
		} else {
			w.WriteHeader(401)
		}
		return
	}
	if r.URL.Path != "/login" && r.URL.Path != "/logout" {
		http.NotFound(w, r)
		return
	}
	if r.Method == "GET" {
		a.render(w, 200, "", a.valid(r))
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	source := r.Header.Get("Origin")
	csrfCookie, err := r.Cookie("__Host-agenova_csrf")
	nonce := r.PostForm.Get("csrf")
	if (source != "" && source != "null" && source != origin) || err != nil || !hmac.Equal([]byte(nonce), []byte(csrfCookie.Value)) || !a.validCSRF(nonce) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.URL.Path == "/logout" {
		if c, err := r.Cookie(cookieName); err == nil {
			a.mu.Lock()
			delete(a.sessions, c.Value)
			a.mu.Unlock()
		}
		a.cookie(w, "", -1)
		http.Redirect(w, r, "/login", 303)
		return
	}
	a.mu.Lock()
	now := a.now()
	limited := now.Before(a.nextLogin)
	if !limited {
		a.nextLogin = now.Add(200 * time.Millisecond)
	}
	a.mu.Unlock()
	if limited {
		w.Header().Set("Retry-After", "1")
		a.render(w, 429, "Please wait a moment and try again.", false)
		return
	}
	hash := sha256.Sum256([]byte(r.PostForm.Get("password")))
	goodPassword := subtle.ConstantTimeCompare([]byte(hex.EncodeToString(hash[:])), []byte(a.credentials.PasswordHash)) == 1
	goodUser := subtle.ConstantTimeCompare([]byte(r.PostForm.Get("username")), []byte(a.credentials.Username)) == 1
	if !goodPassword || !goodUser {
		a.render(w, 401, "Incorrect username or password.", false)
		return
	}
	session := token()
	a.mu.Lock()
	for key, expiry := range a.sessions {
		if !now.Before(expiry) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= 1000 {
		a.mu.Unlock()
		http.Error(w, "Session limit reached", 503)
		return
	}
	if c, err := r.Cookie(cookieName); err == nil {
		delete(a.sessions, c.Value)
	}
	a.sessions[session] = now.Add(lifetime)
	a.mu.Unlock()
	a.cookie(w, session, int(lifetime.Seconds()))
	http.Redirect(w, r, "/?mode=connected#/work", 303)
}
func (a *auth) validCSRF(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || a.now().Unix() >= expiry {
		return false
	}
	mac := hmac.New(sha256.New, a.csrfKey)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	return hmac.Equal([]byte(parts[2]), []byte(hex.EncodeToString(mac.Sum(nil))))
}
func main() {
	data, err := os.ReadFile("/auth/session.json")
	if err != nil {
		log.Fatal("Cannot read demo credential verifier")
	}
	a := &auth{csrfKey: []byte(token()), sessions: map[string]time.Time{}, now: time.Now}
	if json.Unmarshal(data, &a.credentials) != nil || a.credentials.Username == "" || len(a.credentials.PasswordHash) != 64 {
		log.Fatal("Invalid demo credential verifier")
	}
	server := &http.Server{Addr: "127.0.0.1:8090", Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	log.Fatal(server.ListenAndServe())
}
