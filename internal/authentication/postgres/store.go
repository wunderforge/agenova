// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package postgres persists server-owned identity projections and revocation.
// It never turns a token's group/email claim into membership or permissions.
package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"strings"
	"time"

	_ "github.com/lib/pq"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
)

// Schema is applied explicitly by an operator migration, never on API requests.
//
//go:embed schema.sql
var Schema string

type Store struct{ DB *sql.DB }

func (s Store) Resolve(ctx context.Context, id authentication.Identity) (v0.Principal, error) {
	if s.DB == nil || id.Issuer == "" || id.Subject == "" || id.SessionID == "" || id.AuthenticatedAt.IsZero() || !id.ExpiresAt.After(time.Now()) {
		return v0.Principal{}, authentication.ErrForbidden
	}
	// One statement observes membership and grant revocation in one snapshot.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var p v0.Principal
	err := s.DB.QueryRowContext(ctx, `SELECT principal, team FROM agenova_members m
        WHERE issuer=$1 AND subject=$2 AND active AND login_after<$4
        AND NOT EXISTS (SELECT 1 FROM agenova_revocations r
            WHERE r.issuer=m.issuer AND r.subject=m.subject AND r.session_id=$3)`, id.Issuer, id.Subject, id.SessionID, id.AuthenticatedAt).Scan(&p.Subject, &p.Team)
	if err != nil {
		return v0.Principal{}, authentication.ErrForbidden
	}
	p.AuthenticationContext = "oidc"
	return p, nil
}

// Member is an operator-provisioned stable mapping. Email is intentionally absent.
// This single-team version does not allow reassigning a principal to another team.
type Member struct {
	Issuer, Subject, Principal, Team string
	Active                           bool
}

// PutMember is an operator method, not an HTTP self-registration endpoint.
// Identity/team changes conflict; disable the existing membership instead.
func (s Store) PutMember(ctx context.Context, actor string, m Member) error {
	if s.DB == nil {
		return errors.New("identity store unavailable")
	}
	for _, v := range []string{actor, m.Issuer, m.Subject, m.Principal, m.Team} {
		if strings.TrimSpace(v) == "" || len(v) > 2048 {
			return errors.New("invalid membership")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("membership update unavailable")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO agenova_members(issuer,subject,principal,team,active) VALUES($1,$2,$3,$4,$5)
        ON CONFLICT(issuer,subject) DO UPDATE SET active=EXCLUDED.active,login_after=CASE WHEN EXCLUDED.active THEN agenova_members.login_after ELSE now() END
        WHERE agenova_members.principal=EXCLUDED.principal AND agenova_members.team=EXCLUDED.team AND (EXCLUDED.active OR agenova_members.account_role<>'owner')`, m.Issuer, m.Subject, m.Principal, m.Team, m.Active)
	if err != nil {
		return errors.New("membership update rejected")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("membership identity conflicts")
	}
	operation := "membership.disabled"
	if m.Active {
		operation = "membership.enabled"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, actor, operation, m.Principal); err != nil {
		return errors.New("membership audit unavailable")
	}
	if err = tx.Commit(); err != nil {
		return errors.New("membership commit unavailable")
	}
	return nil
}

// Revoke persists an entire refresh-token family. There is no expiry cleanup:
// deleting this tombstone could revive an otherwise valid refreshed token.
func (s Store) Revoke(ctx context.Context, id authentication.Identity) error {
	if s.DB == nil || id.Issuer == "" || id.Subject == "" || id.SessionID == "" {
		return errors.New("invalid session")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("revocation unavailable")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO agenova_revocations(issuer,subject,session_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id.Issuer, id.Subject, id.SessionID)
	if err != nil {
		return errors.New("revocation unavailable")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return errors.New("revocation unavailable")
	}
	if n > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) SELECT principal,'session.revoked',principal FROM agenova_members WHERE issuer=$1 AND subject=$2`, id.Issuer, id.Subject)
		if err != nil {
			return errors.New("revocation audit unavailable")
		}
	}
	if err = tx.Commit(); err != nil {
		return errors.New("revocation commit unavailable")
	}
	return nil
}
