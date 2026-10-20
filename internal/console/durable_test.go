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
	"github.com/wunderforge/agenova/internal/runtime"
	"net/http"
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
	testDurableRestart(t, "")
}
func TestDurableCompletedWorkRepairsLostFinalAcknowledgement(t *testing.T) {
	testDurableRestart(t, "finish")
}

type failedFinishStore struct{ *workstore.Store }

func (failedFinishStore) Finish(context.Context, evidence.View) error {
	return workstore.ErrUnavailable
}
func TestDurableTerminalFactFailureStopsClientPolling(t *testing.T) { testDurableRestart(t, "outcome") }

type failedOutcomeStore struct{ *workstore.Store }

func (s failedOutcomeStore) Append(ctx context.Context, f facts.Fact, state *v0.IssuedState) error {
	if f.Kind == "RunOutcome" {
		return workstore.ErrUnavailable
	}
	return s.Store.Append(ctx, f, state)
}
func testDurableRestart(t *testing.T, failure string) {
	lostFinish := failure != ""
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
			if failure == "finish" {
				durable = failedFinishStore{store}
			} else {
				durable = failedOutcomeStore{store}
			}
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
		done := make(chan error, 1)
		go func() { _, runErr := client.RunFile(path); done <- runErr }()
		select {
		case runErr := <-done:
			if runErr == nil || !strings.Contains(runErr.Error(), "HTTP 503") {
				t.Fatalf("client failed to report storage error: %v", runErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("client kept polling after terminal persistence failure")
		}
		views, listErr := store.ListFor(ctx, p)
		if listErr != nil || len(views) != 1 {
			t.Fatal("missing durable admission", listErr)
		}
		first = views[0]
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
	if lostFinish {
		for _, path := range []string{"/api/requests", "/api/requests/" + first.RequestRef + "/evidence", "/api/claims/" + first.State.Claim.ID + "/evidence"} {
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("Authorization", "Bearer "+raw)
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, req)
			if result.Code != 503 {
				t.Fatalf("%s returned %d instead of unavailable", path, result.Code)
			}
		}
		if provider.calls.Load() != 1 {
			t.Fatal("provider replayed after storage failure")
		}
		if failure == "outcome" {
			return
		} // missing terminal fact requires recovery, not publication repair
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
	failed   atomic.Bool
	failKind string
}

func (s *failedAttemptStore) Append(ctx context.Context, f facts.Fact, state *v0.IssuedState) error {
	if f.Kind == s.failKind {
		s.failed.Store(true)
	}
	if s.failed.Load() {
		return workstore.ErrUnavailable
	}
	return s.Store.Append(ctx, f, state)
}
func TestDurableFactFailurePreventsProviderAndRestartReplay(t *testing.T) {
	testDurableFailure(t, "ProviderAttempt", 0)
}
func TestDurableUnknownAllocationCannotBeInventedDuringRecovery(t *testing.T) {
	testDurableFailure(t, "Runtime", 0)
}
func TestDurableUncertainProviderOutcomeNeverRepeatsCall(t *testing.T) {
	testDurableFailure(t, "ProviderOutcome", 1)
}
func testDurableFailure(t *testing.T, failKind string, calls int32) {
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
	options := Options{Durable: &failedAttemptStore{Store: store, failKind: failKind}, Setup: memberTestSetup, Configure: memberTestConfigure, Access: identity, PrepareFor: func(data []byte, p v0.Principal) (app.PreparedAssignment, error) {
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
	if provider.calls.Load() != calls {
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
		if f.Kind == failKind || f.Kind == "ProviderOutcome" {
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
	if provider.calls.Load() != calls {
		t.Fatal("recovery repeated provider")
	}
	recovery := &recoveryTestBackend{failCleanup: true}
	if failKind == "Runtime" {
		if count, err := RecoverDurableWork(ctx, store, recovery); err != ErrRecoveryRequired || count != 0 {
			t.Fatal("unknown allocation recovered", count, err)
		}
		if recovery.stopCalls != 0 || recovery.cleanupCalls != 0 {
			t.Fatal("unknown allocation mutated backend")
		}
		return
	}
	if count, err := RecoverDurableWork(ctx, store, recovery); err != ErrRecoveryRequired || count != 0 {
		t.Fatal("unconfirmed cleanup accepted", count, err)
	}
	recovery.failCleanup = false
	if count, err := RecoverDurableWork(ctx, store, recovery); err != nil || count != 1 {
		t.Fatal("known allocation recovery", count, err)
	}
	view, err = store.GetFor(ctx, p, first.RequestRef)
	if err != nil || view.Outcome == nil || view.Outcome.Status != "Failed" {
		t.Fatal("recovery did not finalize failure", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, view) }))
	defer server.Close()
	client := connectedclient.Client{Server: server.URL, HTTPClient: server.Client(), AccessToken: func(context.Context) (string, error) { return "test-token", nil }}
	if _, err := client.Show(view.RequestRef); err != nil {
		t.Fatal("recovered evidence violates CLI contract", err)
	}
	// A recovery label cannot turn an unobserved call into a successful task.
	originalView := evidence.Clone(view)
	view.Outcome.Status = "Succeeded"
	view.Outcome.Failure = ""
	view.State.Claim.Phase = v0.ClaimPhaseSucceeded
	for i := range view.Facts {
		f := &view.Facts[i]
		if f.Kind == "Runtime" && f.Operation == "Failed" {
			f.Operation = "Succeeded"
		}
		if f.Kind == "RunOutcome" {
			f.Operation = "Succeeded"
			f.Reason = ""
		}
	}
	for i := range view.State.Evidence.RuntimeEvents {
		if view.State.Evidence.RuntimeEvents[i].Kind == "Failed" {
			view.State.Evidence.RuntimeEvents[i].Kind = "Succeeded"
		}
	}
	if _, err := client.Show(view.RequestRef); err == nil {
		t.Fatal("unknown invocation accepted as success")
	}
	view = originalView

	if count, err := RecoverDurableWork(ctx, store, recovery); err != nil || count != 0 {
		t.Fatal("recovery repeated", count, err)
	}
	if recovery.stopCalls != 2 || recovery.cleanupCalls != 2 || provider.calls.Load() != calls {
		t.Fatal("unexpected replay counts")
	}
}

type recoveryTestBackend struct {
	verticalBackend
	cleanupCalls, stopCalls int
	failCleanup             bool
}

func (*recoveryTestBackend) Allocate(runtime.AllocateRequest) (runtime.Allocation, error) {
	panic("recovery allocated")
}
func (*recoveryTestBackend) Start(v0.SandboxClaimBackendIdentity) error {
	panic("recovery started worker")
}
func (*recoveryTestBackend) RestoreForCleanup(string, v0.SandboxClaimBackendIdentity) error {
	return nil
}
func (b *recoveryTestBackend) Terminate(v0.SandboxClaimBackendIdentity) error {
	b.stopCalls++
	return nil
}
func (b *recoveryTestBackend) Cleanup(id v0.SandboxClaimBackendIdentity) (runtime.CleanupResult, error) {
	b.cleanupCalls++
	if b.failCleanup {
		return runtime.CleanupResult{}, ErrRecoveryRequired
	}
	return runtime.CleanupResult{Identity: id, Released: true}, nil
}
