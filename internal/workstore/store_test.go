// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workstore_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/policy"
	"github.com/wunderforge/agenova/internal/workstore"
)

type principalSource struct{ p v0.Principal }

func (p principalSource) Principal() v0.Principal { return p.p }

type templates struct{}

func (templates) Lookup(string) (*v0.AgentTemplate, error) { return app.ReferenceTemplate(), nil }
func database(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AGENOVA_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("work_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
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
	t.Cleanup(func() { db.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	if err = identitydb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
func snapshot(t *testing.T, p v0.Principal, name string, sequence uint64) workstore.Snapshot {
	t.Helper()
	input := []byte(`{"apiVersion":"agenova.io/v1alpha1","kind":"ClaimRequest","metadata":{"name":"` + name + `"},"spec":{"templateRef":"engineer","projectRef":"payments","task":{"type":"repository-change","input":{"objective":"Explain retry backoff","repository":"acme/payments"}},"requestedAccess":{"tools":["git.read"],"resourceScopes":["repo:acme/payments"],"modelProfile":"approved-coding-model","memoryScopes":["team-docs"]},"runtime":{"profileRef":"standard-isolated","timeout":"20m"}}}`)
	r, parseErr := v0.ParseClaimRequestJSON(input)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	digest := workstore.Digest(r)
	r.Metadata.Name = workstore.Reference(p, name)
	input, _ = json.Marshal(r)
	loader := &policy.Loader{}
	if err := loader.Load(app.ReferencePolicy()); err != nil {
		t.Fatal(err)
	}
	prepared, err := app.PrepareAssignment(input, principalSource{p}, loader, templates{})
	if err != nil {
		t.Fatal(err)
	}
	ref := r.Metadata.Name
	return workstore.Snapshot{Owner: p, InputName: name, InputDigest: digest, View: evidence.View{Version: "agenova.evidence/v0", RequestRef: ref, Request: r, State: prepared.Issued, Facts: []facts.Fact{{ID: fmt.Sprintf("fact:%d", sequence), Sequence: sequence, Timestamp: time.Now(), Kind: "RequestReceived", RequestRef: ref}}}}
}
func TestDurableAdmissionIsolationAndWriterFencing(t *testing.T) {
	ctx := context.Background()
	db := database(t)
	identities := identitydb.Store{DB: db}
	people := map[string]v0.Principal{}
	for _, name := range []string{"alice", "bob", "viewer", "admin"} {
		p := v0.Principal{Subject: name, Team: "team-a", AuthenticationContext: "oidc"}
		people[name] = p
		if err := identities.PutMember(ctx, "operator", identitydb.Member{Issuer: "issuer", Subject: name, Principal: name, Team: p.Team, Active: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO agenova_projects VALUES('team-a','payments'); INSERT INTO agenova_project_grants VALUES('alice','team-a','payments','member'),('bob','team-a','payments','member'),('viewer','team-a','payments','viewer'); UPDATE agenova_members SET account_role='admin' WHERE principal='admin'`); err != nil {
		t.Fatal(err)
	}
	s, err := workstore.Acquire(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if other, err := workstore.Acquire(ctx, db); err == nil {
		other.Close()
		t.Fatal("second writer acquired lease")
	}
	alice := snapshot(t, people["alice"], "same-name", 1)
	bob := snapshot(t, people["bob"], "same-name", 2)
	var wg sync.WaitGroup
	var admitted atomic.Int32
	var failures atomic.Int32
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			existing, err := s.Reserve(ctx, alice)
			if err != nil {
				failures.Add(1)
			} else if !existing {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if admitted.Load() != 1 || failures.Load() != 0 {
		t.Fatal("idempotency race", admitted.Load(), failures.Load())
	}
	if existing, err := s.Reserve(ctx, bob); err != nil || existing {
		t.Fatal("same name collided across users", err)
	}
	changed := alice
	changed.InputDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := s.Reserve(ctx, changed); err != workstore.ErrConflict {
		t.Fatal("changed input replaced admission")
	}
	if _, err := s.GetFor(ctx, people["bob"], alice.View.RequestRef); err != workstore.ErrNotFound {
		t.Fatal("foreign Work disclosure")
	}
	if _, err := s.GetFor(ctx, people["admin"], alice.View.RequestRef); err != workstore.ErrNotFound {
		t.Fatal("account admin bypassed project grants")
	}
	if _, err := s.GetFor(ctx, people["viewer"], alice.View.RequestRef); err != nil {
		t.Fatal("project viewer read failed", err)
	}
	for _, name := range []string{"alice", "bob"} {
		views, err := s.ListFor(ctx, people[name])
		if err != nil || len(views) != 1 {
			t.Fatal("owner isolation", name, err)
		}
	}
	staged := evidence.Outcome{Status: "Succeeded", Text: "Observed response"}
	if err := s.StageResult(ctx, alice.View.RequestRef, staged); err != nil {
		t.Fatal(err)
	}
	f := facts.Fact{ID: "fact:3", Sequence: 3, Timestamp: time.Now(), Kind: "WorkerActivity", RequestRef: alice.View.RequestRef, ClaimID: alice.View.State.Claim.ID, Operation: "FinalAnswer"}
	if err := s.Append(ctx, f, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := workstore.Acquire(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	pending, err := restarted.Pending(ctx)
	if err != nil || len(pending) != 2 {
		t.Fatal("pending recovery data lost", err)
	}
	found := false
	for _, record := range pending {
		if record.Owner.Subject == "alice" {
			found = record.StagedResult != nil && record.StagedResult.Text == staged.Text && len(record.View.Facts) == 2
		}
	}
	if !found {
		t.Fatal("uncertain result/facts were lost")
	}
	if seq, err := restarted.LastSequence(ctx); err != nil || seq != 3 {
		t.Fatal("durable sequence lost")
	}
	if _, found, err := restarted.Existing(ctx, people["alice"], alice.InputName, alice.InputDigest); err != nil || !found {
		t.Fatal("restart idempotency lost")
	}
	if err := s.Append(ctx, f, nil); err == nil {
		t.Fatal("closed writer mutated data")
	}
	// Kill only the exact dedicated test lease holder, then verify no automatic
	// reconnection can write under a lost fencing lock.
	var pid int
	if err := db.QueryRow(`SELECT pid FROM pg_locks WHERE locktype='advisory' AND objid=215005 AND classid=(hashtext(current_schema())::bigint & 4294967295)::oid AND objsubid=2 AND granted AND database=(SELECT oid FROM pg_database WHERE datname=current_database())`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`SELECT pg_terminate_backend($1)`, pid); err != nil {
		t.Fatal(err)
	}
	f.ID = "fact:4"
	f.Sequence = 4
	if err := restarted.Append(ctx, f, nil); err == nil {
		t.Fatal("disconnected writer reacquired permission")
	}
	if seq, err := restarted.LastSequence(ctx); err != nil || seq != 3 {
		t.Fatal("fenced write persisted")
	}
}
