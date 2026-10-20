// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package landing

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExactPublicAllowlist(t *testing.T) {
	dir := t.TempDir()
	for _, item := range files {
		file := filepath.Join(dir, "landing", filepath.FromSlash(item.file))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("public fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "landing", "private.txt"), []byte("not public"), 0600); err != nil {
		t.Fatal(err)
	}
	h := New(dir)
	for path, item := range files {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, "https://service.example"+path, nil))
			if w.Code != 200 || w.Header().Get("Content-Type") != item.mime || w.Header().Get("Content-Length") != strconv.Itoa(len("public fixture")) {
				t.Fatalf("%s %s: %d", method, path, w.Code)
			}
			if method == "GET" && w.Body.String() != "public fixture" || method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("incorrect public body", method, path)
			}
			if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src 'none'") {
				t.Fatal("missing public security headers")
			}
		}
	}
	for _, path := range []string{"/api/requests", "/admin", "/assets/index.js", "/landing/private.txt", "/landing/assets/", "/landing/.env", "/landing/../index.html", "/landing/%2e%2e/index.html"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "https://service.example"+path, nil))
		if w.Code != 404 || strings.Contains(w.Body.String(), "not public") {
			t.Fatal("unlisted file exposed", path, w.Code)
		}
	}
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "https://service.example/", nil))
		if w.Code != 405 {
			t.Fatal("public mutation accepted", method)
		}
	}
}

func TestMissingLandingNeverFallsBackToPortal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("private portal"), 0600)
	w := httptest.NewRecorder()
	New(dir).ServeHTTP(w, httptest.NewRequest("GET", "https://service.example/", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private portal") || strings.Contains(w.Body.String(), dir) {
		t.Fatal("missing landing exposed private content or path", w.Code)
	}
}

func TestReviewedLandingUsesExistingLoginOnly(t *testing.T) {
	h := New(filepath.Join("..", "..", "ui", "public"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "https://service.example/", nil))
	page := w.Body.String()
	if w.Code != 200 || strings.Count(page, `href="/login"`) != 3 || !strings.Contains(page, "illustrative test data") {
		t.Fatal("public product/login contract missing")
	}
	for _, forbidden := range []string{"data-signin", "signin-dialog", "Sign-in is not connected", "agenova-account-navigation", "/api/setup"} {
		if strings.Contains(page, forbidden) {
			t.Fatal("mock behavior or private marker remains", forbidden)
		}
	}
}
