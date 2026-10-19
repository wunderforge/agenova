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
	"github.com/wunderforge/agenova/internal/login"
)

type InvitationManager interface {
	ManageGoogleInvitation(context.Context, v0.Principal, string, string) (Invitation, string, error)
}

func ManageHandler(store InvitationManager, auth Authenticator, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if store == nil || auth == nil || !login.Origin(origin) {
			http.Error(w, "Unavailable", 503)
			return
		}
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return
		}
		p, err := auth.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			http.Error(w, "Use application/json", 415)
			return
		}
		var change struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&change) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "Invalid operation", 400)
			return
		}
		i, proof, err := store.ManageGoogleInvitation(r.Context(), p, change.ID, change.Action)
		if err != nil {
			status := 503
			if errors.Is(err, authentication.ErrForbidden) {
				status = 403
			}
			if errors.Is(err, ErrInvalid) {
				status = 400
			}
			http.Error(w, "Invitation change rejected", status)
			return
		}
		if proof != "" {
			i.AcceptanceURL = origin + "/login#invite=" + proof
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(i)
	})
}
