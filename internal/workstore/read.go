// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/history"
)

// The SQL read predicate mirrors the private application-role policy: current
// membership + own Member/Admin evidence, or explicit project Admin/Viewer read.
// Account Admin alone does not imply access to every project's Work.
const visibleSQL = `EXISTS(SELECT 1 FROM agenova_members m WHERE m.principal=$1 AND m.team=$2 AND m.active AND (m.account_role='owner' OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=m.principal AND g.team=m.team AND g.project=w.project AND (g.role IN ('admin','viewer') OR (g.role='member' AND w.owner=m.principal)))))`
const snapshotColumns = `w.owner,w.team,w.input_name,w.input_digest,w.view,w.staged_result,w.complete`

func readSnapshot(ctx context.Context, tx *sql.Tx, query string, args ...any) (Snapshot, error) {
	var s Snapshot
	var raw, staged []byte
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&s.Owner.Subject, &s.Owner.Team, &s.InputName, &s.InputDigest, &raw, &staged, &s.Complete); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, ErrNotFound
		}
		return s, ErrUnavailable
	}
	s.Owner.AuthenticationContext = "oidc"
	var private struct {
		Admission *history.Admission `json:"admissionSnapshot"`
	}
	if json.Unmarshal(raw, &private) != nil {
		return s, ErrUnavailable
	}
	s.Admission = private.Admission
	if len(raw) > 1<<20 || json.Unmarshal(raw, &s.View) != nil {
		return s, ErrUnavailable
	}
	if len(staged) > 0 && json.Unmarshal(staged, &s.StagedResult) != nil {
		return s, ErrUnavailable
	}
	rows, err := tx.QueryContext(ctx, `SELECT payload FROM agenova_work_facts WHERE request_ref=$1 ORDER BY sequence`, s.View.RequestRef)
	if err != nil {
		return s, ErrUnavailable
	}
	defer rows.Close()
	s.View.Facts = []facts.Fact{}
	for rows.Next() {
		var b []byte
		var f facts.Fact
		if rows.Scan(&b) != nil || len(b) > 128<<10 || json.Unmarshal(b, &f) != nil {
			return s, ErrUnavailable
		}
		s.View.Facts = append(s.View.Facts, f)
	}
	if rows.Err() != nil {
		return s, ErrUnavailable
	}
	return s, nil
}
func (s *Store) GetFor(ctx context.Context, p v0.Principal, ref string) (evidence.View, error) {
	return s.getFor(ctx, p, ref, false)
}
func (s *Store) GetClaimFor(ctx context.Context, p v0.Principal, ref string) (evidence.View, error) {
	return s.getFor(ctx, p, ref, true)
}
func (s *Store) getFor(ctx context.Context, p v0.Principal, ref string, claim bool) (evidence.View, error) {
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return evidence.View{}, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return evidence.View{}, ErrUnavailable
	}
	defer tx.Rollback()
	predicate := "w.ref=$3"
	if claim {
		predicate = "w.view#>>'{state,claim,id}'=$3"
	}
	snapshot, err := readSnapshot(ctx, tx, `SELECT `+snapshotColumns+` FROM agenova_work w WHERE w.team=$2 AND `+predicate+` AND `+visibleSQL, p.Subject, p.Team, ref)
	return snapshot.View, err
}
func (s *Store) ListFor(ctx context.Context, p v0.Principal) ([]evidence.View, error) {
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return nil, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, ErrUnavailable
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT w.ref FROM agenova_work w WHERE w.team=$2 AND `+visibleSQL+` ORDER BY w.created_at DESC,w.ref LIMIT 32`, p.Subject, p.Team)
	if err != nil {
		return nil, ErrUnavailable
	}
	refs := []string{}
	for rows.Next() {
		var ref string
		if rows.Scan(&ref) != nil {
			rows.Close()
			return nil, ErrUnavailable
		}
		refs = append(refs, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, ErrUnavailable
	}
	views := []evidence.View{}
	for _, ref := range refs {
		snapshot, err := readSnapshot(ctx, tx, `SELECT `+snapshotColumns+` FROM agenova_work w WHERE w.ref=$1`, ref)
		if err != nil {
			return nil, err
		}
		views = append(views, snapshot.View)
	}
	return views, nil
}

// Existing is owner-scoped admission idempotency, not an administrative read.
func (s *Store) Existing(ctx context.Context, p v0.Principal, name, digest string) (evidence.View, bool, error) {
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return evidence.View{}, false, ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return evidence.View{}, false, ErrUnavailable
	}
	defer tx.Rollback()
	snapshot, err := readSnapshot(ctx, tx, `SELECT `+snapshotColumns+` FROM agenova_work w WHERE w.owner=$1 AND w.team=$2 AND w.input_name=$3 AND `+visibleSQL, p.Subject, p.Team, name)
	if err == ErrNotFound {
		return evidence.View{}, false, nil
	}
	if err != nil {
		return evidence.View{}, false, err
	}
	if snapshot.InputDigest != digest {
		return evidence.View{}, true, ErrConflict
	}
	return snapshot.View, true, nil
}

// Pending is privileged recovery input, never exposed as an unauthenticated API.
// The caller must reconcile; it must NEVER re-dispatch these stored requests.
func (s *Store) Pending(ctx context.Context) ([]Snapshot, error) {
	result := []Snapshot{}
	err := s.transaction(ctx, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT ref FROM agenova_work WHERE NOT complete ORDER BY created_at,ref`)
		if err != nil {
			return ErrUnavailable
		}
		refs := []string{}
		for rows.Next() {
			var ref string
			if rows.Scan(&ref) != nil {
				rows.Close()
				return ErrUnavailable
			}
			refs = append(refs, ref)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return ErrUnavailable
		}
		for _, ref := range refs {
			snapshot, err := readSnapshot(ctx, tx, `SELECT `+snapshotColumns+` FROM agenova_work w WHERE ref=$1`, ref)
			if err != nil {
				return err
			}
			result = append(result, snapshot)
		}
		return nil
	})
	return result, err
}
