// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/authentication/googleoidc"
	"github.com/wunderforge/agenova/internal/invitations"
)

// ManageGoogleInvitation replaces a lost/expired link or cancels pending intent.
// Replacement invalidates the previous proof atomically and never changes roles.
func (s Store) ManageGoogleInvitation(ctx context.Context, p v0.Principal, id, action string) (invitations.Invitation, string, error) {
	if s.DB == nil || len(id) != 32 || (action != "cancel" && action != "reissue") {
		return invitations.Invitation{}, "", invitations.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.Invitation{}, "", invitations.ErrUnavailable
	}
	defer tx.Rollback()
	if err = invitationActor(ctx, tx, p, MemberRole); err != nil {
		return invitations.Invitation{}, "", err
	}
	var i invitations.Invitation
	var account string
	err = tx.QueryRowContext(ctx, `SELECT id,email,project,project_role,team,issuer,status,expires_at,account_role FROM agenova_invitations WHERE id=$1 AND team=$2 AND issuer=$3 AND subject IS NULL AND status IN ('reserved','pending','cancelled') FOR UPDATE`, id, p.Team, googleoidc.Issuer).Scan(&i.ID, &i.Email, &i.Project, &i.Role, &i.Team, &i.Issuer, &i.Status, &i.ExpiresAt, &account)
	if err != nil || account == Owner {
		return i, "", authentication.ErrForbidden
	}
	if err = invitationActor(ctx, tx, p, account); err != nil {
		return i, "", err
	}
	var scoped bool
	if tx.QueryRowContext(ctx, `SELECT m.account_role='owner' OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=m.principal AND g.team=m.team AND g.project=$3 AND g.role='admin') FROM agenova_members m WHERE m.principal=$1 AND m.team=$2 AND active`, p.Subject, p.Team, i.Project).Scan(&scoped) != nil || !scoped {
		return i, "", authentication.ErrForbidden
	}
	proof := ""
	if action == "reissue" {
		var recent int
		if tx.QueryRowContext(ctx, `SELECT count(*) FROM agenova_identity_audit WHERE actor=$1 AND operation='invitation.reissue' AND created_at>now()-interval '1 hour'`, p.Subject).Scan(&recent) != nil || recent >= 10 {
			return i, "", invitations.ErrUnavailable
		}
		var b [32]byte
		if _, err = rand.Read(b[:]); err != nil {
			return i, "", invitations.ErrUnavailable
		}
		proof = base64.RawURLEncoding.EncodeToString(b[:])
		i.Status = "pending"
		err = tx.QueryRowContext(ctx, `UPDATE agenova_invitations SET acceptance_hash=$2,status='pending',actor=$3,expires_at=now()+interval '7 days' WHERE id=$1 RETURNING expires_at`, id, applicationTokenHash(proof), p.Subject).Scan(&i.ExpiresAt)
	} else {
		i.Status = "cancelled"
		_, err = tx.ExecContext(ctx, `UPDATE agenova_invitations SET acceptance_hash=NULL,status='cancelled' WHERE id=$1`, id)
	}
	if err != nil {
		return i, "", invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, p.Subject, "invitation."+action, "invitation:"+id); err != nil {
		return i, "", invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return i, "", invitations.ErrUnavailable
	}
	return i, proof, nil
}
