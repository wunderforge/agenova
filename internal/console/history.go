// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/history"
	"net/http"
	"strconv"
	"strings"
)

// ReadBoundary is injected by trusted server composition. There is deliberately
// no built-in header, query parameter, or fixed-principal authentication here.
type ReadBoundary struct {
	Viewer     func(*http.Request) (v0.Principal, error)
	Authorizer history.Authorizer
}

// HandlerWithReadBoundary applies one E14-compatible boundary to all read paths.
func HandlerWithReadBoundary(service *Service, boundary ReadBoundary) http.Handler {
	legacy := Handler(service)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path == "/api/setup" {
			legacy.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if !sameOrigin(r) {
			writeError(w, 403, "origin_rejected", "Use the same-origin console.")
			return
		}
		if boundary.Viewer == nil || boundary.Authorizer == nil {
			writeError(w, 401, "viewer_required", "Verified viewer required.")
			return
		}
		viewer, err := boundary.Viewer(r)
		if err != nil || viewer.Subject == "" {
			writeError(w, 401, "viewer_required", "Verified viewer required.")
			return
		}
		if service == nil || service.repository == nil {
			writeError(w, 503, "history_unavailable", "History is unavailable.")
			return
		}
		if r.URL.Path == "/api/history" {
			limit := 20
			if raw := r.URL.Query().Get("limit"); raw != "" {
				limit, err = strconv.Atoi(raw)
				if err != nil || limit < 1 || limit > 100 {
					writeError(w, 400, "invalid_cursor", "Invalid history page.")
					return
				}
			}
			page, err := service.repository.History(r.Context(), viewer, boundary.Authorizer, limit, r.URL.Query().Get("cursor"))
			if err != nil {
				if err == history.ErrCursor {
					writeError(w, 400, "invalid_cursor", "Invalid history page.")
				} else {
					writeError(w, 503, "history_unavailable", "History is unavailable.")
				}
				return
			}
			writeJSON(w, 200, page)
			return
		}
		// Retain the legacy list wire shape, but authorize before materializing views.
		if r.URL.Path == "/api/requests" {
			page, err := service.repository.History(r.Context(), viewer, boundary.Authorizer, 32, "")
			if err != nil {
				writeError(w, 503, "history_unavailable", "History is unavailable.")
				return
			}
			views := []any{}
			for _, item := range page.Items {
				v, err := history.Read(r.Context(), service.repository, viewer, boundary.Authorizer, item.RequestRef)
				if err != nil {
					writeError(w, 503, "history_unavailable", "History is unavailable.")
					return
				}
				views = append(views, v)
			}
			writeJSON(w, 200, views)
			return
		}
		// Authorize the canonical request/claim reference before delegating route validation.
		ref, ok := evidenceRoute(r.URL.Path)
		if ok {
			if !validReference(ref) {
				writeError(w, 400, "invalid_reference", "Provide a bounded record reference.")
				return
			}
			_, err := history.Read(r.Context(), service.repository, viewer, boundary.Authorizer, ref)
			if err != nil {
				if err == history.ErrNotFound {
					writeError(w, 404, "not_found", "The record was not found.")
				} else {
					writeError(w, 503, "history_unavailable", "History is unavailable.")
				}
				return
			}
		}
		legacy.ServeHTTP(w, r)
	})
}
func (s *Service) History(ctx context.Context, p v0.Principal, a history.Authorizer, limit int, cursor string) (history.Page, error) {
	if s.repository == nil {
		return history.Page{}, history.ErrUnavailable
	}
	return s.repository.History(ctx, p, a, limit, cursor)
}

func evidenceRoute(path string) (string, bool) {
	for _, prefix := range []string{"/api/requests/", "/api/claims/"} {
		if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, "/evidence") {
			return strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/evidence"), true
		}
	}
	return "", false
}
