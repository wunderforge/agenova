// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/authentication"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/policy"
)

func TestDurableSessionLogoutCancelsRunningWork(t *testing.T) {
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("session_work_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := identitydb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := identitydb.Store{DB: db}
	member := identitydb.Member{Issuer: "https://accounts.google.com", Subject: "alice-google", Principal: "alice", Team: "team-a", Active: true}
	if err := store.PutMember(ctx, "operator", member); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments'); INSERT INTO agenova_project_grants VALUES('alice','team-a','payments','member')`); err != nil {
		t.Fatal(err)
	}
	raw, id, err := store.IssueApplicationSession(ctx, member.Issuer, member.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := store.IssueApplicationSession(ctx, member.Issuer, member.Subject, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	boundary, err := authentication.New(identitydb.ApplicationSessions{Store: store}, store)
	if err != nil {
		t.Fatal(err)
	}
	provider := &httpProvider{started: make(chan struct{}), release: make(chan struct{})}
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	service, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{Setup: memberTestSetup, Configure: memberTestConfigure, Access: store, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	handler, err := AuthenticatedHandler(service, boundary)
	if err != nil {
		t.Fatal(err)
	}
	got := httpCall(handler, "POST", "/api/requests", httpInput("session-work"), map[string]string{"Authorization": "Bearer " + raw})
	if got.Code != 202 {
		t.Fatal(got.Code, got.Body.String())
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}
	if err := store.Revoke(ctx, id); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(6 * time.Second)
	cancelled := false
	for time.Now().Before(deadline) {
		view, _ := service.QueryRequest("session-work")
		if view.Outcome != nil {
			cancelled = view.Outcome.Status == "Cancelled"
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !cancelled || provider.calls.Load() != 1 {
		t.Fatal("logout did not cancel within bound, or repeated provider")
	}
	if got := httpCall(handler, "POST", "/api/requests", httpInput("revoked-followup"), map[string]string{"Authorization": "Bearer " + raw}); got.Code != 401 {
		t.Fatal("revoked session retained access")
	}
	r := httptest.NewRequest("GET", "https://app.example/auth/me", nil)
	r.Header.Set("Authorization", "Bearer "+other)
	if _, err := boundary.Authenticate(r); err != nil {
		t.Fatal("logout revoked an unrelated login")
	}
}
