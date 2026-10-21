// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/history"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestRestartUnknownAttemptAndExclusiveWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Open(path); err == nil {
		second.Close()
		t.Fatal("accepted second writer")
	}
	view := evidence.View{Version: "agenova.evidence/v0", RequestRef: "one", State: &v0.IssuedState{Claim: &v0.SandboxClaim{ID: "claim:one", Phase: v0.ClaimPhaseRunning}}}
	if err = r.Create(view, history.Admission{Principal: v0.Principal{Subject: "alice"}}); err != nil {
		t.Fatal(err)
	}
	if err = r.Append(facts.Fact{ID: "f:1", Sequence: 1, Kind: "ProviderAttempt", RequestRef: "one", ClaimID: "claim:one", InvocationID: "i:1"}, nil); err != nil {
		t.Fatal(err)
	}
	if err = r.Append(facts.Fact{ID: "f:1", Sequence: 2, Kind: "ProviderOutcome", RequestRef: "one", ClaimID: "claim:one"}, nil); err == nil {
		t.Fatal("accepted duplicate fact")
	}
	r.Close()
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for i := 0; i < 2; i++ {
		if err = r.Recover(); err != nil {
			t.Fatal(err)
		}
	}
	e, err := r.Get("claim:one")
	if err != nil {
		t.Fatal(err)
	}
	if e.View.State.Claim.Phase != v0.ClaimPhaseRunning || e.View.Outcome != nil || e.View.Recovery == nil || e.View.Recovery.Cleanup != "Unknown" || e.View.Diagnosis.Code != "provider-outcome-unknown" || len(e.View.Facts) != 1 {
		t.Fatalf("invented completion: %+v", e.View)
	}
	n, err := r.LastSequence()
	if err != nil || n != 1 {
		t.Fatalf("rollback changed sequence: %d %v", n, err)
	}
}
func TestHistoryFiltersBeforePagingAndReauthorizesCursor(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, name := range []string{"a", "hidden", "c", "d"} {
		owner := "alice"
		if name == "hidden" {
			owner = "bob"
		}
		if err = r.Create(evidence.View{RequestRef: name}, history.Admission{Principal: v0.Principal{Subject: owner}}); err != nil {
			t.Fatal(err)
		}
	}
	auth := history.AuthorizerFunc(func(_ context.Context, p v0.Principal, a history.Admission) bool {
		return p.Subject == a.Principal.Subject
	})
	viewer := v0.Principal{Subject: "alice"}
	page, err := r.History(context.Background(), viewer, auth, 2, "")
	if err != nil || len(page.Items) != 2 || page.Items[0].RequestRef != "d" || page.Items[1].RequestRef != "c" || page.NextCursor == "" {
		t.Fatalf("page: %+v %v", page, err)
	}
	r.Create(evidence.View{RequestRef: "new"}, history.Admission{Principal: viewer})
	next, err := r.History(context.Background(), viewer, auth, 2, page.NextCursor)
	if err != nil || len(next.Items) != 1 || next.Items[0].RequestRef != "a" || next.NextCursor != "" {
		t.Fatalf("unstable page: %+v %v", next, err)
	}
	if _, err = history.Read(context.Background(), r, viewer, auth, "hidden"); !errors.Is(err, history.ErrNotFound) {
		t.Fatal("hidden record exposed")
	}
	if _, err = r.History(context.Background(), viewer, nil, 2, ""); !errors.Is(err, history.ErrDenied) {
		t.Fatal("missing authorizer accepted")
	}
	if _, err = r.History(context.Background(), viewer, auth, 2, "not-a-cursor"); !errors.Is(err, history.ErrCursor) {
		t.Fatal("bad cursor accepted")
	}
}

