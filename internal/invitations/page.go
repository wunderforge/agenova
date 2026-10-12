// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
)

//go:embed page.html
var pageHTML string

//go:embed page.css
var pageCSS string

//go:embed page.js
var pageJS string

func Page(s Service, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if s.Store == nil || auth == nil {
			http.Error(w, "Unavailable", 503)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		if _, err = s.Store.ListInvitations(r.Context(), p); err != nil {
			http.Error(w, "Administrator access required", 403)
			return
		}
		css := sha256.Sum256([]byte(pageCSS))
		js := sha256.Sum256([]byte(pageJS))
		w.Header().Set("Content-Security-Policy", "default-src 'none'; connect-src 'self'; style-src 'sha256-"+base64.StdEncoding.EncodeToString(css[:])+"'; script-src 'sha256-"+base64.StdEncoding.EncodeToString(js[:])+"'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, strings.Replace(strings.Replace(pageHTML, "{{styles}}", pageCSS, 1), "{{script}}", pageJS, 1))
	})
}
