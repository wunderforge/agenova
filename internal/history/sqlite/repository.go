// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package sqlite owns the single-writer SQLite evidence adapter.
package sqlite

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/history"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Repository struct {
	db     *sql.DB
	lock   *os.File
	mu     sync.Mutex
	cursor cipher.AEAD
}

func Open(path string) (*Repository, error) {
	if !filepath.IsAbs(path) {
		return nil, history.ErrUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, history.ErrUnavailable
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, history.ErrUnavailable
	}
	if lockFile(lock) != nil {
		lock.Close()
		return nil, history.ErrUnavailable
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		lock.Close()
		return nil, history.ErrUnavailable
	}
	db.SetMaxOpenConns(1)
	r := &Repository{db: db, lock: lock}
	fail := func() (*Repository, error) { r.Close(); return nil, history.ErrUnavailable }
	for _, q := range []string{"PRAGMA busy_timeout=3000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON"} {
		if _, err = db.Exec(q); err != nil {
			return fail()
		}
	}
	var version int
	if db.QueryRow("PRAGMA user_version").Scan(&version) != nil || version > 2 {
		return fail()
	}
	if version == 0 {
		tx, err := db.Begin()
		if err != nil {
			return fail()
		}
		for _, q := range []string{
			`CREATE TABLE work (seq INTEGER PRIMARY KEY AUTOINCREMENT, ref TEXT NOT NULL UNIQUE, claim TEXT UNIQUE, admission BLOB NOT NULL, view BLOB NOT NULL, created TEXT NOT NULL, updated TEXT NOT NULL)`,
			`CREATE TABLE fact (sequence INTEGER PRIMARY KEY, id TEXT NOT NULL UNIQUE, ref TEXT NOT NULL REFERENCES work(ref), payload BLOB NOT NULL)`,
			`CREATE INDEX fact_ref ON fact(ref,sequence)`, `PRAGMA user_version=1`,
		} {
			if _, err = tx.Exec(q); err != nil {
				tx.Rollback()
				return fail()
			}
		}
		if tx.Commit() != nil {
			return fail()
		}
	}
	if err := r.initializeCursor(version); err != nil {
		return fail()
	}
	if _, err = db.Exec("SELECT ref FROM work LIMIT 0"); err != nil {
		return fail()
	}
	if os.Chmod(path, 0600) != nil {
		return fail()
	}
	return r, nil
}
func (r *Repository) Close() error {
	err := r.db.Close()
	if r.lock != nil {
		unlockFile(r.lock)
		r.lock.Close()
	}
	return err
}
func encode(value any) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, history.ErrUnavailable
	}
	return b, nil
}
func (r *Repository) Create(view evidence.View, admission history.Admission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if view.RequestRef == "" || admission.Principal.Subject == "" {
		return history.ErrUnavailable
	}
	b, err := encode(view)
	if err != nil {
		return err
	}
	a, err := encode(admission)
	if err != nil {
		return err
	}
	claim := any(nil)
	if view.State != nil && view.State.Claim != nil {
		claim = view.State.Claim.ID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var n int
	if err = r.db.QueryRow("SELECT count(*) FROM work WHERE ref=? OR claim=? OR ref=? OR claim=?", view.RequestRef, view.RequestRef, claim, claim).Scan(&n); err != nil {
		return history.ErrUnavailable
	}
	if n != 0 {
		return history.ErrConflict
	}
	tx, err := r.db.Begin()
	if err != nil {
		return history.ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO work(ref,claim,admission,view,created,updated) VALUES(?,?,?,?,?,?)", view.RequestRef, claim, a, b, now, now); err != nil {
		return history.ErrUnavailable
	}
	for _, fact := range view.Facts {
		if fact.RequestRef != view.RequestRef {
			return history.ErrUnavailable
		}
		payload, err := encode(fact)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO fact(sequence,id,ref,payload) VALUES(?,?,?,?)", fact.Sequence, fact.ID, fact.RequestRef, payload); err != nil {
			return history.ErrUnavailable
		}
	}
	if tx.Commit() != nil {
		return history.ErrUnavailable
	}
	return nil
}
func load(tx interface{ QueryRow(string, ...any) *sql.Row }, ref string) (history.Entry, error) {
	var e history.Entry
	var b, a []byte
	var created, updated string
	err := tx.QueryRow("SELECT seq,view,admission,created,updated FROM work WHERE ref=? OR claim=?", ref, ref).Scan(&e.Order, &b, &a, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return e, history.ErrNotFound
	}
	if err != nil {
		return e, history.ErrUnavailable
	}
	if json.Unmarshal(b, &e.View) != nil || json.Unmarshal(a, &e.Admission) != nil {
		return e, history.ErrUnavailable
	}
	e.Created, _ = time.Parse(time.RFC3339Nano, created)
	e.Updated, _ = time.Parse(time.RFC3339Nano, updated)
	e.View.Facts = []facts.Fact{}
	return e, nil
}
func readFacts(tx interface {
	Query(string, ...any) (*sql.Rows, error)
}, ref string) ([]facts.Fact, error) {
	rows, err := tx.Query("SELECT payload FROM fact WHERE ref=? ORDER BY sequence", ref)
	if err != nil {
		return nil, history.ErrUnavailable
	}
	defer rows.Close()
	result := []facts.Fact{}
	for rows.Next() {
		var b []byte
		var f facts.Fact
		if rows.Scan(&b) != nil || json.Unmarshal(b, &f) != nil {
			return nil, history.ErrUnavailable
		}
		result = append(result, f)
	}
	if rows.Err() != nil {
		return nil, history.ErrUnavailable
	}
	return result, nil
}
func (r *Repository) Get(ref string) (history.Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return history.Entry{}, history.ErrUnavailable
	}
	defer tx.Rollback()
	e, err := load(tx, ref)
	if err != nil {
		return e, err
	}
	e.View.Facts, err = readFacts(tx, e.View.RequestRef)
	if err != nil {
		return e, err
	}
	evidence.Decorate(&e.View)
	return e, nil
}
func (r *Repository) Append(f facts.Fact, state *v0.IssuedState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, err := r.db.Begin()
	if err != nil {
		return history.ErrUnavailable
	}
	defer tx.Rollback()
	e, err := load(tx, f.RequestRef)
	if err != nil {
		return err
	}
	if f.ClaimID != "" && (e.View.State == nil || e.View.State.Claim == nil || e.View.State.Claim.ID != f.ClaimID) {
		return history.ErrUnavailable
	}
	b, err := encode(f)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO fact(sequence,id,ref,payload) VALUES(?,?,?,?)", f.Sequence, f.ID, f.RequestRef, b); err != nil {
		return history.ErrUnavailable
	}
	if state != nil {
		e.View.State = state
	}
	b, err = encode(e.View)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE work SET view=?,updated=? WHERE ref=?", b, time.Now().UTC().Format(time.RFC3339Nano), f.RequestRef); err != nil {
		return history.ErrUnavailable
	}
	if tx.Commit() != nil {
		return history.ErrUnavailable
	}
	return nil
}
func (r *Repository) Finish(ref string, state *v0.IssuedState, outcome *evidence.Outcome) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, err := load(r.db, ref)
	if err != nil {
		return err
	}
	if state != nil {
		e.View.State = state
	}
	e.View.Outcome = outcome
	b, err := encode(e.View)
	if err != nil {
		return err
	}
	if _, err = r.db.Exec("UPDATE work SET view=?,updated=? WHERE ref=?", b, time.Now().UTC().Format(time.RFC3339Nano), ref); err != nil {
		return history.ErrUnavailable
	}
	return nil
}
func (r *Repository) LastSequence() (uint64, error) {
	var n uint64
	if r.db.QueryRow("SELECT COALESCE(MAX(sequence),0) FROM fact").Scan(&n) != nil {
		return 0, history.ErrUnavailable
	}
	return n, nil
}
func (r *Repository) Recover() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Startup only, before accepting traffic. Do not restore live authority or infer cleanup.
	rows, err := r.db.Query("SELECT ref FROM work ORDER BY seq")
	if err != nil {
		return history.ErrUnavailable
	}
	var refs []string
	for rows.Next() {
		var ref string
		if rows.Scan(&ref) != nil {
			rows.Close()
			return history.ErrUnavailable
		}
		refs = append(refs, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return history.ErrUnavailable
	}
	for _, ref := range refs {
		tx, err := r.db.Begin()
		if err != nil {
			return history.ErrUnavailable
		}
		e, err := load(tx, ref)
		if err != nil {
			tx.Rollback()
			return err
		}
		if e.View.Outcome != nil || e.View.Recovery != nil {
			tx.Rollback()
			continue
		}
		fs, err := readFacts(tx, ref)
		if err != nil {
			tx.Rollback()
			return err
		}
		e.View.Recovery = &evidence.Recovery{Code: "service-restarted", Cleanup: "Unknown"}
		// Preserve the last committed phase, including terminal phases; interruption
		// is evidence about missing completion, not a new claim lifecycle transition.
		for _, f := range fs {
			if f.Kind == "Runtime" && f.Operation == "CleanupSucceeded" {
				e.View.Recovery.Cleanup = "Succeeded"
			}
			if f.Kind == "Runtime" && f.Operation == "CleanupFailed" {
				e.View.Recovery.Cleanup = "Failed"
			}
		}
		b, err := encode(e.View)
		if err != nil {
			tx.Rollback()
			return err
		}
		if _, err = tx.Exec("UPDATE work SET view=?,updated=? WHERE ref=?", b, time.Now().UTC().Format(time.RFC3339Nano), ref); err != nil {
			tx.Rollback()
			return history.ErrUnavailable
		}
		if tx.Commit() != nil {
			return history.ErrUnavailable
		}
	}
	return nil
}
func (r *Repository) History(ctx context.Context, viewer v0.Principal, auth history.Authorizer, limit int, cursor string) (history.Page, error) {
	page := history.Page{Items: []history.Summary{}}
	if viewer.Subject == "" || auth == nil {
		return page, history.ErrDenied
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return page, history.ErrCursor
	}
	upper, before := uint64(0), uint64(0)
	if cursor != "" {
		var err error
		upper, before, err = r.openCursor(cursor)
		if err != nil {
			return page, history.ErrCursor
		}
	} else {
		if r.db.QueryRow("SELECT COALESCE(MAX(seq),0) FROM work").Scan(&upper) != nil {
			return page, history.ErrUnavailable
		}
		before = upper + 1
	}
	// Scan bounded batches and authorize before counting visible items.
	for before > 1 {
		if ctx.Err() != nil {
			return page, ctx.Err()
		}
		rows, err := r.db.Query("SELECT seq,ref,admission,view,created,updated FROM work WHERE seq<=? AND seq<? ORDER BY seq DESC LIMIT 100", upper, before)
		if err != nil {
			return page, history.ErrUnavailable
		}
		type candidate struct {
			seq  uint64
			ref  string
			a    history.Admission
			v    evidence.View
			c, u time.Time
		}
		var batch []candidate
		for rows.Next() {
			var c candidate
			var a, b []byte
			var created, updated string
			if rows.Scan(&c.seq, &c.ref, &a, &b, &created, &updated) != nil || json.Unmarshal(a, &c.a) != nil || json.Unmarshal(b, &c.v) != nil {
				rows.Close()
				return page, history.ErrUnavailable
			}
			c.c, _ = time.Parse(time.RFC3339Nano, created)
			c.u, _ = time.Parse(time.RFC3339Nano, updated)
			batch = append(batch, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return page, history.ErrUnavailable
		}
		if len(batch) == 0 {
			break
		}
		for _, c := range batch {
			if !auth.CanRead(ctx, viewer, c.a) {
				before = c.seq
				continue
			}
			if len(page.Items) == limit {
				var err error
				page.NextCursor, err = r.sealCursor(upper, before)
				if err != nil {
					return page, err
				}
				return page, nil
			}
			before = c.seq
			status := "Pending"
			if c.v.State != nil {
				if c.v.State.Claim != nil {
					status = string(c.v.State.Claim.Phase)
				} else {
					status = string(c.v.State.Decision.Result)
				}
			}
			if c.v.Outcome != nil {
				status = c.v.Outcome.Status
			}
			if c.v.Recovery != nil {
				status = "Interrupted"
			}
			fs, err := readFacts(r.db, c.ref)
			if err != nil {
				return page, err
			}
			c.v.Facts = fs
			evidence.Decorate(&c.v)
			item := history.Summary{RequestRef: c.ref, Status: status, Created: c.c, Updated: c.u, TraceID: c.v.TraceID, Diagnosis: c.v.Diagnosis}
			if c.v.Outcome != nil {
				item.Completion = c.v.Outcome.Completion
			}
			if c.v.Request != nil && c.v.Request.Spec.Task != nil {
				item.TaskType = c.v.Request.Spec.Task.Type
				if len(item.TaskType) > 128 {
					item.TaskType = item.TaskType[:128]
				}
			}
			if c.v.State != nil && c.v.State.Claim != nil {
				item.ClaimID = c.v.State.Claim.ID
			}
			page.Items = append(page.Items, item)
		}
	}
	return page, nil
}
