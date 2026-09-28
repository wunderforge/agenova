// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	now := time.Now()
	hash := sha256.Sum256([]byte("test-password"))
	a := &auth{csrfKey: []byte("test-key"), credentials: credentials{"demo", hex.EncodeToString(hash[:])}, sessions: map[string]time.Time{}, now: func() time.Time { return now }}
	request := func(method, path, source, password string, cookie *http.Cookie) *httptest.ResponseRecorder {
		now = now.Add(time.Second)
		form := httptest.NewRecorder()
		a.render(form, 200, "", false)
		csrf := form.Result().Cookies()[0]
		r := httptest.NewRequest(method, path, strings.NewReader(url.Values{"username": {"demo"}, "password": {password}, "csrf": {csrf.Value}}.Encode()))
		r.AddCookie(csrf)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", source)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/check", "", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	for _, source := range []string{"https://attacker.example"} {
		if w := request("POST", "/login", source, "test-password", nil); w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	if w := request("POST", "/login", origin, "bad", nil); w.Code != 401 || w.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("bad login")
	}
	w := request("POST", "/login", "null", "test-password", nil)
	if w.Code != 303 {
		t.Fatal(w.Code)
	}
	c := w.Result().Cookies()[0]
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatal("unsafe cookie")
	}
	if w := request("GET", "/check", "", "", c); w.Code != 204 {
		t.Fatal(w.Code)
	}
	bad := *c
	bad.Value += "tampered"
	if w := request("GET", "/check", "", "", &bad); w.Code != 401 {
		t.Fatal("tamper accepted")
	}
	if w := request("POST", "/logout", "https://attacker.example", "", c); w.Code != 403 {
		t.Fatal("logout csrf")
	}
	request("POST", "/logout", origin, "", c)
	if w := request("GET", "/check", "", "", c); w.Code != 401 {
		t.Fatal("logout not revoked")
	}
	w = request("POST", "/login", origin, "test-password", nil)
	c = w.Result().Cookies()[0]
	now = now.Add(lifetime)
	if w := request("GET", "/check", "", "", c); w.Code != 401 {
		t.Fatal("expiry ignored")
	}
}

func TestMissingOrForgedCSRF(t *testing.T) {
	a := &auth{csrfKey: []byte("test-key"), sessions: map[string]time.Time{}, now: time.Now}
	for _, value := range []string{"", "forged.9999999999.bad"} {
		r := httptest.NewRequest("POST", "/login", strings.NewReader(url.Values{"csrf": {value}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "null")
		r.AddCookie(&http.Cookie{Name: "__Host-agenova_csrf", Value: value})
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("forged csrf accepted: %d", w.Code)
		}
	}
	now := time.Now()
	a.now = func() time.Time { return now }
	w := httptest.NewRecorder()
	a.render(w, 200, "", false)
	value := w.Result().Cookies()[0].Value
	now = now.Add(21 * time.Minute)
	if a.validCSRF(value) {
		t.Fatal("expired csrf accepted")
	}
}
