// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/connectedclient"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/workstore"
)

func durableTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("durable_console_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
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
	t.Cleanup(func() { db.Close() })
	if err := identitydb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDurableWorkSurvivesServiceRestartWithoutRedispatch(t *testing.T) {
	testDurableRestart(t, false)
}
func TestDurableCompletedWorkRepairsLostFinalAcknowledgement(t *testing.T) {
	testDurableRestart(t, true)
}

type failedFinishStore struct{ *workstore.Store }

func (failedFinishStore) Finish(context.Context, evidence.View) error {
	return workstore.ErrUnavailable
}
func testDurableRestart(t *testing.T, lostFinish bool) {
	input := func() []byte {
		return []byte(strings.Replace(string(httpInput("durable-run")), `"repository":"acme/payments"`, `"repository":"acme/payments","attempts":3`, 1))
	}
	db := durableTestDB(t)
	ctx := context.Background()
	identity := identitydb.Store{DB: db}
	p := v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}
	if err := identity.PutMember(ctx, "operator", identitydb.Member{Issuer: "https://accounts.google.com", Subject: "alice-google", Principal: p.Subject, Team: p.Team, Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments'); INSERT INTO agenova_project_grants VALUES('alice','team-a','payments','member')`); err != nil {
		t.Fatal(err)
	}
	provider := &httpProvider{}
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	firstProcess := true
	newService := func() (*Service, *workstore.Store) {
		store, err := workstore.Acquire(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		var durable DurableWork = store
		if firstProcess && lostFinish {
			durable = failedFinishStore{store}
		}
		firstProcess = false
		service, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, Options{Durable: durable, Setup: memberTestSetup, Configure: memberTestConfigure, Access: identity, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
			return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
		}})
		if err != nil {
			store.Close()
			t.Fatal(err)
		}
		t.Cleanup(func() { service.Close(); store.Close() })
		return service, store
	}
	service, store := newService()
	raw, _, err := identity.IssueApplicationSession(ctx, "https://accounts.google.com", "alice-google", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	boundary, err := authentication.New(identitydb.ApplicationSessions{Store: identity}, identity)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := AuthenticatedHandler(service, boundary)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	client := connectedclient.Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return raw, nil }, PollInterval: time.Millisecond}
	path := filepath.Join(t.TempDir(), "work.yaml")
	if err := os.WriteFile(path, input(), 0600); err != nil {
		t.Fatal(err)
	}
	var first evidence.View
	if lostFinish {
		first, err = service.SubmitFor(input(), p)
	} else {
		first, err = client.RunFile(path)
	}
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if lostFinish && service.storageFailed.Load() {
			break
		}
		view, err := service.QueryFor(p, first.RequestRef, false)
		if err != nil {
			t.Fatal(err)
		}
		if view.Outcome != nil {
			if view.Outcome.Status != "Succeeded" {
				t.Fatalf("outcome: %#v", view.Outcome)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("durable outcome not published")
		}
		time.Sleep(10 * time.Millisecond)
	}
	service.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, _ := newService()
	view, err := restarted.QueryFor(p, first.RequestRef, false)
	if err != nil || view.Outcome == nil || view.Outcome.Status != "Succeeded" || len(view.Facts) == 0 {
		t.Fatal("lost evidence", err)
	}
	claimView, err := restarted.QueryFor(p, view.State.Claim.ID, true)
	if err != nil || claimView.RequestRef != view.RequestRef {
		t.Fatal("lost claim lookup", err)
	}
	repeated, err := restarted.SubmitFor(input(), p)
	if err != nil || repeated.RequestRef != first.RequestRef || provider.calls.Load() != 1 {
		t.Fatal("redispatched", err, provider.calls.Load())
	}
	changed := strings.Replace(string(input()), "Explain retry backoff", "Different task", 1)
	if _, err := restarted.SubmitFor([]byte(changed), p); err != workstore.ErrConflict {
		t.Fatal("changed input reused name", err)
	}
	if _, err := restarted.QueryFor(v0.Principal{Subject: "bob", Team: "team-a", AuthenticationContext: "oidc"}, first.RequestRef, false); err == nil {
		t.Fatal("foreign member read evidence")
	}
}

// Inject a failed durable commit at the last boundary before a provider call.
type failedAttemptStore struct {
	*workstore.Store
	failed atomic.Bool
}

func (s *failedAttemptStore) Append(ctx context.Context, f facts.Fact, state *v0.IssuedState) error {
	if f.Kind == "ProviderAttempt" {
		s.failed.Store(true)
	}
	if s.failed.Load() {
		return workstore.ErrUnavailable
	}
	return s.Store.Append(ctx, f, state)
}
func TestDurableFactFailurePreventsProviderAndRestartReplay(t *testing.T) {
	db := durableTestDB(t)
	ctx := context.Background()
	identity := identitydb.Store{DB: db}
	p := v0.Principal{Subject: "alice", Team: "team-a", AuthenticationContext: "oidc"}
	if err := identity.PutMember(ctx, "operator", identitydb.Member{Issuer: "google", Subject: "alice", Principal: p.Subject, Team: p.Team, Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments'); INSERT INTO agenova_project_grants VALUES('alice','team-a','payments','member')`); err != nil {
		t.Fatal(err)
	}
	store, err := workstore.Acquire(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	provider := &httpProvider{}
	options := Options{Durable: &failedAttemptStore{Store: store}, Setup: memberTestSetup, Configure: memberTestConfigure, Access: identity, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
		return app.PrepareAssignment(data, principalSource{p}, loader, registeredTemplate{})
	}}
	service, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, options)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	first, err := service.SubmitFor(httpInput("uncommitted-attempt"), p)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !service.storageFailed.Load() {
		if time.Now().After(deadline) {
			t.Fatal("fault did not reach durable boundary")
		}
		time.Sleep(time.Millisecond)
	}
	service.Close()
	if provider.calls.Load() != 0 {
		t.Fatal("provider called without committed attempt")
	}
	if _, err := service.SubmitFor(httpInput("next"), p); err != workstore.ErrUnavailable {
		t.Fatal("accepted after evidence failure", err)
	}
	view, err := store.GetFor(ctx, p, first.RequestRef)
	if err != nil {
		t.Fatal(err)
	}
	if view.Outcome != nil {
		t.Fatal("storage failure fabricated a complete result")
	}
	for _, f := range view.Facts {
		if f.Kind == "ProviderAttempt" || f.Kind == "ProviderOutcome" {
			t.Fatal("uncommitted provider evidence published")
		}
	}
	options.Durable = store
	if restarted, err := NewServiceWithOptions(&verticalBackend{}, httpExecutor{}, provider, app.ReferencePrincipalTeamA, options); err != ErrRecoveryRequired {
		if restarted != nil {
			restarted.Close()
		}
		t.Fatal("unfinished Work silently replayed", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("recovery repeated provider")
	}
}
