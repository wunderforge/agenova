// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package workstore persists the existing evidence view. Stored snapshots are
// evidence, never revived claim authority or instructions to repeat execution.
package workstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
)

// Schema is migration 005; only the explicit operator migration applies it.
//
//go:embed schema.sql
var Schema string
var ErrUnavailable = errors.New("durable Work store unavailable")
var ErrConflict = errors.New("Work name already has different input")
var ErrNotFound = errors.New("Work not found")
var ErrCapacity = errors.New("durable Work capacity reached")

type Snapshot struct {
	Owner        v0.Principal
	InputName    string
	InputDigest  string
	View         evidence.View
	StagedResult *evidence.Outcome
	Complete     bool
}

// Store owns one dedicated PostgreSQL session lock. All writes use that SAME
// connection, fencing a disconnected writer; there is no automatic reacquisition.
// A second control-plane process must not dispatch while this writer owns it.
type Store struct {
	db     *sql.DB
	mu     sync.Mutex
	writer *sql.Conn
	closed bool
}

func Acquire(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, ErrUnavailable
	}
	var locked bool
	if conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtext(current_schema()),215005)`).Scan(&locked) != nil || !locked {
		conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
		return nil, ErrUnavailable
	}
	return &Store{db: db, writer: conn}, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := s.writer.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext(current_schema()),215005)`)
	_ = s.writer.Raw(func(any) error { return driver.ErrBadConn })
	_ = s.writer.Close()
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) transaction(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if err = fn(ctx, tx); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return ErrUnavailable
	}
	return nil
}

