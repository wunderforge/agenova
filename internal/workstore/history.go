// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workstore

import (
	"context"
	"database/sql"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/history"
	"time"
)

type pagePosition struct {
	Upper     time.Time
	UpperRef  string
	Before    time.Time
	BeforeRef string
}

// HistoryFor uses the SAME membership/project predicate as item reads before
// LIMIT, never an owner identity supplied by a caller. Cursors are private
// ordering hints, invalidated when the exclusive writer process restarts.
func (s *Store) HistoryFor(ctx context.Context, p v0.Principal, limit int, cursor string) (history.Page, error) {
	page := history.Page{Items: []history.Summary{}}
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return page, ErrNotFound
	}
	if limit < 1 || limit > 100 {
		return page, history.ErrCursor
	}
	var pos pagePosition
	if cursor != "" {
		if err := history.OpenPage(s.cursorKey[:], cursor, &pos); err != nil {
			return page, err
		}
		if pos.Upper.IsZero() || pos.Before.IsZero() || pos.UpperRef == "" || pos.BeforeRef == "" || pos.Before.After(pos.Upper) {
			return page, history.ErrCursor
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return page, ErrUnavailable
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT w.ref,w.created_at,w.updated_at FROM agenova_work w WHERE w.team=$2 AND `+visibleSQL+` AND ($3::timestamptz IS NULL OR (w.created_at,w.ref)<=($3,$4)) AND ($5::timestamptz IS NULL OR (w.created_at,w.ref)<($5,$6)) ORDER BY w.created_at DESC,w.ref DESC LIMIT $7`, p.Subject, p.Team, nullableTime(pos.Upper), pos.UpperRef, nullableTime(pos.Before), pos.BeforeRef, limit+1)
	if err != nil {
		return page, ErrUnavailable
	}
	type entry struct {
		ref              string
		created, updated time.Time
	}
	entries := []entry{}
	for rows.Next() {
		var e entry
		if rows.Scan(&e.ref, &e.created, &e.updated) != nil {
			rows.Close()
			return page, ErrUnavailable
		}
		entries = append(entries, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, ErrUnavailable
	}
	if len(entries) == 0 {
		return page, nil
	}
	if pos.Upper.IsZero() {
		pos.Upper, pos.UpperRef = entries[0].created, entries[0].ref
	}
	more := len(entries) > limit
	if more {
		entries = entries[:limit]
	}
	for _, e := range entries {
		snapshot, err := readSnapshot(ctx, tx, `SELECT `+snapshotColumns+` FROM agenova_work w WHERE w.ref=$1`, e.ref)
		if err != nil {
			return page, err
		}
		v := snapshot.View
		evidence.Decorate(&v)
		item := history.Summary{RequestRef: e.ref, Created: e.created, Updated: e.updated, TraceID: v.TraceID, Diagnosis: v.Diagnosis, Status: "Pending"}
		if v.Request != nil {
			item.TaskType = v.Request.Spec.Task.Type
		}
		if v.State != nil && v.State.Claim != nil {
			item.ClaimID = v.State.Claim.ID
			item.Status = string(v.State.Claim.Phase)
		}
		if v.Outcome != nil {
			item.Status = v.Outcome.Status
			item.Completion = v.Outcome.Completion
		}
		page.Items = append(page.Items, item)
	}
	if more {
		last := entries[len(entries)-1]
		pos.Before, pos.BeforeRef = last.created, last.ref
		page.NextCursor, err = history.SealPage(s.cursorKey[:], pos)
		if err != nil {
			return page, err
		}
	}
	return page, nil
}
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
