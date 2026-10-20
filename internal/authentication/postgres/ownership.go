// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
	"time"
)

// TransferOwner preserves exactly one active Owner in a single transaction. The
// existing unique Owner index remains in force. No cloud permissions are changed.
func (s Store) TransferOwner(ctx context.Context, p v0.Principal, c invitations.MemberChange) error {
	if s.DB == nil || p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" || c.Principal == p.Subject || c.Confirmation != c.Principal || c.Role != "" || c.Project != "" || c.ScopeKind != "" || c.ScopeRef != "" {
		return authentication.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.ErrUnavailable
	}
	defer tx.Rollback()
	actor, target, err := lockMemberPair(ctx, tx, p, c.Principal)
	if err != nil {
		return err
	}
	if actor != Owner || target != Admin {
		return authentication.ErrForbidden
	}
	var active bool
	if tx.QueryRowContext(ctx, `SELECT active FROM agenova_members WHERE principal=$1 AND team=$2`, c.Principal, p.Team).Scan(&active) != nil || !active {
		return authentication.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_members SET account_role='admin' WHERE principal=$1 AND team=$2`, p.Subject, p.Team); err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_members SET account_role='owner' WHERE principal=$1 AND team=$2`, c.Principal, p.Team); err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'ownership.transfer',$2)`, p.Subject, c.Principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}
