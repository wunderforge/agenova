// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Package management transports authenticated project configuration; it does not
// provide authentication or grant claim authority.
package management

import (
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/invitations"
	"github.com/wunderforge/agenova/internal/registration"
	"io"
	"mime"
	"net/http"
)

type Authenticator interface {
	Authenticate(*http.Request) (v0.Principal, error)
}

func reject(w http.ResponseWriter, err error) {
	status := 503
	switch {
	case errors.Is(err, authentication.ErrForbidden):
		status = 403
	case errors.Is(err, registration.ErrConflict):
		status = 409
	case errors.Is(err, invitations.ErrInvalid):
		status = 400
	}
	http.Error(w, "Management operation rejected; reload state before retrying", status)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		http.Error(w, "Use JSON", 415)
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, (1<<20)+4096))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return false
	}
	return true
}
func Governance(store identitydb.Store, auth Authenticator, validate func(*v0.AgentTemplate) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if auth == nil || store.DB == nil {
			http.Error(w, "Unavailable", 503)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		if r.Method == "GET" {
			history, err := store.PolicyHistoryFor(r.Context(), p, r.URL.Query().Get("project"))
			if err != nil {
				reject(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(history)
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		var in struct {
			Project        string              `json:"project"`
			Kind           string              `json:"kind"`
			Document       string              `json:"document"`
			ExpectedActive *v0.PolicyReference `json:"expectedActive,omitempty"`
		}
		if !decode(w, r, &in) {
			return
		}
		result, err := store.ApplyGovernance(r.Context(), p, in.Project, in.Kind, []byte(in.Document), validate, in.ExpectedActive)
		if err != nil {
			if errors.Is(err, authentication.ErrForbidden) || errors.Is(err, registration.ErrConflict) {
				reject(w, err)
			} else {
				http.Error(w, "Configuration rejected; check schema, limits and installed capabilities", 400)
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
}
func Organization(store identitydb.Store, auth Authenticator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if auth == nil || store.DB == nil {
			http.Error(w, "Unavailable", 503)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		if r.Method == "GET" {
			value, err := store.Organization(r.Context(), p)
			if err != nil {
				reject(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(value)
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		var in struct {
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err = store.UpdateOrganization(r.Context(), p, in.DisplayName, in.Description); err != nil {
			reject(w, err)
			return
		}
		w.WriteHeader(204)
	})
}
