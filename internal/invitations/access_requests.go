// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"io"
	"mime"
	"net/http"
)

type AccessRequest struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Status string `json:"status"`
}
type AccessReview struct {
	PlatformRole string `json:"platformRole,omitempty"`
	Installation string `json:"-"`
	ID           string `json:"id"`
	Action       string `json:"action"`
	AccountRole  string `json:"accountRole"`
	Project      string `json:"project"`
	Role         string `json:"role"`
}
type AccessRequests interface {
	ListAccessRequests(context.Context, v0.Principal) ([]AccessRequest, error)
	ReviewAccessRequest(context.Context, v0.Principal, AccessReview) error
}

func AccessRequestHandler(store AccessRequests, auth Authenticator, installation ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
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
			http.Error(w, "Access review rejected", status)
		}
		switch r.Method {
		case "GET":
			rows, err := store.ListAccessRequests(r.Context(), p)
			if err != nil {
				fail(err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rows)
		case "POST":
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				http.Error(w, "Use application/json", 415)
				return
			}
			var change AccessReview
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
			d.DisallowUnknownFields()
			if d.Decode(&change) != nil || d.Decode(new(any)) != io.EOF {
				fail(ErrInvalid)
				return
			}
			if change.PlatformRole != "" {
				if len(installation) != 1 || installation[0] == "" {
					fail(ErrInvalid)
					return
				}
				change.Installation = installation[0]
			}
			if err := store.ReviewAccessRequest(r.Context(), p, change); err != nil {
				fail(err)
				return
			}
			w.WriteHeader(204)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", 405)
		}
	})
}
