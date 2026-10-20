// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/authentication"
	"github.com/wunderforge/agenova/internal/invitations"
)

func inviteID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func invitationActor(ctx context.Context, tx *sql.Tx, p v0.Principal, role string) error {
	if p.AuthenticationContext != "oidc" || p.Subject == "" || p.Team == "" {
		return authentication.ErrForbidden
	}
	var account string
	if tx.QueryRowContext(ctx, `SELECT account_role FROM agenova_members WHERE principal=$1 AND team=$2 AND active FOR UPDATE`, p.Subject, p.Team).Scan(&account) != nil {
		return authentication.ErrForbidden
	}
	if account != Owner && (role == Admin || role == Owner) {
		return authentication.ErrForbidden
	}
	if account != Owner && account != Admin {
		var projectAdmin bool
		if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_project_grants WHERE principal=$1 AND team=$2 AND role='admin')`, p.Subject, p.Team).Scan(&projectAdmin) != nil || !projectAdmin {
			return authentication.ErrForbidden
		}
	}
	return nil
}
func (s Store) Reserve(ctx context.Context, p v0.Principal, issuer string, r invitations.Request) (invitations.Invitation, error) {
	return s.reserve(ctx, p, issuer, r, false)
}

// ReserveOwner is operator-only bootstrap, never mounted as a member API.
func (s Store) ReserveOwner(ctx context.Context, issuer, team string, r invitations.Request) (invitations.Invitation, error) {
	if team == "" || len(team) > 128 || r.Role != Admin {
		return invitations.Invitation{}, invitations.ErrInvalid
	}
	return s.reserve(ctx, v0.Principal{Subject: "operator-bootstrap", Team: team}, issuer, r, true)
}
func (s Store) reserve(ctx context.Context, p v0.Principal, issuer string, r invitations.Request, bootstrap bool) (invitations.Invitation, error) {
	if !invitations.Validate(r) || issuer == "" || s.DB == nil {
		return invitations.Invitation{}, invitations.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	defer tx.Rollback()
	if bootstrap {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215003)`); err != nil {
			return invitations.Invitation{}, invitations.ErrUnavailable
		}
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agenova_members WHERE team=$1) OR EXISTS(SELECT 1 FROM agenova_invitations WHERE team=$1)`, p.Team).Scan(&exists); err != nil || exists {
			return invitations.Invitation{}, errors.New("team is already initialized or has pending bootstrap")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_projects(team,project) VALUES($1,$2)`, p.Team, r.Project); err != nil {
			return invitations.Invitation{}, invitations.ErrUnavailable
		}
	} else if err = invitationActor(ctx, tx, p, r.Role); err != nil {
		return invitations.Invitation{}, err
	}

	if !bootstrap {
		if err = invitationActor(ctx, tx, p, r.AccountRole); err != nil {
			return invitations.Invitation{}, err
		}
		var permitted bool
		if tx.QueryRowContext(ctx, `SELECT m.account_role='owner' OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=m.principal AND g.team=m.team AND g.project=$3 AND g.role='admin') FROM agenova_members m WHERE m.principal=$1 AND m.team=$2 AND active`, p.Subject, p.Team, r.Project).Scan(&permitted) != nil || !permitted {
			return invitations.Invitation{}, authentication.ErrForbidden
		}
	}

	// Bound invitations before any chargeable external directory operation.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,215004))`, p.Team); err != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	var recent, total int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE created_at>now()-interval '1 hour'),count(*) FROM agenova_invitations WHERE team=$1`, p.Team).Scan(&recent, &total); err != nil || recent >= 10 || total >= 200 {
		return invitations.Invitation{}, errors.New("invitation capacity reached")
	}
	id, err := inviteID()
	if err != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	invite := invitations.Invitation{ID: id, Email: r.Email, Project: r.Project, Role: r.Role, Team: p.Team, Issuer: issuer, Status: "reserved", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}
	account := MemberRole
	if r.AccountRole == Admin {
		account = Admin
	}
	if bootstrap {
		account = Owner
	}
	invite.AccountRole = account
	_, err = tx.ExecContext(ctx, `INSERT INTO agenova_invitations(id,team,email,actor,project,account_role,project_role,issuer,status,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'reserved',$9)`, id, p.Team, r.Email, p.Subject, r.Project, account, r.Role, issuer, invite.ExpiresAt)
	if err != nil {
		return invitations.Invitation{}, errors.New("invitation conflicts or project is unavailable")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'invitation.reserved',$2)`, p.Subject, "invitation:"+id); err != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	return invite, nil
}
func (s Store) Bind(ctx context.Context, p v0.Principal, id, subject string) (invitations.Invitation, error) {
	return s.bind(ctx, p, id, subject, false)
}
func (s Store) BindOwner(ctx context.Context, team, id, subject string) (invitations.Invitation, error) {
	return s.bind(ctx, v0.Principal{Subject: "operator-bootstrap", Team: team}, id, subject, true)
}
func (s Store) bind(ctx context.Context, p v0.Principal, id, subject string, bootstrap bool) (invitations.Invitation, error) {
	if subject == "" || len(subject) > 256 || s.DB == nil {
		return invitations.Invitation{}, invitations.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.Invitation{}, invitations.ErrUnavailable
	}
	defer tx.Rollback()
	// Lock actor before invitation, matching Reserve and subsequent management.
	if bootstrap {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(215003)`); err != nil {
			return invitations.Invitation{}, invitations.ErrUnavailable
		}
	} else if err = invitationActor(ctx, tx, p, MemberRole); err != nil {
		return invitations.Invitation{}, err
	}
	var i invitations.Invitation
	var account string
	err = tx.QueryRowContext(ctx, `SELECT id,email,project,project_role,team,issuer,expires_at,account_role FROM agenova_invitations WHERE id=$1 AND actor=$2 AND team=$3 AND status='reserved' AND expires_at>now() FOR UPDATE`, id, p.Subject, p.Team).Scan(&i.ID, &i.Email, &i.Project, &i.Role, &i.Team, &i.Issuer, &i.ExpiresAt, &account)
	if err != nil {
		return i, authentication.ErrForbidden
	}
	if bootstrap {
		if account != Owner {
			return i, authentication.ErrForbidden
		}
	} else if err = invitationActor(ctx, tx, p, account); err != nil {
		return i, err
	}
	if !bootstrap {
		if err = invitationActor(ctx, tx, p, i.Role); err != nil {
			return i, err
		}
	}
	i.AccountRole = account
	i.Subject = subject
	i.Principal = "user:" + id
	i.Status = "pending"
	_, err = tx.ExecContext(ctx, `INSERT INTO agenova_members(issuer,subject,principal,team,active,account_role) VALUES($1,$2,$3,$4,false,$5)`, i.Issuer, subject, i.Principal, i.Team, account)
	if err != nil {
		return i, invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_project_grants(principal,team,project,role) VALUES($1,$2,$3,$4)`, i.Principal, i.Team, i.Project, i.Role); err != nil {
		return i, invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `UPDATE agenova_invitations SET subject=$2,principal=$3,status='pending' WHERE id=$1`, id, subject, i.Principal); err != nil {
		return i, invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'invitation.bound',$2)`, p.Subject, i.Principal); err != nil {
		return i, invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return i, invitations.ErrUnavailable
	}
	return i, nil
}
func (s Store) Delivered(ctx context.Context, id string) error {
	return s.invitationStatus(ctx, id, "sent")
}
func (s Store) Uncertain(ctx context.Context, id string) error {
	return s.invitationStatus(ctx, id, "uncertain")
}
func (s Store) invitationStatus(ctx context.Context, id, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return invitations.ErrUnavailable
	}
	defer tx.Rollback()
	var actor, principal string
	err = tx.QueryRowContext(ctx, `UPDATE agenova_invitations SET status=$2 WHERE id=$1 AND status IN ('reserved','pending') AND ($2<>'sent' OR subject IS NOT NULL) RETURNING actor,COALESCE(principal,'invitation:'||id)`, id, status).Scan(&actor, &principal)
	if err != nil {
		return invitations.ErrUnavailable
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,$2,$3)`, actor, "invitation."+status, principal); err != nil {
		return invitations.ErrUnavailable
	}
	if tx.Commit() != nil {
		return invitations.ErrUnavailable
	}
	return nil
}

func (s Store) ListInvitations(ctx context.Context, p v0.Principal) ([]invitations.Invitation, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, invitations.ErrUnavailable
	}
	defer tx.Rollback()
	if err = invitationActor(ctx, tx, p, MemberRole); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,email,project,project_role,account_role,CASE WHEN status IN ('pending','reserved') AND expires_at<=now() THEN 'expired' ELSE status END,expires_at,(issuer='https://accounts.google.com' AND subject IS NULL AND account_role<>'owner') FROM agenova_invitations i WHERE team=$1 AND (EXISTS(SELECT 1 FROM agenova_members m WHERE m.principal=$2 AND m.team=i.team AND m.account_role IN ('owner','admin')) OR EXISTS(SELECT 1 FROM agenova_project_grants g WHERE g.principal=$2 AND g.team=i.team AND g.project=i.project AND g.role='admin')) ORDER BY created_at DESC LIMIT 200`, p.Team, p.Subject)
	if err != nil {
		return nil, invitations.ErrUnavailable
	}
	defer rows.Close()
	result := []invitations.Invitation{}
	for rows.Next() {
		var i invitations.Invitation
		if rows.Scan(&i.ID, &i.Email, &i.Project, &i.Role, &i.AccountRole, &i.Status, &i.ExpiresAt, &i.LinkManaged) != nil {
			return nil, invitations.ErrUnavailable
		}
		result = append(result, i)
	}
	if rows.Err() != nil {
		return nil, invitations.ErrUnavailable
	}
	return result, nil
}

