// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"

	"github.com/wunderforge/agenova/internal/app"

	"github.com/wunderforge/agenova/internal/management"
	"io"
	"mime"
	"net/http"
	"os"
)

func (d *memberDependencies) governanceHandler() http.Handler {
	return management.Governance(d.store, d.boundary, app.ValidateControlledTemplate)
}
func (d *memberDependencies) organizationHandler() http.Handler {
	return management.Organization(d.store, d.boundary)
}

func (d *memberDependencies) platformAuthorizationHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := d.boundary.Authenticate(r)
		if err != nil {
			http.Error(w, "Sign in required", 401)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Use POST", 405)
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			http.Error(w, "Use JSON", 415)
			return
		}
		var input struct {
			Installation string `json:"installation"`
			Action       string `json:"action"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "Invalid request", 400)
			return
		}
		if input.Action == "grafana" {
			installation := os.Getenv("AGENOVA_PLATFORM_NAME")
			if installation == "" || input.Installation != installation {
				http.Error(w, "Installation access denied", 403)
				return
			}
			role, err := d.store.GrafanaRole(r.Context(), p, installation)
			if err != nil {
				http.Error(w, "Diagnostics access denied", 403)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"role": role})
			return
		}
		action := ""
		switch input.Action {
		case "validate", "plan", "apply":
			action = "platform.configure"
		case "status":
			action = "platform.diagnose"
		default:
			http.Error(w, "Unknown action", 400)
			return
		}
		if d.store.AuthorizeInstallation(r.Context(), p, action, input.Installation, os.Getenv("AGENOVA_PLATFORM_NAME")) != nil {
			http.Error(w, "Installation access denied", 403)
			return
		}
		// Authorization does not execute deployment or return cluster credentials.
		w.WriteHeader(http.StatusNoContent)
	})
}