// Reference scopes a caller-selected idempotency name without exposing identity
// metadata in ClaimRequest. The resulting request reference is opaque and stable.
func Reference(p v0.Principal, name string) string {
	b, _ := json.Marshal([]string{p.Team, p.Subject, name})
	sum := sha256.Sum256(b)
	return "work-" + hex.EncodeToString(sum[:])
}
func Digest(request *v0.ClaimRequest) string {
	b, _ := json.Marshal(request)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func snapshotJSON(view evidence.View) ([]byte, error) {
	view = evidence.Clone(view)
	view.Facts = []facts.Fact{}
	b, err := json.Marshal(view)
	if err != nil || len(b) > 1<<20 {
		return nil, ErrUnavailable
	}
	return b, nil
}

// Reserve atomically commits resolved admission + initial facts BEFORE dispatch.
// Reusing the same owner/name/input returns existing=true, never a second dispatch.
func (s *Store) Reserve(ctx context.Context, snapshot Snapshot) (bool, error) {
	v := snapshot.View
	p := snapshot.Owner
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" || snapshot.InputName == "" || len(snapshot.InputDigest) != 64 || v.Request == nil || v.State == nil || v0.ValidateIssuedState(v.State) != nil || v.State.Principal != p || v.RequestRef != Reference(p, snapshot.InputName) || v.Request.Metadata.Name != v.RequestRef || v.State.RequestRef != v.RequestRef {
		return false, ErrUnavailable
	}
	raw, err := snapshotJSON(v)
	if err != nil {
		return false, err
	}
	existing := false
	err = s.transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var digest string
		err := tx.QueryRowContext(ctx, `SELECT input_digest FROM agenova_work WHERE team=$1 AND owner=$2 AND input_name=$3`, p.Team, p.Subject, snapshot.InputName).Scan(&digest)
		if err == nil {
			if digest != snapshot.InputDigest {
				return ErrConflict
			}
			existing = true
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return ErrUnavailable
		}
		var count, active int
		if tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE NOT complete) FROM agenova_work WHERE team=$1`, p.Team).Scan(&count, &active) != nil {
			return ErrUnavailable
		}
		if count >= 1000 || active >= 32 {
			return ErrCapacity
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO agenova_work(ref,team,owner,project,input_name,input_digest,view,complete) SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE EXISTS(SELECT 1 FROM agenova_members WHERE principal=$3 AND team=$2 AND active)`, v.RequestRef, p.Team, p.Subject, v.Request.Spec.ProjectRef, snapshot.InputName, snapshot.InputDigest, raw, v.Outcome != nil)
		if err != nil {
			return ErrUnavailable
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return ErrUnavailable
		}
		for _, fact := range v.Facts {
			if fact.RequestRef != v.RequestRef {
				return ErrUnavailable
			}
			if err := insertFact(ctx, tx, fact); err != nil {
				return err
			}
		}
		return nil
	})
	return existing, err
}
func insertFact(ctx context.Context, tx *sql.Tx, f facts.Fact) error {
	if f.ID == "" || f.RequestRef == "" || f.Sequence == 0 {
		return ErrUnavailable
	}
	b, err := json.Marshal(f)
	if err != nil || len(b) > 128<<10 {
		return ErrUnavailable
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agenova_work_facts(sequence,id,request_ref,payload) VALUES($1,$2,$3,$4)`, f.Sequence, f.ID, f.RequestRef, b)
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// Append commits a fact and optional lifecycle snapshot together, before the
// caller publishes the transition or performs its next governed side effect.
func (s *Store) Append(ctx context.Context, f facts.Fact, state *v0.IssuedState) error {
	return s.transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var complete bool
		if tx.QueryRowContext(ctx, `SELECT complete FROM agenova_work WHERE ref=$1 FOR UPDATE`, f.RequestRef).Scan(&complete) != nil || complete {
			return ErrUnavailable
		}
		if state != nil {
			if state.RequestRef != f.RequestRef || state.Claim == nil || state.Claim.ID != f.ClaimID {
				return ErrUnavailable
			}
			b, err := json.Marshal(state)
			if err != nil {
				return ErrUnavailable
			}
			if _, err = tx.ExecContext(ctx, `UPDATE agenova_work SET view=jsonb_set(view,'{state}',$2::jsonb),updated_at=now() WHERE ref=$1`, f.RequestRef, b); err != nil {
				return ErrUnavailable
			}
		}
		return insertFact(ctx, tx, f)
	})
}
func (s *Store) StageResult(ctx context.Context, ref string, outcome evidence.Outcome) error {
	b, err := json.Marshal(outcome)
	if err != nil || len(b) > 128<<10 {
		return ErrUnavailable
	}
	return s.transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `UPDATE agenova_work SET staged_result=$2,updated_at=now() WHERE ref=$1 AND NOT complete AND staged_result IS NULL`, ref, b)
		if err != nil {
			return ErrUnavailable
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return ErrUnavailable
		}
		return nil
	})
}
func (s *Store) Finish(ctx context.Context, view evidence.View) error {
	if view.Outcome == nil || view.State == nil || v0.ValidateIssuedState(view.State) != nil || view.State.Claim == nil || (view.State.Claim.Phase != v0.ClaimPhaseSucceeded && view.State.Claim.Phase != v0.ClaimPhaseFailed && view.State.Claim.Phase != v0.ClaimPhaseExpired) {
		return ErrUnavailable
	}
	b, err := snapshotJSON(view)
	if err != nil {
		return err
	}
	return s.transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `UPDATE agenova_work SET view=$2,complete=true,staged_result=NULL,updated_at=now() WHERE ref=$1 AND NOT complete
 AND view->'request'=$2::jsonb->'request'
 AND view#>'{state,principal}'=$2::jsonb#>'{state,principal}'
 AND view#>'{state,action}'=$2::jsonb#>'{state,action}'
 AND view#>'{state,decision}'=$2::jsonb#>'{state,decision}'
 AND view#>'{state,effectiveAuthority}'=$2::jsonb#>'{state,effectiveAuthority}'
 AND view#>>'{state,claim,id}'=$2::jsonb#>>'{state,claim,id}'
 AND (SELECT payload->>'kind' FROM agenova_work_facts WHERE request_ref=$1 ORDER BY sequence DESC LIMIT 1)='RunOutcome'
 AND (SELECT payload->>'operation' FROM agenova_work_facts WHERE request_ref=$1 ORDER BY sequence DESC LIMIT 1)=$2::jsonb#>>'{outcome,status}'`, view.RequestRef, b)
		if err != nil {
			return ErrUnavailable
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return ErrUnavailable
		}
		return nil
	})
}
func (s *Store) LastSequence(ctx context.Context) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var seq uint64
	if s.db.QueryRowContext(ctx, `SELECT COALESCE(max(sequence),0) FROM agenova_work_facts`).Scan(&seq) != nil {
		return 0, ErrUnavailable
	}
	return seq, nil
}
