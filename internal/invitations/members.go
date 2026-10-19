// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package invitations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

type ProjectGrant struct {
	Project string `json:"project"`
	Role    string `json:"role"`
}
type ScopedRole struct {
	Role      string `json:"role"`
	ScopeKind string `json:"scopeKind"`
	ScopeRef  string `json:"scopeRef"`
}

type MemberView struct {
	Roles       []ScopedRole   `json:"roles"`
	Principal   string         `json:"principal"`
	Email       string         `json:"email"`
	AccountRole string         `json:"accountRole"`
	Active      bool           `json:"active"`
	Projects    []ProjectGrant `json:"projects"`
}
type MemberChange struct {
	Confirmation string `json:"confirmation,omitempty"`
	ScopeKind    string `json:"scopeKind,omitempty"`
	ScopeRef     string `json:"scopeRef,omitempty"`
	Principal    string `json:"principal"`
	Action       string `json:"action"`
	Role         string `json:"role,omitempty"`
	Project      string `json:"project,omitempty"`
}
type MemberStore interface {
	ListMembers(context.Context, v0.Principal) ([]MemberView, error)
	ChangeMember(context.Context, v0.Principal, MemberChange) error
}

// MemberHandler is behind the session BFF's origin checks. Identity/team are
// taken only from authentication, never from a management payload.
func MemberHandler(store MemberStore, auth Authenticator, installation ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if store == nil || auth == nil {
			http.Error(w, "Unavailable", 503)
			return
		}
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
			http.Error(w, "Member operation rejected", status)
		}
		switch r.Method {
		case "GET":
			rows, err := store.ListMembers(r.Context(), p)
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
			var change MemberChange
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
			d.DisallowUnknownFields()
			if d.Decode(&change) != nil || d.Decode(new(any)) != io.EOF {
				fail(ErrInvalid)
				return
			}
			if change.Action == "platform-role" {
				if change.ScopeRef != "" || len(installation) != 1 || installation[0] == "" {
					fail(ErrInvalid)
					return
				}
				change.ScopeRef = installation[0]
			}
			if err := store.ChangeMember(r.Context(), p, change); err != nil {
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