func TestCommittedAttemptSurvivesProcessExit(t *testing.T) {
	if path := os.Getenv("AGENOVA_TEST_CRASH_DB"); path != "" {
		r, err := Open(path)
		if err != nil {
			os.Exit(2)
		}
		if r.Create(evidence.View{RequestRef: "crash"}, history.Admission{Principal: v0.Principal{Subject: "alice"}}) != nil {
			os.Exit(3)
		}
		if r.Append(facts.Fact{Sequence: 1, ID: "crash-attempt", Kind: "ProviderAttempt", RequestRef: "crash", InvocationID: "invocation"}, nil) != nil {
			os.Exit(4)
		}
		os.Exit(0) // Deliberately omit Close: committed WAL must survive process death.
	}
	path := filepath.Join(t.TempDir(), "crash.db")
	cmd := exec.Command(os.Args[0], "-test.run=^TestCommittedAttemptSurvivesProcessExit$")
	cmd.Env = append(os.Environ(), "AGENOVA_TEST_CRASH_DB="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child failed: %v %s", err, output)
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = r.Recover(); err != nil {
		t.Fatal(err)
	}
	entry, err := r.Get("crash")
	if err != nil || len(entry.View.Facts) != 1 || entry.View.Diagnosis.Code != "provider-outcome-unknown" {
		t.Fatalf("lost committed attempt: %+v %v", entry, err)
	}
}
func TestInitialAdmissionRollbackLeavesNoPartialWork(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	initial := evidence.View{RequestRef: "partial", Facts: []facts.Fact{{ID: "duplicate", Sequence: 1, RequestRef: "partial"}, {ID: "duplicate", Sequence: 2, RequestRef: "partial"}}}
	if err = r.Create(initial, history.Admission{Principal: v0.Principal{Subject: "alice"}}); err == nil {
		t.Fatal("accepted duplicate admission facts")
	}
	if _, err = r.Get("partial"); !errors.Is(err, history.ErrNotFound) {
		t.Fatalf("published partial admission: %v", err)
	}
	if n, _ := r.LastSequence(); n != 0 {
		t.Fatal("rollback consumed ordering")
	}
}

func TestUnsupportedSchemaFailsWithoutErasingRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Create(evidence.View{RequestRef: "preserved"}, history.Admission{Principal: v0.Principal{Subject: "alice"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if opened, err := Open(path); err == nil {
		opened.Close()
		t.Fatal("accepted unsupported schema")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var ref string
	if err = db.QueryRow("SELECT ref FROM work").Scan(&ref); err != nil || ref != "preserved" {
		t.Fatal("failed startup erased evidence")
	}
}
func TestConcurrentValidatedAppendsSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Create(evidence.View{RequestRef: "parallel"}, history.Admission{Principal: v0.Principal{Subject: "alice"}}); err != nil {
		t.Fatal(err)
	}
	journal := facts.NewJournal()
	journal, err = facts.NewDurableJournal(0, func(f facts.Fact) error { return r.Append(f, nil) })
	if err != nil {
		t.Fatal(err)
	}
	journal.RegisterRequest("parallel", v0.Principal{Subject: "alice"})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := journal.Append(facts.Fact{Kind: "WorkerActivity", RequestRef: "parallel"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	r.Close()
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	entry, err := r.Get("parallel")
	if err != nil || len(entry.View.Facts) != 32 {
		t.Fatalf("lost concurrent append: %v", err)
	}
	for i, f := range entry.View.Facts {
		if f.Sequence != uint64(i+1) {
			t.Fatal("ordering changed")
		}
	}
}

func TestRequestAndClaimNamespacesCannotAlias(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	owner := history.Admission{Principal: v0.Principal{Subject: "alice"}}
	initial := evidence.View{RequestRef: "first", State: &v0.IssuedState{Claim: &v0.SandboxClaim{ID: "claim:first"}}}
	if err = r.Create(initial, owner); err != nil {
		t.Fatal(err)
	}
	if err = r.Create(evidence.View{RequestRef: "claim:first"}, owner); !errors.Is(err, history.ErrConflict) {
		t.Fatal("request shadows claim lookup")
	}
	if err = r.Create(evidence.View{RequestRef: "second", State: &v0.IssuedState{Claim: &v0.SandboxClaim{ID: "first"}}}, owner); !errors.Is(err, history.ErrConflict) {
		t.Fatal("claim shadows request lookup")
	}
	entry, err := r.Get("claim:first")
	if err != nil || entry.View.RequestRef != "first" {
		t.Fatal("identity lookup changed")
	}
}

func TestOpaqueCursorSurvivesRestartAndRejectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cursor.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := r.sealCursor(42, 12)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	upper, before, err := r.openCursor(cursor)
	if err != nil || upper != 42 || before != 12 {
		t.Fatal("cursor did not survive reopen")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(cursor)
	raw[len(raw)-1] ^= 1
	if _, _, err = r.openCursor(base64.RawURLEncoding.EncodeToString(raw)); !errors.Is(err, history.ErrCursor) {
		t.Fatal("accepted tampered cursor")
	}
	if _, _, err = r.openCursor(base64.RawURLEncoding.EncodeToString([]byte("42:12"))); !errors.Is(err, history.ErrCursor) {
		t.Fatal("exposed ordering state as cursor")
	}
}
func TestVersionOneMigrationPreservesEvidence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Create(evidence.View{RequestRef: "v1"}, history.Admission{Principal: v0.Principal{Subject: "alice"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("DROP TABLE history_settings"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	r.Close()
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if entry, err := r.Get("v1"); err != nil || entry.View.RequestRef != "v1" {
		t.Fatal("migration lost evidence")
	}
	var version int
	if r.db.QueryRow("PRAGMA user_version").Scan(&version) != nil || version != 2 {
		t.Fatal("migration did not commit")
	}
}