// AcceptVerifiedInvitation is called only after cryptographic access-token
// verification. A subject-bound, sent and unexpired invitation activates once.
func (s Store) AcceptVerifiedInvitation(ctx context.Context, id authentication.Identity) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return authentication.ErrForbidden
	}
	defer tx.Rollback()
	var principal string
	err = tx.QueryRowContext(ctx, `UPDATE agenova_invitations SET status='accepted',accepted_at=now() WHERE issuer=$1 AND subject=$2 AND status='sent' AND expires_at>now() AND date_trunc('second',created_at)<=$3 AND NOT EXISTS(SELECT 1 FROM agenova_revocations WHERE issuer=$1 AND subject=$2 AND session_id=$4) RETURNING principal`, id.Issuer, id.Subject, id.AuthenticatedAt, id.SessionID).Scan(&principal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return authentication.ErrForbidden
	}
	result, err := tx.ExecContext(ctx, `UPDATE agenova_members SET active=true WHERE issuer=$1 AND subject=$2 AND principal=$3 AND NOT active AND login_after='epoch'`, id.Issuer, id.Subject, principal)
	if err != nil {
		return authentication.ErrForbidden
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return authentication.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO agenova_identity_audit(actor,operation,principal) VALUES($1,'invitation.accepted',$1)`, principal); err != nil {
		return authentication.ErrForbidden
	}
	if tx.Commit() != nil {
		return authentication.ErrForbidden
	}
	return nil
}

// InvitedMemberships adds one-time invitation acceptance to the current lookup.
// Only the authentication boundary may supply this verified private identity.
type InvitedMemberships struct{ Store }

func (s InvitedMemberships) Resolve(ctx context.Context, id authentication.Identity) (v0.Principal, error) {
	if id.Issuer == "" || id.Subject == "" || id.AuthenticatedAt.IsZero() || !id.ExpiresAt.After(time.Now()) {
		return v0.Principal{}, authentication.ErrForbidden
	}
	if err := s.AcceptVerifiedInvitation(ctx, id); err != nil {
		return v0.Principal{}, err
	}
	return s.Store.Resolve(ctx, id)
}
