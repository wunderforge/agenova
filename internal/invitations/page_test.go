// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJuneAdministrationPreservesPageBoundary(t *testing.T) {
	h := Page(Service{Store: pageStore{}, Registration: true}, memberAuth{})
	unauthenticated := httptest.NewRecorder()
	h.ServeHTTP(unauthenticated, httptest.NewRequest("GET", "https://app.example/admin", nil))
	if unauthenticated.Code != 401 || strings.Contains(unauthenticated.Body.String(), "governance-document") {
		t.Fatal("anonymous caller received administration content")
	}
	r := httptest.NewRequest("GET", "https://app.example/admin", nil)
	r.Header.Set("Authorization", "Bearer member")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("page unavailable or private cache protection changed")
	}
	for _, marker := range []string{`name="color-scheme" content="only light"`, `name="darkreader-lock"`, `data-registration="true"`, `id="admin-main"`, `id="governance-document"`, `method="post" action="/logout"`} {
		if !strings.Contains(w.Body.String(), marker) {
			t.Fatalf("missing presentation or existing control: %s", marker)
		}
	}
	for directive, source := range map[string]string{"style-src": pageCSS, "script-src": pageJS} {
		hash := sha256.Sum256([]byte(source))
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), directive+" 'sha256-"+base64.StdEncoding.EncodeToString(hash[:])+"'") {
			t.Fatalf("embedded %s no longer matches CSP hash", directive)
		}
	}
	if strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("presentation change loosened CSP")
	}
}
