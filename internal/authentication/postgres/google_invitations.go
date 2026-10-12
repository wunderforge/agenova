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

// ReserveGoogleInvitation creates application intent, not a Google account.
// The proof is returned once for delivery; only its hash is stored. This does not
// claim email delivery. A failed/uncertain delivery needs explicit reconciliation.
func (s Store) ReserveGoogleInvitation(ctx context.Context, actor v0.Principal, r invitations.Request) (invitations.Invitation, string, error) {
	i, err := s.Reserve(ctx, actor, googleoidc.Issuer, r)
	if err != nil {
		return i, "", err
	}
	return s.googleProof(ctx, i)
}

// ReserveGoogleOwner is private operator bootstrap, never a public signup route.
func (s Store) ReserveGoogleOwner(ctx context.Context, team string, r invitations.Request) (invitations.Invitation, string, error) {
	i, err := s.ReserveOwner(ctx, googleoidc.Issuer, team, r)
	if err != nil {
		return i, "", err
	}
	return s.googleProof(ctx, i)
}
func (s Store) googleProof(ctx context.Context, i invitations.Invitation) (invitations.Invitation, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return i, "", invitations.ErrUnavailable
	}
	proof := base64.RawURLEncoding.EncodeToString(b[:])
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := s.DB.ExecContext(ctx, `UPDATE agenova_invitations SET acceptance_hash=$2,status='pending' WHERE id=$1 AND issuer=$3 AND status='reserved' AND subject IS NULL AND acceptance_hash IS NULL AND expires_at>now()`, i.ID, applicationTokenHash(proof), googleoidc.Issuer)
	if err != nil {
		return i, "", invitations.ErrUnavailable
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return i, "", invitations.ErrUnavailable
	}
	i.Status = "pending"
	return i, proof, nil
}

// AcceptGoogleInvitation receives ONLY a callback-verified identity and a
// server-bound invitation proof. Neither parameter may come from an API identity
// payload. It binds issuer+subject once; matching email never merges accounts.
func (s Store) AcceptGoogleInvitation(ctx context.Context, id googleoidc.LoginIdentity, proof string) (v0.Principal, error) {
	denied := authentication.ErrForbidden
	if s.DB == nil || id.Issuer != googleoidc.Issuer || id.Subject == "" || id.Email == "" || !id.EmailAuthoritative || len(proof) != 43 {
		return v0.Principal{}, denied
	}
	if b, err := base64.RawURLEncoding.Strict().DecodeString(proof); err != nil || len(b) != 32 {
		return v0.Principal{}, denied
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return v0.Principal{}, denied
	}
	defer tx.Rollback()
	hash := applicationTokenHash(proof)
	var team, actor, account string
	// Discover actor then take locks in the same actor -> invitation order as
	// existing administrative writes. The second read rechecks every condition.
	if tx.QueryRowContext(ctx, `SELECT team,actor,account_role FROM agenova_invitations WHERE acceptance_hash=$1 AND issuer=$2 AND email=$3 AND status='pending' AND expires_at>now()`, hash, id.Issuer, id.Email).Scan(&team, &actor, &account) != nil {
		return v0.Principal{}, denied
	}
	if account == Owner {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215003)`); err != nil || actor != "operator-bootstrap" {
			return v0.Principal{}, denied
		}
	} else if invitationActor(ctx, tx, v0.Principal{Subject: actor, Team: team, AuthenticationContext: "oidc"}, account) != nil {
		return v0.Principal{}, denied
	}
	var inviteID, project, role string
	if tx.QueryRowContext(ctx, `SELECT id,project,project_role FROM agenova_invitations WHERE acceptance_hash=$1 AND issuer=$2 AND email=$3 AND team=$4 AND actor=$5 AND account_role=$6 AND status='pending' AND subject IS NULL AND expires_at>now() FOR UPDATE`, hash, id.Issuer, id.Email, team, actor, account).Scan(&inviteID, &project, &role) != nil {
		return v0.Principal{}, denied
	}
	principal := "user:" + inviteID
	// Unique issuer/subject and principal constraints reject existing identities,
	// including disabled members; an invitation can never reactivate or link them.
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_members(issuer,subject,principal,team,active,account_role) VALUES($1,$2,$3,$4,true,$5)`, id.Issuer, id.Subject, principal, team, account); err != nil {
		return v0.Principal{}, denied
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_project_grants(principal,team,project,role) VALUES($1,$2,$3,$4)`, principal, team, project, role); err != nil {
		return v0.Principal{}, denied
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_invitations SET subject=$2,principal=$3,status='accepted',accepted_at=now(),acceptance_hash=NULL WHERE id=$1`, inviteID, id.Subject, principal); err != nil {
		return v0.Principal{}, denied
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'invitation.accepted',$1)`, principal); err != nil {
		return v0.Principal{}, denied
	}
	if tx.Commit() != nil {
		return v0.Principal{}, denied
	}
	return v0.Principal{Subject: principal, Team: team, AuthenticationContext: "oidc"}, nil
}
