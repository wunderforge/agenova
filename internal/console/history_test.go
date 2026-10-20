// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/history"
	sqlstore "github.com/wunderforge/agenova/internal/history/sqlite"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHistoryReadBoundaryDeniesMissingAndForeignViewers(t *testing.T) {
	r, err := sqlstore.Open(filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	s, err := NewServiceWithOptions(&verticalBackend{}, &verticalExecutor{}, &verticalProvider{}, app.ReferencePrincipalTeamA, Options{Repository: r})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Submit(verticalRequest(t, "owned")); err != nil {
		t.Fatal(err)
	}
	awaitVertical(t, s, "owned")
	req := httptest.NewRequest(http.MethodGet, "http://localhost/api/history", nil)
	response := httptest.NewRecorder()
	HandlerWithReadBoundary(s, ReadBoundary{}).ServeHTTP(response, req)
	if response.Code != 401 {
		t.Fatal("missing viewer accepted")
	}
	auth := history.AuthorizerFunc(func(_ context.Context, p v0.Principal, a history.Admission) bool {
		return p.Subject == a.Principal.Subject
	})
	boundary := ReadBoundary{Viewer: func(*http.Request) (v0.Principal, error) { return v0.Principal{Subject: "someone-else"}, nil }, Authorizer: auth}
	response = httptest.NewRecorder()
	HandlerWithReadBoundary(s, boundary).ServeHTTP(response, req)
	if response.Code != 200 || response.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("foreign history exposed: %s", response.Body.String())
	}
	for _, ref := range []string{"owned", "absent"} {
		response = httptest.NewRecorder()
		HandlerWithReadBoundary(s, boundary).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/api/requests/"+ref+"/evidence", nil))
		if response.Code != 404 || response.Body.String() != "{\"code\":\"not_found\",\"message\":\"The record was not found.\"}\n" {
			t.Fatal("existence disclosure")
		}
	}
	source, _ := app.NewReferencePrincipalSource(app.ReferencePrincipalTeamA)
	boundary.Viewer = func(*http.Request) (v0.Principal, error) { return source.Principal(), nil }
	response = httptest.NewRecorder()
	HandlerWithReadBoundary(s, boundary).ServeHTTP(response, req)
	if response.Code != 200 || response.Body.Len() < 40 {
		t.Fatal("allowed positive control failed")
	}
}
