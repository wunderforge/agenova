// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

type Authenticator interface {
	Authenticate(*http.Request) (v0.Principal, error)
}

// Handler is mounted behind the BFF's origin/CSRF checks and still independently
// validates the caller. It accepts no caller-authored principal, issuer or team.
func Handler(s Service, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if auth == nil || s.Store == nil {
			http.Error(w, "Unavailable", 503)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Access denied", 401)
			return
		}
		fail := func(err error) {
			status := 503
			if errors.Is(err, authentication.ErrForbidden) {
				status = 403
			}
			if errors.Is(err, ErrInvalid) {
				status = 400
			}
			http.Error(w, "Invitation operation failed; inspect invitation status before retrying.", status)
		}
		switch r.Method {
		case "GET":
			result, err := s.Store.ListInvitations(r.Context(), p)
			if err != nil {
				fail(err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result)
		case "POST":
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				http.Error(w, "Use application/json", 415)
				return
			}
			var request Request
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
			d.DisallowUnknownFields()
			if d.Decode(&request) != nil || d.Decode(new(any)) != io.EOF {
				fail(ErrInvalid)
				return
			}
			result, err := s.Invite(r.Context(), p, request)
			if err != nil {
				fail(err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(result)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", 405)
		}
	})
}
